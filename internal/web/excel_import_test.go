package web

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/importxlsx"
)

// fakeWorkbookImporter simulates a persisted batch without a real XLSX file.
// RU: fakeWorkbookImporter имитирует импорт без записи настоящей книги.
type fakeWorkbookImporter struct {
	count int
	err error
	calls int
	groupID int64
	heading string
}

func (f *fakeWorkbookImporter) ImportFile(_ context.Context, groupID int64, _ []byte, nameHeader, idHeader string) (int,error) {
	f.calls++
	f.groupID = groupID
	f.heading = nameHeader+"|"+idHeader
	return f.count,f.err
}

// uploadWorkbook submits multipart XLSX data with the existing group token.
// RU: uploadWorkbook отправляет XLSX с общим защитным токеном группы.
func uploadWorkbook(t *testing.T, handler http.Handler, groupID, token, nameHeader, idHeader string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer:=multipart.NewWriter(&buf)
	file,err:=writer.CreateFormFile("file","students.xlsx")
	if err!=nil { t.Fatal(err) }
	if _,err=file.Write(data);err!=nil { t.Fatal(err) }
	for k,v:=range map[string]string{"csrf_token":token,"name_header":nameHeader,"id_header":idHeader} {
		if err=writer.WriteField(k,v);err!=nil { t.Fatal(err) }
	}
	if err:=writer.Close();err!=nil { t.Fatal(err) }
	req:=httptest.NewRequest(http.MethodPost,"/groups/"+groupID+"/import",&buf)
	req.Header.Set("Content-Type",writer.FormDataContentType())
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,req)
	return rec
}

// TestExcelImportFormExists verifies user-accessible header mapping.
// Scenario: load the group's page with the importer configured.
// Expected: multipart upload form with file and header-name fields.
//
// RU: Проверяет форму Excel на странице группы.
// Сценарий: открыть группу с доступным импортом.
// Ожидается: выбор XLSX и поля сопоставления заголовков колонок.
func TestExcelImportFormExists(t *testing.T) {
	g:=&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}}
	h,err:=NewHandler(g,&fakeStudentLister{},&fakeWorkbookImporter{})
	if err!=nil { t.Fatal(err) }
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/groups/17",nil))
	for _,part:=range []string{`action="/groups/17/import"`,`enctype="multipart/form-data"`,`name="file"`,`name="name_header"`,`name="id_header"`} {
		if !strings.Contains(rec.Body.String(),part) { t.Errorf("missing %q in form",part) }
	}
}

// TestExcelImportRequiresCSRFAndValidGroup denies forged writes.
// Scenario: submit without token or submit to a group not in the workspace.
// Expected: HTTP 403 or 404 and no importer calls.
//
// RU: Проверяет CSRF и существование группы до импорта.
// Сценарий: отсутствует токен либо ID группы не существует.
// Ожидается: HTTP 403/404 без сохранения студентов.
func TestExcelImportRequiresCSRFAndValidGroup(t *testing.T) {
	g:=&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}}
	imp:=&fakeWorkbookImporter{count:2}
	h,err:=NewHandler(g,&fakeStudentLister{},imp)
	if err!=nil { t.Fatal(err) }
	for _,tc:=range []struct{id,token string; status int}{
		{"17","bad",http.StatusForbidden},
		{"999",studentFormToken(t,h,"17"),http.StatusNotFound},
	}{
		rec:=uploadWorkbook(t,h,tc.id,tc.token,"ФИО","Учебный ID",[]byte("xlsx data"))
		if rec.Code!=tc.status {t.Errorf("group=%s status=%d want=%d",tc.id,rec.Code,tc.status)}
	}
	if imp.calls!=0 { t.Fatalf("importer called %d times",imp.calls) }
}

// TestExcelImportRedirectsAfterSuccess checks one-shot submission.
// Scenario: upload XLSX with matching header names.
// Expected: importer called once; PRG redirects to the group page.
//
// RU: Проверяет импорт и переход на страницу группы.
// Сценарий: загрузить XLSX с выбранными именами колонок.
// Ожидается: один импорт и HTTP 303.
func TestExcelImportRedirectsAfterSuccess(t *testing.T) {
	g:=&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}}
	imp:=&fakeWorkbookImporter{count:2}
	h,err:=NewHandler(g,&fakeStudentLister{},imp)
	if err!=nil { t.Fatal(err) }
	rec:=uploadWorkbook(t,h,"17",studentFormToken(t,h,"17"),"ФИО","Учебный ID",[]byte("xlsx data"))
	if rec.Code!=http.StatusSeeOther || rec.Header().Get("Location")!="/groups/17" {
		t.Errorf("status=%d redirect=%q",rec.Code,rec.Header().Get("Location"))
	}
	if imp.calls!=1 || imp.groupID!=17 || imp.heading!="ФИО|Учебный ID" {t.Errorf("importer state = %+v",imp)}
}

// TestExcelImportReportsSafeErrors verifies mapping/duplicate failures.
// Scenario: the importer reports missing header or duplicate educational ID.
// Expected: user-facing HTTP 400/409, no private error details.
//
// RU: Проверяет безопасные сообщения при неверных колонках и дубликатах.
// Сценарий: ошибка сопоставления или повтор номера зачётки.
// Ожидается: HTTP 400/409 без раскрытия деталей БД.
func TestExcelImportReportsSafeErrors(t *testing.T) {
	for _,tc:=range []struct{err error;status int}{
		{importxlsx.ErrMissingColumn,http.StatusBadRequest},
		{errors.New("private SQLite filename"),http.StatusInternalServerError},
	}{
		g:=&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}}
		imp:=&fakeWorkbookImporter{err:tc.err}
		h,err:=NewHandler(g,&fakeStudentLister{},imp)
		if err!=nil {t.Fatal(err)}
		rec:=uploadWorkbook(t,h,"17",studentFormToken(t,h,"17"),"ФИО","Учебный ID",[]byte("xlsx data"))
		if rec.Code!=tc.status || strings.Contains(rec.Body.String(),"private SQLite") {t.Errorf("status=%d body=%q",rec.Code,rec.Body.String())}
	}
}

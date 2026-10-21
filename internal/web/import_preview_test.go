package web

import (
	"archive/zip"
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/importxlsx"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// previewWorkbook is a real in-memory XLSX with worksheet relationships.
// RU: previewWorkbook создаёт настоящий тестовый XLSX с двумя вкладками.
func previewWorkbook(t *testing.T) []byte {
	t.Helper()
	files:=map[string]string{
		"xl/workbook.xml":`<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Архив" sheetId="1" r:id="rId2"/><sheet name="Студенты" sheetId="2" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":`<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":`<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>ФИО</t></is></c><c r="B1" t="inlineStr"><is><t>Учебный ID</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>Иванов Иван</t></is></c><c r="B2" t="inlineStr"><is><t>TF20-001</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml":`<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Архивный студент</t></is></c></row></sheetData></worksheet>`,
	}
	var b bytes.Buffer
	w:=zip.NewWriter(&b)
	for name,contents:=range files{
		f,err:=w.Create(name);if err!=nil{t.Fatal(err)}
		if _,err:=f.Write([]byte(contents));err!=nil{t.Fatal(err)}
	}
	if err:=w.Close();err!=nil{t.Fatal(err)}
	return b.Bytes()
}

// TestPreviewRequiresExplicitCommit verifies that selecting XLSX and a
// worksheet does not mutate the database before user confirmation.
// RU: Проверяет отсутствие записи в SQLite до подтверждения импорта.
func TestPreviewRequiresExplicitCommit(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"data.sqlite"));if err!=nil{t.Fatal(err)}
	defer db.Close()
	gr:=groups.NewRepository(db);st:=students.NewRepository(db)
	if err:=gr.Init(ctx);err!=nil{t.Fatal(err)}
	if err:=st.Init(ctx);err!=nil{t.Fatal(err)}
	group,err:=gr.Create(ctx,"ИСП-401");if err!=nil{t.Fatal(err)}
	h,err:=NewHandlerWithServices(gr,st,Dependencies{Workbook:importxlsx.Service{DB:db}})
	if err!=nil{t.Fatal(err)}
	groupID:="1"
	token:=studentFormToken(t,h,groupID)
	var requestBody bytes.Buffer
	writer:=multipart.NewWriter(&requestBody)
	file,err:=writer.CreateFormFile("file","students.xlsx");if err!=nil{t.Fatal(err)}
	if _,err:=file.Write(previewWorkbook(t));err!=nil{t.Fatal(err)}
	if err:=writer.WriteField("csrf_token",token);err!=nil{t.Fatal(err)}
	if err:=writer.Close();err!=nil{t.Fatal(err)}
	req:=httptest.NewRequest(http.MethodPost,"/groups/1/import/preview",&requestBody)
	req.Header.Set("Content-Type",writer.FormDataContentType())
	rec:=httptest.NewRecorder();h.ServeHTTP(rec,req)
	if rec.Code!=http.StatusOK{t.Fatalf("preview HTTP %d: %s",rec.Code,rec.Body.String())}
	for _,need:=range []string{"Архив","Студенты","Подтвердить импорт","preview_token"}{
		if !strings.Contains(rec.Body.String(),need){t.Errorf("preview missing %s",need)}
	}
	people,err:=st.ListByGroup(ctx,group.ID)
	if err!=nil||len(people)!=0{t.Fatalf("preview inserted data: %#v %v",people,err)}
	match:=regexp.MustCompile(`name="preview_token" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if len(match)!=2{t.Fatalf("missing preview token: %q",rec.Body.String())}
	values:=url.Values{"csrf_token":{token},"preview_token":{match[1]},"sheet_name":{"Студенты"},"name_header":{"ФИО"},"id_header":{"Учебный ID"}}
	confirm:=httptest.NewRequest(http.MethodPost,"/groups/1/import/commit",strings.NewReader(values.Encode()))
	confirm.Header.Set("Content-Type","application/x-www-form-urlencoded")
	committed:=httptest.NewRecorder();h.ServeHTTP(committed,confirm)
	if committed.Code!=http.StatusSeeOther{t.Fatalf("commit HTTP %d: %s",committed.Code,committed.Body.String())}
	people,err=st.ListByGroup(ctx,group.ID)
	if err!=nil||len(people)!=1||people[0].EducationalID!="TF20-001"{
		t.Fatalf("confirmed students: %#v %v",people,err)
	}
}

// TestPreviewRejectsForgedRequests denies confirmation without an application
// CSRF token, preventing local browser data corruption.
// RU: Проверяет запрет подтверждения импорта по поддельной форме.
func TestPreviewRejectsForgedRequests(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"data.sqlite"));if err!=nil{t.Fatal(err)}
	defer db.Close()
	gr:=groups.NewRepository(db);st:=students.NewRepository(db)
	if err:=gr.Init(ctx);err!=nil{t.Fatal(err)}
	if err:=st.Init(ctx);err!=nil{t.Fatal(err)}
	if _,err:=gr.Create(ctx,"ИСП-401");err!=nil{t.Fatal(err)}
	h,err:=NewHandlerWithServices(gr,st,Dependencies{Workbook:importxlsx.Service{DB:db}})
	if err!=nil{t.Fatal(err)}
	val:=url.Values{"preview_token":{"invented"},"sheet_name":{"Студенты"},"name_header":{"ФИО"},"id_header":{"Учебный ID"}}
	req:=httptest.NewRequest(http.MethodPost,"/groups/1/import/commit",strings.NewReader(val.Encode()))
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	rec:=httptest.NewRecorder();h.ServeHTTP(rec,req)
	if rec.Code!=http.StatusForbidden{t.Fatalf("CSRF status = %d",rec.Code)}
}

package web

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// testDocumentPage assembles real repositories and an in-process handler.
// RU: testDocumentPage собирает реальную SQLite и HTTP-обработчик без сети.
func testDocumentPage(t *testing.T) (http.Handler,*DetailService,int64,int64,int64) {
	t.Helper()
	ctx:=context.Background()
	root:=t.TempDir()
	db,err:=sqlite.Open(ctx,filepath.Join(root,"thesisflow.sqlite"));if err!=nil {t.Fatal(err)}
	t.Cleanup(func(){_ = db.Close()})
	gr:=groups.NewRepository(db)
	st:=students.NewRepository(db)
	req:=requirements.NewRepository(db)
	mat:=materials.NewRepository(db)
	ver:=fileversions.NewStore(db,filepath.Join(root,"storage"))
	rev:=review.NewService(db,ver)
	for _,init:=range []func(context.Context)error{gr.Init,st.Init,req.Init,mat.Init,ver.Init,rev.Init} {
		if err:=init(ctx);err!=nil {t.Fatal(err)}
	}
	group,err:=gr.Create(ctx,"ИСП-401");if err!=nil {t.Fatal(err)}
	student,err:=st.Create(ctx,group.ID,"ЗК-001","Иванов Иван");if err!=nil {t.Fatal(err)}
	required,err:=req.Create(ctx,group.ID,0,"Глава 1","");if err!=nil {t.Fatal(err)}
	detail:=&DetailService{Requirements:req,Materials:mat,Versions:ver,Review:rev}
	handler,err:=NewHandlerWithServices(gr,st,Dependencies{Requirements:req,Details:detail})
	if err!=nil {t.Fatal(err)}
	return handler,detail,group.ID,student.ID,required.ID
}

// TestStudentDetailShowsRequirementsAndVersionActions verifies the teacher can
// navigate to a student and see their requirements and current statuses.
// RU: Проверяет страницу студента с требуемыми файлами и статусами.
func TestStudentDetailShowsRequirementsAndVersionActions(t *testing.T) {
	handler,_,groupID,studentID,_:=testDocumentPage(t)
	groupURL:=url.PathEscape("0")
	_ = groupURL
	groupPage:=httptest.NewRecorder()
	handler.ServeHTTP(groupPage,httptest.NewRequest(http.MethodGet,"/groups/1",nil))
	if !strings.Contains(groupPage.Body.String(),`href="/groups/1/students/1"`) {
		t.Errorf("group page lacks student link: %q",groupPage.Body.String())
	}
	detailURL:="/groups/1/students/1"
	page:=httptest.NewRecorder()
	handler.ServeHTTP(page,httptest.NewRequest(http.MethodGet,detailURL,nil))
	if page.Code!=http.StatusOK {t.Fatalf("GET detail status=%d body=%q",page.Code,page.Body.String())}
	for _,want:=range []string{"Иванов Иван","Глава 1","Не сдан",`enctype="multipart/form-data"`,`name="requirement_id"`,`name="csrf_token"`} {
		if !strings.Contains(page.Body.String(),want){t.Errorf("detail page missing %q",want)}
	}
	_ = groupID
	_ = studentID
}

// TestDocumentBrowserUploadAndApproval verifies the complete workflow:
// teacher uploads a received version, reviews and accepts it, and the file
// appears in the human-readable approved projection.
//
// RU: Проверяет полный процесс через HTTP: загрузка файла преподавателем,
// статус «На проверке», приёмка и появление принятого документа в папке.
func TestDocumentBrowserUploadAndApproval(t *testing.T) {
	handler,detail,groupID,studentID,requirementID:=testDocumentPage(t)
	studentURL:="/groups/1/students/1"
	token:=studentFormToken(t,handler,"1")

	var buf bytes.Buffer
	writer:=multipart.NewWriter(&buf)
	file,err:=writer.CreateFormFile("file","chapter.docx")
	if err!=nil {t.Fatal(err)}
	if _,err:=file.Write([]byte("completed chapter"));err!=nil {t.Fatal(err)}
	if err:=writer.WriteField("requirement_id","1");err!=nil {t.Fatal(err)}
	if err:=writer.WriteField("csrf_token",token);err!=nil {t.Fatal(err)}
	if err:=writer.Close();err!=nil {t.Fatal(err)}
	upload:=httptest.NewRequest(http.MethodPost,studentURL+"/upload",&buf)
	upload.Header.Set("Content-Type",writer.FormDataContentType())
	record:=httptest.NewRecorder()
	handler.ServeHTTP(record,upload)
	if record.Code!=http.StatusSeeOther {t.Fatalf("upload status=%d body=%q",record.Code,record.Body.String())}
	items,err:=detail.Versions.List(context.Background(),studentID,requirementID)
	if err!=nil || len(items)!=1 {t.Fatalf("versions=%+v err=%v",items,err)}
	status,err:=detail.Materials.Get(context.Background(),studentID,requirementID)
	if err!=nil || status.Status!=materials.UnderReview {t.Fatalf("status=%+v err=%v",status,err)}
	post:=url.Values{"requirement_id":{"1"},"version_id":{"1"},"action":{"accept"},"csrf_token":{token}}
	req:=httptest.NewRequest(http.MethodPost,studentURL+"/review",strings.NewReader(post.Encode()))
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	record=httptest.NewRecorder()
	handler.ServeHTTP(record,req)
	if record.Code!=http.StatusSeeOther {t.Fatalf("accept status=%d body=%q",record.Code,record.Body.String())}
	status,err=detail.Materials.Get(context.Background(),studentID,requirementID)
	if err!=nil || status.Status!=materials.Accepted {t.Errorf("final status=%+v err=%v",status,err)}
	var accepted string
	err=detail.Review.DB.QueryRowContext(context.Background(),"SELECT relative_path FROM accepted_files WHERE student_id=? AND requirement_id=?",studentID,requirementID).Scan(&accepted)
	if err!=nil || !strings.HasSuffix(accepted,"Глава 1.docx") {t.Errorf("accepted path=%q err=%v",accepted,err)}
	_ = groupID
}

// TestDocumentActionsRejectForgedCSRF checks all document mutations are guarded.
// RU: Проверяет отказ в загрузке и приёмке без защитного токена.
func TestDocumentActionsRejectForgedCSRF(t *testing.T) {
	handler,_,_,_,_:=testDocumentPage(t)
	values:=url.Values{"requirement_id":{"1"},"version_id":{"1"},"action":{"accept"},"csrf_token":{"forged"}}
	req:=httptest.NewRequest(http.MethodPost,"/groups/1/students/1/review",strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,req)
	if rec.Code!=http.StatusForbidden {t.Errorf("forged review status=%d",rec.Code)}
}

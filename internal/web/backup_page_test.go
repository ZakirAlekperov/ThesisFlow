package web

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

type fakeBackupService struct {
	sent int
	restored int
}
func (f *fakeBackupService) Export(_ context.Context, out io.Writer)error{
	f.sent++
	_,err:=io.WriteString(out,"fake-zip-contents")
	return err
}
func (f *fakeBackupService) Restore(_ context.Context, in io.Reader)(string,error){
	f.restored++
	_,err:=io.ReadAll(in)
	if err!=nil{return "",err}
	return "/local/ThesisFlow-restored-TEST",nil
}

// TestBackupPageOffersDownloadAndProtectedRestore validates browser use, not
// internal test-only code paths. RU: Проверяет форму скачивания и восстановления.
func TestBackupPageOffersDownloadAndProtectedRestore(t *testing.T) {
	backup:=&fakeBackupService{}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Backup:backup})
	if err!=nil{t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/backup",nil))
	if rec.Code!=200 || !strings.Contains(rec.Body.String(),"Восстановление"){
		t.Fatalf("backup page: %d %s",rec.Code,rec.Body.String())
	}
	match:=regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if len(match)!=2 {t.Fatal("no CSRF token")}
	rec=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/backup/download",nil))
	if rec.Code!=200 || !strings.Contains(rec.Body.String(),"fake-zip-contents")||backup.sent!=1 {
		t.Fatalf("backup download HTTP %d, calls %d",rec.Code,backup.sent)
	}
	var body bytes.Buffer
	writer:=multipart.NewWriter(&body)
	file,err:=writer.CreateFormFile("archive","work.zip")
	if err!=nil{t.Fatal(err)}
	if _,err:=file.Write([]byte("fake ZIP"));err!=nil{t.Fatal(err)}
	if err:=writer.WriteField("csrf_token",match[1]);err!=nil{t.Fatal(err)}
	if err:=writer.Close();err!=nil{t.Fatal(err)}
	req:=httptest.NewRequest(http.MethodPost,"/backup/restore",&body)
	req.Header.Set("Content-Type",writer.FormDataContentType())
	rec=httptest.NewRecorder();h.ServeHTTP(rec,req)
	if rec.Code!=200||backup.restored!=1||!strings.Contains(rec.Body.String(),"ThesisFlow-restored-TEST"){
		t.Fatalf("restored HTTP %d, calls %d, body %s",rec.Code,backup.restored,rec.Body.String())
	}
}

// TestBackupRestoreRequiresCSRF denies a forged ZIP restore.
// RU: Проверяет отклонение загрузки ZIP без CSRF-токена.
func TestBackupRestoreRequiresCSRF(t *testing.T) {
	svc:=&fakeBackupService{}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Backup:svc})
	if err!=nil{t.Fatal(err)}
	var body bytes.Buffer
	writer:=multipart.NewWriter(&body)
	file,_:=writer.CreateFormFile("archive","fake.zip")
	_,_=file.Write([]byte("fake ZIP"))
	_ = writer.Close()
	req:=httptest.NewRequest(http.MethodPost,"/backup/restore",&body)
	req.Header.Set("Content-Type",writer.FormDataContentType())
	rec:=httptest.NewRecorder();h.ServeHTTP(rec,req)
	if rec.Code!=http.StatusForbidden||svc.restored!=0{
		t.Errorf("forged restore status %d calls %d",rec.Code,svc.restored)
	}
}

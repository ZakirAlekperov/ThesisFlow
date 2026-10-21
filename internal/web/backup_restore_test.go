package web

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"bytes"
	"testing"
)

// fakeBackupRestorer returns a fixed new directory and counts invocations.
// RU: fakeBackupRestorer возвращает путь нового каталога и считает вызовы.
type fakeBackupRestorer struct {
	calls int
	err error
}
func (f *fakeBackupRestorer) Restore(_ context.Context, path string) (string,error) {
	f.calls++
	if f.err != nil {return "",f.err}
	return "/fictional/workspace/restored-123",nil
}

func submitRestore(handler http.Handler, token string) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	_ = writer.WriteField("csrf_token",token)
	file,_:=writer.CreateFormFile("archive","test.zip")
	_,_=io.WriteString(file,"test ZIP bytes")
	_=writer.Close()
	req:=httptest.NewRequest(http.MethodPost,"/settings/restore",&payload)
	req.Header.Set("Content-Type",writer.FormDataContentType())
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,req)
	return rec
}

// TestSettingsOffersRestoreForm verifies user-accessible recovery without CLI.
// RU: Проверяет форму восстановления в новый каталог через интерфейс.
func TestSettingsOffersRestoreForm(t *testing.T) {
	ex:=&fakeBackupExporter{}
	restore:=&fakeBackupRestorer{}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Backup:ex,Restore:restore})
	if err!=nil {t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/settings",nil))
	for _,want:=range []string{`action="/settings/restore"`,`enctype="multipart/form-data"`,"Восстановить в новую папку"} {
		if !strings.Contains(rec.Body.String(),want) {t.Errorf("missing %q",want)}
	}
}

// TestRestoreFormKeepsLiveWorkspaceUntouched verifies explicit isolated
// recovery and a human-readable destination in the response.
// RU: Проверяет восстановление в отдельный каталог и сообщение о пути.
func TestRestoreFormKeepsLiveWorkspaceUntouched(t *testing.T) {
	restorer:=&fakeBackupRestorer{}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Restore:restorer})
	if err!=nil {t.Fatal(err)}
	rec:=submitRestore(h,settingsToken(t,h))
	if rec.Code!=http.StatusOK {t.Fatalf("status=%d body=%q",rec.Code,rec.Body.String())}
	if !strings.Contains(rec.Body.String(),"restored-123") || restorer.calls!=1 {
		t.Errorf("recovery result=%q calls=%d",rec.Body.String(),restorer.calls)
	}
}

// TestRestoreFormRejectsCSRF ensures external pages cannot import archives.
// RU: Проверяет блокировку POST-запросов с поддельным CSRF-токеном.
func TestRestoreFormRejectsCSRF(t *testing.T) {
	r:=&fakeBackupRestorer{}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Restore:r})
	if err!=nil {t.Fatal(err)}
	for _,token:=range []string{"","forged"} {
		rec:=submitRestore(h,token)
		if rec.Code!=http.StatusForbidden {t.Errorf("HTTP %d",rec.Code)}
	}
	if r.calls!=0 {t.Errorf("unexpected restore calls=%d",r.calls)}
}

// TestRestoreFormHidesStorageErrors checks private paths stay confidential.
// RU: Проверяет отсутствие приватных путей при ошибках восстановления.
func TestRestoreFormHidesStorageErrors(t *testing.T) {
	r:=&fakeBackupRestorer{err:errors.New("private backup restore SQL path")}
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Restore:r})
	if err!=nil {t.Fatal(err)}
	rec:=submitRestore(h,settingsToken(t,h))
	if rec.Code!=http.StatusBadRequest || strings.Contains(rec.Body.String(),"private") {
		t.Errorf("status=%d body=%q",rec.Code,rec.Body.String())
	}
}

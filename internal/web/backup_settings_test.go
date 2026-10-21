package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// fakeBackupExporter writes fixed ZIP-like bytes to a temporary file.
// RU: fakeBackupExporter сохраняет заданные байты во временный файл.
type fakeBackupExporter struct {
	calls int
	err   error
}

func (f *fakeBackupExporter) Export(_ context.Context, target string) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(target, []byte("fake-zip-test-content"), 0600)
}

// settingsToken extracts the CSRF token from the settings page.
// RU: settingsToken получает защитный токен страницы настроек.
func settingsToken(t *testing.T, handler http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settings: %d", rec.Code)
	}
	const prefix = `name="csrf_token" value="`
	body := rec.Body.String()
	start := strings.Index(body, prefix)
	if start < 0 {
		t.Fatalf("settings CSRF field missing: %q", body)
	}
	rest := body[start+len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 1 {
		t.Fatalf("settings CSRF token empty")
	}
	return rest[:end]
}

// postBackup simulates pressing Download backup in the local browser.
// RU: postBackup имитирует кнопку скачивания резервной копии.
func postBackup(handler http.Handler, token string) *httptest.ResponseRecorder {
	form := url.Values{"csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/settings/backup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestSettingsOffersBackupDownload checks that a user sees the backup form.
// Scenario: open /settings with a backup exporter connected.
// Expected: POST form, token and explicit notice that backups are unencrypted.
//
// RU: Проверяет страницу настроек с кнопкой резервной копии.
// Сценарий: включён сервис экспорта; открыта /settings.
// Ожидается: POST-форма, CSRF и предупреждение об отсутствии шифрования.
func TestSettingsOffersBackupDownload(t *testing.T) {
	ex := &fakeBackupExporter{}
	h, err := NewHandlerWithServices(&fakeGroupLister{}, &fakeStudentLister{}, Dependencies{Backup: ex})
	if err != nil {
		t.Fatal(err)
	}
	token := settingsToken(t, h)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))
	for _, want := range []string{`action="/settings/backup"`, "Скачать резервную копию", "не зашифрован", token} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %q in settings", want)
		}
	}
}

// TestBackupDownloadExportsZIP checks the protected browser download.
// Scenario: submit a valid CSRF token once.
// Expected: ZIP attachment with exactly the exported bytes.
//
// RU: Проверяет скачивание архива после отправки правильного токена.
// Ожидается: ZIP-вложение с сохранёнными тестовыми байтами.
func TestBackupDownloadExportsZIP(t *testing.T) {
	ex := &fakeBackupExporter{}
	h, err := NewHandlerWithServices(&fakeGroupLister{}, &fakeStudentLister{}, Dependencies{Backup: ex})
	if err != nil {
		t.Fatal(err)
	}
	rec := postBackup(h, settingsToken(t, h))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "fake-zip-test-content" {
		t.Errorf("downloaded %q", rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), ".zip") {
		t.Errorf("missing filename: %s", rec.Header().Get("Content-Disposition"))
	}
	if ex.calls != 1 {
		t.Errorf("exports=%d, want=1", ex.calls)
	}
}

// TestBackupDownloadRejectsForgedCSRF prevents other sites from silently
// retrieving potentially private academic data from the local app.
// RU: Проверяет запрет скачивания персональных данных с поддельным токеном.
func TestBackupDownloadRejectsForgedCSRF(t *testing.T) {
	ex := &fakeBackupExporter{}
	h, err := NewHandlerWithServices(&fakeGroupLister{}, &fakeStudentLister{}, Dependencies{Backup: ex})
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", "not-your-token"} {
		rec := postBackup(h, token)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status=%d, want=403", rec.Code)
		}
	}
	if ex.calls != 0 {
		t.Errorf("unexpected export count: %d", ex.calls)
	}
}

// TestBackupDownloadHidesStorageErrors does not expose SQLite paths to clients.
// RU: Проверяет отсутствие внутренних путей и ошибок файлового хранилища.
func TestBackupDownloadHidesStorageErrors(t *testing.T) {
	ex := &fakeBackupExporter{err: errors.New("secret filesystem location")}
	h, err := NewHandlerWithServices(&fakeGroupLister{}, &fakeStudentLister{}, Dependencies{Backup: ex})
	if err != nil {
		t.Fatal(err)
	}
	rec := postBackup(h, settingsToken(t, h))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

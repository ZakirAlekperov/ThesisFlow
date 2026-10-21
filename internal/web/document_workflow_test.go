package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
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

// documentFixture builds real repositories to verify the entire workflow.
// RU: documentFixture создаёт настоящие хранилища для проверки всего сценария.
func documentFixture(t *testing.T) (http.Handler, int64, int64, int64, *materials.Repository, *fileversions.Store) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	req := requirements.NewRepository(db)
	mat := materials.NewRepository(db)
	versions := fileversions.NewStore(db, filepath.Join(root, "storage"))
	reviews := review.NewService(db, versions)
	for _, init := range []func(context.Context) error{gr.Init, st.Init, req.Init, mat.Init, versions.Init, reviews.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	g, err := gr.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	s, err := st.Create(ctx, g.ID, "ЗК-0017", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	r, err := req.Create(ctx, g.ID, 0, "Глава 1", "2026-12-01")
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandlerWithServices(gr, st, Dependencies{
		Requirements: req,
		Documents:    &DocumentServices{Requirements: req, Statuses: mat, Versions: versions, Reviews: reviews},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, g.ID, s.ID, r.ID, mat, versions
}

// documentURL returns the scoped URL for the student's materials page.
// RU: documentURL формирует адрес материалов конкретного студента.
func documentURL(groupID, studentID int64) string {
	return fmt.Sprintf("/groups/%d/students/%d", groupID, studentID)
}

// documentToken retrieves the authenticated form token from the page.
// RU: documentToken получает токен формы страницы документов.
func documentToken(t *testing.T, handler http.Handler, page string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", page, rec.Code, rec.Body.String())
	}
	match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(rec.Body.String())
	if len(match) != 2 {
		t.Fatalf("form token not found in %s", rec.Body.String())
	}
	return match[1]
}

// TestStudentDocumentsShowsExpectedRequirements checks the teacher's view.
// Scenario: a student with one expected chapter but no submitted files.
// Expected: page shows requirement, deadline, Missing status and upload form.
//
// RU: Проверяет страницу ожидаемых документов студента.
// Сценарий: одна обязательная глава без загруженных файлов.
// Ожидается: название, срок, состояние «Не сдан» и форма загрузки.
func TestStudentDocumentsShowsExpectedRequirements(t *testing.T) {
	h, groupID, studentID, _, _, _ := documentFixture(t)
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page, nil))
	for _, want := range []string{"Иванов Иван", "Глава 1", "2026-12-01", "Не сдан", "Загрузить", token} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

// TestDocumentUploadAcceptDownload verifies a complete document lifecycle.
// Scenario: submit a DOCX version then accept it through protected forms.
// Expected: UnderReview after upload, Accepted after review and exact bytes
// available for download. The approved projection is created on disk.
//
// RU: Проверяет загрузку версии, приёмку и скачивание документа.
// Сценарий: отправка DOCX и принятие через защищённые формы.
// Ожидается: «На проверке», затем «Принят», исходные байты и принятая копия.
func TestDocumentUploadAcceptDownload(t *testing.T) {
	h, groupID, studentID, requirementID, statusStore, versionStore := documentFixture(t)
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	route := fmt.Sprintf("%s/documents/%d/upload", page, requirementID)
	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	if err := writer.WriteField("csrf_token", token); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("document", "Глава 1.docx")
	if err != nil {
		t.Fatal(err)
	}
	const original = "docx-test-binary"
	if _, err := file.Write([]byte(original)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, route, &data)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload HTTP %d: %s", rec.Code, rec.Body.String())
	}
	entry, err := statusStore.Get(context.Background(), studentID, requirementID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Status != materials.UnderReview {
		t.Errorf("status after upload = %s", entry.Status)
	}
	versions, err := versionStore.List(context.Background(), studentID, requirementID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions = %#v, error = %v", versions, err)
	}

	acceptURL := fmt.Sprintf("%s/documents/%d/accept/%d", page, requirementID, versions[0].ID)
	form := strings.NewReader("csrf_token=" + token)
	req = httptest.NewRequest(http.MethodPost, acceptURL, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("accept HTTP %d: %s", rec.Code, rec.Body.String())
	}
	entry, err = statusStore.Get(context.Background(), studentID, requirementID)
	if err != nil || entry.Status != materials.Accepted {
		t.Fatalf("accepted status = %+v, err=%v", entry, err)
	}
	downloadURL := fmt.Sprintf("%s/documents/%d/versions/%d/download", page, requirementID, versions[0].ID)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, downloadURL, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != original {
		t.Errorf("download HTTP %d: %q", rec.Code, rec.Body.String())
	}
	matches, err := filepath.Glob(filepath.Join(versionStore.Root, "ИСП-401 [[]*[]]"))
	if err != nil || len(matches) != 1 {
		t.Errorf("expected one accepted group directory, found %v (error: %v)", matches, err)
	}
}

// TestDocumentUploadRejectsForgedRequests protects student data from drive-by
// local-browser writes. Scenario: post a file without a valid CSRF token.
// Expected: HTTP 403 and no version saved.
//
// RU: Проверяет отказ в сохранении файла без CSRF-токена.
// Ожидается: HTTP 403 и отсутствие созданной версии.
func TestDocumentUploadRejectsForgedRequests(t *testing.T) {
	h, groupID, studentID, requirementID, _, versionStore := documentFixture(t)
	page := documentURL(groupID, studentID)
	route := fmt.Sprintf("%s/documents/%d/upload", page, requirementID)
	var data bytes.Buffer
	writer := multipart.NewWriter(&data)
	f, _ := writer.CreateFormFile("document", "bad.docx")
	_, _ = io.WriteString(f, "test")
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, route, &data)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST returned %d, want 403", rec.Code)
	}
	items, err := versionStore.List(context.Background(), studentID, requirementID)
	if err != nil || len(items) != 0 {
		t.Errorf("unexpected versions %#v: %v", items, err)
	}
}

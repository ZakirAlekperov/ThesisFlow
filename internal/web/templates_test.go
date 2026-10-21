package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/importxlsx"
)

// TestTemplatesCatalogIsAccessible verifies that users can navigate to a
// separate templates section without creating or modifying group records.
// RU: Проверяет отдельный раздел шаблонов без добавления записей в базу.
func TestTemplatesCatalogIsAccessible(t *testing.T) {
	groups := &fakeGroupLister{}
	students := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t, groups, students)
	for _, path := range []string{"/", "/templates"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `href="/templates"`) {
			t.Errorf("GET %s: no templates tab", path)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/templates", nil))
	for _, want := range []string{"Шаблоны", "Excel", "20 студентов", "Глава 1", "Скачать"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %q from templates page", want)
		}
	}
	if groups.created != 0 || students.created != 0 {
		t.Fatalf("opening templates changed records: groups=%d students=%d", groups.created, students.created)
	}
}

// TestSampleRosterDownloadsAsImportableXLSX validates a real download against
// the exact production XLSX reader; it never calls SQLite insert functions.
// RU: Проверяет скачиваемый XLSX настоящим парсером импорта без записи SQLite.
func TestSampleRosterDownloadsAsImportableXLSX(t *testing.T) {
	handler := newTestHandlerWithStudents(t, &fakeGroupLister{}, &fakeStudentLister{})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/templates/download/students-sample", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("missing attachment header: %q", rec.Header().Get("Content-Disposition"))
	}
	rows, err := importxlsx.Read(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("downloaded XLSX incompatible with real importer: %v", err)
	}
	if len(rows) != 21 || len(rows[0]) < 2 || rows[0][0] != "ФИО" || rows[0][1] != "Учебный ID" {
		t.Fatalf("unexpected roster headers/count: %#v", rows)
	}
	if rows[1][0] != "Иванов Артём Сергеевич" || rows[1][1] != "ТФ-260001" || rows[20][1] != "ТФ-260020" {
		t.Fatalf("unexpected sample first/last row: %#v %#v", rows[1], rows[20])
	}
}

// TestBlankRosterDownload verifies editable Excel template fields.
// RU: Проверяет пустой Excel-шаблон со столбцами для ручного заполнения.
func TestBlankRosterDownload(t *testing.T) {
	handler := newTestHandlerWithStudents(t, &fakeGroupLister{}, &fakeStudentLister{})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/templates/download/students-blank", nil))
	rows, err := importxlsx.Read(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("blank workbook invalid: %v", err)
	}
	if rec.Code != http.StatusOK || len(rows) == 0 || len(rows[0]) < 2 || rows[0][0] != "ФИО" || rows[0][1] != "Учебный ID" {
		t.Errorf("bad empty worksheet: status=%d data=%#v", rec.Code, rows)
	}
}

// TestThesisDocumentsAreRealDOCXFiles validates the OOXML package and ensures
// editable document examples are not plain text served under DOCX extensions.
// RU: Проверяет, что скачиваются настоящие DOCX, а не текст с другим расширением.
func TestThesisDocumentsAreRealDOCXFiles(t *testing.T) {
	handler := newTestHandlerWithStudents(t, &fakeGroupLister{}, &fakeStudentLister{})
	for _, id := range []string{"chapter1-draft", "chapter1-revision", "chapter2-sample"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/templates/download/"+id, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			t.Fatalf("%s: code=%d type=%q", id, rec.Code, rec.Header().Get("Content-Type"))
		}
		z, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil { t.Fatalf("%s: not a real DOCX archive: %v", id, err) }
		found := false
		for _, part := range z.File {
			if part.Name == "word/document.xml" {
				f, err := part.Open()
				if err != nil { t.Fatal(err) }
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil { t.Fatal(err) }
				found = strings.Contains(string(data), "ТЕСТОВЫЙ")
			}
		}
		if !found { t.Errorf("%s: missing expected Word content", id) }
	}
}

// TestTemplatesDoNotExposeArbitraryFilesystemPaths ensures users can retrieve
// only allowlisted embedded documents; unsafe paths and POST return errors.
// RU: Проверяет, что раздел не отдаёт произвольные файлы и не принимает POST.
func TestTemplatesDoNotExposeArbitraryFilesystemPaths(t *testing.T) {
	handler := newTestHandlerWithStudents(t, &fakeGroupLister{}, &fakeStudentLister{})
	for _, path := range []string{"/templates/download/unknown", "/templates/download/..%2Fsecret", "/templates/download/thesisflow.sqlite"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: got %d, want 404/400", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/templates", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /templates: got %d, want 405", rec.Code)
	}
}

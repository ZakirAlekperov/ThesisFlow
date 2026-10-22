package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExcelPreviewUsesSemanticTableHeader verifies that the first workbook
// row becomes a table heading, not an ordinary cell with hardcoded colors.
// RU: Заголовки XLSX находятся в thead/th, а не в первой строке td.
func TestExcelPreviewUsesSemanticTableHeader(t *testing.T) {
	b := new(bytes.Buffer)
	err := previewTemplate.Execute(b, previewPage{
		GroupID: 1, Sheets: []string{"Студенты"}, SheetName: "Студенты",
		Headers:      []string{"ФИО", "Учебный ID"},
		Rows:         [][]string{{"Иванов Иван", "А-101"}},
		PreviewToken: "test", CSRFToken: "valid",
	})
	if err != nil {
		t.Fatal(err)
	}
	page := b.String()
	for _, want := range []string{"class=\"tf-preview\"", "<thead>",
		"<th scope=\"col\">ФИО</th>", "<tbody>", "<td>Иванов Иван</td>",
		"class=\"preview-table\""} {
		if !strings.Contains(page, want) {
			t.Errorf("preview missing %q", want)
		}
	}
}

// TestPageHeadersProhibitCaching protects status pages from browser caching.
// RU: Ответы не должны кешироваться после принятия документа.
func TestPageHeadersProhibitCaching(t *testing.T) {
	h, err := NewHandler(&fakeGroupLister{}, &fakeStudentLister{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control: %q", rec.Header().Get("Cache-Control"))
	}
}

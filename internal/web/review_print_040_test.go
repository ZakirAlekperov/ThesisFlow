package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestReviewHistoryPrintUsesRealVersionNotes ensures a browser print view
// contains persisted comments and uses html/template escaping.
// RU: Проверяем печать реальных замечаний после загрузки версии.
func TestReviewHistoryPrintUsesRealVersionNotes(t *testing.T) {
	h, groupID, studentID, reqID, _, versions := documentFixture(t)
	page := documentURL(groupID, studentID)
	csrf := documentToken(t, h, page)
	uploadReviewedVersion(t, h, page, csrf, reqID)
	items, err := versions.List(context.Background(), studentID, reqID)
	if err != nil || len(items) != 1 {
		t.Fatalf("versions %+v / %v", items, err)
	}
	commentURL := fmt.Sprintf("%s/documents/%d/comment/%d", page, reqID, items[0].ID)
	body := url.Values{"csrf_token": {csrf}, "comment": {"Проверка <script>alert(1)</script>"}}
	rec := reviewForm(h, commentURL, body)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save comment: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page+"/reviews/print", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("print HTTP %d: %s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	for _, want := range []string{"Журнал рецензирования", "Глава 1", "Иванов Иван",
		"Версия #", "&lt;script&gt;", "Проверка", "Сохраните страницу в PDF"} {
		if !strings.Contains(html, want) {
			t.Errorf("print missing %q", want)
		}
	}
	if strings.Contains(html, "<script>alert") {
		t.Fatal("unescaped HTML in printable feedback")
	}
	if rec.Header().Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'" {
		t.Errorf("print CSP missing: %v", rec.Header())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/groups/%d/students/999999/reviews/print", groupID), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown student HTTP %d", rec.Code)
	}
}

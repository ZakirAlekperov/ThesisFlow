package web

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

// TestConceptCThemeStyle ensures local assets include contrasting navigation,
// page-wide dark backgrounds, and Excel-specific contrast. It is a structural
// contract: visual screenshots remain an additional manual acceptance step.
// RU: Проверяем стили темы C; визуальная проверка браузера обязательна отдельно.
func TestConceptCThemeStyle(t *testing.T) {
    h, err := NewHandler(&fakeGroupLister{}, &fakeStudentLister{})
    if err != nil { t.Fatal(err) }
    r := httptest.NewRecorder()
    h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/static/theme.css", nil))
    if r.Code != http.StatusOK { t.Fatalf("CSS HTTP %d", r.Code) }
    css := r.Body.String()
    for _, want := range []string{
        "html, body", "--tf-button-text", ".tabs a[aria-current="page"]",
        ".tf-preview", "color: var(--tf-button-text)", "background: var(--tf-bg)",
    } {
        if !strings.Contains(css, want) { t.Errorf("CSS missing %q", want) }
    }
}

func TestConceptCHomeNavigation(t *testing.T) {
    h, err := NewHandler(&fakeGroupLister{}, &fakeStudentLister{})
    if err != nil { t.Fatal(err) }
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
    if rec.Code != http.StatusOK { t.Fatalf("home HTTP %d", rec.Code) }
    for _, want := range []string{
        "Учебные группы", "Шаблоны", "Панель преподавателя",
        "Поиск студентов", "Управление и архив", "aria-current="page"",
    } {
        if !strings.Contains(rec.Body.String(), want) { t.Errorf("navigation missing %q", want) }
    }
}

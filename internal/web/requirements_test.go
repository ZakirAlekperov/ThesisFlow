package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
)

// fakeRequirements provides a writable read test double for the group page.
// RU: fakeRequirements имитирует чтение и создание требований.
type fakeRequirements struct {
	items   []requirements.Requirement
	created int
}

func (f *fakeRequirements) ListForGroup(_ context.Context, groupID int64) ([]requirements.Requirement, error) {
	return f.items, nil
}
func (f *fakeRequirements) Create(_ context.Context, groupID, studentID int64, title, dueDate string) (requirements.Requirement, error) {
	f.created++
	req := requirements.Requirement{ID: int64(f.created), GroupID: groupID, StudentID: studentID, Title: strings.TrimSpace(title), DueDate: dueDate}
	f.items = append(f.items, req)
	return req, nil
}

// TestGroupRequirementsPageShowsFormAndExistingDocuments verifies the teacher
// can see group-wide requirements and a protected creation form.
// RU: Проверяет вывод требований группы и защищённой формы создания.
func TestGroupRequirementsPageShowsFormAndExistingDocuments(t *testing.T) {
	store := &fakeRequirements{items: []requirements.Requirement{{ID: 6, GroupID: 17, Title: "Глава 1", DueDate: "2026-11-10"}}}
	h, err := NewHandlerWithServices(
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, &fakeStudentLister{},
		Dependencies{Requirements: store},
	)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/groups/17", nil))
	for _, expected := range []string{"Глава 1", "2026-11-10", `action="/groups/17/requirements"`, `name="title"`, `name="due_date"`} {
		if !strings.Contains(rec.Body.String(), expected) {
			t.Errorf("missing %q in page", expected)
		}
	}
}

// TestGroupRequirementsFormCreatesAndRedirects verifies scope and CSRF.
// Scenario: submit title/deadline to selected group with valid token.
// Expected: one Create call and HTTP 303 redirect to the same group.
//
// RU: Проверяет создание требования для выбранной группы.
// Сценарий: POST названия/срока с подлинным CSRF-токеном.
// Ожидается: одно создание и перенаправление HTTP 303.
func TestGroupRequirementsFormCreatesAndRedirects(t *testing.T) {
	store := &fakeRequirements{}
	h, err := NewHandlerWithServices(
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, &fakeStudentLister{},
		Dependencies{Requirements: store},
	)
	if err != nil {
		t.Fatal(err)
	}
	token := studentFormToken(t, h, "17")
	values := url.Values{"title": {"Глава 1"}, "due_date": {"2026-11-10"}, "csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/groups/17/requirements", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/groups/17" || store.created != 1 {
		t.Errorf("POST status=%d location=%q created=%d", rec.Code, rec.Header().Get("Location"), store.created)
	}
}

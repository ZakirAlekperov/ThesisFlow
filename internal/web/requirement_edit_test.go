package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestRequirementEditThroughBrowser verifies the existing group screen exposes
// a form and persists edits through HTTP, not direct test-fixture writes.
// RU: Меняем название/срок через обычную форму браузера.
func TestRequirementEditThroughBrowser(t *testing.T) {
	h, groupID, _, requirementID, _, _ := documentFixture(t)
	groupURL := fmt.Sprintf("/groups/%d", groupID)
	token := documentToken(t, h, groupURL)
	render := httptest.NewRecorder()
	h.ServeHTTP(render, httptest.NewRequest(http.MethodGet, groupURL, nil))
	if !strings.Contains(render.Body.String(), fmt.Sprintf("/requirements/%d/update", requirementID)) {
		t.Fatalf("edit form absent")
	}
	submit := func(csrf, title, date string) *httptest.ResponseRecorder {
		form := url.Values{"csrf_token": {csrf}, "title": {title}, "due_date": {date}}
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/groups/%d/requirements/%d/update", groupID, requirementID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if got := submit("forged", "Заменили главу", "2026-12-12"); got.Code != http.StatusForbidden {
		t.Errorf("forged = %d", got.Code)
	}
	if got := submit(token, "Введение", "2026-12-12"); got.Code != http.StatusSeeOther {
		t.Fatalf("edit = %d: %s", got.Code, got.Body.String())
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, groupURL, nil))
	if !strings.Contains(rec.Body.String(), "2026-12-12") || !strings.Contains(rec.Body.String(), "Введение") {
		t.Errorf("updated requirement not shown")
	}
	if got := submit(token, "Введение", "2026-13-41"); got.Code != http.StatusBadRequest {
		t.Errorf("invalid date = %d", got.Code)
	}
}

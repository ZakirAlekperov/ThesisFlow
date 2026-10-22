package web

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
)

// uploadReviewedVersion exercises the existing multipart browser route,
// rather than populating test data directly through a repository.
// RU: Загружаем версию через настоящий HTTP-маршрут приложения.
func uploadReviewedVersion(t *testing.T, h http.Handler, page, csrf string, requirementID int64) {
	t.Helper()
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	if err := writer.WriteField("csrf_token", csrf); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("document", "Глава 1.docx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("review test bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/documents/%d/upload", page, requirementID), &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload HTTP %d: %s", rec.Code, rec.Body.String())
	}
}

// reviewForm posts a teacher review decision with the same origin and CSRF
// semantics as the browser form.
// RU: Настоящая HTML-форма проверки, включая токен и редиректы.
func reviewForm(h http.Handler, route string, values url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestReviewFeedbackBrowserWorkflow verifies comments are shown escaped,
// and rejection with a mandatory reason persists along with the new status.
// RU: Проверяет комментарии, причины возврата и XSS-экранирование.
func TestReviewFeedbackBrowserWorkflow(t *testing.T) {
	h, groupID, studentID, requirementID, statuses, versions := documentFixture(t)
	page := documentURL(groupID, studentID)
	csrf := documentToken(t, h, page)
	uploadReviewedVersion(t, h, page, csrf, requirementID)
	saved, err := versions.List(context.Background(), studentID, requirementID)
	if err != nil || len(saved) != 1 {
		t.Fatalf("versions=%+v err=%v", saved, err)
	}
	id := saved[0].ID
	commentPath := fmt.Sprintf("%s/documents/%d/comment/%d", page, requirementID, id)
	rejectPath := fmt.Sprintf("%s/documents/%d/reject/%d", page, requirementID, id)

	// Malformed text is rejected, and forged submissions cannot add notes.
	if response := reviewForm(h, rejectPath, url.Values{"csrf_token": {csrf}, "reason": {" "}}); response.Code != http.StatusBadRequest {
		t.Fatalf("blank reason: HTTP %d", response.Code)
	}
	if response := reviewForm(h, commentPath, url.Values{"csrf_token": {"wrong"}, "comment": {"forged"}}); response.Code != http.StatusForbidden {
		t.Fatalf("forged comment: HTTP %d", response.Code)
	}
	if response := reviewForm(h, commentPath, url.Values{"csrf_token": {csrf}, "comment": {"Проверить подпись <script>alert('x')</script>"}}); response.Code != http.StatusSeeOther {
		t.Fatalf("comment HTTP %d: %s", response.Code, response.Body.String())
	}
	state, err := statuses.Get(context.Background(), studentID, requirementID)
	if err != nil || state.Status != materials.UnderReview {
		t.Fatalf("comment changed status: %+v %v", state, err)
	}

	reason := "Отсутствует ссылка на источник 2026 года."
	if response := reviewForm(h, rejectPath, url.Values{"csrf_token": {csrf}, "reason": {reason}}); response.Code != http.StatusSeeOther {
		t.Fatalf("reject HTTP %d: %s", response.Code, response.Body.String())
	}
	state, err = statuses.Get(context.Background(), studentID, requirementID)
	if err != nil || state.Status != materials.NeedsRevision {
		t.Fatalf("rejection status: %+v %v", state, err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page, nil))
	html := rec.Body.String()
	for _, want := range []string{"История рецензирования", "Причина возврата", reason, "Версия #", "Проверить подпись"} {
		if !strings.Contains(html, want) {
			t.Errorf("feedback page missing %q", want)
		}
	}
	if strings.Contains(html, "<script>alert") {
		t.Fatal("unescaped review comment")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("HTML auto-escaping missing")
	}

	// Duplicate review decisions must not create additional history.
	if response := reviewForm(h, rejectPath, url.Values{"csrf_token": {csrf}, "reason": {"Ещё раз"}}); response.Code != http.StatusConflict {
		t.Fatalf("repeated rejection: HTTP %d, want 409", response.Code)
	}
	notes, err := review.NewService(versions.DB, versions).ListFeedback(context.Background(), studentID, requirementID)
	if err != nil || len(notes) != 2 {
		t.Fatalf("stored comments/reasons=%+v %v", notes, err)
	}
}

// TestReviewFeedbackCannotCrossRequirementScope ensures a teacher cannot
// attach a note to a guessed foreign version from another student's route.
// RU: Версия не может быть прокомментирована через чужую карточку.
func TestReviewFeedbackCannotCrossRequirementScope(t *testing.T) {
	h, groupID, studentID, requirementID, _, versions := documentFixture(t)
	page := documentURL(groupID, studentID)
	csrf := documentToken(t, h, page)
	uploadReviewedVersion(t, h, page, csrf, requirementID)
	saved, err := versions.List(context.Background(), studentID, requirementID)
	if err != nil || len(saved) != 1 {
		t.Fatal(err)
	}
	target := fmt.Sprintf("%s/documents/%d/comment/%d", page, requirementID+1234, saved[0].ID)
	response := reviewForm(h, target, url.Values{"csrf_token": {csrf}, "comment": {"cross-scope"}})
	if response.Code != http.StatusNotFound {
		t.Fatalf("wrong requirement route HTTP %d", response.Code)
	}
}

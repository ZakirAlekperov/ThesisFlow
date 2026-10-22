package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
)

// TestDocumentRecoveryConflictsProvideClearActions verifies that a teacher
// receives a recoverable 409 instead of an unexplained 500 or false success.
// RU: Ошибки восстановления дают понятное безопасное действие.
func TestDocumentRecoveryConflictsProvideClearActions(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{review.ErrPendingApproval, "снова запустите ThesisFlow"},
		{review.ErrRecoveryConflict, "Документ не перезаписан"},
	}
	for _, tc := range tests {
		rec := httptest.NewRecorder()
		documentError(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
		if rec.Code != http.StatusConflict {
			t.Fatalf("%v: HTTP %d", tc.err, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%v: missing action %q: %s", tc.err, tc.want, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), tc.err.Error()) {
			t.Errorf("internal recovery error leaked: %v", tc.err)
		}
	}
	// Do not convert arbitrary storage failures into a false 409.
	rec := httptest.NewRecorder()
	documentError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("private IO detail"))
	if rec.Code != http.StatusInternalServerError ||
		strings.Contains(rec.Body.String(), "private IO detail") {
		t.Fatalf("unexpected error response: %d %s", rec.Code, rec.Body.String())
	}
}

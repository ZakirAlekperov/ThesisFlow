package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/readiness"
)

// fakeReadiness supplies deterministic progress values without SQLite.
// RU: fakeReadiness возвращает заданные показатели без SQLite.
type fakeReadiness struct {
	summary readiness.Summary
	err     error
}

func (f fakeReadiness) Group(_ context.Context, groupID int64, _ time.Time) (readiness.Summary, error) {
	if f.err != nil { return readiness.Summary{}, f.err }
	result := f.summary
	result.GroupID = groupID
	return result, nil
}

// TestPrintableReportShowsMetrics checks the teacher-visible report.
// Scenario: a valid group has 2 students and one accepted of four documents.
// Expected: HTML contains correct completion, overdue and printable CSS.
//
// RU: Проверяет печатную ведомость с готовностью и просрочками.
// Сценарий: два студента, принят 1 документ из 4.
// Ожидается: 25% готовности, число просрочек и правила печати.
func TestPrintableReportShowsMetrics(t *testing.T) {
	groupStore := &fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}
	handler, err := NewHandlerWithServices(groupStore, &fakeStudentLister{}, Dependencies{
		Report: fakeReadiness{summary: readiness.Summary{StudentCount: 2, Required: 4, Accepted: 1, Overdue: 2, Percent: 25}},
	})
	if err != nil { t.Fatal(err) }
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17/report", nil))
	if response.Code != http.StatusOK { t.Fatalf("status = %d", response.Code) }
	for _, want := range []string{"ИСП-401", "25%", "2", "4", "@media print"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("report missing %q: %s", want, response.Body.String())
		}
	}
}

// TestPrintableReportRejectsUnknownGroup prevents showing data under a
// fabricated group ID. A missing group must produce HTTP 404.
//
// RU: Проверяет HTTP 404 для несуществующей учебной группы.
func TestPrintableReportRejectsUnknownGroup(t *testing.T) {
	handler, err := NewHandlerWithServices(&fakeGroupLister{}, &fakeStudentLister{}, Dependencies{
		Report: fakeReadiness{},
	})
	if err != nil { t.Fatal(err) }
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/999/report", nil))
	if response.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", response.Code)
	}
}

// TestPrintableReportHidesStorageErrors checks that internal SQLite messages
// never appear in a printable teacher report.
//
// RU: Проверяет отсутствие внутренних ошибок SQLite в печатной ведомости.
func TestPrintableReportHidesStorageErrors(t *testing.T) {
	handler, err := NewHandlerWithServices(
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}},
		&fakeStudentLister{}, Dependencies{Report: fakeReadiness{err: errors.New("secret database path")}},
	)
	if err != nil { t.Fatal(err) }
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17/report", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret") {
		t.Errorf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

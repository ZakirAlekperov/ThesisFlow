package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)

type fakeDoctor struct {
	report workspace.Report
	err    error
}

func (d fakeDoctor) Check(context.Context) (workspace.Report, error) { return d.report, d.err }

// TestWorkspaceHealthShowsRealNonSensitiveResults validates the local
// diagnostics route and escaped issue messages, with no private file paths.
// RU: Диагностика не раскрывает имена и пути студенческих файлов.
func TestWorkspaceHealthShowsRealNonSensitiveResults(t *testing.T) {
	_, _, _, _, _, versions := documentFixture(t)
	reporter := fakeDoctor{report: workspace.Report{
		VersionsChecked: 2, VersionsIntact: 1, AcceptedChecked: 1, AcceptedIntact: 0,
		Issues: []workspace.Issue{{Code: "accepted_corrupt", VersionID: 12, RequirementID: 3, Detail: "<script>alert(1)</script>"}},
	}}
	h, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Doctor: reporter})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health HTTP %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Обнаружены нарушения", "Версии документов: 1 из 2", "accepted_corrupt", "&lt;script&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("health missing %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatal("unescaped health issue")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("health page is cacheable")
	}
}

func TestWorkspaceHealthHandlesScanFailures(t *testing.T) {
	_, _, _, _, _, versions := documentFixture(t)
	h, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Doctor: fakeDoctor{err: errors.New("path private user info")}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/health", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("failed check status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "path private user info") {
		t.Fatal("private scan error leaked")
	}
}

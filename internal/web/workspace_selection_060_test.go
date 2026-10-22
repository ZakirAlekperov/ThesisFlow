package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspaceselect"
)

type fakeSelection struct {
	id     string
	called string
	err    error
}

func (f *fakeSelection) List(context.Context) ([]workspaceselect.Option, error) {
	return []workspaceselect.Option{{ID: "restore-0123456789abcdef01234567", Created: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}}, nil
}
func (f *fakeSelection) Current() (string, error) { return f.id, nil }
func (f *fakeSelection) Choose(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	f.called = id
	f.id = id
	return nil
}

func TestWorkspaceWizardRequiresCSRFAndShowsNextLaunchWarning(t *testing.T) {
	_, _, _, _, _, versions := documentFixture(t)
	selection := &fakeSelection{id: workspaceselect.Default}
	app, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Selection: selection})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/workspaces", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("wizard GET %d %s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	for _, want := range []string{"Шаг 1", "Шаг 2", "Шаг 3", "restore-0123456789abcdef01234567", "следующего запуска", "csrf_token"} {
		if !strings.Contains(html, want) {
			t.Errorf("wizard missing %q", want)
		}
	}
	match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(html)
	if len(match) != 2 {
		t.Fatal("missing CSRF hidden field")
	}
	submit := func(token, id string) int {
		t.Helper()
		form := url.Values{"csrf_token": {token}, "workspace": {id}}
		request := httptest.NewRequest(http.MethodPost, "/settings/workspaces", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		app.ServeHTTP(response, request)
		return response.Code
	}
	if got := submit("BAD", "restore-0123456789abcdef01234567"); got != http.StatusForbidden {
		t.Fatalf("CSRF accepted %d", got)
	}
	if selection.called != "" {
		t.Fatal("failed CSRF changed selection")
	}
	if got := submit(match[1], "restore-0123456789abcdef01234567"); got != http.StatusOK {
		t.Fatalf("valid wizard POST %d", got)
	}
	if selection.called != "restore-0123456789abcdef01234567" {
		t.Fatalf("selection not applied %q", selection.called)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("wizard is cacheable")
	}
}

func TestWorkspaceWizardReportsRejectedCopyWithoutSwitching(t *testing.T) {
	_, _, _, _, _, versions := documentFixture(t)
	fake := &fakeSelection{id: workspaceselect.Default, err: workspaceselect.ErrInvalidChoice}
	app, err := NewHandlerWithServices(groups.NewRepository(versions.DB),
		students.NewRepository(versions.DB), Dependencies{Selection: fake})
	if err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	app.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/settings/workspaces", nil))
	match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(page.Body.String())
	form := url.Values{"csrf_token": {match[1]}, "workspace": {"../../data"}}
	request := httptest.NewRequest(http.MethodPost, "/settings/workspaces", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || fake.called != "" {
		t.Fatalf("invalid choice changed settings: %d, %q", response.Code, fake.called)
	}
	fake.err = errors.New("database unavailable")
	response = httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("internal failure HTTP %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "database unavailable") {
		t.Fatal("private storage error leaked")
	}
}

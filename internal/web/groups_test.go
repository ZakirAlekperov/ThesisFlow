package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
)

// fakeGroupLister supplies predictable group data without opening a real database.
// RU: fakeGroupLister возвращает заданные группы без подключения к настоящей БД.
type fakeGroupLister struct {
	items     []groups.Group
	err       error
	createErr error
	created   int
	lastName  string
}

func (f *fakeGroupLister) List(context.Context) ([]groups.Group, error) {
	return f.items, f.err
}

// Create tracks calls, returning an injected failure or an in-memory group.
// RU: Create учитывает вызовы, возвращает заданную ошибку или тестовую группу.
func (f *fakeGroupLister) Create(_ context.Context, name string) (groups.Group, error) {
	f.created++
	f.lastName = name
	if f.createErr != nil {
		return groups.Group{}, f.createErr
	}
	group := groups.Group{ID: int64(len(f.items) + 1), Name: strings.TrimSpace(name)}
	f.items = append(f.items, group)
	return group, nil
}

// newTestHandler creates the production handler and reports constructor errors.
// RU: newTestHandler создаёт настоящий обработчик и сообщает об ошибке создания.
func newTestHandler(t *testing.T, store *fakeGroupLister) http.Handler {
	t.Helper()
	handler, err := NewHandler(store)
	if err != nil {
		t.Fatalf("NewHandler(): %v", err)
	}
	return handler
}

// TestHomePageShowsEmptyGroupsState verifies useful feedback for a fresh workspace.
// Scenario: the repository returns no groups on GET /.
// Expected: HTTP 200 and a clear empty-state message in Russian.
//
// RU: Проверяет понятное состояние главной страницы без групп.
// Сценарий: при GET / хранилище возвращает пустой список.
// Ожидается: HTTP 200 и сообщение об отсутствии групп на русском языке.
func TestHomePageShowsEmptyGroupsState(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(t, &fakeGroupLister{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Группы пока не добавлены") {
		t.Errorf("missing empty state in %q", response.Body.String())
	}
}

// TestHomePageListsSavedGroups verifies that names, IDs and repository order
// are preserved in the rendered HTML.
// Scenario: the repository returns two groups with their permanent IDs.
// Expected: both groups are visible, in order, with matching data-group-id values.
//
// RU: Проверяет вывод названий, ID и порядка из базы.
// Сценарий: хранилище возвращает две группы с постоянными ID.
// Ожидается: обе видны на странице в заданном порядке и с правильными ID.
func TestHomePageListsSavedGroups(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(t, &fakeGroupLister{items: []groups.Group{
		{ID: 17, Name: "ИСП-401"},
		{ID: 29, Name: "ПР-301"},
	}}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	html := response.Body.String()
	for _, expected := range []string{"Учебные группы", "ИСП-401", "ПР-301", "data-group-id=\"17\"", "data-group-id=\"29\""} {
		if !strings.Contains(html, expected) {
			t.Errorf("missing %q in HTML: %q", expected, html)
		}
	}
	if strings.Index(html, "ИСП-401") > strings.Index(html, "ПР-301") {
		t.Errorf("group display order changed: %q", html)
	}
	if strings.Contains(html, "Группы пока не добавлены") {
		t.Errorf("unexpected empty state with saved groups: %q", html)
	}
}

// TestHomePageEscapesGroupNames protects the document from HTML injection.
// Scenario: a group name contains markup-like text from the database.
// Expected: markup is escaped, never interpreted as HTML.
//
// RU: Проверяет защиту страницы от HTML-инъекции в названии группы.
// Сценарий: название из базы содержит текст, похожий на HTML.
// Ожидается: спецсимволы экранируются; HTML-код не выполняется.
func TestHomePageEscapesGroupNames(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(t, &fakeGroupLister{items: []groups.Group{
		{ID: 1, Name: "<script>alert(1)</script>"},
	}}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	html := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("group name is not safely escaped: %q", html)
	}
}

// TestHomePageHandlesGroupQueryError distinguishes database errors from an
// empty list and does not reveal internal error details to the browser.
// Scenario: the group repository returns a sensitive storage error.
// Expected: HTTP 500 with a generic message, without the error details.
//
// RU: Проверяет отличие ошибки БД от пустого списка без утечки деталей.
// Сценарий: хранилище возвращает внутреннюю ошибку чтения.
// Ожидается: HTTP 500 и общее сообщение без подробностей ошибки.
func TestHomePageHandlesGroupQueryError(t *testing.T) {
	response := httptest.NewRecorder()
	newTestHandler(t, &fakeGroupLister{err: errors.New("secret database path")}).ServeHTTP(
		response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	body := response.Body.String()
	if strings.Contains(body, "secret database path") || strings.Contains(body, "Группы пока не добавлены") {
		t.Errorf("query error leaked or disguised as empty list: %q", body)
	}
	if !strings.Contains(body, "Не удалось загрузить список групп") {
		t.Errorf("missing generic error message: %q", body)
	}
}

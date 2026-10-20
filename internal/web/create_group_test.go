package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
)

// extractFormToken obtains the hidden token from a rendered test form.
// RU: extractFormToken извлекает скрытый токен из тестовой HTML-формы.
func extractFormToken(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", response.Code)
	}
	const prefix = `name="csrf_token" value="`
	html := response.Body.String()
	start := strings.Index(html, prefix)
	if start < 0 {
		t.Fatalf("missing CSRF input: %q", html)
	}
	rest := html[start+len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 || end == 0 {
		t.Fatalf("missing CSRF token value: %q", html)
	}
	return rest[:end]
}

// submitGroup simulates a browser form POST with the specified token.
// RU: submitGroup имитирует отправку HTML-формы с указанным токеном.
func submitGroup(handler http.Handler, name, token string) *httptest.ResponseRecorder {
	values := url.Values{"name": {name}, "csrf_token": {token}}
	request := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

// TestGroupFormIsAvailable checks the browser-facing create form.
// Scenario: open the home page with an empty repository.
// Expected: accessible POST form, a named input and nonempty CSRF token.
//
// RU: Проверяет доступность формы создания группы в браузере.
// Сценарий: главная страница без созданных групп.
// Ожидается: POST-форма, поле названия и непустой CSRF-токен.
func TestGroupFormIsAvailable(t *testing.T) {
	handler := newTestHandler(t, &fakeGroupLister{})
	token := extractFormToken(t, handler)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, want := range []string{`action="/groups"`, `method="post"`, `name="name"`, "Создать группу", token} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("form missing %q", want)
		}
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

// TestGroupFormCreatesAndRedirects checks successful browser creation.
// Scenario: submit a name and a valid CSRF token.
// Expected: one Create call and HTTP 303 redirect to the home page.
//
// RU: Проверяет создание группы и переход на главную страницу.
// Сценарий: правильное название и действительный CSRF-токен.
// Ожидается: один вызов Create и перенаправление HTTP 303.
func TestGroupFormCreatesAndRedirects(t *testing.T) {
	store := &fakeGroupLister{}
	handler := newTestHandler(t, store)
	response := submitGroup(handler, "  ИСП-401  ", extractFormToken(t, handler))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("POST response = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
	if store.created != 1 || store.lastName != "  ИСП-401  " {
		t.Errorf("Create calls = %d, name = %q", store.created, store.lastName)
	}
}

// TestGroupFormShowsValidationErrors verifies non-destructive failures.
// Scenarios: blank group name, then a duplicate name.
// Expected: HTTP 400 or 409, a readable message, and no silent redirect.
//
// RU: Проверяет отображение ошибок ввода без ложного подтверждения.
// Сценарии: пустое название и уже существующая группа.
// Ожидается: HTTP 400 или 409, понятное сообщение и отсутствие перенаправления.
func TestGroupFormShowsValidationErrors(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		storeErr error
		status   int
		message  string
	}{
		{"empty", "  ", groups.ErrInvalidName, http.StatusBadRequest, "Введите название группы"},
		{"duplicate", "ИСП-401", groups.ErrDuplicateName, http.StatusConflict, "Группа с таким названием уже существует"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeGroupLister{createErr: tc.storeErr}
			handler := newTestHandler(t, store)
			response := submitGroup(handler, tc.input, extractFormToken(t, handler))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if !strings.Contains(response.Body.String(), tc.message) {
				t.Errorf("missing validation error: %q", response.Body.String())
			}
			if tc.input == "ИСП-401" && !strings.Contains(response.Body.String(), `value="ИСП-401"`) {
				t.Errorf("form did not preserve input: %q", response.Body.String())
			}
		})
	}
}

// TestGroupFormRejectsMissingAndIncorrectCSRFToken checks cross-site request
// protection. Scenario: submit without a token or with an invented token.
// Expected: HTTP 403 and no group is created.
//
// RU: Проверяет защиту от межсайтовой подделки запросов.
// Сценарий: отправка без токена или с поддельным токеном.
// Ожидается: HTTP 403, создание группы не выполняется.
func TestGroupFormRejectsMissingAndIncorrectCSRFToken(t *testing.T) {
	store := &fakeGroupLister{}
	handler := newTestHandler(t, store)
	for _, token := range []string{"", "not-the-secret-token"} {
		response := submitGroup(handler, "ИСП-401", token)
		if response.Code != http.StatusForbidden {
			t.Errorf("invalid token %q: status = %d, want 403", token, response.Code)
		}
	}
	if store.created != 0 {
		t.Errorf("Create called %d times for invalid tokens", store.created)
	}
}

// TestGroupFormRejectsOversizeBody verifies that form data is bounded.
// Scenario: send a form body larger than the configured limit.
// Expected: HTTP 400 or 413 and no database write.
//
// RU: Проверяет ограничение размера отправляемой формы.
// Сценарий: тело запроса больше допустимого предела.
// Ожидается: HTTP 400/413 и отсутствие записи в базу.
func TestGroupFormRejectsOversizeBody(t *testing.T) {
	store := &fakeGroupLister{}
	handler := newTestHandler(t, store)
	response := submitGroup(handler, strings.Repeat("x", 5000), extractFormToken(t, handler))
	if response.Code != http.StatusBadRequest && response.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize response = %d, want 400 or 413", response.Code)
	}
	if store.created != 0 {
		t.Errorf("oversize form created %d groups", store.created)
	}
}

// TestGroupFormHidesStorageErrors prevents internal paths or SQL from leaking.
// Scenario: group creation returns a private database error.
// Expected: generic HTTP 500 without the internal error text.
//
// RU: Проверяет скрытие внутренних ошибок SQLite.
// Сценарий: Create возвращает ошибку с конфиденциальными подробностями.
// Ожидается: общее сообщение HTTP 500 без текста внутренней ошибки.
func TestGroupFormHidesStorageErrors(t *testing.T) {
	store := &fakeGroupLister{createErr: errors.New("private database file path")}
	handler := newTestHandler(t, store)
	response := submitGroup(handler, "ИСП-401", extractFormToken(t, handler))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "private database file path") {
		t.Errorf("storage error leaked in %q", response.Body.String())
	}
}

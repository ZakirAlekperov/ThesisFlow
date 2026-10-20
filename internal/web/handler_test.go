package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewHandlerServesHomePage verifies the HTTP contract of the first page.
// Scenario: an HTTP GET request is sent to the application root path.
// Expected: HTTP 200, an HTML content type and a page titled ThesisFlow.
//
// RU: Проверяет HTTP-контракт главной страницы приложения.
// Сценарий: GET-запрос отправлен по корневому пути приложения.
// Ожидается: HTTP 200, тип содержимого HTML и заголовок ThesisFlow.
func TestNewHandlerServesHomePage(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	NewHandler(fakeGroupLister{}).ServeHTTP(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", result.StatusCode, http.StatusOK)
	}

	if got := result.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("content type = %q, want HTML with UTF-8", got)
	}

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), "<title>ThesisFlow</title>") {
		t.Errorf("home page has no ThesisFlow title: %q", body)
	}
}

// TestNewHandlerRejectsUnsupportedRoutes verifies that the public handler
// does not accidentally accept unknown paths or write methods.
// Scenarios: GET to an unknown path, and POST to the home page.
// Expected: HTTP 404 and HTTP 405 respectively.
//
// RU: Проверяет, что обработчик не принимает неизвестные маршруты и методы.
// Сценарии: GET по неизвестному пути и POST по главной странице.
// Ожидается: соответственно HTTP 404 и HTTP 405.
func TestNewHandlerRejectsUnsupportedRoutes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "unknown path", method: http.MethodGet, path: "/missing", want: http.StatusNotFound},
		{name: "unsupported method", method: http.MethodPost, path: "/", want: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()

			NewHandler(fakeGroupLister{}).ServeHTTP(response, request)

			if got := response.Code; got != test.want {
				t.Errorf("status = %d, want %d", got, test.want)
			}
		})
	}
}

package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// studentFormToken fetches the CSRF token displayed on one group's page.
// RU: studentFormToken получает CSRF-токен со страницы выбранной группы.
func studentFormToken(t *testing.T, handler http.Handler, groupID string) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/"+groupID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET group page status = %d, want 200", response.Code)
	}
	const prefix = `name="csrf_token" value="`
	body := response.Body.String()
	start := strings.Index(body, prefix)
	if start < 0 {
		t.Fatalf("group page has no CSRF field: %q", body)
	}
	value := body[start+len(prefix):]
	end := strings.Index(value, `"`)
	if end <= 0 {
		t.Fatalf("group page has empty CSRF token: %q", body)
	}
	return value[:end]
}

// submitStudent makes an HTTP form request without following redirects.
// RU: submitStudent отправляет форму по HTTP без перехода по редиректу.
func submitStudent(handler http.Handler, groupID, fullName, educationalID, token string) *httptest.ResponseRecorder {
	values := url.Values{
		"full_name":      {fullName},
		"educational_id": {educationalID},
		"csrf_token":     {token},
	}
	req := httptest.NewRequest(http.MethodPost, "/groups/"+groupID+"/students", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

// TestStudentFormAppearsOnGroupPage verifies the browser's writable form.
// Scenario: open an existing academic group.
// Expected: labeled inputs, group-specific action and a nonempty CSRF token.
//
// RU: Проверяет форму добавления студента в существующую группу.
// Сценарий: открыть страницу созданной учебной группы.
// Ожидается: поля с подписями, правильный адрес и непустой CSRF-токен.
func TestStudentFormAppearsOnGroupPage(t *testing.T) {
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}},
		&fakeStudentLister{},
	)
	token := studentFormToken(t, handler, "17")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17", nil))
	for _, want := range []string{`action="/groups/17/students"`, `method="post"`, `name="full_name"`, `name="educational_id"`, "Добавить студента", token} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("form missing %q", want)
		}
	}
}

// TestStudentFormCreatesAndRedirects verifies Post/Redirect/Get.
// Scenario: submit a student's name and ID with the page's CSRF token.
// Expected: one Create call for the chosen group and a 303 back to its page.
//
// RU: Проверяет создание студента и безопасный редирект после POST.
// Сценарий: отправить ФИО, учебный ID и CSRF-токен.
// Ожидается: один вызов Create и HTTP 303 на страницу той же группы.
func TestStudentFormCreatesAndRedirects(t *testing.T) {
	store := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store,
	)
	response := submitStudent(handler, "17", "  Иванов Иван  ", " ЗК-0017 ", studentFormToken(t, handler, "17"))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/groups/17" {
		t.Fatalf("response status=%d location=%q, want 303 /groups/17", response.Code, response.Header().Get("Location"))
	}
	if store.created != 1 || store.lastGroupID != 17 || store.lastName != "  Иванов Иван  " || store.lastEducationalID != " ЗК-0017 " {
		t.Errorf("Create called incorrectly: count=%d group=%d name=%q ID=%q", store.created, store.lastGroupID, store.lastName, store.lastEducationalID)
	}
}

// TestStudentFormDisplaysValidationErrors preserves entered values and the
// current students when the repository rejects incomplete or duplicate data.
// Scenario: blank name, blank educational ID or globally duplicate student ID.
// Expected: HTTP 400/409, Russian error and escaped, retained form fields.
//
// RU: Проверяет сообщения об ошибке и сохранение введённых данных.
// Сценарий: пустое ФИО, пустой учебный ID или повтор номера зачётки.
// Ожидается: HTTP 400/409 и понятная ошибка без потери значений формы.
func TestStudentFormDisplaysValidationErrors(t *testing.T) {
	cases := []struct {
		name    string
		full    string
		eduID   string
		failure error
		code    int
		message string
	}{
		{"empty full name", "  ", "ЗК-0017", students.ErrInvalidFullName, 400, "Введите ФИО студента"},
		{"empty educational ID", "Иванов Иван", "  ", students.ErrInvalidEducationalID, 400, "Введите учебный ID студента"},
		{"duplicate educational ID", "Иванов Иван", "ЗК-0017", students.ErrDuplicateEducationalID, 409, "Студент с таким учебным ID уже существует"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStudentLister{createErr: tc.failure}
			handler := newTestHandlerWithStudents(t,
				&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
			response := submitStudent(handler, "17", tc.full, tc.eduID, studentFormToken(t, handler, "17"))
			if response.Code != tc.code {
				t.Fatalf("status=%d, want=%d; body=%q", response.Code, tc.code, response.Body.String())
			}
			body := response.Body.String()
			if !strings.Contains(body, tc.message) || !strings.Contains(body, `value="`+strings.TrimSpace(tc.eduID)+`"`) {
				t.Errorf("error or preserved educational ID missing: %q", body)
			}
			if strings.TrimSpace(tc.full) != "" && !strings.Contains(body, `value="`+tc.full+`"`) {
				t.Errorf("form lost entered name: %q", body)
			}
			if strings.Contains(response.Header().Get("Location"), "/groups/") {
				t.Errorf("validation failure redirected: %q", response.Header().Get("Location"))
			}
		})
	}
}

// TestStudentFormRejectsInvalidTokens rejects cross-site form submissions.
// Scenario: missing and invented CSRF values.
// Expected: HTTP 403 without any student insert attempt.
//
// RU: Проверяет защиту от поддельных POST-запросов.
// Сценарий: отсутствующий либо выдуманный CSRF-токен.
// Ожидается: HTTP 403 и отсутствие вызовов Create.
func TestStudentFormRejectsInvalidTokens(t *testing.T) {
	store := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
	for _, token := range []string{"", "forged-token"} {
		response := submitStudent(handler, "17", "Иванов Иван", "ЗК-0017", token)
		if response.Code != http.StatusForbidden {
			t.Errorf("token %q status=%d, want 403", token, response.Code)
		}
	}
	if store.created != 0 {
		t.Errorf("Create unexpectedly called %d times", store.created)
	}
}

// TestStudentFormRejectsInvalidGroup blocks writes to nonexistent groups.
// Scenario: submit to unknown, negative or malformed group URL with a token.
// Expected: HTTP 404 and no student inserted.
//
// RU: Проверяет отсутствие записи студента для неверной группы.
// Сценарий: POST на неизвестный или некорректный ID учебной группы.
// Ожидается: HTTP 404 и ни одного вызова Create.
func TestStudentFormRejectsInvalidGroup(t *testing.T) {
	store := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
	token := studentFormToken(t, handler, "17")
	for _, id := range []string{"999", "0", "-1", "invalid", "99999999999999999999999"} {
		response := submitStudent(handler, id, "Иванов Иван", "ЗК-0017", token)
		if response.Code != http.StatusNotFound {
			t.Errorf("invalid group %q: status=%d, want 404", id, response.Code)
		}
	}
	if store.created != 0 {
		t.Errorf("Create called %d times for invalid groups", store.created)
	}
}

// TestStudentFormRejectsOversizedAndUnsupportedBodies bounds input handling.
// Scenario: large user input or a JSON body on the form-only route.
// Expected: HTTP 413 or 415 with no database writes.
//
// RU: Проверяет ограничение размера и типа входящих данных.
// Сценарий: большая форма или тело в формате JSON.
// Ожидается: HTTP 413 или 415 без изменения SQLite.
func TestStudentFormRejectsOversizedAndUnsupportedBodies(t *testing.T) {
	store := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
	token := studentFormToken(t, handler, "17")
	large := submitStudent(handler, "17", strings.Repeat("X", 5000), "ЗК-0017", token)
	if large.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("large request status=%d, want 413", large.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/groups/17/students", strings.NewReader(`{"name":"fake"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Errorf("JSON request status=%d, want 415", response.Code)
	}
	if store.created != 0 {
		t.Errorf("Create called %d times for invalid input", store.created)
	}
}

// TestStudentFormHidesInternalErrors checks confidentiality on failures.
// Scenario: the repository returns an error containing a private DB path.
// Expected: generic HTTP 500 with no internal error in the response.
//
// RU: Проверяет скрытие конфиденциальных подробностей ошибок SQLite.
// Сценарий: хранилище возвращает ошибку с приватным путём к файлу.
// Ожидается: HTTP 500 без текста этой ошибки.
func TestStudentFormHidesInternalErrors(t *testing.T) {
	store := &fakeStudentLister{createErr: errors.New("secret SQLite database path")}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
	response := submitStudent(handler, "17", "Иванов Иван", "ЗК-0017", studentFormToken(t, handler, "17"))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret") {
		t.Errorf("storage failure status=%d body=%q", response.Code, response.Body.String())
	}
}

// TestStudentFormEscapesSubmittedValues prevents stored/requested HTML from
// executing when a validation failure re-renders the form.
// Scenario: a malicious-looking name and a duplicate educational ID.
// Expected: escaped HTML attribute value and no injected tag.
//
// RU: Проверяет экранирование значений формы при повторном отображении.
// Сценарий: ФИО содержит HTML-теги, номер зачётки уже существует.
// Ожидается: экранированное значение и отсутствие исполняемой разметки.
func TestStudentFormEscapesSubmittedValues(t *testing.T) {
	store := &fakeStudentLister{createErr: students.ErrDuplicateEducationalID}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, store)
	response := submitStudent(handler, "17", `"><script>alert(1)</script>`, "ЗК-0017", studentFormToken(t, handler, "17"))
	body := response.Body.String()
	if response.Code != http.StatusConflict || strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("submitted value not safely escaped, status=%d body=%q", response.Code, body)
	}
}

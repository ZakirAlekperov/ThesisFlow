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
)

// fakeStudentLister supplies student records or a controlled error.
// RU: fakeStudentLister возвращает заданные записи студентов либо ошибку.
type fakeStudentLister struct {
	items             []students.Student
	err               error
	createErr         error
	calls             []int64
	created           int
	lastGroupID       int64
	lastName          string
	lastEducationalID string
}

func (f *fakeStudentLister) ListByGroup(_ context.Context, id int64) ([]students.Student, error) {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return nil, f.err
	}
	var result []students.Student
	for _, student := range f.items {
		if student.GroupID == id {
			result = append(result, student)
		}
	}
	return result, nil
}

// Create tracks writes and updates the in-memory list for redirect tests.
// RU: Create запоминает попытки записи и добавляет студента в тестовый список.
func (f *fakeStudentLister) Create(_ context.Context, groupID int64, educationalID, fullName string) (students.Student, error) {
	f.created++
	f.lastGroupID = groupID
	f.lastName = fullName
	f.lastEducationalID = educationalID
	if f.createErr != nil {
		return students.Student{}, f.createErr
	}
	student := students.Student{
		ID: int64(len(f.items) + 1), GroupID: groupID,
		EducationalID: strings.TrimSpace(educationalID),
		FullName:      strings.TrimSpace(fullName),
	}
	f.items = append(f.items, student)
	return student, nil
}

// newTestHandlerWithStudents provides the group and student test repositories.
// RU: newTestHandlerWithStudents передаёт тестовые хранилища групп и студентов.
func newTestHandlerWithStudents(t *testing.T, groupStore *fakeGroupLister, studentStore *fakeStudentLister) http.Handler {
	t.Helper()
	handler, err := NewHandler(groupStore, studentStore)
	if err != nil {
		t.Fatalf("NewHandler(): %v", err)
	}
	return handler
}

// TestHomePageLinksToGroupStudents checks the navigation contract.
// Scenario: the home page lists a group with a permanent numeric ID.
// Expected: group name is a link to its read-only student page.
//
// RU: Проверяет навигацию из списка групп к студентам.
// Сценарий: главная страница содержит группу с постоянным ID.
// Ожидается: название группы ведёт на страницу её студентов.
func TestHomePageLinksToGroupStudents(t *testing.T) {
	groupsStore := &fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}
	handler := newTestHandlerWithStudents(t, groupsStore, &fakeStudentLister{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `href="/groups/17"`) {
		t.Errorf("missing group students link: %q", response.Body.String())
	}
}

// TestGroupPageShowsStudentsAndStableIDs verifies correct scoped presentation.
// Scenario: two students belong to group 17 and another student to group 29.
// Expected: only group 17's original ordered records, group heading and back link.
//
// RU: Проверяет вывод студентов только выбранной группы.
// Сценарий: два студента в группе 17 и один студент в группе 29.
// Ожидается: только записи группы 17 с исходными ID, заголовком и ссылкой назад.
func TestGroupPageShowsStudentsAndStableIDs(t *testing.T) {
	groupsStore := &fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}, {ID: 29, Name: "ПР-301"}}}
	studentStore := &fakeStudentLister{items: []students.Student{
		{ID: 12, GroupID: 17, EducationalID: "ЗК-002", FullName: "Иванов Иван"},
		{ID: 13, GroupID: 17, EducationalID: "ЗК-003", FullName: "Петров Пётр"},
		{ID: 14, GroupID: 29, EducationalID: "ЗК-004", FullName: "Студент Другой"},
	}}
	handler := newTestHandlerWithStudents(t, groupsStore, studentStore)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /groups/17 status = %d; body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{"ИСП-401", "Иванов Иван", "Петров Пётр", "ЗК-002", "ЗК-003", `data-student-id="12"`, `data-student-id="13"`, `href="/"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in student page: %q", want, body)
		}
	}
	for _, forbidden := range []string{"Студент Другой", "ЗК-004", `data-student-id="14"`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("student of another group exposed: %q", forbidden)
		}
	}
	if strings.Index(body, "Иванов Иван") >= strings.Index(body, "Петров Пётр") {
		t.Errorf("student record order changed: %q", body)
	}
	if len(studentStore.calls) != 1 || studentStore.calls[0] != 17 {
		t.Errorf("student store queried for %v, want [17]", studentStore.calls)
	}
}

// TestGroupPageShowsEmptyState checks a valid but empty group's page.
// Scenario: request an existing group without students.
// Expected: HTTP 200 with a clear message, not a 404 error.
//
// RU: Проверяет страницу существующей группы без студентов.
// Сценарий: запрошена группа, в которой нет записей.
// Ожидается: HTTP 200 и понятное сообщение об отсутствии студентов.
func TestGroupPageShowsEmptyState(t *testing.T) {
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 7, Name: "ПР-301"}}},
		&fakeStudentLister{},
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/7", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Студенты пока не добавлены") {
		t.Errorf("empty group: status=%d body=%q", response.Code, response.Body.String())
	}
}

// TestGroupPageRejectsUnknownAndInvalidIDs distinguishes bad paths from a valid
// empty group and never queries students for unknown groups.
// Scenario: nonexistent IDs, zero, negative, nonnumeric and overflowed values.
// Expected: HTTP 404; no calls to the students repository.
//
// RU: Проверяет отклонение недопустимых и несуществующих ID групп.
// Сценарий: неизвестный, нулевой, отрицательный, нечисловой и переполненный ID.
// Ожидается: HTTP 404 без запросов к таблице студентов.
func TestGroupPageRejectsUnknownAndInvalidIDs(t *testing.T) {
	studentsStore := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}},
		studentsStore,
	)
	for _, path := range []string{"/groups/999", "/groups/0", "/groups/-1", "/groups/abc", "/groups/999999999999999999999999"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s returned %d, want 404", path, response.Code)
		}
	}
	if len(studentsStore.calls) != 0 {
		t.Errorf("queried students for invalid group: %v", studentsStore.calls)
	}
}

// TestGroupPageEscapesNamesAndIDs prevents markup from becoming executable.
// Scenario: saved group name, full name and educational ID contain HTML.
// Expected: escaped text without raw script/image elements in the response.
//
// RU: Проверяет защиту от HTML-инъекций в данных группы и студентов.
// Сценарий: название, ФИО и учебный ID содержат HTML-подобные данные.
// Ожидается: текст экранирован, исполняемые теги отсутствуют.
func TestGroupPageEscapesNamesAndIDs(t *testing.T) {
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "<script>bad()</script>"}}},
		&fakeStudentLister{items: []students.Student{{ID: 1, GroupID: 17, FullName: "<img src=x onerror=alert(1)>", EducationalID: "<b>test</b>"}}},
	)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, forbidden := range []string{"<script>", "<img src=x", "<b>test</b>"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("unescaped markup %q in body", forbidden)
		}
	}
	for _, want := range []string{"&lt;script&gt;", "&lt;img", "&lt;b&gt;test&lt;/b&gt;"} {
		if !strings.Contains(body, want) {
			t.Errorf("escaped content %q missing", want)
		}
	}
}

// TestGroupPageHidesDatabaseErrors ensures private storage details do not leak.
// Scenario: group listing or student listing returns a private SQL error.
// Expected: generic HTTP 500 rather than success, empty state or SQL details.
//
// RU: Проверяет скрытие внутренних ошибок хранилища на странице студентов.
// Сценарий: ошибка при чтении групп или студентов с секретным текстом.
// Ожидается: общий HTTP 500 без утечки деталей и ложного пустого списка.
func TestGroupPageHidesDatabaseErrors(t *testing.T) {
	cases := []struct {
		name  string
		group *fakeGroupLister
		stud  *fakeStudentLister
	}{
		{"groups", &fakeGroupLister{err: errors.New("secret group SQLite path")}, &fakeStudentLister{}},
		{"students", &fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}}, &fakeStudentLister{err: errors.New("secret student SQLite path")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newTestHandlerWithStudents(t, tc.group, tc.stud)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/17", nil))
			if response.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500", response.Code)
			}
			body := response.Body.String()
			if strings.Contains(body, "secret") || strings.Contains(body, "Студенты пока не добавлены") {
				t.Errorf("database error leaked or disguised: %q", body)
			}
		})
	}
}

// TestGroupPageRejectsWriteMethods preserves the read-only view.
// Scenario: POST /groups/17 is attempted.
// Expected: HTTP 405 with no student repository call.
//
// RU: Проверяет запрет записи на странице просмотра студентов.
// Сценарий: POST на адрес /groups/17.
// Ожидается: HTTP 405; репозиторий студентов не вызывается.
func TestGroupPageRejectsWriteMethods(t *testing.T) {
	studentStore := &fakeStudentLister{}
	handler := newTestHandlerWithStudents(t,
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}},
		studentStore)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/groups/17", nil))
	if response.Code != http.StatusMethodNotAllowed || len(studentStore.calls) != 0 {
		t.Errorf("POST /groups/17: status=%d calls=%v", response.Code, studentStore.calls)
	}
}

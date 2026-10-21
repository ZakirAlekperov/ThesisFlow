package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// StudentStore provides student reads and creation for the group page.
// The production implementation uses the existing SQLite repository.
//
// RU: StudentStore читает и создаёт студентов выбранной группы.
// Рабочая реализация использует существующий SQLite-репозиторий.
type StudentStore interface {
	ListByGroup(context.Context, int64) ([]students.Student, error)
	Create(context.Context, int64, string, string) (students.Student, error)
}

// groupStudentsData supplies the escaped group detail HTML template.
// RU: groupStudentsData передаёт данные в безопасный шаблон страницы группы.
type groupStudentsData struct {
	Group         groups.Group
	Students      []students.Student
	Requirements  []requirements.Requirement
	CSRFToken     string
	FullName      string
	EducationalID string
	Error         string
}

// groupStudentsTemplate renders the student form and list without JavaScript.
// html/template escapes names, entered values and educational identifiers.
//
// RU: groupStudentsTemplate показывает форму и список без JavaScript.
// html/template экранирует ФИО, значения формы и учебные идентификаторы.
var groupStudentsTemplate = template.Must(template.New("group-students").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Студенты группы {{.Group.Name}} — ThesisFlow</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, sans-serif; background: #f6f7fb; color: #202939; }
    * { box-sizing: border-box; }
    body { margin: 0; }
    main { width: min(800px, 100% - 32px); margin: 64px auto; }
    .tabs { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 30px; border-bottom: 1px solid #dce2eb; padding-bottom: 12px; }
    .tabs a { display: inline-block; padding: 10px 16px; border-radius: 8px; text-decoration: none; color: #214eaf; }
    .back { display: inline-block; margin-bottom: 24px; color: #214eaf; text-decoration: none; }
    .back:hover { text-decoration: underline; }
    h1 { font-size: 2rem; margin-bottom: 8px; overflow-wrap: anywhere; }
    .subtitle { color: #596578; margin-bottom: 36px; line-height: 1.6; }
    .panel { background: white; border: 1px solid #e4e8f0; border-radius: 16px; padding: 24px; margin-bottom: 24px; }
    h2 { font-size: 1.25rem; margin-top: 0; }
    .student-form, .requirement-form { display: grid; gap: 12px; margin-bottom: 24px; }
    .student-form label, .requirement-form label { font-weight: 600; }
    .student-form input[type=text], .requirement-form input { width: 100%; padding: 11px 12px; border: 1px solid #bfc9d7; border-radius: 8px; font: inherit; }
    .student-form button, .requirement-form button { justify-self: start; border: 0; border-radius: 8px; background: #214eaf; color: white; font: inherit; font-weight: 600; padding: 11px 18px; cursor: pointer; }
    .student-form :focus-visible { outline: 2px solid #214eaf; outline-offset: 2px; }
    .form-error { color: #a51d2d; margin: -12px 0 20px; }
    .import-form { margin-top: 32px; padding-top: 24px; border-top: 1px solid #e4e8f0; display: grid; gap: 12px; }
    .import-form label { font-weight: 600; }
    .import-form input { padding: 10px; border: 1px solid #bfc9d7; border-radius: 8px; font: inherit; width: 100%; }
    .import-form button { justify-self: start; border: 0; border-radius: 8px; background: #214eaf; color: white; font: inherit; font-weight: 600; padding: 11px 18px; cursor: pointer; }
    .import-hint { font-size: 0.875rem; color: #687488; line-height: 1.5; }
    .student-list { list-style: none; margin: 0; padding: 0; }
    .student-list li { padding: 15px 0; border-top: 1px solid #e4e8f0; display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; }
    .student-list li:first-child { border-top: 0; }
    .student-name { font-weight: 600; overflow-wrap: anywhere; }
    .student-id, .empty { color: #687488; }
    .empty { line-height: 1.6; }
  </style>
</head>
<body>
  <main>
    <nav class="tabs" aria-label="Разделы приложения">
      <a href="/">Учебные группы</a>
      <a href="/templates">Шаблоны</a>
    </nav>
    <a class="back" href="/">← Все учебные группы</a>
    <h1>{{.Group.Name}}</h1>
    <p class="subtitle">Список студентов учебной группы.</p>
    <p><a href="/groups/{{.Group.ID}}/report">Печатная ведомость группы</a></p>
    <section class="panel" aria-labelledby="students-heading">
      <h2 id="students-heading">Студенты</h2>
      <form class="student-form" action="/groups/{{.Group.ID}}/students" method="post">
        <label for="student-full-name">ФИО студента</label>
        <input id="student-full-name" type="text" name="full_name" value="{{.FullName}}" placeholder="Иванов Иван Иванович" required autocomplete="name">
        <label for="student-educational-id">Учебный ID (номер зачётки)</label>
        <input id="student-educational-id" type="text" name="educational_id" value="{{.EducationalID}}" placeholder="ЗК-0017" required autocomplete="off">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <button type="submit">Добавить студента</button>
      </form>
      {{if .Error}}<p class="form-error" role="alert">{{.Error}}</p>{{end}}
      {{if .Students}}
      <ul class="student-list">
        {{range .Students}}
        <li data-student-id="{{.ID}}">
          <span class="student-name"><a href="/groups/{{$.Group.ID}}/students/{{.ID}}">{{.FullName}}</a></span>
          <span class="student-id">Учебный ID: {{.EducationalID}}</span>
        </li>
        {{end}}
      </ul>
      {{else}}
      <p class="empty">Студенты пока не добавлены.</p>
      {{end}}
      <form class="import-form" action="/groups/{{.Group.ID}}/import" method="post" enctype="multipart/form-data">
        <h2>Импорт из Excel</h2>
        <label for="student-xlsx">Файл со студентами (.xlsx)</label>
        <input id="student-xlsx" type="file" name="file" accept=".xlsx" required>
        <label for="xlsx-name-column">Заголовок столбца с ФИО</label>
        <input id="xlsx-name-column" type="text" name="name_header" value="ФИО" required>
        <label for="xlsx-id-column">Заголовок столбца с учебным ID</label>
        <input id="xlsx-id-column" type="text" name="id_header" value="Учебный ID" required>
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <p class="import-hint">Заголовки должны совпадать с первой строкой первого листа Excel. Если обнаружена ошибка, ни один студент из файла не будет добавлен. Максимальный размер — 10 МиБ.</p>
        <button type="submit">Импортировать студентов</button>
      </form>
    </section>
    <section class="panel" aria-labelledby="requirements-heading">
      <h2 id="requirements-heading">Обязательные документы ВКР</h2>
      <form class="requirement-form" action="/groups/{{.Group.ID}}/requirements" method="post">
        <label for="requirement-title">Название документа</label>
        <input id="requirement-title" type="text" name="title" placeholder="Глава 1" required>
        <label for="requirement-deadline">Срок сдачи (необязательно)</label>
        <input id="requirement-deadline" type="date" name="due_date">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <button type="submit">Добавить требование</button>
      </form>
      {{if .Requirements}}
      <ul class="student-list">
        {{range .Requirements}}
        <li data-requirement-id="{{.ID}}">
          <span class="student-name">{{.Title}}</span>
          {{if .DueDate}}<span class="student-id">Срок: {{.DueDate}}</span>{{end}}
        </li>
        {{end}}
      </ul>
      {{else}}
      <p class="empty">Требования к документам пока не заданы.</p>
      {{end}}
    </section>
  </main>
</body>
</html>
`))

// findGroup resolves an existing academic group by its permanent ID.
// A missing group is distinguished from a database failure.
//
// RU: findGroup находит учебную группу по постоянному ID и отличает
// отсутствие группы от ошибки чтения SQLite.
func findGroup(ctx context.Context, store GroupStore, groupID int64) (groups.Group, error) {
	allGroups, err := store.List(ctx)
	if err != nil {
		return groups.Group{}, err
	}
	for _, item := range allGroups {
		if item.ID == groupID {
			return item, nil
		}
	}
	return groups.Group{}, students.ErrGroupNotFound
}

// parseGroupPathID validates the numeric group ID from the HTTP route.
// RU: parseGroupPathID проверяет числовой ID из адреса группы.
func parseGroupPathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, students.ErrGroupNotFound
	}
	return id, nil
}

// renderGroupStudents displays the selected group's students and any form
// validation error. Unknown groups return 404; storage failures return 500.
//
// RU: renderGroupStudents показывает студентов и ошибку заполнения формы.
// Несуществующие группы дают 404; ошибки SQLite дают общий HTTP 500.
func renderGroupStudents(w http.ResponseWriter, r *http.Request, groupStore GroupStore, studentStore StudentStore, token, name, educationalID, message string, status int, requirementStores ...RequirementStore) {
	groupID, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	group, err := findGroup(r.Context(), groupStore, groupID)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось загрузить учебную группу.", http.StatusInternalServerError)
		}
		return
	}
	items, err := studentStore.ListByGroup(r.Context(), groupID)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось загрузить список студентов.", http.StatusInternalServerError)
		}
		return
	}
	var required []requirements.Requirement
	if len(requirementStores) > 0 && requirementStores[0] != nil {
		required, err = requirementStores[0].ListForGroup(r.Context(), groupID)
		if err != nil {
			http.Error(w, "Не удалось загрузить требования к документам.", http.StatusInternalServerError)
			return
		}
	}
	var rendered bytes.Buffer
	if err := groupStudentsTemplate.Execute(&rendered, groupStudentsData{
		Group: group, Students: items, Requirements: required, CSRFToken: token,
		FullName: name, EducationalID: educationalID, Error: message,
	}); err != nil {
		http.Error(w, "Не удалось отобразить страницу группы.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(rendered.Bytes())
}

// createStudentFromForm validates an HTTP form before writing a student.
// It uses the same per-process CSRF token as the existing group form and
// redirects after a successful insert to avoid duplicate submissions.
//
// RU: createStudentFromForm проверяет форму до записи в SQLite, использует
// общий CSRF-токен приложения и перенаправляет после успешного создания.
func createStudentFromForm(w http.ResponseWriter, r *http.Request, groupStore GroupStore, studentStore StudentStore, token string, requirementStores ...RequirementStore) {
	groupID, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "Неподдерживаемый формат формы.", http.StatusUnsupportedMediaType)
		return
	}
	// Limit request bodies and reject data that exceeds the form's capacity.
	// RU: Ограничиваем тело запроса и отклоняем слишком большие формы.
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Размер формы превышен.", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Не удалось прочитать форму.", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return
	}
	if _, err := findGroup(r.Context(), groupStore, groupID); err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось загрузить учебную группу.", http.StatusInternalServerError)
		}
		return
	}

	fullName := r.PostForm.Get("full_name")
	educationalID := r.PostForm.Get("educational_id")
	_, err = studentStore.Create(r.Context(), groupID, educationalID, fullName)
	if err != nil {
		message := ""
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, students.ErrInvalidFullName):
			message = "Введите ФИО студента."
		case errors.Is(err, students.ErrInvalidEducationalID):
			message = "Введите учебный ID студента."
		case errors.Is(err, students.ErrDuplicateEducationalID):
			message = "Студент с таким учебным ID уже существует."
			status = http.StatusConflict
		case errors.Is(err, students.ErrGroupNotFound):
			http.NotFound(w, r)
			return
		default:
			http.Error(w, "Не удалось добавить студента.", http.StatusInternalServerError)
			return
		}
		renderGroupStudents(w, r, groupStore, studentStore, token, fullName, strings.TrimSpace(educationalID), message, status, requirementStores...)
		return
	}
	http.Redirect(w, r, "/groups/"+strconv.FormatInt(groupID, 10), http.StatusSeeOther)
}

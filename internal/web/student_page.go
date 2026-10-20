package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// StudentLister supplies the read-only student records for a selected group.
// The production implementation is backed by SQLite.
//
// RU: StudentLister читает студентов выбранной группы без права изменения.
// В рабочем приложении данные поступают из SQLite.
type StudentLister interface {
	ListByGroup(context.Context, int64) ([]students.Student, error)
}

// groupStudentsData supplies the escaped group detail HTML template.
// RU: groupStudentsData передаёт данные в безопасный шаблон страницы группы.
type groupStudentsData struct {
	Group    groups.Group
	Students []students.Student
}

// groupStudentsTemplate is a read-only page requiring no client JavaScript.
// html/template automatically escapes names and educational identifiers.
//
// RU: groupStudentsTemplate — страница просмотра без JavaScript.
// html/template автоматически экранирует ФИО и учебные идентификаторы.
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
    .back { display: inline-block; margin-bottom: 24px; color: #214eaf; text-decoration: none; }
    .back:hover { text-decoration: underline; }
    h1 { font-size: 2rem; margin-bottom: 8px; overflow-wrap: anywhere; }
    .subtitle { color: #596578; margin-bottom: 36px; line-height: 1.6; }
    .panel { background: white; border: 1px solid #e4e8f0; border-radius: 16px; padding: 24px; }
    h2 { font-size: 1.25rem; margin-top: 0; }
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
    <a class="back" href="/">← Все учебные группы</a>
    <h1>{{.Group.Name}}</h1>
    <p class="subtitle">Список студентов учебной группы.</p>
    <section class="panel" aria-labelledby="students-heading">
      <h2 id="students-heading">Студенты</h2>
      {{if .Students}}
      <ul class="student-list">
        {{range .Students}}
        <li data-student-id="{{.ID}}">
          <span class="student-name">{{.FullName}}</span>
          <span class="student-id">Учебный ID: {{.EducationalID}}</span>
        </li>
        {{end}}
      </ul>
      {{else}}
      <p class="empty">Студенты пока не добавлены.</p>
      {{end}}
    </section>
  </main>
</body>
</html>
`))

// renderGroupStudents validates the URL, resolves a group by its permanent ID,
// and displays only that group's students. Invalid/missing groups yield 404;
// storage failures yield a generic 500 without leaking internal details.
//
// RU: renderGroupStudents проверяет ID, находит группу и показывает только
// её студентов. Для отсутствующих групп возвращается 404, при ошибках БД —
// общее сообщение 500 без раскрытия внутренних подробностей.
func renderGroupStudents(w http.ResponseWriter, r *http.Request, groupStore GroupStore, studentStore StudentLister) {
	groupID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || groupID <= 0 {
		http.NotFound(w, r)
		return
	}

	allGroups, err := groupStore.List(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить учебную группу.", http.StatusInternalServerError)
		return
	}
	var selected groups.Group
	for _, item := range allGroups {
		if item.ID == groupID {
			selected = item
			break
		}
	}
	if selected.ID == 0 {
		http.NotFound(w, r)
		return
	}

	items, err := studentStore.ListByGroup(r.Context(), groupID)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Не удалось загрузить список студентов.", http.StatusInternalServerError)
		return
	}

	var rendered bytes.Buffer
	if err := groupStudentsTemplate.Execute(&rendered, groupStudentsData{Group: selected, Students: items}); err != nil {
		http.Error(w, "Не удалось отобразить страницу группы.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(rendered.Bytes())
}

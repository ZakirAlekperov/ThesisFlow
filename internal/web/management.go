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

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/management"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// RecordManager is the small service contract for reversible record changes.
// RU: RecordManager описывает операции исправления, архивирования и возврата.
type RecordManager interface {
	RenameGroup(context.Context, int64, string) error
	SetGroupArchived(context.Context, int64, bool) error
	UpdateStudent(context.Context, int64, string, string) error
	MoveStudent(context.Context, int64, int64) error
	SetStudentArchived(context.Context, int64, bool) error
	ListArchivedGroups(context.Context) ([]groups.Group, error)
	ListArchivedStudents(context.Context) ([]students.Student, error)
}

type managedGroup struct {
	Group    groups.Group
	Students []students.Student
}

type managementData struct {
	Groups           []managedGroup
	ArchivedGroups   []groups.Group
	ArchivedStudents []students.Student
	Token            string
}

// managementTemplate deliberately uses standard Go html/template and forms,
// not React, a JavaScript build chain, or client-side data persistence.
// RU: managementTemplate использует штатные html/template и HTML-формы Go.
var managementTemplate = template.Must(template.New("management").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Управление данными — ThesisFlow</title>
<style>:root{font-family:system-ui,sans-serif;color:#202939;background:#f6f7fb}body{max-width:1050px;margin:32px auto;padding:0 20px}.panel{background:white;border:1px solid #dfe5ee;border-radius:12px;padding:20px;margin:16px 0}h1{margin:10px 0}form{display:inline-flex;flex-wrap:wrap;gap:8px;margin:6px 4px 6px 0}input,select,button{padding:9px;border:1px solid #cad3df;border-radius:7px;font:inherit}button{background:#214eaf;color:white;cursor:pointer}a{color:#214eaf}.row{border-top:1px solid #e4e8f0;padding:14px 0}.muted{color:#637084;font-size:.92rem}</style><link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
</head><body><nav><a href="/">← Группы</a> · <a href="/templates">Шаблоны</a></nav>
<h1>Управление группами и студентами</h1>
<p class="muted">Изменения выполняются без удаления записей. Изменение имени/группы при наличии принятых документов запрещено до безопасного переноса их файлов.</p>
{{range .Groups}}<section class="panel"><h2>{{.Group.Name}}</h2>
<form method="post" action="/manage/groups/{{.Group.ID}}/rename"><input type="hidden" name="csrf_token" value="{{$.Token}}"><input aria-label="Новое название группы" name="name" value="{{.Group.Name}}" required><button>Переименовать группу</button></form>
<form method="post" action="/manage/groups/{{.Group.ID}}/archive"><input type="hidden" name="csrf_token" value="{{$.Token}}"><button>В архив</button></form>
{{range .Students}}<div class="row"><strong>{{.FullName}}</strong> <span class="muted">{{.EducationalID}}</span>
<form method="post" action="/manage/students/{{.ID}}/update"><input type="hidden" name="csrf_token" value="{{$.Token}}"><input name="full_name" aria-label="ФИО" value="{{.FullName}}" required><input name="educational_id" aria-label="Учебный ID" value="{{.EducationalID}}" required><button>Сохранить студента</button></form>
<form method="post" action="/manage/students/{{.ID}}/move"><input type="hidden" name="csrf_token" value="{{$.Token}}"><select name="group_id" aria-label="Перевести в группу">{{range $.Groups}}<option value="{{.Group.ID}}">{{.Group.Name}}</option>{{end}}</select><button>Перевести</button></form>
<form method="post" action="/manage/students/{{.ID}}/archive"><input type="hidden" name="csrf_token" value="{{$.Token}}"><button>Архивировать студента</button></form></div>{{end}}
</section>{{end}}
<section class="panel"><h2>Архив групп</h2>{{range .ArchivedGroups}}
<div class="row">{{.Name}} <form method="post" action="/manage/groups/{{.ID}}/restore"><input type="hidden" name="csrf_token" value="{{$.Token}}"><button>Восстановить группу</button></form></div>
{{else}}<p class="muted">Архив пуст.</p>{{end}}</section>
<section class="panel"><h2>Архив студентов</h2>{{range .ArchivedStudents}}
<div class="row">{{.FullName}} · {{.EducationalID}}
<form method="post" action="/manage/students/{{.ID}}/restore"><input type="hidden" name="csrf_token" value="{{$.Token}}"><button>Восстановить студента</button></form></div>
{{else}}<p class="muted">Архив пуст.</p>{{end}}</section>
</body></html>`))

// renderManagement loads current and archived records and writes escaped HTML.
// RU: renderManagement строит безопасную страницу активных/архивных записей.
func renderManagement(w http.ResponseWriter, r *http.Request, gr GroupStore, st StudentStore, manager RecordManager, token string) {
	groupsList, err := gr.List(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить группы.", 500)
		return
	}
	data := managementData{Token: token}
	for _, group := range groupsList {
		people, err := st.ListByGroup(r.Context(), group.ID)
		if err != nil {
			http.Error(w, "Не удалось загрузить студентов.", 500)
			return
		}
		data.Groups = append(data.Groups, managedGroup{Group: group, Students: people})
	}
	data.ArchivedGroups, err = manager.ListArchivedGroups(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить архив групп.", 500)
		return
	}
	data.ArchivedStudents, err = manager.ListArchivedStudents(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить архив студентов.", 500)
		return
	}
	var body bytes.Buffer
	if err := managementTemplate.Execute(&body, data); err != nil {
		http.Error(w, "Ошибка отображения.", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body.Bytes())
}

// applyManagementAction checks the app CSRF token and dispatches only known
// updates. Invalid and conflicting operations return typed HTTP errors.
// RU: applyManagementAction проверяет CSRF и выполняет только разрешённые действия.
func applyManagementAction(w http.ResponseWriter, r *http.Request, manager RecordManager, token, kind string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/x-www-form-urlencoded" {
		http.Error(w, "Неверный формат формы.", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Некорректная форма.", 400)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", 403)
		return
	}
	action := r.PathValue("action")
	switch kind {
	case "groups":
		switch action {
		case "rename":
			err = manager.RenameGroup(r.Context(), id, r.PostForm.Get("name"))
		case "archive":
			err = manager.SetGroupArchived(r.Context(), id, true)
		case "restore":
			err = manager.SetGroupArchived(r.Context(), id, false)
		default:
			http.NotFound(w, r)
			return
		}
	case "students":
		switch action {
		case "update":
			err = manager.UpdateStudent(r.Context(), id, r.PostForm.Get("educational_id"), r.PostForm.Get("full_name"))
		case "move":
			target, parseErr := strconv.ParseInt(r.PostForm.Get("group_id"), 10, 64)
			if parseErr != nil {
				http.Error(w, "Неверная группа.", 400)
				return
			}
			err = manager.MoveStudent(r.Context(), id, target)
		case "archive":
			err = manager.SetStudentArchived(r.Context(), id, true)
		case "restore":
			err = manager.SetStudentArchived(r.Context(), id, false)
		default:
			http.NotFound(w, r)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, management.ErrAcceptedFiles):
			http.Error(w, "У записи есть принятые файлы: сначала нужен безопасный перенос документов.", 409)
		case errors.Is(err, management.ErrDuplicate):
			http.Error(w, "Такое название или учебный номер уже существует.", 409)
		case errors.Is(err, management.ErrInvalid):
			http.Error(w, "Проверьте введённые значения.", 400)
		case errors.Is(err, management.ErrNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, "Не удалось изменить запись.", 500)
		}
		return
	}
	http.Redirect(w, r, "/manage", http.StatusSeeOther)
}

package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/workspaceselect"
)

// WorkspaceSelection is the minimal read/write contract for choosing a
// verified workspace on the next application launch.
// RU: Изменение не заменяет БД работающего сервера.
type WorkspaceSelection interface {
	List(context.Context) ([]workspaceselect.Option, error)
	Current() (string, error)
	Choose(context.Context, string) error
}

type workspaceChoice struct {
	ID       string
	Modified string
	Selected bool
}
type workspaceSelectionData struct {
	CSRFToken       string
	Options         []workspaceChoice
	DefaultSelected bool
	Message         string
}

// selectionTemplate provides an offline, accessible 3-stage recovery wizard
// without JavaScript. HTML/template escapes all displayed names and tokens.
// RU: Выбор восстановленного каталога подтверждается в отдельной форме.
var selectionTemplate = template.Must(template.New("workspace-choice").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Рабочие пространства — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script defer src="/static/theme.js"></script>
<style>
body{font:16px/1.5 system-ui,sans-serif;max-width:920px;margin:30px auto;padding:0 18px 45px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);padding:20px;border-radius:12px;margin:16px 0}
label{display:block;padding:12px 0;border-bottom:1px solid var(--tf-border);overflow-wrap:anywhere}
label:last-child{border:0}.actions{display:flex;gap:14px;flex-wrap:wrap;align-items:center}
button{font:inherit;padding:10px 16px;border-radius:8px}
small{color:var(--tf-muted);display:block}.important{font-weight:600}
input[type=radio]{accent-color:var(--tf-button);margin-right:10px}
@media print{.actions{display:none}}
</style></head><body>
<nav><a href="/settings">← Настройки</a> · <a href="/settings/health">Диагностика</a></nav>
<h1>Восстановление и выбор рабочего пространства</h1>
<p>Выбор подготовит данные только для следующего запуска программы. Текущий браузерный сеанс и открытая SQLite продолжат работать без изменений.</p>
<section class="panel"><h2>Шаг 1. Создайте проверенную копию</h2>
<p>Откройте <a href="/settings">Настройки → Восстановление из ZIP</a>, загрузите архив и дождитесь сообщения об успешном восстановлении. Созданная папка появится ниже.</p></section>
<section class="panel"><h2>Шаг 2. Выберите данные следующего запуска</h2>
{{if .Message}}<p role="status" class="important">{{.Message}}</p>{{end}}
<form method="post" action="/settings/workspaces">
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
<label><input type="radio" name="workspace" value="default" required {{if .DefaultSelected}}checked{{end}}>
Основное рабочее пространство <small>Стандартные данные приложения</small></label>
{{range .Options}}
<label><input type="radio" name="workspace" value="{{.ID}}" {{if .Selected}}checked{{end}}>
Восстановленная копия {{.ID}} <small>Создана: {{.Modified}}{{if .Selected}} · Выбрана для следующего запуска{{end}}</small></label>
{{else}}<p>Восстановленных копий ещё нет. Сначала загрузите ZIP в разделе настроек.</p>{{end}}
<div class="actions"><button type="submit">Проверить и выбрать для следующего запуска</button></div>
</form></section>
<section class="panel"><h2>Шаг 3. Закройте и запустите ThesisFlow снова</h2>
<p class="important">Не заменяйте и не удаляйте вручную текущую базу. Закройте приложение обычным способом и запустите его повторно. Новая папка проверяется ещё раз до открытия интерфейса.</p>
<p>При повреждении выбранной копии программа сообщит об ошибке и не переключится молча на пустую БД. Вернуться к основному пространству можно, выбрав его на этой странице до перезапуска.</p>
</section></body></html>`))

func showWorkspaceSelection(w http.ResponseWriter, r *http.Request, selector WorkspaceSelection, token, message string) {
	chosen, err := selector.Current()
	if err != nil {
		http.Error(w, "Не удалось прочитать сохранённый выбор рабочего пространства.", http.StatusInternalServerError)
		return
	}
	options, err := selector.List(r.Context())
	if err != nil {
		http.Error(w, "Не удалось получить список восстановленных копий.", http.StatusInternalServerError)
		return
	}
	data := workspaceSelectionData{CSRFToken: token, DefaultSelected: chosen == workspaceselect.Default, Message: message}
	for _, option := range options {
		data.Options = append(data.Options, workspaceChoice{
			ID: option.ID, Modified: option.Created.Local().Format("02.01.2006 15:04"),
			Selected: option.ID == chosen,
		})
	}
	var page bytes.Buffer
	if err := selectionTemplate.Execute(&page, data); err != nil {
		http.Error(w, "Ошибка формирования страницы.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(page.Bytes())
}

// saveWorkspaceSelection validates a bounded form and CSRF. It does not
// restart the server and never changes the current SQLite connection.
// RU: Изменение выбора атомарно, активная БД не перезаписывается.
func saveWorkspaceSelection(w http.ResponseWriter, r *http.Request, selector WorkspaceSelection, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неверная форма.", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return
	}
	if len(r.PostForm["workspace"]) != 1 {
		http.Error(w, "Выберите одно рабочее пространство.", http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(r.PostForm.Get("workspace"))
	if err := selector.Choose(r.Context(), id); err != nil {
		if errors.Is(err, workspaceselect.ErrInvalidChoice) {
			http.Error(w, "Рабочее пространство не прошло проверку. Исходные данные не изменены.", http.StatusBadRequest)
		} else {
			http.Error(w, "Не удалось сохранить выбор рабочего пространства.", http.StatusInternalServerError)
		}
		return
	}
	showWorkspaceSelection(w, r, selector, token, "Выбор сохранён. Закройте программу и запустите её снова.")
}

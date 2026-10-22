package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/schedule"
)

type ScheduleManager interface {
	Calendar(context.Context, int64) ([]schedule.CalendarEntry, error)
	GroupDeadlines(context.Context, int64) ([]schedule.Deadline, error)
	Preview(context.Context, int64, map[int64]string) ([]schedule.Deadline, string, error)
	ApplyDeadlines(context.Context, int64, map[int64]string, string) error
	SetOverride(context.Context, int64, int64, int64, string, string) error
}
type calendarView struct {
	Rows     []schedule.CalendarEntry
	CSRF     string
	GroupID  int64
	Category string
}

var calendarTemplate = template.Must(template.New("calendar").Parse(`<!doctype html><html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Календарь ВКР — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css">
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:1250px;margin:25px auto;padding:0 15px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px;padding:18px;margin:16px 0}
table{border-collapse:collapse;width:100%;min-width:930px}td,th{padding:10px;text-align:left;border-bottom:1px solid var(--tf-border)}
.scroll{overflow-x:auto}select,input,button{font:inherit;padding:7px}form.inline{display:flex;gap:6px;flex-wrap:wrap}
</style></head><body><nav><a href="/">← Группы</a> · <a href="/profiles">Профили требований</a></nav>
<h1>Календарь документов ВКР</h1><p>Только актуальные сроки и статусы активных студентов. Пересчёт выполняется при каждом открытии без фоновой службы.</p>
<section class="panel"><form method="get" action="/calendar">
<label>Группа ID (0 — все) <input type="number" min="0" name="group" value="{{.GroupID}}"></label>
<label>Показать <select name="category">
<option value="">Все</option><option value="просрочено">Просрочено</option><option value="сегодня">Сегодня</option>
<option value="7 дней">Следующие 7 дней</option><option value="позже">Позже</option>
<option value="без срока">Без срока</option><option value="принято">Принято</option></select></label>
<button>Фильтровать</button></form></section>
<section class="panel scroll"><table><thead><tr><th>Группа</th><th>Студент</th><th>Документ</th><th>Статус</th><th>Срок</th><th>Категория</th><th>Личный срок</th></tr></thead>
<tbody>{{range .Rows}}<tr><td><a href="/groups/{{.GroupID}}/deadlines">{{.GroupName}}</a></td>
<td><a href="/groups/{{.GroupID}}/students/{{.StudentID}}">{{.StudentName}}</a></td><td>{{.Title}}</td><td>{{.Status}}</td>
<td>{{if .DueDate}}{{.DueDate}}{{else}}—{{end}}</td><td>{{.Category}}{{if gt .OverdueDays 0}} · {{.OverdueDays}} дн.{{end}}</td>
<td><form class="inline" method="post" action="/calendar/override">
<input type="hidden" name="csrf_token" value="{{$.CSRF}}">
<input type="hidden" name="group" value="{{.GroupID}}"><input type="hidden" name="student" value="{{.StudentID}}">
<input type="hidden" name="requirement" value="{{.RequirementID}}">
<select name="kind" aria-label="Исключение для студента"><option value="inherit">Наследовать</option>
<option value="date">Личная дата</option><option value="none">Без срока</option></select>
<input type="date" name="date" aria-label="Индивидуальная дата">
<button>Применить</button></form></td>
</tr>{{else}}<tr><td colspan="7">Нет документов для указанных условий.</td></tr>{{end}}</tbody></table></section>
<p>Изменение срока не меняет системную дату сдачи документов. История сроков хранится отдельно от истории проверки.</p></body></html>`))

type deadlineView struct {
	GroupID int64
	CSRF    string
	Rows    []schedule.Deadline
	Token   string
	Preview bool
}

var deadlineTemplate = template.Must(template.New("deadlines").Parse(`<!doctype html><html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Сроки группы — ThesisFlow</title><link rel="stylesheet" href="/static/theme.css">
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:1000px;margin:25px auto;padding:0 16px}
table{width:100%;border-collapse:collapse}td,th{text-align:left;padding:12px;border-bottom:1px solid var(--tf-border)}
.panel{padding:20px;background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px}
button,input{font:inherit;padding:9px}</style></head><body>
<a href="/groups/{{.GroupID}}">← Группа</a><h1>Групповые сроки сдачи</h1>
<p>Изменения затронут всех студентов, кроме тех, кому назначен индивидуальный срок. Статусы и дата регистрации сдачи останутся прежними.</p>
<section class="panel"><form action="/groups/{{.GroupID}}/deadlines/{{if .Preview}}commit{{else}}preview{{end}}" method="post">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
{{if .Preview}}<input type="hidden" name="preview_token" value="{{.Token}}">{{end}}
<table><thead><tr><th>Документ</th><th>Предыдущая дата</th><th>Новая дата</th></tr></thead><tbody>
{{range .Rows}}<tr><td>{{.Title}}</td><td>{{if .Previous}}{{.Previous}}{{else}}—{{end}}</td>
<td>{{if $.Preview}}<strong>{{.Next}}</strong><input type="hidden" name="due-{{.RequirementID}}" value="{{.Next}}">
{{else}}<input type="date" name="due-{{.RequirementID}}" value="{{.Next}}">{{end}}</td></tr>{{end}}</tbody></table>
{{if .Preview}}<p>Проверьте обновлённые даты. Изменения будут внесены единой транзакцией.</p>
<button>Подтвердить изменения сроков</button>
{{else}}<button>Предпросмотр изменений</button>{{end}}</form></section></body></html>`))

func parseGroupDueForm(r *http.Request) (map[int64]string, error) {
	out := map[int64]string{}
	for k, v := range r.PostForm {
		if !strings.HasPrefix(k, "due-") {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(k, "due-"), 10, 64)
		if err != nil || id <= 0 || len(v) != 1 {
			return nil, schedule.ErrInvalid
		}
		out[id] = v[0]
	}
	return out, nil
}
func showCalendar(w http.ResponseWriter, r *http.Request, s ScheduleManager, csrf string) {
	group := int64(0)
	if id := r.URL.Query().Get("group"); id != "" {
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil || v < 0 {
			http.Error(w, "Неверная группа.", 400)
			return
		}
		group = v
	}
	category := r.URL.Query().Get("category")
	allowed := map[string]bool{"": true, "просрочено": true, "сегодня": true, "7 дней": true, "позже": true, "без срока": true, "принято": true}
	if !allowed[category] {
		http.Error(w, "Неверный фильтр.", 400)
		return
	}
	list, err := s.Calendar(r.Context(), group)
	if err != nil {
		http.Error(w, "Ошибка календаря.", 500)
		return
	}
	filtered := make([]schedule.CalendarEntry, 0)
	for _, x := range list {
		if category == "" || x.Category == category {
			filtered = append(filtered, x)
		}
	}
	renderPage(w, calendarTemplate, calendarView{Rows: filtered, CSRF: csrf, GroupID: group, Category: category})
}
func groupDeadlines(w http.ResponseWriter, r *http.Request, s ScheduleManager, csrf string) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rows, err := s.GroupDeadlines(r.Context(), id)
	if err != nil {
		http.Error(w, "Ошибка сроков.", 500)
		return
	}
	renderPage(w, deadlineTemplate, deadlineView{GroupID: id, CSRF: csrf, Rows: rows})
}
func previewDeadlines(w http.ResponseWriter, r *http.Request, s ScheduleManager, csrf string) {
	if !formToken(w, r, csrf) {
		return
	}
	group, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	values, err := parseGroupDueForm(r)
	if err != nil {
		http.Error(w, "Ошибка дат.", 400)
		return
	}
	rows, token, err := s.Preview(r.Context(), group, values)
	if err != nil {
		http.Error(w, "Некорректные сроки.", 400)
		return
	}
	renderPage(w, deadlineTemplate, deadlineView{GroupID: group, CSRF: csrf, Rows: rows, Token: token, Preview: true})
}
func commitDeadlines(w http.ResponseWriter, r *http.Request, s ScheduleManager, csrf string) {
	if !formToken(w, r, csrf) {
		return
	}
	group, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	dates, err := parseGroupDueForm(r)
	if err != nil {
		http.Error(w, "Ошибка дат.", 400)
		return
	}
	if err := s.ApplyDeadlines(r.Context(), group, dates, r.PostForm.Get("preview_token")); err != nil {
		if err == schedule.ErrStale {
			http.Error(w, "Сроки изменились: начните с нового предпросмотра.", 409)
			return
		}
		http.Error(w, "Не удалось изменить сроки.", 400)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d/deadlines", group), 303)
}
func applyIndividualDeadline(w http.ResponseWriter, r *http.Request, s ScheduleManager, csrf string) {
	if !formToken(w, r, csrf) {
		return
	}
	vals := []string{r.PostForm.Get("group"), r.PostForm.Get("student"), r.PostForm.Get("requirement")}
	ids := make([]int64, 3)
	for i, v := range vals {
		n, err := parseID(v)
		if err != nil {
			http.Error(w, "Неверная область действия.", 400)
			return
		}
		ids[i] = n
	}
	if err := s.SetOverride(r.Context(), ids[0], ids[1], ids[2], r.PostForm.Get("kind"), r.PostForm.Get("date")); err != nil {
		http.Error(w, "Невозможно назначить индивидуальный срок.", 400)
		return
	}
	http.Redirect(w, r, "/calendar?"+url.Values{"group": {fmt.Sprint(ids[0])}}.Encode(), 303)
}

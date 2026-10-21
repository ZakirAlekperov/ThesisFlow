package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/readiness"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

//go:embed static/theme.css static/theme.js
var localAssets embed.FS

// serveLocalAsset delivers only known CSS and JS embedded in the Go binary.
// RU: serveLocalAsset отдаёт собственные CSS/JS без стороннего сервера.
func serveLocalAsset(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("name")
	if filename != "theme.css" && filename != "theme.js" {
		http.NotFound(w, r)
		return
	}
	body, err := localAssets.ReadFile("static/" + filename)
	if err != nil {
		http.Error(w, "Asset is unavailable.", http.StatusInternalServerError)
		return
	}
	if filename == "theme.css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	_, _ = w.Write(body)
}

type dashboardGroup struct {
	Group   groups.Group
	Summary readiness.Summary
}

type dashboardData struct {
	Groups []dashboardGroup
	TotalStudents int
	Required int
	Accepted int
	Overdue int
	Percent int
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Панель преподавателя — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px system-ui,sans-serif;max-width:1040px;margin:35px auto;padding:0 20px}.links{display:flex;gap:14px;flex-wrap:wrap;margin:20px 0}.metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(155px,1fr));gap:12px}.panel{background:white;border:1px solid #dce3ed;border-radius:12px;padding:18px;margin:12px 0}.value{font-size:2rem;font-weight:700}progress{width:100%;height:16px}a{color:#214eaf}</style>
</head><body><nav class="links"><a href="/">Группы</a><a href="/search">Поиск</a><a href="/manage">Управление</a><a href="/templates">Шаблоны</a></nav>
<h1>Панель преподавателя</h1>
<section class="metrics" aria-label="Сводка по ВКР">
<div class="panel">Студентов<div class="value">{{.TotalStudents}}</div></div>
<div class="panel">Документов требуется<div class="value">{{.Required}}</div></div>
<div class="panel">Принято<div class="value">{{.Accepted}}</div></div>
<div class="panel">Просрочено<div class="value">{{.Overdue}}</div></div>
<div class="panel">Общая готовность<div class="value">{{.Percent}}%</div></div></section>
<h2>Готовность групп</h2>
{{range .Groups}}<section class="panel">
<a href="/groups/{{.Group.ID}}"><strong>{{.Group.Name}}</strong></a>
<p>Студентов: {{.Summary.StudentCount}} · Принято: {{.Summary.Accepted}} / {{.Summary.Required}} · Просрочено: {{.Summary.Overdue}}</p>
<label>Готовность {{.Summary.Percent}}% <progress max="100" value="{{.Summary.Percent}}">{{.Summary.Percent}}%</progress></label>
</section>{{else}}<p>Учебные группы ещё не добавлены.</p>{{end}}
</body></html>`))

// renderDashboard recomputes summary numbers from current group/material state.
// It never stores derived percentages in SQLite.
// RU: renderDashboard пересчитывает готовность без дублирования данных в БД.
func renderDashboard(w http.ResponseWriter, r *http.Request, store GroupStore, report ReportProvider) {
	items, err := store.List(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить группы.", http.StatusInternalServerError)
		return
	}
	data := dashboardData{}
	for _, group := range items {
		summary, err := report.Group(r.Context(), group.ID, time.Now())
		if err != nil {
			http.Error(w, "Не удалось рассчитать готовность.", http.StatusInternalServerError)
			return
		}
		data.Groups = append(data.Groups, dashboardGroup{Group: group, Summary: summary})
		data.TotalStudents += summary.StudentCount
		data.Required += summary.Required
		data.Accepted += summary.Accepted
		data.Overdue += summary.Overdue
	}
	if data.Required > 0 {
		data.Percent = data.Accepted * 100 / data.Required
	}
	var output bytes.Buffer
	if err := dashboardTemplate.Execute(&output, data); err != nil {
		http.Error(w, "Не удалось отобразить панель.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(output.Bytes())
}

type matchedStudent struct {
	Student students.Student
	Group groups.Group
}

type searchData struct {
	Query string
	Results []matchedStudent
}

var searchTemplate = template.Must(template.New("search").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Поиск студентов — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px system-ui,sans-serif;max-width:850px;margin:35px auto;padding:0 20px}.panel{background:white;border:1px solid #dce3ed;padding:20px;margin:15px 0;border-radius:12px}input,button{font:inherit;padding:10px;border-radius:8px;border:1px solid #abb9c9}button{background:#214eaf;color:white}</style></head><body>
<nav><a href="/dashboard">← Панель преподавателя</a></nav><h1>Поиск студентов</h1>
<form method="get" action="/search"><label>ФИО, учебный номер или группа <input name="q" value="{{.Query}}" maxlength="100" autofocus></label><button>Найти</button></form>
{{if .Query}}<h2>Результаты поиска</h2>{{range .Results}}
<div class="panel"><a href="/groups/{{.Group.ID}}/students/{{.Student.ID}}">{{.Student.FullName}}</a>
<p>Учебный ID: {{.Student.EducationalID}} · Группа: {{.Group.Name}}</p></div>
{{else}}<p>Совпадений не найдено.</p>{{end}}{{end}}
</body></html>`))

// renderStudentSearch performs bounded case-insensitive search through active
// students. Display is escaped by html/template rather than client HTML.
// RU: renderStudentSearch ищет активных студентов и безопасно выводит ФИО.
func renderStudentSearch(w http.ResponseWriter, r *http.Request, groupsStore GroupStore, people StudentStore) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(query)) > 100 {
		http.Error(w, "Поисковый запрос слишком длинный.", http.StatusBadRequest)
		return
	}
	data := searchData{Query: query}
	if query != "" {
		items, err := groupsStore.List(r.Context())
		if err != nil {
			http.Error(w, "Не удалось выполнить поиск.", http.StatusInternalServerError)
			return
		}
		needle := strings.ToLower(query)
		for _, group := range items {
			studentsList, err := people.ListByGroup(r.Context(), group.ID)
			if err != nil {
				http.Error(w, "Не удалось выполнить поиск.", http.StatusInternalServerError)
				return
			}
			for _, student := range studentsList {
				haystack := strings.ToLower(student.FullName + " " + student.EducationalID + " " + group.Name)
				if strings.Contains(haystack, needle) && len(data.Results) < 100 {
					data.Results = append(data.Results, matchedStudent{Student: student, Group: group})
				}
			}
		}
	}
	var output bytes.Buffer
	if err := searchTemplate.Execute(&output, data); err != nil {
		http.Error(w, "Не удалось отобразить поиск.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(output.Bytes())
}


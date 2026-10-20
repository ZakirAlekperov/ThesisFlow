// Package web provides HTTP handlers for the local ThesisFlow interface.
// Пакет web содержит HTTP-обработчики локального интерфейса ThesisFlow.
package web

import (
	"bytes"
	"context"
	"html/template"
	"net/http"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
)

// GroupLister is the read-only interface required by the home page.
// A narrow interface lets tests use a fake and production use SQLite.
//
// RU: GroupLister — минимальный интерфейс чтения групп для главной страницы.
// В тестах используется имитатор, в приложении — хранилище SQLite.
type GroupLister interface {
	List(context.Context) ([]groups.Group, error)
}

type homeData struct {
	Groups []groups.Group
}

// homePageTemplate escapes names from SQLite before including them in HTML.
// RU: homePageTemplate автоматически экранирует названия групп из SQLite.
var homePageTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ThesisFlow</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, sans-serif; background: #f6f7fb; color: #202939; }
    * { box-sizing: border-box; }
    body { margin: 0; }
    main { width: min(800px, 100% - 32px); margin: 64px auto; }
    h1 { font-size: 2rem; margin-bottom: 8px; }
    .subtitle { color: #596578; margin-bottom: 36px; line-height: 1.6; }
    .panel { background: white; border: 1px solid #e4e8f0; border-radius: 16px; padding: 24px; }
    h2 { margin-top: 0; font-size: 1.25rem; }
    .group-list { padding: 0; margin: 0; list-style: none; }
    .group-list li { padding: 15px 0; border-top: 1px solid #e4e8f0; display: flex; justify-content: space-between; gap: 16px; align-items: center; }
    .group-list li:first-child { border-top: 0; }
    .group-name { font-weight: 600; }
    .group-id, .empty { color: #687488; }
    .empty { line-height: 1.6; }
  </style>
</head>
<body>
  <main>
    <h1>ThesisFlow</h1>
    <p class="subtitle">Локальный помощник преподавателя для учёта материалов ВКР.</p>
    <section class="panel" aria-labelledby="groups-heading">
      <h2 id="groups-heading">Учебные группы</h2>
      {{if .Groups}}
      <ul class="group-list">
        {{range .Groups}}
        <li data-group-id="{{.ID}}"><span class="group-name">{{.Name}}</span><span class="group-id">ID: {{.ID}}</span></li>
        {{end}}
      </ul>
      {{else}}
      <p class="empty">Группы пока не добавлены. Создание групп через браузер появится в следующем этапе.</p>
      {{end}}
    </section>
  </main>
</body>
</html>
`))

// NewHandler creates the local HTTP interface using a read-only group store.
// GET / shows current saved groups; unknown paths and methods remain 404/405.
// Read failures produce HTTP 500 without exposing internal storage details.
//
// RU: NewHandler создаёт локальный HTTP-интерфейс с чтением учебных групп.
// GET / показывает сохранённые группы; остальные пути и методы дают 404/405.
// При ошибке чтения возвращается HTTP 500 без раскрытия деталей хранилища.
func NewHandler(lister GroupLister) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		items, err := lister.List(r.Context())
		if err != nil {
			http.Error(w, "Не удалось загрузить список групп.", http.StatusInternalServerError)
			return
		}

		// Prepare HTML before writing HTTP headers to avoid partial pages.
		// RU: Готовим HTML до отправки заголовков, не выдавая неполную страницу.
		var rendered bytes.Buffer
		if err := homePageTemplate.Execute(&rendered, homeData{Groups: items}); err != nil {
			http.Error(w, "Не удалось отобразить страницу.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(rendered.Bytes())
	})
	return mux
}

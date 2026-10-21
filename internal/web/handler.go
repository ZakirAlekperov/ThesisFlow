// Package web provides HTTP handlers for the local ThesisFlow interface.
// Пакет web содержит HTTP-обработчики локального интерфейса ThesisFlow.
package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net/http"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
)

// GroupStore is the minimal group repository contract needed by the UI.
// Tests use an in-memory fake, while production uses the SQLite repository.
//
// RU: GroupStore — минимальный контракт чтения и создания учебных групп.
// Тесты используют имитатор, рабочее приложение — SQLite-репозиторий.
type GroupStore interface {
	List(context.Context) ([]groups.Group, error)
	Create(context.Context, string) (groups.Group, error)
}

type homeData struct {
	Groups    []groups.Group
	CSRFToken string
	Name      string
	Error     string
}

// homePageTemplate escapes group names, form values and tokens automatically.
// RU: homePageTemplate экранирует названия групп, значения формы и токен.
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
    .tabs { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 30px; border-bottom: 1px solid #dce2eb; padding-bottom: 12px; }
    .tabs a { display: inline-block; padding: 10px 16px; border-radius: 8px; text-decoration: none; color: #214eaf; }
    .tabs a[aria-current=page] { background: #214eaf; color: white; font-weight: 600; }
    h1 { font-size: 2rem; margin-bottom: 8px; }
    .subtitle { color: #596578; margin-bottom: 36px; line-height: 1.6; }
    .panel { background: white; border: 1px solid #e4e8f0; border-radius: 16px; padding: 24px; }
    h2 { margin-top: 0; font-size: 1.25rem; }
    .group-form { display: flex; flex-wrap: wrap; gap: 12px; margin-bottom: 24px; }
    .group-form label { flex-basis: 100%; font-weight: 600; }
    .group-form input[type=text] { flex: 1 1 240px; padding: 11px 12px; border: 1px solid #bfc9d7; border-radius: 8px; font: inherit; }
    .group-form button { border: 0; border-radius: 8px; background: #214eaf; color: white; font: inherit; font-weight: 600; padding: 11px 18px; cursor: pointer; }
    .group-form :focus-visible { outline: 2px solid #214eaf; outline-offset: 2px; }
    .form-error { color: #a51d2d; margin: -12px 0 20px; }
    .group-list { padding: 0; margin: 0; list-style: none; }
    .group-list li { padding: 15px 0; border-top: 1px solid #e4e8f0; display: flex; justify-content: space-between; gap: 16px; align-items: center; }
    .group-list li:first-child { border-top: 0; }
    .group-list a { color: #214eaf; text-decoration: none; overflow-wrap: anywhere; }
    .group-list a:hover { text-decoration: underline; }
    .group-name { font-weight: 600; }
    .group-id, .empty { color: #687488; }
    .empty { line-height: 1.6; }
  </style>
</head>
<body>
  <main>
    <nav class="tabs" aria-label="Разделы приложения">
      <a href="/" aria-current="page">Учебные группы</a>
      <a href="/templates">Шаблоны</a>
      <a href="/manage">Управление и архив</a>
    </nav>
    <h1>ThesisFlow</h1>
    <p class="subtitle">Локальный помощник преподавателя для учёта материалов ВКР.</p>
    <section class="panel" aria-labelledby="groups-heading">
      <h2 id="groups-heading">Учебные группы</h2>
      <form class="group-form" action="/groups" method="post">
        <label for="group-name">Название группы</label>
        <input id="group-name" type="text" name="name" value="{{.Name}}" placeholder="Например, ИСП-401" required autocomplete="off">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <button type="submit">Создать группу</button>
      </form>
      {{if .Error}}<p class="form-error" role="alert">{{.Error}}</p>{{end}}
      {{if .Groups}}
      <ul class="group-list">
        {{range .Groups}}
        <li data-group-id="{{.ID}}"><a class="group-name" href="/groups/{{.ID}}">{{.Name}}</a><span class="group-id">ID: {{.ID}}</span></li>
        {{end}}
      </ul>
      {{else}}
      <p class="empty">Группы пока не добавлены.</p>
      {{end}}
    </section>
  </main>
</body>
</html>
`))

// renderHome prepares complete HTML before sending headers. Database errors
// are not exposed to the browser and never become a misleading empty state.
//
// RU: renderHome формирует HTML до отправки заголовков; внутренние ошибки
// БД не раскрываются и не маскируются под пустой список.
func renderHome(w http.ResponseWriter, r *http.Request, store GroupStore, token, name, message string, status int) {
	items, err := store.List(r.Context())
	if err != nil {
		http.Error(w, "Не удалось загрузить список групп.", http.StatusInternalServerError)
		return
	}
	var rendered bytes.Buffer
	if err := homePageTemplate.Execute(&rendered, homeData{
		Groups: items, CSRFToken: token, Name: name, Error: message,
	}); err != nil {
		http.Error(w, "Не удалось отобразить страницу.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(rendered.Bytes())
}

// NewHandler constructs the localhost web UI with a student read/write store
// and a random per-process CSRF token. It returns an error if secure randomness
// is unavailable instead of exposing an unprotected writable route.
//
// RU: NewHandler создаёт локальный интерфейс с отдельным хранилищем студентов
// и случайным CSRF-токеном на время работы приложения. При недоступности
// безопасной случайности защищённый обработчик не создаётся.
// Dependencies collects optional local features without forcing callers to
// install external services or change previously supported forms.
// RU: Dependencies объединяет необязательные локальные возможности.
type Dependencies struct {
	Workbook     WorkbookImporter
	Requirements RequirementStore
	Report       ReportProvider
	Documents    *DocumentServices
	Management   RecordManager
}

// NewHandler preserves the minimal constructor used by existing tests.
// RU: NewHandler сохраняет простой конструктор для существующих тестов.
func NewHandler(store GroupStore, studentStore StudentStore, importers ...WorkbookImporter) (http.Handler, error) {
	var deps Dependencies
	if len(importers) > 0 {
		deps.Workbook = importers[0]
	}
	return NewHandlerWithServices(store, studentStore, deps)
}

// NewHandlerWithServices adds document requirements to the existing app.
// RU: NewHandlerWithServices подключает требования к основному интерфейсу.
func NewHandlerWithServices(store GroupStore, studentStore StudentStore, deps Dependencies) (http.Handler, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("create form protection token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(secret)

	mux := http.NewServeMux()
	if deps.Management != nil {
		mux.HandleFunc("GET /manage", func(w http.ResponseWriter, r *http.Request) {
			renderManagement(w,r,store,studentStore,deps.Management,token)
		})
		mux.HandleFunc("POST /manage/groups/{id}/{action}",func(w http.ResponseWriter,r *http.Request){
			applyManagementAction(w,r,deps.Management,token,"groups")
		})
		mux.HandleFunc("POST /manage/students/{id}/{action}",func(w http.ResponseWriter,r *http.Request){
			applyManagementAction(w,r,deps.Management,token,"students")
		})
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		renderHome(w, r, store, token, "", "", http.StatusOK)
	})
	mux.HandleFunc("GET /templates", renderTemplates)
	mux.HandleFunc("GET /templates/download/{id}", downloadTemplate)
	mux.HandleFunc("GET /groups/{id}", func(w http.ResponseWriter, r *http.Request) {
		renderGroupStudents(w, r, store, studentStore, token, "", "", "", http.StatusOK, deps.Requirements)
	})
	mux.HandleFunc("POST /groups/{id}/students", func(w http.ResponseWriter, r *http.Request) {
		createStudentFromForm(w, r, store, studentStore, token, deps.Requirements)
	})
	if deps.Workbook != nil {
		mux.HandleFunc("POST /groups/{id}/import", func(w http.ResponseWriter, r *http.Request) {
			importStudentsFromWorkbook(w, r, store, deps.Workbook, token)
		})
	}
	if deps.Requirements != nil {
		mux.HandleFunc("POST /groups/{id}/requirements", func(w http.ResponseWriter, r *http.Request) {
			createGroupRequirement(w, r, store, deps.Requirements, token)
		})
	}
	if deps.Report != nil {
		mux.HandleFunc("GET /groups/{id}/report", func(w http.ResponseWriter, r *http.Request) {
			renderPrintableReport(w, r, store, deps.Report)
		})
	}
	if deps.Documents != nil {
		mux.HandleFunc("GET /groups/{id}/students/{student}", func(w http.ResponseWriter, r *http.Request) {
			renderDocuments(w, r, store, studentStore, deps.Documents, token)
		})
		for _, operation := range []string{"upload", "accept", "reject", "download"} {
			action := operation
			switch action {
			case "upload":
				mux.HandleFunc("POST /groups/{id}/students/{student}/documents/{requirement}/upload", func(w http.ResponseWriter, r *http.Request) {
					handleDocumentAction(w, r, store, studentStore, deps.Documents, token, action)
				})
			case "download":
				mux.HandleFunc("GET /groups/{id}/students/{student}/documents/{requirement}/versions/{version}/download", func(w http.ResponseWriter, r *http.Request) {
					handleDocumentAction(w, r, store, studentStore, deps.Documents, token, action)
				})
			default:
				mux.HandleFunc("POST /groups/{id}/students/{student}/documents/{requirement}/"+action+"/{version}", func(w http.ResponseWriter, r *http.Request) {
					handleDocumentAction(w, r, store, studentStore, deps.Documents, token, action)
				})
			}
		}
	}
	mux.HandleFunc("POST /groups", func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" {
			http.Error(w, "Неподдерживаемый формат формы.", http.StatusUnsupportedMediaType)
			return
		}

		// Limit request bodies, including malicious oversized form submissions.
		// RU: Ограничиваем тело запроса, в том числе при вредоносной отправке.
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

		name := r.PostForm.Get("name")
		if _, err := store.Create(r.Context(), name); err != nil {
			switch {
			case errors.Is(err, groups.ErrInvalidName):
				renderHome(w, r, store, token, name, "Введите название группы.", http.StatusBadRequest)
			case errors.Is(err, groups.ErrDuplicateName):
				renderHome(w, r, store, token, name, "Группа с таким названием уже существует.", http.StatusConflict)
			default:
				http.Error(w, "Не удалось создать группу.", http.StatusInternalServerError)
			}
			return
		}
		// PRG prevents duplicate insertions when the user refreshes the page.
		// RU: Redirect после POST исключает повторную отправку при обновлении.
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// The CSRF token must not be cached or leaked to unrelated websites.
	// RU: Запрещаем кеширование токена и передачу его посторонним сайтам.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	}), nil
}

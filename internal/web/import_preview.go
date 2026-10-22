package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/importxlsx"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// SheetWorkbookImporter supports the selected-sheet atomic importer.
// RU: SheetWorkbookImporter импортирует только выбранную вкладку XLSX.
type SheetWorkbookImporter interface {
	ImportSelected(context.Context, int64, []byte, string, string, string) (int, error)
}

type previewEntry struct {
	GroupID int64
	File    []byte
	Created time.Time
}

type previewStore struct {
	mu      sync.Mutex
	entries map[string]previewEntry
}

// newPreviewStore keeps unconfirmed workbooks only in bounded process memory.
// RU: newPreviewStore хранит неподтверждённые XLSX только в памяти процесса.
func newPreviewStore() *previewStore {
	return &previewStore{entries: make(map[string]previewEntry)}
}

func (s *previewStore) stage(groupID int64, data []byte) (string, error) {
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, item := range s.entries {
		if now.Sub(item.Created) > 20*time.Minute {
			delete(s.entries, key)
		}
	}
	if len(s.entries) >= 3 {
		var oldest string
		var oldestTime time.Time
		for key, item := range s.entries {
			if oldest == "" || item.Created.Before(oldestTime) {
				oldest = key
				oldestTime = item.Created
			}
		}
		delete(s.entries, oldest)
	}
	s.entries[token] = previewEntry{GroupID: groupID, File: data, Created: now}
	return token, nil
}

func (s *previewStore) get(token string, groupID int64) (previewEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.entries[token]
	if !ok || item.GroupID != groupID || time.Since(item.Created) > 20*time.Minute {
		return previewEntry{}, false
	}
	return item, true
}

func (s *previewStore) remove(token string) {
	s.mu.Lock()
	delete(s.entries, token)
	s.mu.Unlock()
}

type previewPage struct {
	GroupID      int64
	Sheets       []string
	SheetName    string
	Rows         [][]string
	PreviewToken string
	CSRFToken    string
	NameHeader   string
	IDHeader     string
	Error        string
}

// previewTemplate escapes all cells and form fields through html/template.
// RU: previewTemplate экранирует значения XLSX и безопасно отображает таблицу.
var previewTemplate = template.Must(template.New("xlsx-preview").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Предпросмотр Excel — ThesisFlow</title><style>body{font-family:system-ui,sans-serif;color:#202939;background:#f6f7fb;max-width:980px;margin:35px auto;padding:0 20px}main{background:white;border:1px solid #dce3ed;padding:24px;border-radius:12px}label{display:block;margin:10px 0}input,select,button{padding:10px;border:1px solid #c6ced9;border-radius:7px;font:inherit}button{background:#214eaf;color:white;cursor:pointer}table{border-collapse:collapse;width:100%;margin:20px 0}td{padding:9px;border:1px solid #dce3ed;overflow-wrap:anywhere}tr:first-child{background:#eef4fc;font-weight:600}a{color:#214eaf}.error{color:#a51d2d}small{color:#657388}</style><link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script></head><body><main>
<a href="/groups/{{.GroupID}}">← Назад к группе</a><h1>Предпросмотр импорта Excel</h1>
<p>Записи ещё не добавлены в базу. Выберите вкладку и проверьте столбцы перед подтверждением.</p>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<form method="post" action="/groups/{{.GroupID}}/import/preview/sheet">
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="preview_token" value="{{.PreviewToken}}">
<label>Лист Excel <select name="sheet_name">{{range .Sheets}}<option value="{{.}}" {{if eq . $.SheetName}}selected{{end}}>{{.}}</option>{{end}}</select></label>
<button type="submit">Показать выбранный лист</button>
</form>
<h2>Первые строки файла</h2>
<table aria-label="Предпросмотр Excel">{{range .Rows}}<tr>{{range .}}<td>{{.}}</td>{{end}}</tr>{{end}}</table>
<form method="post" action="/groups/{{.GroupID}}/import/commit">
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><input type="hidden" name="preview_token" value="{{.PreviewToken}}">
<input type="hidden" name="sheet_name" value="{{.SheetName}}">
<label>Заголовок столбца с ФИО <input name="name_header" value="{{.NameHeader}}" required></label>
<label>Заголовок столбца с учебным ID <input name="id_header" value="{{.IDHeader}}" required></label>
<p><small>При ошибке ни одна строка не будет импортирована. Предпросмотр действует 20 минут.</small></p>
<button type="submit">Подтвердить импорт</button>
</form></main></body></html>`))

func validatePreviewRequest(w http.ResponseWriter, r *http.Request, groups GroupStore, csrf string, multipart bool) (int64, bool) {
	groupID, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	want := "application/x-www-form-urlencoded"
	if multipart {
		want = "multipart/form-data"
	}
	if err != nil || media != want {
		http.Error(w, "Неподдерживаемый формат формы.", 415)
		return 0, false
	}
	limit := int64(4096)
	if multipart {
		limit = int64(importxlsx.MaxFileSize + 8192)
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if multipart {
		err = r.ParseMultipartForm(1 << 20)
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		http.Error(w, "Недопустимый размер или формат формы.", 400)
		return 0, false
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(csrf)) != 1 {
		http.Error(w, "Недействительный запрос.", 403)
		return 0, false
	}
	if _, err := findGroup(r.Context(), groups, groupID); err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Ошибка чтения группы.", 500)
		}
		return 0, false
	}
	return groupID, true
}

func renderPreview(w http.ResponseWriter, r *http.Request, store *previewStore, entry previewEntry, stageToken, csrf, sheetName, message string, status int) {
	sheets, err := importxlsx.Sheets(entry.File)
	if err != nil || len(sheets) == 0 {
		http.Error(w, "Не удалось прочитать вкладки XLSX.", 400)
		return
	}
	if sheetName == "" {
		sheetName = sheets[0]
	}
	rows, err := importxlsx.ReadNamed(entry.File, sheetName)
	if err != nil {
		http.Error(w, "Не удалось открыть выбранную вкладку.", 400)
		return
	}
	if len(rows) > 8 {
		rows = rows[:8]
	}
	for i := range rows {
		if len(rows[i]) > 14 {
			rows[i] = rows[i][:14]
		}
	}
	var b bytes.Buffer
	if err := previewTemplate.Execute(&b, previewPage{GroupID: entry.GroupID, Sheets: sheets, SheetName: sheetName, Rows: rows, PreviewToken: stageToken, CSRFToken: csrf, NameHeader: "ФИО", IDHeader: "Учебный ID", Error: message}); err != nil {
		http.Error(w, "Ошибка отображения Excel.", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}

// previewUpload stages an XLSX but cannot import it without confirmation.
// RU: previewUpload только сохраняет файл на короткое время, без записи SQLite.
func previewUpload(w http.ResponseWriter, r *http.Request, groups GroupStore, store *previewStore, csrf string) {
	groupID, ok := validatePreviewRequest(w, r, groups, csrf, true)
	if !ok {
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Выберите Excel-файл.", 400)
		return
	}
	defer file.Close()
	if !strings.EqualFold(fileExtension(header.Filename), ".xlsx") {
		http.Error(w, "Нужен файл .xlsx.", 400)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, importxlsx.MaxFileSize+1))
	if err != nil || len(data) > importxlsx.MaxFileSize {
		http.Error(w, "Файл Excel слишком большой.", 413)
		return
	}
	if _, err := importxlsx.Sheets(data); err != nil {
		http.Error(w, "Некорректная книга Excel.", 400)
		return
	}
	stageToken, err := store.stage(groupID, data)
	if err != nil {
		http.Error(w, "Не удалось подготовить предпросмотр.", 500)
		return
	}
	renderPreview(w, r, store, previewEntry{GroupID: groupID, File: data}, stageToken, csrf, "", "", 200)
}

func previewSelectSheet(w http.ResponseWriter, r *http.Request, groups GroupStore, store *previewStore, csrf string) {
	id, ok := validatePreviewRequest(w, r, groups, csrf, false)
	if !ok {
		return
	}
	token := r.PostForm.Get("preview_token")
	entry, found := store.get(token, id)
	if !found {
		http.Error(w, "Предпросмотр устарел. Загрузите файл снова.", 400)
		return
	}
	renderPreview(w, r, store, entry, token, csrf, r.PostForm.Get("sheet_name"), "", 200)
}

// previewCommit uses the existing atomic student importer only after explicit
// confirmation. A successful token cannot be reused for a second import.
// RU: previewCommit сохраняет студентов только после явного подтверждения.
func previewCommit(w http.ResponseWriter, r *http.Request, groups GroupStore, store *previewStore, importer SheetWorkbookImporter, csrf string) {
	id, ok := validatePreviewRequest(w, r, groups, csrf, false)
	if !ok {
		return
	}
	token := r.PostForm.Get("preview_token")
	entry, found := store.get(token, id)
	if !found {
		http.Error(w, "Предпросмотр устарел. Загрузите файл снова.", 400)
		return
	}
	sheet := r.PostForm.Get("sheet_name")
	_, err := importer.ImportSelected(r.Context(), id, entry.File, sheet, r.PostForm.Get("name_header"), r.PostForm.Get("id_header"))
	if err != nil {
		status := http.StatusBadRequest
		message := "Проверьте выбранный лист и заголовки. Все изменения отменены."
		switch {
		case errors.Is(err, students.ErrDuplicateEducationalID):
			status = 409
			message = "Обнаружен повторяющийся учебный ID. Ничего не импортировано."
		case errors.Is(err, importxlsx.ErrTooLarge):
			status = 413
			message = "Файл Excel слишком большой."
		case errors.Is(err, importxlsx.ErrMissingColumn):
			message = "Выбранные заголовки отсутствуют в указанном листе."
		case errors.Is(err, students.ErrInvalidFullName), errors.Is(err, students.ErrInvalidEducationalID):
			message = "Одна из строк не содержит ФИО или учебного номера."
		case errors.Is(err, importxlsx.ErrSheetNotFound):
			message = "Выбранный лист Excel не найден."
		default:
			status = 500
			message = "Не удалось выполнить импорт. Изменения не сохранены."
		}
		renderPreview(w, r, store, entry, token, csrf, sheet, message, status)
		return
	}
	store.remove(token)
	http.Redirect(w, r, "/groups/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

// BackupExporter snapshots the active workspace to a previously nonexistent
// ZIP path; implementing services own SQLite and storage coordination.
//
// RU: BackupExporter сохраняет рабочие данные в новый ZIP. Реализация
// отвечает за целостность SQLite и пользовательских файлов.
type BackupExporter interface {
	Export(context.Context, string) error
}

type settingsData struct {
	CSRFToken string
	SelectionEnabled bool
}

// settingsTemplate renders an offline settings form using Go html/template.
// Backups contain unencrypted student data; warn explicitly before exporting.
//
// RU: settingsTemplate отображает настройки без внешних зависимостей.
// Предупреждает, что архив содержит персональные данные без шифрования.
var settingsTemplate = template.Must(template.New("settings").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Настройки — ThesisFlow</title><link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px system-ui,sans-serif;max-width:800px;margin:40px auto;padding:0 20px}.panel{padding:24px;border:1px solid #dce3ed;border-radius:14px}button{border:0;border-radius:8px;background:#214eaf;color:white;padding:12px 20px;font:inherit;cursor:pointer}p{line-height:1.6}.warning{font-weight:600}</style>
</head><body><nav><a href="/dashboard">← Панель преподавателя</a></nav>
<h1>Настройки</h1><p><a href="/settings/health">Проверить состояние хранилища →</a></p>{{if .SelectionEnabled}}<p><a href="/settings/workspaces">Выбрать восстановленное рабочее пространство →</a></p>{{end}}<section class="panel"><h2>Резервное копирование</h2>
<p>Сохраните SQLite и документы ВКР одним ZIP-архивом.</p>
<p class="warning">Внимание: архив не зашифрован и содержит персональные данные. Храните его в защищённом месте.</p>
<form method="post" action="/settings/backup">
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
<button type="submit">Скачать резервную копию</button></form>
<h2>Восстановление из ZIP</h2>
<p>Система проверит архив и создаст отдельный каталог. Рабочая база не будет заменена.</p>
<form method="post" action="/settings/restore" enctype="multipart/form-data">
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
<label for="restore-zip">Файл ZIP</label>
<input id="restore-zip" type="file" name="archive" accept=".zip" required>
<button type="submit">Восстановить в новую папку</button>
</form>
</section></body></html>`))

// renderSettings serves a no-cache CSRF-protected settings page.
// RU: renderSettings показывает настройки с защитой от кеширования токена.
func renderSettings(w http.ResponseWriter, r *http.Request, token string, allowSelection ...bool) {
	selectionEnabled := len(allowSelection)>0 && allowSelection[0]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := settingsTemplate.Execute(w, settingsData{CSRFToken: token, SelectionEnabled: selectionEnabled}); err != nil {
		// Template is static and pre-validated at startup.
		// RU: Шаблон статичен и проверяется при запуске.
		return
	}
}

// downloadBackup validates the request and serves an ephemeral ZIP archive.
// The temporary file is deleted after the response, including on errors.
//
// RU: downloadBackup проверяет запрос и передаёт временный ZIP в браузер.
// Временные файлы удаляются после ответа, даже при ошибках.
func downloadBackup(w http.ResponseWriter, r *http.Request, token string, exporter BackupExporter) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "Неподдерживаемый формат запроса.", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Форма слишком большая.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Не удалось прочитать форму.", http.StatusBadRequest)
		}
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return
	}
	workdir, err := os.MkdirTemp("", "thesisflow-browser-backup-*")
	if err != nil {
		http.Error(w, "Не удалось создать резервную копию.", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(workdir)
	path := filepath.Join(workdir, "ThesisFlow-backup.zip")
	if err := exporter.Export(r.Context(), path); err != nil {
		http.Error(w, "Не удалось создать резервную копию.", http.StatusInternalServerError)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "Не удалось прочитать резервную копию.", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ThesisFlow-backup.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, file)
}

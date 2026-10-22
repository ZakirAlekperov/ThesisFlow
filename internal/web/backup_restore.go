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

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
)

// BackupRestorer validates and restores an archive into a new directory.
// RU: BackupRestorer проверяет архив и восстанавливает в новую папку.
type BackupRestorer interface {
	Restore(context.Context, string) (string, error)
}

var restoredTemplate = template.Must(template.New("restored").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><title>Восстановление — ThesisFlow</title></head>
<body><a href="/settings">← Настройки</a><h1>Копия восстановлена</h1>
<p>Новый каталог: <code>{{.}}</code></p><p>Рабочие данные не заменены. Проверьте содержимое новой папки.</p><p><a href="/settings/workspaces">Перейти к выбору восстановленного рабочего пространства →</a></p>
</body></html>`))

// restoreBackup handles a bounded CSRF-protected multipart ZIP upload.
// RU: restoreBackup обрабатывает ограниченную ZIP-форму с CSRF-защитой.
func restoreBackup(w http.ResponseWriter, r *http.Request, token string, service BackupRestorer) {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/form-data" {
		http.Error(w, "Ожидается форма загрузки ZIP.", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, backup.MaxArchiveBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "ZIP слишком большой.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Некорректная форма ZIP.", http.StatusBadRequest)
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return
	}
	file, _, err := r.FormFile("archive")
	if err != nil {
		http.Error(w, "Выберите ZIP-файл.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	dir, err := os.MkdirTemp("", "thesisflow-restore-upload-*")
	if err != nil {
		http.Error(w, "Ошибка подготовки.", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)
	temp := filepath.Join(dir, "upload.zip")
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		http.Error(w, "Ошибка записи.", http.StatusInternalServerError)
		return
	}
	_, copyErr := io.Copy(out, io.LimitReader(file, backup.MaxArchiveBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		http.Error(w, "Ошибка записи ZIP.", http.StatusInternalServerError)
		return
	}
	info, err := os.Stat(temp)
	if err != nil || info.Size() > backup.MaxArchiveBytes {
		http.Error(w, "ZIP слишком большой.", http.StatusRequestEntityTooLarge)
		return
	}
	target, err := service.Restore(r.Context(), temp)
	if err != nil {
		http.Error(w, "Не удалось проверить и восстановить архив.", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = restoredTemplate.Execute(w, target)
}

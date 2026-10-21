package web

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
)

// BackupProvider isolates ZIP export/recovery from HTTP transport.
// RU: BackupProvider разделяет создание ZIP и HTTP-представление.
type BackupProvider interface {
	Export(context.Context, io.Writer) error
	Restore(context.Context, io.Reader) (string, error)
}

type backupData struct {
	Token string
	Result string
}

// backupTemplate is local server-rendered HTML with no JavaScript dependency.
// RU: backupTemplate — локальная серверная HTML-форма без фреймворков.
var backupTemplate = template.Must(template.New("backup").Parse(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Резервные копии — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px system-ui,sans-serif;max-width:800px;margin:35px auto;padding:0 20px}.panel{background:white;border:1px solid #dce3ed;border-radius:12px;padding:24px;margin:20px 0}button,.button{padding:12px 18px;border-radius:9px;border:0;background:#2456a9;color:white;text-decoration:none;font:inherit;cursor:pointer}label{display:block;margin:15px 0}input{padding:8px;width:100%}.warn{color:#9c4c00}a{color:#214eaf}</style>
</head><body><a href="/dashboard">← Панель преподавателя</a><h1>Резервное копирование</h1>
<div class="panel"><h2>Скачать резервную копию</h2>
<p>Скачайте архив ZIP с базой SQLite и файлами ВКР.</p>
<p class="warn">Важно: при загрузке документов в других вкладках резервная копия файлов может оказаться несогласованной. Надёжнее остановить приложение и использовать команду резервного копирования в терминале.</p>
<p><a class="button" href="/backup/download">Скачать архив ZIP</a></p></div>
<div class="panel"><h2>Восстановление резервной копии</h2>
<p>Архив восстанавливается строго в отдельную новую папку; текущая база не перезаписывается.</p>
<form method="post" enctype="multipart/form-data" action="/backup/restore">
<input type="hidden" name="csrf_token" value="{{.Token}}">
<label>Файл резервной копии ZIP <input name="archive" type="file" accept=".zip" required></label>
<button type="submit">Проверить и восстановить отдельно</button>
</form>{{if .Result}}<p role="status">{{.Result}}</p>{{end}}</div></body></html>`))

func renderBackupPage(w http.ResponseWriter, provider BackupProvider, token, result string) {
	var b bytes.Buffer
	if err := backupTemplate.Execute(&b, backupData{Token: token, Result: result}); err != nil {
		http.Error(w, "Не удалось отобразить резервные копии.", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b.Bytes())
}

// downloadBackup transfers a snapshot as an attachment rather than exposing
// local filesystem paths. RU: downloadBackup отдаёт ZIP как вложение.
func downloadBackup(w http.ResponseWriter,r *http.Request,provider BackupProvider) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ThesisFlow-backup.zip"`)
	if err:=provider.Export(r.Context(),w);err!=nil{
		// The HTTP response may already be streaming; details remain in logs
		// rather than exposing local paths to the browser.
		return
	}
}

// restoreBackup validates multipart upload size, CSRF, and file extension
// before restoring to a separate workspace. Active files are never replaced.
// RU: restoreBackup проверяет форму и восстанавливает только в новый каталог.
func restoreBackup(w http.ResponseWriter,r *http.Request,provider BackupProvider,token string) {
	media,_,err:=mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err!=nil||media!="multipart/form-data"{
		http.Error(w,"Требуется ZIP-файл.",http.StatusUnsupportedMediaType)
		return
	}
	r.Body=http.MaxBytesReader(w,r.Body,backup.MaxArchiveBytes+8192)
	if err:=r.ParseMultipartForm(1<<20);err!=nil{
		http.Error(w,"Файл слишком большой или форма повреждена.",http.StatusRequestEntityTooLarge)
		return
	}
	if r.MultipartForm!=nil{defer r.MultipartForm.RemoveAll()}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")),[]byte(token))!=1{
		http.Error(w,"Недействительный запрос.",http.StatusForbidden)
		return
	}
	file,header,err:=r.FormFile("archive")
	if err!=nil{http.Error(w,"Выберите ZIP-файл.",400);return}
	defer file.Close()
	if !strings.EqualFold(fileExtension(header.Filename),".zip"){
		http.Error(w,"Требуется архив .zip.",400)
		return
	}
	folder,err:=provider.Restore(r.Context(),io.LimitReader(file,backup.MaxArchiveBytes+1))
	if err!=nil {
		switch {
		case errors.Is(err,backup.ErrTooLarge):http.Error(w,"Архив слишком большой.",413)
		case errors.Is(err,backup.ErrUnsafeArchive):http.Error(w,"Резервная копия повреждена или небезопасна.",400)
		case errors.Is(err,backup.ErrDestinationExists):http.Error(w,"Каталог восстановления уже существует.",409)
		default:http.Error(w,"Не удалось восстановить архив.",500)
		}
		return
	}
	renderBackupPage(w,provider,token,"Копия восстановлена в отдельную папку: "+folder)
}

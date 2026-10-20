package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/importxlsx"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// WorkbookImporter is the narrow interface required to import XLSX data into
// an already initialized academic group.
// RU: WorkbookImporter — интерфейс импорта Excel в существующую группу.
type WorkbookImporter interface {
	ImportFile(context.Context, int64, []byte, string, string) (int, error)
}

// importStudentsFromWorkbook validates a multipart XLSX upload, checks the
// shared CSRF token and group ID, then atomically imports mapped students.
// Internal errors never expose the local filesystem or database paths.
//
// RU: importStudentsFromWorkbook проверяет XLSX, CSRF-токен и ID группы,
// а затем атомарно импортирует студентов по заголовкам столбцов.
// Внутренние ошибки не раскрывают пути файлов или базы данных.
func importStudentsFromWorkbook(w http.ResponseWriter, r *http.Request, groupsStore GroupStore, importer WorkbookImporter, token string) {
	groupID, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		http.Error(w, "Ожидается форма с файлом Excel.", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, importxlsx.MaxFileSize+8192)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Файл Excel слишком большой.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Не удалось прочитать файл Excel.", http.StatusBadRequest)
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
	if _, err := findGroup(r.Context(), groupsStore, groupID); err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось загрузить учебную группу.", http.StatusInternalServerError)
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Выберите файл Excel.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if !strings.EqualFold(".xlsx", fileExtension(header.Filename)) {
		http.Error(w, "Выберите файл формата .xlsx.", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, importxlsx.MaxFileSize+1))
	if err != nil {
		http.Error(w, "Не удалось прочитать файл Excel.", http.StatusBadRequest)
		return
	}
	if len(data) > importxlsx.MaxFileSize {
		http.Error(w, "Файл Excel слишком большой.", http.StatusRequestEntityTooLarge)
		return
	}
	_, err = importer.ImportFile(
		r.Context(), groupID, data, r.PostForm.Get("name_header"), r.PostForm.Get("id_header"),
	)
	if err != nil {
		switch {
		case errors.Is(err, importxlsx.ErrTooLarge):
			http.Error(w, "Файл Excel слишком большой.", http.StatusRequestEntityTooLarge)
		case errors.Is(err, importxlsx.ErrMissingColumn):
			http.Error(w, "Не удалось найти выбранные столбцы в Excel.", http.StatusBadRequest)
		case errors.Is(err, students.ErrDuplicateEducationalID):
			http.Error(w, "В Excel обнаружен повторяющийся учебный ID.", http.StatusConflict)
		case errors.Is(err, students.ErrInvalidEducationalID), errors.Is(err, students.ErrInvalidFullName):
			http.Error(w, "В Excel есть строка без ФИО или учебного ID.", http.StatusBadRequest)
		case errors.Is(err, students.ErrGroupNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, "Не удалось импортировать файл Excel.", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/groups/"+strconv.FormatInt(groupID, 10), http.StatusSeeOther)
}

// fileExtension extracts a filename extension without using paths supplied by
// a potentially untrusted browser upload.
// RU: fileExtension проверяет расширение имени из недоверенной формы.
func fileExtension(name string) string {
	dot := strings.LastIndexByte(name, '.')
	if dot < 0 {
		return ""
	}
	return name[dot:]
}

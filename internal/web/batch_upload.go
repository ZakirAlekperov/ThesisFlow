package web

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
)

// The batch budget is intentionally smaller than an unlimited collection of
// 20 MiB files. Preflight rejects malformed batches before the first write.
// RU: Общий предел не даёт неограниченно накапливать файлы в памяти.
const (
	maxBatchDocuments = 10
	maxBatchBytes     = int64(40 << 20)
)

type queuedDocument struct {
	requirementID int64
	filename      string
	data          []byte
}

// permittedBatchName mirrors the file version store's documented extension
// allowlist. The store remains the final authority when Save is called.
// RU: Проверяем расширение до записи, чтобы одна ошибка не дала полупартию.
func permittedBatchName(name string) bool {
	extension := strings.ToLower(filepath.Ext(strings.ReplaceAll(name, "\\", "/")))
	switch extension {
	case ".docx", ".pdf", ".odt", ".rtf", ".txt", ".xlsx":
		return true
	default:
		return false
	}
}

// batchUploadDocuments supports one explicitly assigned file per requirement.
// It keeps each existing single-document upload route unchanged.
//
// RU: Пакетная форма сопоставляет файл требованию по постоянному ID.
// Все введённые файлы проверяются до первого сохранения версии.
func batchUploadDocuments(w http.ResponseWriter, r *http.Request, groupStore GroupStore, people StudentStore, docs *DocumentServices, csrfToken string) {
	groupID, studentID, _, err := documentScope(r, groupStore, people, docs)
	if err != nil {
		documentError(w, r, err)
		return
	}
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/form-data" {
		http.Error(w, "Ожидается форма с файлами.", http.StatusUnsupportedMediaType)
		return
	}
	// The multipart parser spills large parts to temporary files; RemoveAll
	// also runs for rejected forms to avoid disk litter.
	// RU: Удаляем временные multipart-файлы после запроса.
	r.Body = http.MaxBytesReader(w, r.Body, maxBatchBytes+(64<<10))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Общий размер документов превышает 40 МиБ.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Не удалось прочитать пакет документов.", http.StatusBadRequest)
		}
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if !verifyDocumentToken(w, r, csrfToken) {
		return
	}
	requirements, err := docs.Requirements.ListForStudent(r.Context(), groupID, studentID)
	if err != nil {
		documentError(w, r, err)
		return
	}
	allowed := make(map[int64]bool, len(requirements))
	for _, requirement := range requirements {
		allowed[requirement.ID] = true
	}
	uploads := make(map[int64]*multipart.FileHeader)
	var total int64
	for field, files := range r.MultipartForm.File {
		if !strings.HasPrefix(field, "document_") || len(files) != 1 {
			http.Error(w, "Для каждого требования выберите не более одного файла.", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(field, "document_"), 10, 64)
		if err != nil || !allowed[id] {
			http.Error(w, "Неизвестное требование к документу.", http.StatusBadRequest)
			return
		}
		f := files[0]
		if f.Filename == "" || !permittedBatchName(f.Filename) || f.Size == 0 {
			http.Error(w, "Недопустимый файл. Допускаются DOCX, PDF, ODT, RTF, TXT, XLSX.", http.StatusBadRequest)
			return
		}
		if f.Size > fileversions.MaxFileSize {
			http.Error(w, "Один из файлов превышает 20 МиБ.", http.StatusRequestEntityTooLarge)
			return
		}
		total += f.Size
		if total > maxBatchBytes {
			http.Error(w, "Общий размер документов превышает 40 МиБ.", http.StatusRequestEntityTooLarge)
			return
		}
		uploads[id] = f
		if len(uploads) > maxBatchDocuments {
			http.Error(w, "В одной партии допускается не более 10 документов.", http.StatusBadRequest)
			return
		}
	}
	if len(uploads) == 0 {
		http.Error(w, "Выберите хотя бы один документ.", http.StatusBadRequest)
		return
	}
	// Read and validate the full bounded batch before persisting any version.
	// RU: Ошибки чтения/формата выявляются до первой записи в SQLite.
	queue := make([]queuedDocument, 0, len(uploads))
	for _, requirement := range requirements {
		header, exists := uploads[requirement.ID]
		if !exists {
			continue
		}
		stream, err := header.Open()
		if err != nil {
			http.Error(w, "Не удалось открыть документ.", http.StatusBadRequest)
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, int64(fileversions.MaxFileSize)+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || len(data) == 0 {
			http.Error(w, "Не удалось прочитать один из документов.", http.StatusBadRequest)
			return
		}
		if len(data) > fileversions.MaxFileSize {
			http.Error(w, "Один из файлов превышает 20 МиБ.", http.StatusRequestEntityTooLarge)
			return
		}
		queue = append(queue, queuedDocument{requirementID: requirement.ID, filename: header.Filename, data: data})
	}
	saved := 0
	for _, item := range queue {
		if _, err := docs.Versions.Save(r.Context(), studentID, item.requirementID, item.filename, item.data); err != nil {
			// There is no filesystem-wide transaction spanning all versions.
			// Report partial success; previously written versions remain intact.
			// RU: Не скрываем частичный успех при ошибке файловой системы/БД.
			http.Error(w, fmt.Sprintf("Загружено %d из %d файлов. Проверьте карточку перед повторной отправкой.", saved, len(queue)), http.StatusInternalServerError)
			return
		}
		saved++
	}
	http.Redirect(w, r, fmt.Sprintf("/groups/%d/students/%d", groupID, studentID), http.StatusSeeOther)
}

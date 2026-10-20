package web

import (
	"context"
	"crypto/subtle"
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// RequirementStore provides group requirements and creates inherited or
// individual documents through the existing SQLite repository.
// RU: RequirementStore читает требования группы и создаёт обязательные файлы.
type RequirementStore interface {
	ListForGroup(context.Context, int64) ([]requirements.Requirement,error)
	Create(context.Context, int64, int64, string, string) (requirements.Requirement,error)
}

// createGroupRequirement handles group-wide document requirements.
// It validates CSRF, group ownership, and the title/date before storing.
// RU: createGroupRequirement проверяет CSRF, группу, название и срок сдачи.
func createGroupRequirement(w http.ResponseWriter, r *http.Request, groupsStore GroupStore, store RequirementStore, token string) {
	groupID, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "Ожидается форма с названием документа.", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Размер формы превышен.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Не удалось прочитать форму.", http.StatusBadRequest)
		}
		return
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
	_, err = store.Create(r.Context(), groupID, 0, r.PostForm.Get("title"), r.PostForm.Get("due_date"))
	if err != nil {
		switch {
		case errors.Is(err, requirements.ErrInvalidTitle):
			http.Error(w, "Введите название обязательного документа.", http.StatusBadRequest)
		case errors.Is(err, requirements.ErrInvalidDate):
			http.Error(w, "Укажите корректную дату сдачи.", http.StatusBadRequest)
		case errors.Is(err, requirements.ErrDuplicateTitle):
			http.Error(w, "Такой документ уже требуется в этой группе.", http.StatusConflict)
		case errors.Is(err, requirements.ErrInvalidScope):
			http.NotFound(w, r)
		default:
			http.Error(w, "Не удалось добавить обязательный документ.", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/groups/"+strconv.FormatInt(groupID, 10), http.StatusSeeOther)
}

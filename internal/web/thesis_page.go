package web

import (
	"crypto/subtle"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/thesis"
)

// saveThesisTopic verifies student membership and CSRF before accepting an
// approved topic or a change backed by a new order reference.
// RU: saveThesisTopic проверяет группу/студента и новый приказ до записи темы.
func saveThesisTopic(w http.ResponseWriter, r *http.Request, groups GroupStore, people StudentStore, docs *DocumentServices, token string) {
	groupID, studentID, _, err := documentScope(r, groups, people, docs)
	if err != nil {
		documentError(w, r, err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "Требуется обычная HTML-форма.", http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Не удалось прочитать форму.", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return
	}
	topic := strings.TrimSpace(r.PostForm.Get("topic"))
	order := strings.TrimSpace(r.PostForm.Get("order_reference"))
	if topic == "" || order == "" {
		http.Error(w, "Заполните тему и реквизиты приказа.", http.StatusBadRequest)
		return
	}
	_, err = docs.Thesis.Set(r.Context(), studentID, topic, order)
	if err != nil {
		switch {
		case errors.Is(err, thesis.ErrOrderRequired):
			http.Error(w, "Изменение темы требует нового приказа.", http.StatusConflict)
		case errors.Is(err, thesis.ErrInvalidTopic):
			http.Error(w, "Введите тему ВКР.", http.StatusBadRequest)
		case errors.Is(err, thesis.ErrStudentNotFound):
			http.NotFound(w, r)
		default:
			http.Error(w, "Не удалось сохранить тему ВКР.", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/groups/"+strconv.FormatInt(groupID, 10)+"/students/"+strconv.FormatInt(studentID, 10), http.StatusSeeOther)
}

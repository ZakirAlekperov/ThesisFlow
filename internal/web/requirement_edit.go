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

// RequirementEditor is optional so existing read/create-only stores continue
// to work without changing their interfaces.
// RU: Редактирование подключается отдельно и не ломает тестовые имитаторы.
type RequirementEditor interface {
 Update(context.Context, int64, int64, string, string) (requirements.Requirement, error)
}

// updateGroupRequirement is a CSRF-protected, bounded HTML form handler.
// RU: Обработка изменения названия и срока общегруппового требования.
func updateGroupRequirement(w http.ResponseWriter, r *http.Request, groups GroupStore, editor RequirementEditor, token string) {
 groupID, err := parseGroupPathID(r)
 if err != nil { http.NotFound(w, r); return }
 requirementID, err := strconv.ParseInt(r.PathValue("requirement"), 10, 64)
 if err != nil || requirementID <= 0 { http.NotFound(w, r); return }
 kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
 if err != nil || kind != "application/x-www-form-urlencoded" {
  http.Error(w, "Ожидается HTML-форма.", http.StatusUnsupportedMediaType)
  return
 }
 r.Body = http.MaxBytesReader(w, r.Body, 4096)
 if err := r.ParseForm(); err != nil {
  http.Error(w, "Форма повреждена или превышен размер.", http.StatusBadRequest)
  return
 }
 if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
  http.Error(w, "Недействительный запрос.", http.StatusForbidden)
  return
 }
 if _, err := findGroup(r.Context(), groups, groupID); err != nil {
  if errors.Is(err, students.ErrGroupNotFound) { http.NotFound(w, r) } else {
   http.Error(w, "Не удалось проверить учебную группу.", http.StatusInternalServerError)
  }
  return
 }
 _, err = editor.Update(r.Context(), groupID, requirementID, r.PostForm.Get("title"), r.PostForm.Get("due_date"))
 if err != nil {
  switch {
  case errors.Is(err, requirements.ErrInvalidTitle), errors.Is(err, requirements.ErrInvalidDate):
   http.Error(w, "Проверьте название и срок сдачи.", http.StatusBadRequest)
  case errors.Is(err, requirements.ErrDuplicateTitle):
   http.Error(w, "Такое название уже есть в группе.", http.StatusConflict)
  case errors.Is(err, requirements.ErrAcceptedRename):
   http.Error(w, "Название менять нельзя: по требованию есть принятый файл. Срок сдачи можно изменить отдельно.", http.StatusConflict)
  case errors.Is(err, requirements.ErrInvalidScope):
   http.NotFound(w, r)
  default:
   http.Error(w, "Не удалось сохранить изменения требования.", http.StatusInternalServerError)
  }
  return
 }
 http.Redirect(w, r, "/groups/"+strconv.FormatInt(groupID, 10), http.StatusSeeOther)
}

package web

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
	"github.com/ZakirAlekperov/ThesisFlow/internal/thesis"
)

// DocumentServices coordinates document files and reviews using existing stores.
// RU: DocumentServices объединяет хранилища документов и рецензирования.
type DocumentServices struct {
	Requirements *requirements.Repository
	Statuses     *materials.Repository
	Versions     *fileversions.Store
	Reviews      *review.Service
	Thesis       *thesis.Repository
}

type documentItem struct {
	Requirement requirements.Requirement
	Status      string
	Versions    []fileversions.Version
	History     []documentHistoryItem
	IsOverdue   bool
}

// documentHistoryItem is a read-only projection of a persisted material event.
// RU: documentHistoryItem отображает сохранённое событие без изменения БД.
type documentHistoryItem struct {
	Status string
	At     string
}

type documentPage struct {
	FullName     string
	GroupName    string
	EducationalID string
	GroupID      int64
	StudentID    int64
	CSRFToken    string
	Documents    []documentItem
	ThesisTopic  string
	CanEditTopic bool
	OrderRef     string
	Total        int
	Accepted     int
	UnderReview  int
	NeedsRevision int
	Missing      int
	Overdue      int
	Progress     int
}

// The separate template keeps the card layout readable and testable.
// RU: Разметка карточки вынесена в отдельный Go-шаблон.


// documentScope verifies route IDs against the group, student, and requirement.
// RU: documentScope сверяет группу, студента и применимость документа.
func documentScope(r *http.Request, groups GroupStore, people StudentStore, docs *DocumentServices) (int64, int64, int64, error) {
	groupID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || groupID <= 0 {
		return 0, 0, 0, students.ErrGroupNotFound
	}
	studentID, err := strconv.ParseInt(r.PathValue("student"), 10, 64)
	if err != nil || studentID <= 0 {
		return 0, 0, 0, students.ErrGroupNotFound
	}
	if _, err = findGroup(r.Context(), groups, groupID); err != nil {
		return 0, 0, 0, err
	}
	items, err := people.ListByGroup(r.Context(), groupID)
	if err != nil {
		return 0, 0, 0, err
	}
	exists := false
	for _, person := range items {
		if person.ID == studentID {
			exists = true
			break
		}
	}
	if !exists {
		return 0, 0, 0, students.ErrGroupNotFound
	}
	requirementID := int64(0)
	if value := r.PathValue("requirement"); value != "" {
		requirementID, err = strconv.ParseInt(value, 10, 64)
		if err != nil || requirementID <= 0 {
			return 0, 0, 0, students.ErrGroupNotFound
		}
		expected, err := docs.Requirements.ListForStudent(r.Context(), groupID, studentID)
		if err != nil {
			return 0, 0, 0, err
		}
		found := false
		for _, item := range expected {
			if item.ID == requirementID {
				found = true
				break
			}
		}
		if !found {
			return 0, 0, 0, students.ErrGroupNotFound
		}
	}
	return groupID, studentID, requirementID, nil
}

func documentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, students.ErrGroupNotFound) || errors.Is(err, requirements.ErrInvalidScope) || errors.Is(err, fileversions.ErrInvalidScope) || errors.Is(err, review.ErrInvalidVersion) {
		http.NotFound(w, r)
		return
	}
	http.Error(w, "Не удалось обработать документ.", http.StatusInternalServerError)
}

func renderDocuments(w http.ResponseWriter, r *http.Request, groups GroupStore, people StudentStore, docs *DocumentServices, token string) {
	groupID, studentID, _, err := documentScope(r, groups, people, docs)
	if err != nil {
		documentError(w, r, err)
		return
	}
	group, err := findGroup(r.Context(), groups, groupID)
	if err != nil {
		documentError(w, r, err)
		return
	}
	persons, err := people.ListByGroup(r.Context(), groupID)
	if err != nil {
		documentError(w, r, err)
		return
	}
	title := ""
	educationalID := ""
	for _, person := range persons {
		if person.ID == studentID {
			title = person.FullName
			educationalID = person.EducationalID
			break
		}
	}
	requirementsList, err := docs.Requirements.ListForStudent(r.Context(), groupID, studentID)
	if err != nil {
		documentError(w, r, err)
		return
	}
	data := documentPage{FullName: title, GroupName: group.Name, EducationalID: educationalID, GroupID: groupID, StudentID: studentID, CSRFToken: token}
	if docs.Thesis != nil {
		record, err := docs.Thesis.Get(r.Context(), studentID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		data.ThesisTopic = record.Topic
		data.CanEditTopic = true
		data.OrderRef = record.OrderReference
	}
	for _, requirement := range requirementsList {
		status, err := docs.Statuses.Get(r.Context(), studentID, requirement.ID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		versions, err := docs.Versions.List(r.Context(), studentID, requirement.ID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		labels := map[materials.Status]string{materials.Missing: "Не сдан", materials.UnderReview: "На проверке", materials.NeedsRevision: "На доработке", materials.Accepted: "Принят"}
		display := labels[status.Status]
		item := documentItem{Requirement: requirement, Status: display, Versions: versions}
		data.Total++
		switch status.Status {
		case materials.Accepted:
			data.Accepted++
		case materials.UnderReview:
			data.UnderReview++
		case materials.NeedsRevision:
			data.NeedsRevision++
		default:
			data.Missing++
		}
		// Dates are stored as ISO calendar days, not timestamps.
		// RU: Сроки — календарные даты; сравнение выполняется по местной дате.
		if requirement.DueDate != "" && requirement.DueDate < time.Now().Format("2006-01-02") && status.Status != materials.Accepted {
			item.IsOverdue = true
			data.Overdue++
		}
		events, err := docs.Statuses.History(r.Context(), studentID, requirement.ID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		for _, event := range events {
			item.History = append(item.History, documentHistoryItem{Status: labels[event.Status], At: event.At.Local().Format("02.01.2006 15:04")})
		}
		data.Documents = append(data.Documents, item)
	}
	if data.Total > 0 {
		data.Progress = data.Accepted * 100 / data.Total
	}
	var b bytes.Buffer
	if err := documentTemplate.Execute(&b, data); err != nil {
		documentError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b.Bytes())
}

func verifyDocumentToken(w http.ResponseWriter, r *http.Request, token string) bool {
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
		http.Error(w, "Недействительный запрос.", http.StatusForbidden)
		return false
	}
	return true
}

func handleDocumentAction(w http.ResponseWriter, r *http.Request, groups GroupStore, people StudentStore, docs *DocumentServices, token, action string) {
	groupID, studentID, requirementID, err := documentScope(r, groups, people, docs)
	if err != nil {
		documentError(w, r, err)
		return
	}
	page := fmt.Sprintf("/groups/%d/students/%d", groupID, studentID)
	if action == "download" {
		versionID, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
		if err != nil || versionID <= 0 {
			http.NotFound(w, r)
			return
		}
		versions, err := docs.Versions.List(r.Context(), studentID, requirementID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		var selected *fileversions.Version
		for i := range versions {
			if versions[i].ID == versionID {
				selected = &versions[i]
				break
			}
		}
		if selected == nil {
			http.NotFound(w, r)
			return
		}
		data, err := docs.Versions.Read(r.Context(), versionID)
		if err != nil {
			documentError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": selected.OriginalName}))
		_, _ = w.Write(data)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(fileversions.MaxFileSize)+65536)
	if action == "upload" {
		if err := r.ParseMultipartForm(int64(fileversions.MaxFileSize)); err != nil {
			http.Error(w, "Ошибка формы или превышен размер.", http.StatusRequestEntityTooLarge)
			return
		}
	} else {
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/x-www-form-urlencoded" {
			http.Error(w, "Неверный формат формы.", http.StatusUnsupportedMediaType)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Некорректная форма.", http.StatusBadRequest)
			return
		}
	}
	if !verifyDocumentToken(w, r, token) {
		return
	}
	switch action {
	case "upload":
		handle, header, err := r.FormFile("document")
		if err != nil {
			http.Error(w, "Выберите документ.", http.StatusBadRequest)
			return
		}
		defer handle.Close()
		data, err := io.ReadAll(io.LimitReader(handle, int64(fileversions.MaxFileSize)+1))
		if err != nil {
			documentError(w, r, err)
			return
		}
		if len(data) > fileversions.MaxFileSize {
			http.Error(w, "Файл слишком большой.", http.StatusRequestEntityTooLarge)
			return
		}
		_, err = docs.Versions.Save(r.Context(), studentID, requirementID, header.Filename, data)
	case "accept", "reject":
		versionID, parseErr := strconv.ParseInt(r.PathValue("version"), 10, 64)
		if parseErr != nil || versionID <= 0 {
			http.NotFound(w, r)
			return
		}
		if action == "accept" {
			_, err = docs.Reviews.Accept(r.Context(), studentID, requirementID, versionID)
		} else {
			err = docs.Reviews.Reject(r.Context(), studentID, requirementID, versionID)
		}
	}
	if err != nil {
		if errors.Is(err, fileversions.ErrTooLarge) {
			http.Error(w, "Файл слишком большой.", http.StatusRequestEntityTooLarge)
			return
		}
		if errors.Is(err, fileversions.ErrInvalidFile) {
			http.Error(w, "Недопустимый формат файла.", http.StatusBadRequest)
			return
		}
		documentError(w, r, err)
		return
	}
	http.Redirect(w, r, page, http.StatusSeeOther)
}

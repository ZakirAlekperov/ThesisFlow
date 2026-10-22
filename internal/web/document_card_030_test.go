package web

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"net/url"
)

// TestThesisWorkspaceSummary verifies that the card starts with a useful
// teacher-facing progress overview rather than an unstructured upload list.
// RU: Проверяет сводку, навигацию и учебный идентификатор в карточке ВКР.
func TestThesisWorkspaceSummary(t *testing.T) {
	h, groupID, studentID, requirementID, _, _ := documentFixture(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, documentURL(groupID, studentID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET card = %d: %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		"ИСП-401", "ЗК-0017", "0 из 1 принято", "Не сдано: 1",
		fmt.Sprintf("href=\"#requirement-%d\"", requirementID),
		fmt.Sprintf("id=\"requirement-%d\"", requirementID),
		"Тема ВКР и приказ",
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("card lacks %q", want)
		}
	}
}

// TestThesisWorkspaceReflectsRealStatuses verifies progress is derived from
// materials in SQLite and immediately reflects a submitted and approved file.
// RU: Статистика карточки отражает реальную загрузку и приёмку.
func TestThesisWorkspaceReflectsRealStatuses(t *testing.T) {
	h, groupID, studentID, requirementID, statuses, versions := documentFixture(t)
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("csrf_token", token); err != nil {
		t.Fatal(err)
	}
	file, err := mw.CreateFormFile("document", "chapter.docx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("chapter bytes")); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/documents/%d/upload", page, requirementID), &body)
	request.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload = %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page, nil))
	if !strings.Contains(rec.Body.String(), "На проверке: 1") {
		t.Fatalf("uploaded document absent from review counter: %s", rec.Body.String())
	}
	list, err := versions.List(context.Background(), studentID, requirementID)
	if err != nil || len(list) != 1 {
		t.Fatalf("versions = %v, %v", list, err)
	}
	values := url.Values{"csrf_token": {token}}
	approveURL := fmt.Sprintf("%s/documents/%d/accept/%d", page, requirementID, list[0].ID)
	approve := httptest.NewRequest(http.MethodPost, approveURL, strings.NewReader(values.Encode()))
	approve.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, approve)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("accept = %d: %s", rec.Code, rec.Body.String())
	}
	entry, err := statuses.Get(context.Background(), studentID, requirementID)
	if err != nil || string(entry.Status) != "accepted" {
		t.Fatalf("stored approval: %+v %v", entry, err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page, nil))
	if !strings.Contains(rec.Body.String(), "1 из 1 принято") {
		t.Errorf("accepted counter not updated: %s", rec.Body.String())
	}
}

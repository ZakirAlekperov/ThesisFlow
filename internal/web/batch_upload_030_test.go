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

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
)

type batchTestFile struct {
	field string
	name string
	data string
}

// sendBatchRequest uses a real multipart/form-data request and the normal UI route.
// RU: Реальная multipart-форма, без внесения фиктивных записей напрямую в БД.
func sendBatchRequest(t *testing.T, h http.Handler, page, csrf string, files []batchTestFile) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("csrf_token", csrf); err != nil { t.Fatal(err) }
	for _, item := range files {
		dst, err := writer.CreateFormFile(item.field, item.name)
		if err != nil { t.Fatal(err) }
		if _, err := dst.Write([]byte(item.data)); err != nil { t.Fatal(err) }
	}
	if err := writer.Close(); err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, page+"/documents/batch", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func TestBatchUploadAssignsDistinctFilesToRequirements(t *testing.T) {
	h, groupID, studentID, first, statuses, versions := documentFixture(t)
	ctx := context.Background()
	second, err := requirements.NewRepository(versions.DB).Create(ctx, groupID, 0, "Глава 2", "")
	if err != nil { t.Fatal(err) }
	page := documentURL(groupID, studentID)
	token := documentToken(t, h, page)
	render := httptest.NewRecorder()
	h.ServeHTTP(render, httptest.NewRequest(http.MethodGet, page, nil))
	for _, want := range []string{"/documents/batch", fmt.Sprintf("name=\"document_%d\"", first), fmt.Sprintf("name=\"document_%d\"", second.ID)} {
		if !strings.Contains(render.Body.String(), want) { t.Errorf("missing batch field %q", want) }
	}

	res := sendBatchRequest(t, h, page, token, []batchTestFile{
		{fmt.Sprintf("document_%d", first), "Первая глава.docx", "chapter one"},
		{fmt.Sprintf("document_%d", second.ID), "Вторая глава.pdf", "%PDF-test"},
	})
	if res.Code != http.StatusSeeOther { t.Fatalf("batch upload = %d: %s", res.Code, res.Body.String()) }
	for _, reqID := range []int64{first, second.ID} {
		list, err := versions.List(ctx, studentID, reqID)
		if err != nil || len(list) != 1 { t.Fatalf("requirement %d versions=%v err=%v", reqID, list, err) }
		status, err := statuses.Get(ctx, studentID, reqID)
		if err != nil || status.Status != materials.UnderReview { t.Errorf("requirement %d status=%+v err=%v", reqID, status, err) }
	}
}

// All form inputs are verified before writing anything. Mixed valid and bad
// entries must never silently import the valid entry first.
// RU: Некорректная партия не изменяет ни один документ.
func TestBatchUploadRejectsInvalidWholeSubmission(t *testing.T) {
	tests := []struct {
		name string
		token string
		files func(int64) []batchTestFile
		want int
	}{
		{"forged token", "wrong", func(id int64) []batchTestFile { return []batchTestFile{{fmt.Sprintf("document_%d", id), "chapter.docx", "one"}} }, http.StatusForbidden},
		{"unknown requirement", "", func(id int64) []batchTestFile { return []batchTestFile{{fmt.Sprintf("document_%d", id), "chapter.docx", "one"}, {"document_999999", "evil.docx", "two"}} }, http.StatusBadRequest},
		{"untrusted field", "", func(id int64) []batchTestFile { return []batchTestFile{{fmt.Sprintf("document_%d", id), "chapter.docx", "one"}, {"document_other", "evil.docx", "two"}} }, http.StatusBadRequest},
		{"invalid extension", "", func(id int64) []batchTestFile { return []batchTestFile{{fmt.Sprintf("document_%d", id), "chapter.exe", "one"}} }, http.StatusBadRequest},
		{"duplicate field", "", func(id int64) []batchTestFile { return []batchTestFile{{fmt.Sprintf("document_%d", id), "one.docx", "one"}, {fmt.Sprintf("document_%d", id), "two.docx", "two"}} }, http.StatusBadRequest},
		{"no files", "", func(id int64) []batchTestFile { return nil }, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, groupID, studentID, id, _, versions := documentFixture(t)
			page := documentURL(groupID, studentID)
			csrf := tt.token
			if csrf == "" { csrf = documentToken(t, h, page) }
			res := sendBatchRequest(t, h, page, csrf, tt.files(id))
			if res.Code != tt.want { t.Errorf("response=%d, want %d: %s", res.Code, tt.want, res.Body.String()) }
			list, err := versions.List(context.Background(), studentID, id)
			if err != nil || len(list) != 0 { t.Fatalf("invalid batch wrote versions: %v %v", list, err) }
		})
	}
}

func TestBatchUploadRejectsWrongStudentScope(t *testing.T) {
	h, groupID, _, id, _, versions := documentFixture(t)
	page := documentURL(groupID, 9876543)
	res := sendBatchRequest(t, h, page, "invalid", []batchTestFile{{fmt.Sprintf("document_%d", id), "test.docx", "one"}})
	if res.Code != http.StatusNotFound { t.Fatalf("wrong student scope = %d", res.Code) }
	list, err := versions.List(context.Background(), 9876543, id)
	if err != nil || len(list) != 0 { t.Fatalf("unexpected cross-scope file: %v %v", list, err) }
}


package web

import (
 "bytes"
 "context"
 "fmt"
 "mime/multipart"
 "net/http"
 "net/http/httptest"
 "net/url"
 "strings"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/materials"
 "github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
 "github.com/ZakirAlekperov/ThesisFlow/internal/statusmatrix"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestMatrixImmediatelyReflectsAcceptedDocument reproduces the reported
// status-delay regression against the actual SQLite and HTTP workflow.
// RU: После первого принятия сразу обновляется ведомость, не после второго.
func TestMatrixImmediatelyReflectsAcceptedDocument(t *testing.T) {
 h, groupID, studentID, ruleID, _, versions := documentFixture(t)
 page := documentURL(groupID, studentID)
 token := documentToken(t, h, page)

 // A fresh independent HTTP handler reads the very same persisted SQLite DB.
 matrix, err := NewHandlerWithServices(
  groups.NewRepository(versions.DB), students.NewRepository(versions.DB),
  Dependencies{Matrix:statusmatrix.New(
   students.NewRepository(versions.DB),
   requirements.NewRepository(versions.DB),
   materials.NewRepository(versions.DB),
  )},
 )
 if err != nil { t.Fatal(err) }
 matrixURL := fmt.Sprintf("/groups/%d/matrix", groupID)
 fetch := func(want string) {
  t.Helper()
  rec := httptest.NewRecorder()
  matrix.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, matrixURL, nil))
  if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
   t.Fatalf("matrix HTTP %d missing %q: %s", rec.Code, want, rec.Body.String())
  }
  if rec.Header().Get("Cache-Control") != "no-store" { t.Fatalf("matrix response must not be cached") }
 }
 fetch("Не сдан")
 var buf bytes.Buffer
 mw := multipart.NewWriter(&buf)
 _ = mw.WriteField("csrf_token", token)
 file, err := mw.CreateFormFile("document", "Введение.docx")
 if err != nil { t.Fatal(err) }
 _, _ = file.Write([]byte("valid version"))
 if err := mw.Close(); err != nil { t.Fatal(err) }
 upload := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/documents/%d/upload", page, ruleID), &buf)
 upload.Header.Set("Content-Type", mw.FormDataContentType())
 rec := httptest.NewRecorder()
 h.ServeHTTP(rec, upload)
 if rec.Code != http.StatusSeeOther { t.Fatalf("upload: %d: %s", rec.Code, rec.Body.String()) }
 fetch("На проверке")
 items, err := versions.List(context.Background(), studentID, ruleID)
 if err != nil || len(items) != 1 { t.Fatalf("stored revisions: %v %v", items, err) }
 approve := url.Values{"csrf_token":{token}}
 req := httptest.NewRequest(http.MethodPost,
  fmt.Sprintf("%s/documents/%d/accept/%d", page, ruleID, items[0].ID),
  strings.NewReader(approve.Encode()))
 req.Header.Set("Content-Type","application/x-www-form-urlencoded")
 rec = httptest.NewRecorder()
 h.ServeHTTP(rec,req)
 if rec.Code != http.StatusSeeOther { t.Fatalf("approve: %d: %s", rec.Code,rec.Body.String()) }
 fetch("Принят")
 rec = httptest.NewRecorder()
 h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,page+"?review=accepted",nil))
 if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(),"Документ принят") {
  t.Fatalf("after acceptance card not refreshed: %d: %s",rec.Code,rec.Body.String())
 }
}

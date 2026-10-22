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
)

// uploadPreviewFixture posts a version through the actual browser route,
// including multipart bounds and CSRF, rather than manipulating tables.
// RU: Сохраняем тестовую версию через настоящий HTTP-обработчик загрузки.
func uploadPreviewFixture(t *testing.T, h http.Handler, page, csrf string, reqID int64, name string, content []byte) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	if err := w.WriteField("csrf_token", csrf); err != nil {
		t.Fatal(err)
	}
	f, err := w.CreateFormFile("document", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s/documents/%d/upload", page, reqID), &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload %s HTTP %d %s", name, rec.Code, rec.Body.String())
	}
}

// TestSafeDocumentPreviewChecksFormatsAndScope checks browser-readable PDF/TXT
// response headers, forbidden markup execution, invalid PDF, and DOCX fallback.
// RU: Сценарий предпросмотра не исполняет HTML или макросы документов.
func TestSafeDocumentPreviewChecksFormatsAndScope(t *testing.T) {
	h, groupID, studentID, reqID, _, versions := documentFixture(t)
	page := documentURL(groupID, studentID)
	csrf := documentToken(t, h, page)
	cases := []struct {
		name           string
		data           []byte
		expectedStatus int
		mime           string
	}{
		{"sample.pdf", []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF"), http.StatusOK, "application/pdf"},
		{"notes.txt", []byte("<script>alert('x')</script>"), http.StatusOK, "text/plain; charset=utf-8"},
		{"chapter.docx", []byte("not a parsed Office file"), http.StatusUnsupportedMediaType, "text/plain; charset=utf-8"},
		{"fake.pdf", []byte("<html>Not PDF</html>"), http.StatusUnsupportedMediaType, "text/plain; charset=utf-8"},
	}
	for _, tc := range cases {
		uploadPreviewFixture(t, h, page, csrf, reqID, tc.name, tc.data)
		all, err := versions.List(context.Background(), studentID, reqID)
		if err != nil || len(all) == 0 {
			t.Fatalf("versions: %v", err)
		}
		v := all[len(all)-1]
		route := fmt.Sprintf("%s/documents/%d/versions/%d/preview", page, reqID, v.ID)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route, nil))
		if rec.Code != tc.expectedStatus {
			t.Errorf("%s status %d want %d: %s", tc.name, rec.Code, tc.expectedStatus, rec.Body.String())
		}
		if tc.expectedStatus == http.StatusOK {
			if rec.Header().Get("Content-Type") != tc.mime {
				t.Errorf("%s content type %q", tc.name, rec.Header().Get("Content-Type"))
			}
			if rec.Header().Get("Content-Security-Policy") != "sandbox" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
				rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s unsafe preview headers: %v", tc.name, rec.Header())
			}
			if !bytes.Equal(rec.Body.Bytes(), tc.data) {
				t.Errorf("%s content bytes changed", tc.name)
			}
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/documents/%d/versions/%d/preview", page, reqID+999, 1), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign requirement HTTP %d", rec.Code)
	}
	pageRec := httptest.NewRecorder()
	h.ServeHTTP(pageRec, httptest.NewRequest(http.MethodGet, page, nil))
	html := pageRec.Body.String()
	if !strings.Contains(html, "Предпросмотр PDF/TXT") {
		t.Fatal("PDF/TXT preview link missing")
	}
	if strings.Count(html, "Предпросмотр PDF/TXT") != 3 {
		t.Errorf("expected links for two PDFs and one TXT, no DOCX: %s", html)
	}
}

// TestPreviewRejectsDamagedUTF8 ensures untrusted text cannot be passed to the
// browser with a misleading UTF-8 content type.
// RU: Повреждённый TXT отклоняется до передачи браузеру.
func TestPreviewRejectsDamagedUTF8(t *testing.T) {
	rec := httptest.NewRecorder()
	serveSafePreview(rec, "broken.txt", []byte{0xff, 0xfe})
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("broken UTF8 HTTP %d", rec.Code)
	}
}

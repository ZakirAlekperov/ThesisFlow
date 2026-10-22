package web

import (
	"bytes"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// canPreviewName intentionally allows only formats browsers can display
// locally without a server-side office document converter.
// RU: DOCX/ODT/RTF/XLSX не интерпретируем и не исполняем в браузере.
func canPreviewName(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf", ".txt":
		return true
	default:
		return false
	}
}

// serveSafePreview streams already verified version bytes with a restrictive
// content type, CSP sandbox, no-sniff and inline filename. It does not
// execute document macros, scripts, HTML or external renderers.
// RU: Безопасный предпросмотр только PDF/TXT с защитными HTTP-заголовками.
func serveSafePreview(w http.ResponseWriter, name string, data []byte) {
	extension := strings.ToLower(filepath.Ext(name))
	contentType := ""
	switch extension {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			http.Error(w, "Неверная сигнатура PDF.", http.StatusUnsupportedMediaType)
			return
		}
		contentType = "application/pdf"
	case ".txt":
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			http.Error(w, "Повреждён текстовый файл.", http.StatusUnsupportedMediaType)
			return
		}
		contentType = "text/plain; charset=utf-8"
	default:
		http.Error(w, "Предпросмотр этого формата недоступен. Скачайте исходный документ.", http.StatusUnsupportedMediaType)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	_, _ = w.Write(data)
}

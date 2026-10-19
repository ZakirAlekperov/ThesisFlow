// Package web provides HTTP handlers for the local ThesisFlow interface.
// Пакет web содержит HTTP-обработчики локального интерфейса ThesisFlow.
package web

import (
	"io"
	"net/http"
)

const homePageHTML = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ThesisFlow</title>
</head>
<body>
  <main>
    <h1>ThesisFlow</h1>
    <p>Локальный помощник преподавателя для учёта материалов ВКР.</p>
  </main>
</body>
</html>
`

// NewHandler creates a handler for the first local application page.
// It serves GET / and leaves unknown paths or unsupported methods to ServeMux.
//
// RU: NewHandler создаёт обработчик первой локальной страницы приложения.
// Он обслуживает GET /; неизвестные пути и методы обрабатывает ServeMux.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, homePageHTML)
	})
	return mux
}

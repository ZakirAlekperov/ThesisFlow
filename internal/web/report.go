package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/readiness"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// ReportProvider calculates progress using current persisted document statuses.
// RU: ReportProvider вычисляет готовность по текущим сохранённым статусам.
type ReportProvider interface {
	Group(context.Context, int64, time.Time) (readiness.Summary, error)
}

// printReportData supplies the group title and verified progress data.
// RU: printReportData передаёт название группы и показатели в HTML-шаблон.
type printReportData struct {
	GroupName string
	Metrics   readiness.Summary
	AsOf      string
}

// printReportTemplate is readable on screen and when printed to PDF or paper.
// It intentionally needs no JavaScript or external CSS resources.
//
// RU: printReportTemplate читаем на экране и при печати в PDF или на бумаге.
// Внешние стили и JavaScript не нужны.
var printReportTemplate = template.Must(template.New("print-report").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Ведомость {{.GroupName}} — ThesisFlow</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 850px; margin: 48px auto; padding: 0 20px; color: #202939; }
    h1 { font-size: 1.8rem; }
    .metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin: 28px 0; }
    .metric { padding: 20px; border: 1px solid #d5dce7; border-radius: 12px; }
    .metric strong { display: block; font-size: 1.8rem; margin-top: 8px; }
    a { color: #214eaf; }
    .note { color: #5c6878; }
    @media print {
      @page { size: A4; margin: 16mm; }
      body { margin: 0; padding: 0; max-width: none; color: black; }
      .navigation { display: none; }
      .metric { break-inside: avoid; }
    }
  </style><link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
</head>
<body>
  <p class="navigation"><a href="/groups/{{.Metrics.GroupID}}">← К группе</a></p>
  <h1>Ведомость группы {{.GroupName}}</h1>
  <p class="note">Состояние на {{.AsOf}}. Готовность определяется только принятыми документами.</p>
  <section class="metrics" aria-label="Показатели">
    <div class="metric">Студентов<strong>{{.Metrics.StudentCount}}</strong></div>
    <div class="metric">Документов требуется<strong>{{.Metrics.Required}}</strong></div>
    <div class="metric">Принято<strong>{{.Metrics.Accepted}}</strong></div>
    <div class="metric">Просрочено<strong>{{.Metrics.Overdue}}</strong></div>
    <div class="metric">Готовность<strong>{{.Metrics.Percent}}%</strong></div>
  </section>
  <p class="note">Сохраните страницу в PDF через команду печати браузера.</p>
</body>
</html>
`))

// renderPrintableReport verifies the group and renders a print-ready summary.
// Unknown groups return 404; errors do not reveal internal database details.
//
// RU: renderPrintableReport проверяет существование группы и выводит ведомость
// для печати. Неизвестные группы дают 404, ошибки SQLite не раскрываются.
func renderPrintableReport(w http.ResponseWriter, r *http.Request, groups GroupStore, service ReportProvider) {
	id, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	group, err := findGroup(r.Context(), groups, id)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось загрузить ведомость.", http.StatusInternalServerError)
		}
		return
	}
	now := time.Now()
	report, err := service.Group(r.Context(), id, now)
	if err != nil {
		http.Error(w, "Не удалось рассчитать показатели.", http.StatusInternalServerError)
		return
	}
	var body bytes.Buffer
	if err := printReportTemplate.Execute(&body, printReportData{
		GroupName: group.Name,
		Metrics:   report,
		AsOf:      now.Format("02.01.2006"),
	}); err != nil {
		http.Error(w, "Не удалось сформировать ведомость.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body.Bytes())
}

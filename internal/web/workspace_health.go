package web

import (
	"bytes"
	"context"
	"html/template"
	"net/http"

	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)

// WorkspaceDoctor provides read-only SQL and file integrity diagnostics.
// RU: Диагностика не удаляет файлы и не меняет статусы.
type WorkspaceDoctor interface {
	Check(context.Context) (workspace.Report, error)
}

type healthPage struct {
	Healthy bool
	Report  workspace.Report
}

var healthTemplate = template.Must(template.New("health").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Состояние хранилища — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:1000px;margin:32px auto;padding:0 18px 32px}
.panel{padding:20px;border:1px solid var(--tf-border);border-radius:13px;background:var(--tf-panel);margin:16px 0}
table{border-collapse:collapse;width:100%}td,th{text-align:left;border-bottom:1px solid var(--tf-border);padding:10px}
.alert{font-weight:700} .health-result{font-size:1.2rem;font-weight:700}
</style></head><body>
<nav><a href="/settings">← Настройки</a> · <a href="/dashboard">Панель преподавателя</a></nav>
<h1>Состояние хранилища</h1>
<p class="muted">Проверяются SQLite, неизменяемые версии документов и принятые копии. Диагностика ничего не исправляет автоматически и не удаляет неизвестные файлы.</p>
<section class="panel">
<p class="health-result">{{if .Healthy}}Проверка пройдена: обнаруженных нарушений нет{{else}}Обнаружены нарушения: требуется внимание{{end}}</p>
<p>Версия схемы SQLite: {{.Report.SchemaVersion}}</p>
<ul><li>Версии документов: {{.Report.VersionsIntact}} из {{.Report.VersionsChecked}} целы</li>
<li>Принятые копии: {{.Report.AcceptedIntact}} из {{.Report.AcceptedChecked}} целы</li><li>Незарегистрированные элементы хранилища: {{.Report.ExtraFiles}}</li></ul>
{{if gt .Report.ExtraFiles 0}}<p>Найдены неучтённые файлы. Они не удаляются автоматически; проверьте их вручную перед переносом или очисткой.</p>{{end}}</section>
{{if .Report.Issues}}
<section class="panel"><h2>Выявленные проблемы</h2>
<div style="overflow-x:auto"><table><thead><tr><th scope="col">Категория</th><th scope="col">Версия №</th><th scope="col">Требование №</th><th scope="col">Описание</th></tr></thead>
<tbody>{{range .Report.Issues}}<tr><td>{{.Code}}</td><td>{{.VersionID}}</td><td>{{.RequirementID}}</td><td>{{.Detail}}</td></tr>{{end}}</tbody></table></div>
<p class="alert">Не заменяйте повреждённые файлы вручную и не удаляйте неизвестные файлы. Сделайте отдельную резервную копию перед восстановлением.</p>
</section>{{end}}
<p class="muted">Примечание: это проверка на момент открытия страницы. Она не гарантирует защиту от будущих сбоев диска или файловой системы.</p>
</body></html>`))

// renderWorkspaceHealth reports actual integrity results and no student names.
// RU: Показываем реальное состояние файлов, скрывая ФИО и пути.
func renderWorkspaceHealth(w http.ResponseWriter, r *http.Request, doctor WorkspaceDoctor) {
	report, err := doctor.Check(r.Context())
	if err != nil {
		http.Error(w, "Проверка хранилища не завершена: проверьте журнал приложения.", http.StatusInternalServerError)
		return
	}
	var output bytes.Buffer
	if err := healthTemplate.Execute(&output, healthPage{Healthy: report.Healthy(), Report: report}); err != nil {
		http.Error(w, "Не удалось показать результаты.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(output.Bytes())
}

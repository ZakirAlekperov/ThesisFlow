package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/statusmatrix"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// MatrixProvider builds a current read-only status matrix for one group.
// RU: MatrixProvider формирует актуальную ведомость одной учебной группы.
type MatrixProvider interface {
	Group(context.Context, int64) (statusmatrix.Matrix, error)
}

type matrixCellView struct {
	Label string
	Key   materials.Status
}
type matrixRowView struct {
	ID            int64
	Name          string
	EducationalID string
	Cells         []matrixCellView
}
type matrixPageData struct {
	GroupID     int64
	GroupName   string
	Columns     []string
	Rows        []matrixRowView
	Filter      string
	Total       int
	Shown       int
	ColumnCount int
}

// matrixTemplate uses Go html/template, custom CSS and no external frontend.
// RU: matrixTemplate использует стандартный шаблонизатор Go и собственный CSS.
var matrixTemplate = template.Must(template.New("matrix").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ведомость группы {{.GroupName}} — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px system-ui,sans-serif;max-width:1250px;margin:32px auto;padding:0 16px}
.grid{overflow-x:auto;border:1px solid #dce3ed;border-radius:12px;background:var(--surface,#fff)}
table{border-collapse:collapse;min-width:100%;font-size:0.88rem}th,td{border:1px solid #dce3ed;padding:12px;text-align:left;white-space:nowrap}
thead{background:var(--surface-secondary,#eef3f9)}th:first-child{position:sticky;left:0;background:var(--surface,#fff);z-index:1}
.matrix-filter{display:flex;flex-wrap:wrap;align-items:center;gap:10px;padding:14px 0}
.matrix-filter select,.matrix-filter button{padding:9px 12px;border-radius:8px;font:inherit}
.matrix-filter button{background:#2456a9;color:white;border:0}
.accepted{color:#166534;font-weight:700}.under_review{color:#1d4ed8}.needs_revision{color:#b45309}.missing{color:#687488}
@media print{body{max-width:none;margin:0}thead{display:table-header-group}th:first-child{position:static}.grid{overflow:visible}table{font-size:0.7rem}th,td{padding:5px}}
</style></head><body>
<nav><a href="/groups/{{.GroupID}}">← Группа {{.GroupName}}</a></nav>
<h1>Ведомость сдачи документов</h1><p>Группа: {{.GroupName}}. Отображаются общие требования группы и текущие статусы студентов.</p>
<form class="matrix-filter" action="/groups/{{.GroupID}}/matrix" method="get">
<label for="matrix-status">Показать студентов по состоянию документа</label>
<select id="matrix-status" name="status">
<option value="" {{if eq .Filter ""}}selected{{end}}>Все студенты</option>
<option value="missing" {{if eq .Filter "missing"}}selected{{end}}>Не сдан</option>
<option value="under_review" {{if eq .Filter "under_review"}}selected{{end}}>На проверке</option>
<option value="needs_revision" {{if eq .Filter "needs_revision"}}selected{{end}}>На доработке</option>
<option value="accepted" {{if eq .Filter "accepted"}}selected{{end}}>Принят (хотя бы один документ)</option>
<option value="overdue" {{if eq .Filter "overdue"}}selected{{end}}>Просрочен</option>
</select>
<button type="submit">Применить</button>
</form>
<p role="status">Показано {{.Shown}} из {{.Total}} студентов.</p>
<div class="grid"><table><thead><tr><th scope="col">Студент</th><th scope="col">Учебный ID</th>{{range .Columns}}<th scope="col">{{.}}</th>{{end}}</tr></thead>
<tbody>{{range .Rows}}<tr><th scope="row"><a href="/groups/{{$.GroupID}}/students/{{.ID}}">{{.Name}}</a></th>
<td>{{.EducationalID}}</td>{{range .Cells}}<td class="{{.Key}}">{{.Label}}</td>{{end}}</tr>{{else}}
<tr><td colspan="{{$.ColumnCount}}">{{if $.Filter}}Нет студентов с выбранным состоянием.{{else}}В группе пока нет студентов.{{end}}</td></tr>{{end}}</tbody></table></div>
<p>Индивидуальные требования отображаются на страницах соответствующих студентов.</p>
</body></html>`))

// materialLabel translates database statuses into concise Russian text.
// RU: materialLabel преобразует ключи статусов для пользовательской таблицы.
func materialLabel(status materials.Status) string {
	switch status {
	case materials.Accepted:
		return "Принят"
	case materials.UnderReview:
		return "На проверке"
	case materials.NeedsRevision:
		return "На доработке"
	default:
		return "Не сдан"
	}
}

// renderGroupMatrix verifies the group and renders the current document grid.
// Unknown groups produce HTTP 404; storage errors are not disclosed.
//
// RU: renderGroupMatrix проверяет группу и отображает статусы из SQLite.
// Неизвестная группа даёт 404, подробности ошибок базы скрываются.
func renderGroupMatrix(w http.ResponseWriter, r *http.Request, groupsStore GroupStore, provider MatrixProvider) {
	id, err := parseGroupPathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	group, err := findGroup(r.Context(), groupsStore, id)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось открыть группу.", http.StatusInternalServerError)
		}
		return
	}
	filter := r.URL.Query().Get("status")
	switch filter {
	case "", string(materials.Missing), string(materials.UnderReview), string(materials.NeedsRevision), string(materials.Accepted), "overdue":
		// Known state filter; an empty value includes every student.
	default:
		http.Error(w, "Неизвестный фильтр ведомости.", http.StatusBadRequest)
		return
	}
	matrix, err := provider.Group(r.Context(), id)
	if err != nil {
		if errors.Is(err, students.ErrGroupNotFound) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Не удалось сформировать ведомость.", http.StatusInternalServerError)
		}
		return
	}
	page := matrixPageData{
		GroupID: id, GroupName: group.Name, Filter: filter,
		Total: len(matrix.Rows), ColumnCount: len(matrix.Requirements) + 2,
		Rows: make([]matrixRowView, 0, len(matrix.Rows)),
	}
	deadlines := make(map[int64]string, len(matrix.Requirements))
	for _, rule := range matrix.Requirements {
		page.Columns = append(page.Columns, rule.Title)
		deadlines[rule.ID] = rule.DueDate
	}
	localToday := time.Now().Format("2006-01-02")
	for _, row := range matrix.Rows {
		if filter != "" {
			matched := false
			for _, cell := range row.Cells {
				if filter == "overdue" {
					deadline := deadlines[cell.RequirementID]
					if deadline != "" && deadline < localToday && cell.Status != materials.Accepted {
						matched = true
						break
					}
				} else if string(cell.Status) == filter {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		page.Shown++
		item := matrixRowView{ID: row.Student.ID, Name: row.Student.FullName, EducationalID: row.Student.EducationalID}
		for _, cell := range row.Cells {
			item.Cells = append(item.Cells, matrixCellView{Label: materialLabel(cell.Status), Key: cell.Status})
		}
		page.Rows = append(page.Rows, item)
	}
	var out bytes.Buffer
	if err := matrixTemplate.Execute(&out, page); err != nil {
		http.Error(w, "Не удалось отобразить ведомость.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out.Bytes())
}

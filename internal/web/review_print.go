package web

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

type reviewPrintNote struct {
	Kind string
	VersionID int64
	At string
	Text string
}

type reviewPrintSection struct {
	Requirement string
	Notes []reviewPrintNote
}

type reviewPrintData struct {
	GroupID int64
	StudentID int64
	GroupName string
	StudentName string
	EducationalID string
	GeneratedAt string
	Sections []reviewPrintSection
	Total int
}

// reviewPrintTemplate deliberately has no remote assets or active content;
// teachers can print to PDF using the browser's built-in print dialog.
// RU: Печатная версия безопасно экранирует замечания html/template.
var reviewPrintTemplate=template.Must(template.New("review-print").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Журнал рецензирования — {{.StudentName}}</title>
<style>body{font:14px/1.6 system-ui,sans-serif;max-width:900px;margin:32px auto;padding:0 16px;color:#182339;background:#fff}
h1{font-size:1.7rem}h2{font-size:1.1rem;border-bottom:1px solid #bbc6d3;padding-bottom:7px}
article{break-inside:avoid;margin:12px 0;border:1px solid #d0d8e4;border-radius:9px;padding:13px}
pre{font:inherit;white-space:pre-wrap;overflow-wrap:anywhere}a{color:#225bb3}
@page{size:A4;margin:18mm}@media print{body{margin:0;padding:0;max-width:none;color:#000}nav{display:none}article{border:1px solid #777}}</style>
</head><body><nav><a href="/groups/{{.GroupID}}/students/{{.StudentID}}">← Назад</a></nav>
<h1>Журнал рецензирования ВКР</h1>
<p>Студент: <strong>{{.StudentName}}</strong><br>Учебный ID: {{.EducationalID}}<br>
Группа: {{.GroupName}}<br>Сформировано: {{.GeneratedAt}}</p>
<p>Замечаний: {{.Total}}. Отчёт содержит фактическую историю комментариев и причин возврата, но не является официальным заключением.</p>
{{range .Sections}}<section><h2>{{.Requirement}}</h2>
{{range .Notes}}<article><strong>{{.Kind}}</strong> · Версия #{{.VersionID}} · {{.At}}
<pre>{{.Text}}</pre></article>{{end}}</section>
{{else}}<p>Замечаний к документам пока нет.</p>{{end}}
<footer>ThesisFlow — локальная история рецензирования. Сохраните страницу в PDF через меню печати браузера.</footer>
</body></html>`))

// renderStudentReviewPrint creates a read-only printable history. It scopes
// student IDs through the existing documentScope and includes no file bytes.
// RU: Печать только замечаний текущего студента, без открытия других файлов.
func renderStudentReviewPrint(w http.ResponseWriter, r *http.Request, groupStore GroupStore, people StudentStore, docs *DocumentServices) {
	groupID, studentID, _, err:=documentScope(r,groupStore,people,docs)
	if err!=nil {documentError(w,r,err);return}
	group,err:=findGroup(r.Context(),groupStore,groupID)
	if errors.Is(err,students.ErrGroupNotFound){http.NotFound(w,r);return}
	if err!=nil {documentError(w,r,err);return}
	personList,err:=people.ListByGroup(r.Context(),groupID)
	if err!=nil {documentError(w,r,err);return}
	data:=reviewPrintData{GroupID:groupID,StudentID:studentID,GroupName:group.Name,GeneratedAt:time.Now().Format("02.01.2006 15:04")}
	for _,s:=range personList {
		if s.ID==studentID {data.StudentName=s.FullName;data.EducationalID=s.EducationalID;break}
	}
	rules,err:=docs.Requirements.ListForStudent(r.Context(),groupID,studentID)
	if err!=nil {documentError(w,r,err);return}
	for _,rule:=range rules {
		notes,err:=docs.Reviews.ListFeedback(r.Context(),studentID,rule.ID)
		if err!=nil {documentError(w,r,err);return}
		if len(notes)==0 {continue}
		section:=reviewPrintSection{Requirement:rule.Title}
		for _,n:=range notes {
			kind:="Комментарий"
			if n.Kind==review.FeedbackRevision {kind="Причина возврата"}
			section.Notes=append(section.Notes,reviewPrintNote{
				Kind:kind,VersionID:n.VersionID,
				At:n.CreatedAt.Local().Format("02.01.2006 15:04"),Text:n.Text,
			})
			data.Total++
		}
		data.Sections=append(data.Sections,section)
	}
	var b bytes.Buffer
	if err:=reviewPrintTemplate.Execute(&b,data);err!=nil {
		http.Error(w,"Не удалось подготовить печать.",http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type","text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy","default-src 'none'; style-src 'unsafe-inline'")
	_,_=w.Write(b.Bytes())
}

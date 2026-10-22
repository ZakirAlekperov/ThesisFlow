package web

import (
 "bytes"
 "context"
 "errors"
 "fmt"
 "html/template"
 "net/http"
 "net/url"
 "strconv"
 "strings"
 "time"

 "github.com/ZakirAlekperov/ThesisFlow/internal/analytics"
 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/reportexport"
)

// AnalyticsProvider supplies one coherent report and factual event timeline.
// RU: AnalyticsProvider отдаёт единые данные для HTML, CSV и XLSX.
type AnalyticsProvider interface {
 Build(context.Context,analytics.Filter,time.Time)(analytics.Report,error)
 Activity(context.Context,time.Time,time.Time,int64)([]analytics.Week,error)
}
type analyticsStudent struct {
 analytics.Student
 Percent int
 HasRequirements bool
}
type analyticsGroup struct {
 analytics.Group
 Percent int
 HasRequirements bool
}
type analyticsDocument struct {
 analytics.Document
 StatusLabel string
}
type analyticsWeek struct {
 analytics.Week
 UploadedWidth int
 AcceptedWidth int
 ReturnedWidth int
}
type analyticsPage struct {
 Report analytics.Report
 Students []analyticsStudent
 Groups []analyticsGroup
 Documents []analyticsDocument
 Weeks []analyticsWeek
 AllGroups []groups.Group
 Query template.URL
 From string
 To string
 DateMessage string
 TotalPercent int
 HasRequirements bool
 Print bool
}

// analyticsTemplate uses only offline server templates and concept C colors.
// Accessible tables remain usable without charts or JavaScript.
// RU: Таблица — доступная альтернатива графикам событий.
var analyticsTemplate=template.Must(template.New("analytics").Parse(`<!doctype html><html lang="ru"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Аналитика ВКР — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>
body{font:16px/1.5 system-ui,sans-serif;max-width:1250px;margin:30px auto;padding:0 18px 40px}
h1{font-size:1.8rem}.report-controls{display:flex;gap:12px;flex-wrap:wrap;align-items:end}
.report-controls label{display:grid;gap:5px}.report-controls input,.report-controls select,.report-controls button{font:inherit;padding:8px 11px;border-radius:8px}
.report-card{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:14px;padding:18px;margin:16px 0}
.report-metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(145px,1fr));gap:10px}
.report-number{font-size:1.6rem;font-weight:700;display:block}
.report-table{overflow-x:auto}table{width:100%;min-width:750px;border-collapse:collapse}
th,td{padding:10px;text-align:left;border-bottom:1px solid var(--tf-border)}
thead th{background:var(--tf-table-head)}.report-actions{display:flex;flex-wrap:wrap;gap:15px;margin:12px 0}
.report-bars{display:flex;flex-direction:column;gap:5px}
.report-track{height:13px;background:var(--tf-panel-alt);border-radius:8px;overflow:hidden}
.report-fill{display:block;height:100%;background:var(--tf-button)}
.report-fill.accept{background:var(--tf-success)}.report-fill.return{background:var(--tf-warning)}
@media print{
 body{max-width:none;margin:0;font-size:10pt;color:#000;background:#fff}
 .report-controls,.report-actions,nav{display:none!important}
 .report-card{break-inside:avoid;border-color:#aaa}
 table{font-size:8pt;min-width:0}tr{break-inside:avoid}
 @page{size:A4 landscape;margin:12mm}
}
</style></head><body>
<nav><a href="/dashboard">← Панель преподавателя</a> · <a href="/">Учебные группы</a></nav>
<h1>Аналитика и отчёты ВКР</h1>
<p class="muted">Текущая готовность по действующим требованиям. Архивные группы и студенты не входят в показатели. Сформировано: {{.Report.GeneratedAt.Format "02.01.2006 15:04"}}.</p>
<form class="report-controls report-card" action="/reports" method="get">
<label>Группа<select name="group"><option value="0">Все группы</option>
{{range .AllGroups}}<option value="{{.ID}}" {{if eq .ID $.Report.Filter.GroupID}}selected{{end}}>{{.Name}}</option>{{end}}</select></label>
<label>Поиск ФИО/ID<input name="q" value="{{.Report.Filter.Search}}" maxlength="120"></label>
<label>Состояние<select name="status"><option value="">Любое</option>
<option value="missing" {{if eq .Report.Filter.Status "missing"}}selected{{end}}>Не сдан</option>
<option value="under_review" {{if eq .Report.Filter.Status "under_review"}}selected{{end}}>На проверке</option>
<option value="needs_revision" {{if eq .Report.Filter.Status "needs_revision"}}selected{{end}}>На доработке</option>
<option value="accepted" {{if eq .Report.Filter.Status "accepted"}}selected{{end}}>Принят</option></select></label>
<label>Сортировка<select name="sort">
<option value="name">ФИО</option>
<option value="ready" {{if eq .Report.Filter.Sort "ready"}}selected{{end}}>Готовность ↑</option>
<option value="overdue" {{if eq .Report.Filter.Sort "overdue"}}selected{{end}}>Просрочки ↓</option>
<option value="deadline" {{if eq .Report.Filter.Sort "deadline"}}selected{{end}}>Ближайший срок</option>
</select></label>
<label><input type="checkbox" name="overdue" value="1" {{if .Report.Filter.OverdueOnly}}checked{{end}}>Только просроченные</label>
<label>Период от<input type="date" name="from" value="{{.From}}"></label>
<label>до<input type="date" name="to" value="{{.To}}"></label>
<button type="submit">Применить</button></form>
<div class="report-actions"><a href="/reports/export.csv?{{.Query}}">Скачать CSV</a>
<a href="/reports/export.xlsx?{{.Query}}">Скачать Excel (XLSX)</a>
<a href="/reports/print?{{.Query}}" target="_blank" rel="noopener">Печать / PDF</a></div>
<section class="report-metrics" aria-label="Основные показатели">
<div class="report-card">Студентов<strong class="report-number">{{.Report.Totals.Students}}</strong></div>
<div class="report-card">Обязательных документов<strong class="report-number">{{.Report.Totals.Required}}</strong></div>
<div class="report-card">Принято<strong class="report-number">{{.Report.Totals.Accepted}}</strong></div>
<div class="report-card">На проверке<strong class="report-number">{{.Report.Totals.UnderReview}}</strong></div>
<div class="report-card">На доработке<strong class="report-number">{{.Report.Totals.NeedsRevision}}</strong></div>
<div class="report-card">Не сдано<strong class="report-number">{{.Report.Totals.Missing}}</strong></div>
<div class="report-card">Просрочено<strong class="report-number">{{.Report.Totals.Overdue}}</strong></div>
<div class="report-card">Готовность<strong class="report-number">{{if .HasRequirements}}{{.TotalPercent}}%{{else}}Нет данных{{end}}</strong></div>
</section>
<section class="report-card"><h2>Сравнение учебных групп</h2>
<div class="report-table"><table><thead><tr><th>Группа</th><th>Студентов</th><th>Принято / всего</th><th>Просрочено</th><th>На проверке</th><th>Готовность</th></tr></thead><tbody>
{{range .Groups}}<tr><th scope="row"><a href="/reports/groups/{{.ID}}">{{.Name}}</a></th><td>{{.Students}}</td><td>{{.Accepted}} / {{.Required}}</td><td>{{.Overdue}}</td><td>{{.UnderReview}}</td><td>{{if .HasRequirements}}{{.Percent}}%{{else}}Нет данных{{end}}</td></tr>
{{else}}<tr><td colspan="6">Нет действующих групп.</td></tr>{{end}}</tbody></table></div></section>
<section class="report-card"><h2>Студенты</h2><div class="report-table"><table><thead><tr><th>Студент</th><th>Группа</th><th>Принято / всего</th><th>На проверке</th><th>Не сдано</th><th>Просрочено</th><th>Ближайший срок</th><th>Готовность</th></tr></thead><tbody>
{{range .Students}}<tr><th scope="row"><a href="/groups/{{.GroupID}}/students/{{.ID}}">{{.Name}}</a><div class="muted">{{.EducationalID}}</div></th><td>{{.GroupName}}</td><td>{{.Accepted}} / {{.Required}}</td><td>{{.UnderReview}}</td><td>{{.Missing}}</td><td>{{.Overdue}}</td><td>{{.NearestDue}}</td><td>{{if .HasRequirements}}{{.Percent}}%{{else}}Нет данных{{end}}</td></tr>
{{else}}<tr><td colspan="8">Нет студентов, соответствующих условиям.</td></tr>{{end}}</tbody></table></div></section>
<section class="report-card"><h2>Документы и сроки</h2><div class="report-table"><table><thead><tr><th>Студент</th><th>Требование</th><th>Статус</th><th>Срок</th><th>Просрочено, дней</th><th>Принятая копия</th></tr></thead><tbody>
{{range .Documents}}<tr><th scope="row"><a href="/groups/{{.GroupID}}/students/{{.StudentID}}#requirement-{{.RequirementID}}">{{.StudentName}}</a><small class="muted"> · {{.GroupName}}</small></th><td>{{.Title}}</td><td>{{.StatusLabel}}</td><td>{{.DueDate}}</td><td>{{if gt .OverdueDays 0}}{{.OverdueDays}}{{else}}—{{end}}</td><td>{{if .ApprovedCopy}}Есть{{else}}Нет{{end}}</td></tr>
{{else}}<tr><td colspan="6">Нет документов с выбранными условиями.</td></tr>{{end}}</tbody></table></div></section>
<section class="report-card"><h2>Динамика событий по неделям</h2>
<p class="muted">Реальные загрузки, решения о приёмке и возвраты за {{.From}} — {{.To}}. Это не реконструкция исторической готовности.</p>
<div class="report-table"><table><thead><tr><th>Неделя от</th><th>Загружено</th><th>Принято</th><th>Возвращено</th><th>Сравнение</th></tr></thead><tbody>
{{range .Weeks}}<tr><th scope="row">{{.Start}}</th><td>{{.Uploaded}}</td><td>{{.Accepted}}</td><td>{{.Returned}}</td><td><div class="report-bars">
<div class="report-track" aria-hidden="true"><span class="report-fill" style="width: {{.UploadedWidth}}%"></span></div>
<div class="report-track" aria-hidden="true"><span class="report-fill accept" style="width: {{.AcceptedWidth}}%"></span></div>
<div class="report-track" aria-hidden="true"><span class="report-fill return" style="width: {{.ReturnedWidth}}%"></span></div></div></td></tr>
{{else}}<tr><td colspan="5">За выбранный период событий нет.</td></tr>{{end}}</tbody></table></div>
</section></body></html>`))

func reportFilters(r *http.Request)(analytics.Filter,time.Time,time.Time,error){
 q:=r.URL.Query()
 f:=analytics.Filter{Search:strings.TrimSpace(q.Get("q")),Status:q.Get("status"),Sort:q.Get("sort"),OverdueOnly:q.Get("overdue")=="1"}
 if value:=q.Get("overdue");value!=""&&value!="1"{return f,time.Time{},time.Time{},analytics.ErrInvalidFilter}
 if raw:=q.Get("group");raw!=""{
  id,err:=strconv.ParseInt(raw,10,64)
  if err!=nil {return f,time.Time{},time.Time{},analytics.ErrInvalidFilter}
  f.GroupID=id
 }
 if raw:=r.PathValue("id");raw!=""{
  id,err:=strconv.ParseInt(raw,10,64)
  if err!=nil||id<=0 {return f,time.Time{},time.Time{},analytics.ErrInvalidFilter}
  f.GroupID=id
 }
 if err:=analytics.CheckFilter(f);err!=nil{return f,time.Time{},time.Time{},err}
 now:=time.Now()
 start:=now.AddDate(0,0,-30)
 end:=now
 if raw:=q.Get("from");raw!=""{
  parsed,err:=time.ParseInLocation("2006-01-02",raw,now.Location())
  if err!=nil||parsed.Format("2006-01-02")!=raw{return f,start,end,analytics.ErrInvalidFilter}
  start=parsed
 }
 if raw:=q.Get("to");raw!=""{
  parsed,err:=time.ParseInLocation("2006-01-02",raw,now.Location())
  if err!=nil||parsed.Format("2006-01-02")!=raw{return f,start,end,analytics.ErrInvalidFilter}
  end=parsed
 }
 if end.Before(start)||end.Sub(start)>366*24*time.Hour{return f,start,end,analytics.ErrInvalidFilter}
 return f,start,end,nil
}

func reportQuery(f analytics.Filter,start,end time.Time)template.URL{
 // Parameters are built from validated values, not user-provided raw URL.
 q:=url.Values{}
 if f.GroupID>0{q.Set("group",fmt.Sprint(f.GroupID))}
 if f.Search!=""{q.Set("q",f.Search)}
 if f.Status!=""{q.Set("status",f.Status)}
 if f.Sort!=""{q.Set("sort",f.Sort)}
 if f.OverdueOnly{q.Set("overdue","1")}
 q.Set("from",start.Format("2006-01-02"));q.Set("to",end.Format("2006-01-02"))
 return template.URL(q.Encode())
}
func reportFailure(w http.ResponseWriter,err error){
 switch {
 case errors.Is(err,analytics.ErrInvalidFilter):http.Error(w,"Неверные параметры отчёта.",http.StatusBadRequest)
 case errors.Is(err,analytics.ErrUnknownGroup):http.Error(w,"Группа не найдена.",http.StatusNotFound)
 default:http.Error(w,"Не удалось сформировать отчёт.",http.StatusInternalServerError)
 }
}

// renderAnalytics uses a single filtered snapshot for the page and its
// exported links; group-only URLs override the query-string group.
// RU: Страница и экспорт используют одинаковый фильтр и реальные данные.
func renderAnalytics(w http.ResponseWriter,r *http.Request,store GroupStore,provider AnalyticsProvider,print bool){
 f,start,end,err:=reportFilters(r)
 if err!=nil{reportFailure(w,err);return}
 now:=time.Now()
 data,err:=provider.Build(r.Context(),f,now)
 if err!=nil{reportFailure(w,err);return}
 history,err:=provider.Activity(r.Context(),start,end,f.GroupID)
 if err!=nil{reportFailure(w,err);return}
 allGroups,err:=store.List(r.Context())
 if err!=nil{reportFailure(w,err);return}
 page:=analyticsPage{Report:data,AllGroups:allGroups,Query:reportQuery(f,start,end),From:start.Format("2006-01-02"),To:end.Format("2006-01-02"),Print:print}
 page.TotalPercent,page.HasRequirements=data.Totals.Percent()
 for _,g:=range data.Groups {p,ok:=g.Metrics.Percent();page.Groups=append(page.Groups,analyticsGroup{Group:g,Percent:p,HasRequirements:ok})}
 for _,s:=range data.Students {p,ok:=s.Metrics.Percent();page.Students=append(page.Students,analyticsStudent{Student:s,Percent:p,HasRequirements:ok})}
 for _,d:=range data.Documents {page.Documents=append(page.Documents,analyticsDocument{Document:d,StatusLabel:materialLabelString(d.Status)})}
 maxValue:=0
 for _,week:=range history {for _,v:=range []int{week.Uploaded,week.Accepted,week.Returned}{if v>maxValue{maxValue=v}}}
 for _,week:=range history {
  row:=analyticsWeek{Week:week}
  if maxValue>0 {row.UploadedWidth=week.Uploaded*100/maxValue;row.AcceptedWidth=week.Accepted*100/maxValue;row.ReturnedWidth=week.Returned*100/maxValue}
  page.Weeks=append(page.Weeks,row)
 }
 var out bytes.Buffer
 if err:=analyticsTemplate.Execute(&out,page);err!=nil{reportFailure(w,err);return}
 w.Header().Set("Content-Type","text/html; charset=utf-8")
 if print {w.Header().Set("Content-Security-Policy","default-src 'none'; style-src 'unsafe-inline'")}
 _,_=w.Write(out.Bytes())
}
func materialLabelString(value string)string{switch value{
case "accepted":return "Принят";case "under_review":return "На проверке";case "needs_revision":return "На доработке";default:return "Не сдан"
}}

// downloadAnalytics returns genuine report data, not renamed CSV. A bounded
// in-memory buffer prevents partially written corrupt workbooks on error.
// RU: XLSX создаётся как OOXML/ZIP с тремя листами, CSV как UTF-8 с BOM.
func downloadAnalytics(w http.ResponseWriter,r *http.Request,provider AnalyticsProvider,format string){
 f,_,_,err:=reportFilters(r)
 if err!=nil{reportFailure(w,err);return}
 data,err:=provider.Build(r.Context(),f,time.Now())
 if err!=nil{reportFailure(w,err);return}
 if len(data.Documents)>100000{http.Error(w,"Для экспорта сузьте выборку.",http.StatusRequestEntityTooLarge);return}
 var out bytes.Buffer
 filename:="thesisflow-report.csv"
 mimeType:="text/csv; charset=utf-8"
 if format=="xlsx"{
  filename="thesisflow-report.xlsx"
  mimeType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
  err=reportexport.WriteXLSX(&out,reportexport.Sheets(data))
 }else{
  err=reportexport.WriteCSV(&out,data)
 }
 if err!=nil{http.Error(w,"Ошибка формирования файла.",http.StatusInternalServerError);return}
 w.Header().Set("Content-Type",mimeType)
 w.Header().Set("Content-Disposition",fmt.Sprintf("attachment; filename=%q",filename))
 w.Header().Set("Content-Length",strconv.Itoa(out.Len()))
 _,_=w.Write(out.Bytes())
}

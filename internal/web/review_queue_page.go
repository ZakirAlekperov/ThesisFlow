package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/reviewqueue"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// ReviewQueueReader is the minimal read-only projection interface for the
// review queue and completeness screen.
// RU: Интерфейс чтения очереди и комплектности без изменения документов.
type ReviewQueueReader interface {
	ListPending(context.Context, int64, int) ([]reviewqueue.Pending, error)
	GroupCompleteness(context.Context, int64, time.Time) ([]reviewqueue.Completeness, error)
}

type reviewQueueData struct {
	Groups []groups.Group
	Items []reviewqueue.Pending
	GroupID int64
	Offset int
	Prev int
	Next int
	HasNext bool
}

// reviewQueueTemplate uses local HTML/CSS and displays only latest pending
// versions, ordered by the actual upload event.
// RU: Очередь показывает актуальные версии по времени загрузки.
var reviewQueueTemplate=template.Must(template.New("review-queue").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Очередь проверки — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>
body{font:16px/1.5 system-ui,sans-serif;max-width:1200px;margin:34px auto;padding:0 18px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:14px;padding:20px;margin:18px 0}
.grid{overflow-x:auto}table{width:100%;border-collapse:collapse;min-width:760px}
th,td{padding:12px;text-align:left;border-bottom:1px solid var(--tf-border)}
.actions{display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin:16px 0}
select,button{font:inherit;padding:9px 12px;border-radius:8px}
</style></head><body>
<nav><a href="/dashboard">← Панель преподавателя</a> · <a href="/">Учебные группы</a></nav>
<h1>Очередь проверки документов</h1>
<p class="muted">Только последние версии со статусом «На проверке» у активных студентов. Старые версии не попадают в очередь.</p>
<section class="panel"><form class="actions" method="get" action="/reviews">
<label>Группа <select name="group">
<option value="0">Все активные группы</option>
{{range .Groups}}<option value="{{.ID}}" {{if eq .ID $.GroupID}}selected{{end}}>{{.Name}}</option>{{end}}
</select></label><button type="submit">Показать</button></form>
<div class="grid"><table><thead><tr><th scope="col">Загружено</th><th scope="col">Группа</th>
<th scope="col">Студент</th><th scope="col">Требование</th><th scope="col">Версия</th><th scope="col">Действие</th></tr></thead>
<tbody>{{range .Items}}<tr>
<td>{{.UploadedAt}}</td><td>{{.GroupName}}</td><td>{{.StudentName}}<small class="muted"> · {{.EducationalID}}</small></td>
<td>{{.RequirementTitle}}</td><td>{{.OriginalName}} (#{{.VersionID}})</td>
<td><a href="/groups/{{.GroupID}}/students/{{.StudentID}}#version-{{.VersionID}}">Проверить</a></td>
</tr>{{else}}<tr><td colspan="6">Нет документов, ожидающих проверки.</td></tr>{{end}}</tbody></table></div>
<div class="actions">{{if gt .Offset 0}}<a href="/reviews?group={{.GroupID}}&offset={{.Prev}}">← Предыдущие</a>{{end}}
{{if .HasNext}}<a href="/reviews?group={{.GroupID}}&offset={{.Next}}">Следующие →</a>{{end}}</div>
</section></body></html>`))

type completenessRow struct {
	StudentID int64
	FullName string
	EducationalID string
	Total int
	Accepted int
	UnderReview int
	NeedsRevision int
	Missing int
	Overdue int
	Percent int
	HasRequirements bool
}

type completenessData struct {
	GroupID int64
	GroupName string
	Students []completenessRow
}

var completenessTemplate=template.Must(template.New("completeness").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Комплектность группы {{.GroupName}} — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:1150px;margin:34px auto;padding:0 18px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:14px;padding:18px}
.grid{overflow-x:auto}table{border-collapse:collapse;width:100%;min-width:740px}td,th{padding:12px;text-align:left;border-bottom:1px solid var(--tf-border)}
</style></head><body>
<nav><a href="/groups/{{.GroupID}}">← {{.GroupName}}</a> · <a href="/groups/{{.GroupID}}/matrix">Ведомость</a></nav>
<h1>Комплектность документов · {{.GroupName}}</h1>
<p class="muted">Учитываются обязательные документы группы и индивидуальные требования. Принятыми считаются только документы с текущим статусом «Принят».</p>
<section class="panel grid"><table><thead><tr><th scope="col">Студент</th><th scope="col">Принято</th><th scope="col">На проверке</th><th scope="col">На доработке</th><th scope="col">Не сдано</th><th scope="col">Просрочено</th><th scope="col">Готовность</th></tr></thead>
<tbody>{{range .Students}}<tr><th scope="row"><a href="/groups/{{$.GroupID}}/students/{{.StudentID}}">{{.FullName}}</a><div class="muted">{{.EducationalID}}</div></th>
<td>{{.Accepted}} / {{.Total}}</td><td>{{.UnderReview}}</td><td>{{.NeedsRevision}}</td><td>{{.Missing}}</td><td>{{.Overdue}}</td>
<td>{{if .HasRequirements}}{{.Percent}}%{{else}}Нет требований{{end}}</td></tr>
{{else}}<tr><td colspan="7">В группе пока нет активных студентов.</td></tr>{{end}}</tbody></table></section>
</body></html>`))

// renderReviewQueue validates GET filters, loads a bounded set and shows
// the pending-version queue. No status is changed by this operation.
// RU: Ограниченная выборка очереди через серверный Go-шаблон.
func renderReviewQueue(w http.ResponseWriter,r *http.Request,groupsStore GroupStore,reader ReviewQueueReader) {
	groupID:=int64(0)
	if value:=r.URL.Query().Get("group");value!="" {
		id,err:=strconv.ParseInt(value,10,64)
		if err!=nil||id<0 {http.Error(w,"Неверная группа.",http.StatusBadRequest);return}
		groupID=id
	}
	offset:=0
	if value:=r.URL.Query().Get("offset");value!="" {
		value64,err:=strconv.ParseInt(value,10,32)
		if err!=nil||value64<0||value64>100000 {http.Error(w,"Неверная страница.",http.StatusBadRequest);return}
		offset=int(value64)
	}
	groupsList,err:=groupsStore.List(r.Context())
	if err!=nil {http.Error(w,"Не удалось загрузить группы.",http.StatusInternalServerError);return}
	if groupID!=0 {
		found:=false
		for _,group:=range groupsList {if group.ID==groupID {found=true;break}}
		if !found {http.NotFound(w,r);return}
	}
	items,err:=reader.ListPending(r.Context(),groupID,offset)
	if err!=nil {http.Error(w,"Не удалось загрузить очередь.",http.StatusInternalServerError);return}
	next:=len(items)>reviewqueue.PageSize
	if next {items=items[:reviewqueue.PageSize]}
	prev:=offset-reviewqueue.PageSize
	if prev<0 {prev=0}
	data:=reviewQueueData{Groups:groupsList,Items:items,GroupID:groupID,
		Offset:offset,Prev:prev,Next:offset+reviewqueue.PageSize,HasNext:next}
	var b bytes.Buffer
	if err:=reviewQueueTemplate.Execute(&b,data);err!=nil {http.Error(w,"Ошибка интерфейса.",http.StatusInternalServerError);return}
	w.Header().Set("Content-Type","text/html; charset=utf-8")
	_,_=w.Write(b.Bytes())
}

// renderGroupCompleteness validates membership via the existing group store
// and calculates statuses from live SQLite, including personal requirements.
// RU: Комплектность никогда не использует кеш процентных показателей.
func renderGroupCompleteness(w http.ResponseWriter,r *http.Request,groupsStore GroupStore,reader ReviewQueueReader) {
	id,err:=parseGroupPathID(r)
	if err!=nil {http.NotFound(w,r);return}
	group,err:=findGroup(r.Context(),groupsStore,id)
	if errors.Is(err,students.ErrGroupNotFound){http.NotFound(w,r);return}
	if err!=nil {http.Error(w,"Не удалось открыть группу.",http.StatusInternalServerError);return}
	items,err:=reader.GroupCompleteness(r.Context(),id,time.Now())
	if err!=nil {http.Error(w,"Не удалось рассчитать комплектность.",http.StatusInternalServerError);return}
	data:=completenessData{GroupID:id,GroupName:group.Name}
	for _,entry:=range items {
		row:=completenessRow{StudentID:entry.StudentID,FullName:entry.FullName,
			EducationalID:entry.EducationalID,Total:entry.Total,Accepted:entry.Accepted,
			UnderReview:entry.UnderReview,NeedsRevision:entry.NeedsRevision,
			Missing:entry.Missing,Overdue:entry.Overdue,HasRequirements:entry.Total>0}
		if entry.Total>0 {row.Percent=entry.Accepted*100/entry.Total}
		data.Students=append(data.Students,row)
	}
	var b bytes.Buffer
	if err:=completenessTemplate.Execute(&b,data);err!=nil {http.Error(w,"Ошибка интерфейса.",http.StatusInternalServerError);return}
	w.Header().Set("Content-Type","text/html; charset=utf-8")
	_,_=w.Write(b.Bytes())
}

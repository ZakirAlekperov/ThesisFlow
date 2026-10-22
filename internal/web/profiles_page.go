package web

import (
 "bytes"
 "context"
 "crypto/subtle"
 "errors"
 "fmt"
 "html/template"
 "net/http"
 "net/url"
 "strconv"
 "strings"

 "github.com/ZakirAlekperov/ThesisFlow/internal/profiles"
)

// ProfileManager exposes transactional, revisioned group document templates.
// RU: Профиль не меняет группу без отдельного подтверждения.
type ProfileManager interface{
 Create(context.Context,profiles.Profile)(profiles.Profile,error)
 Revise(context.Context,int64,int,[]profiles.Item)(profiles.Profile,error)
 List(context.Context)([]profiles.Profile,error)
 Get(context.Context,int64)(profiles.Profile,error)
 Preview(context.Context,int64,int64)(profiles.Plan,error)
 Apply(context.Context,int64,int64,string)(profiles.Plan,error)
}
type profilePageData struct {
 Profiles []profiles.Profile
 Profile profiles.Profile
 Rows string
 CSRF string
 Message string
}
var profilePage=template.Must(template.New("profiles").Parse(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Профили требований — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><script defer src="/static/theme.js"></script>
<style>body{font:16px/1.55 system-ui,sans-serif;max-width:1050px;margin:24px auto;padding:0 18px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:14px;padding:22px;margin:18px 0}
label{display:block;margin:10px 0}input,textarea,button{font:inherit;padding:9px;border-radius:7px}
textarea{width:100%;min-height:210px}input[type=text]{width:min(100%,560px)}.btn{display:inline-block;margin-top:8px}
table{width:100%;border-collapse:collapse}td,th{text-align:left;padding:10px;border-bottom:1px solid var(--tf-border)}
</style></head><body><nav><a href="/">← Группы</a> · <a href="/calendar">Календарь сроков</a> · <a href="/assets">Шаблоны файлов</a></nav>
<h1>Профили обязательных документов ВКР</h1>
<p>Профиль используется как образец. Изменение образца создаёт новую ревизию и никогда автоматически не меняет документы в уже действующих группах.</p>
<section class="panel"><h2>Создать профиль</h2>
<form method="post" action="/profiles">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<label>Название <input type="text" name="name" required maxlength="120" placeholder="ВКР — 10 обязательных документов"></label>
<label>Описание <input type="text" name="description" maxlength="800"></label>
<label>Документы — одна строка на документ, в порядке отображения
<textarea name="items" required>{{.Rows}}</textarea></label>
<p>Формат: <code>ключ | Название</code>. Ключ уникален внутри профиля и сохраняется между ревизиями. Чтобы поменять порядок, переставьте строки. Все пункты обязательны.</p>
<button type="submit">Создать профиль</button></form>
</section>
<section class="panel"><h2>Сохранённые профили</h2>
<table><thead><tr><th>Профиль</th><th>Ревизия</th><th>Действия</th></tr></thead><tbody>
{{range .Profiles}}<tr><td><a href="/profiles/{{.ID}}">{{.Name}}</a></td><td>{{.Revision}}</td><td><a href="/profiles/{{.ID}}/export">Скачать JSON</a></td></tr>
{{else}}<tr><td colspan="3">Пока нет профилей.</td></tr>{{end}}</tbody></table></section>
<p><a href="/profiles/import">Импорт профиля JSON</a></p>
</body></html>`))
var profileDetailPage=template.Must(template.New("profile-detail").Parse(`<!doctype html><html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Профиль ВКР — ThesisFlow</title><link rel="stylesheet" href="/static/theme.css">
<style>body{font:16px/1.5 system-ui,sans-serif;max-width:970px;margin:28px auto;padding:0 18px}
.panel{padding:20px;background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px;margin:15px 0}
textarea{font:inherit;width:100%;min-height:230px}button,select{font:inherit;padding:9px}li{padding:5px}</style></head>
<body><nav><a href="/profiles">← Профили</a></nav>
<h1>{{.Profile.Name}}</h1><p>{{.Profile.Description}}</p>
<p>Текущая ревизия: {{.Profile.Revision}}. Ранее применённые группы не изменяются.</p>
<section class="panel"><h2>Состав профиля</h2><ol>{{range .Profile.Items}}<li><strong>{{.Title}}</strong> ({{.Key}}) {{.Description}}</li>{{end}}</ol>
<a href="/profiles/{{.Profile.ID}}/export">Экспорт JSON</a></section>
<section class="panel"><h2>Новая ревизия</h2>
<form method="post" action="/profiles/{{.Profile.ID}}/revise">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<input type="hidden" name="revision" value="{{.Profile.Revision}}">
<label>Измените строки: <textarea name="items" required>{{.Rows}}</textarea></label>
<p>Для перестановки используйте перенос строк с прежними ключами. Удаление строки из шаблона не удалит уже сданные документы существующих групп.</p>
<button>Сохранить новую ревизию</button></form></section>
</body></html>`))
type groupProfilePreview struct {
 Plan profiles.Plan
 GroupID int64
 Profile profiles.Profile
 CSRF string
}
var groupPreviewPage=template.Must(template.New("group-plan").Parse(`<!doctype html><html lang="ru"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Применение профиля — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css">
<style>body{font:16px/1.55 system-ui,sans-serif;max-width:980px;margin:28px auto;padding:0 18px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px;padding:20px}
table{width:100%;border-collapse:collapse}td,th{padding:12px;text-align:left;border-bottom:1px solid var(--tf-border)}
button{font:inherit;padding:10px}</style></head><body>
<a href="/groups/{{.GroupID}}">← Группа</a><h1>Предпросмотр применения профиля</h1>
<p>{{.Profile.Name}} · ревизия {{.Plan.Revision}}. Новые требования изменят текущие показатели готовности группы. Принятые документы и история не удаляются.</p>
<section class="panel"><table><thead><tr><th>Документ</th><th>Действие</th></tr></thead>
<tbody>{{range .Plan.Actions}}<tr><td>{{.Title}}</td><td>{{if eq .Kind "add"}}Добавить{{else if eq .Kind "unchanged"}}Без изменений{{else}}КОНФЛИКТ: требует ручного разрешения{{end}}</td></tr>{{end}}</tbody></table>
{{if .Plan.Conflict}}<p role="alert">Обнаружены конфликтующие названия. Измените профиль или требования группы вручную перед повторной попыткой.</p>
{{else}}<form action="/groups/{{.GroupID}}/apply-profile" method="post">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<input type="hidden" name="profile" value="{{.Plan.ProfileID}}">
<input type="hidden" name="preview_token" value="{{.Plan.Token}}">
<button type="submit">Подтвердить применение</button></form>{{end}}</section></body></html>`))
const initialProfileRows="title | Титульный лист\nintro | Введение\nabbrev | Список принятых сокращений\nchapter1 | Глава 1\nchapter2 | Глава 2\nchapter3 | Глава 3\nconclusion | Заключение\nsources | Список источников\nappendix1 | Приложение 1\nappendix2 | Приложение 2"
func parseProfileRows(raw string)([]profiles.Item,error){
 if len(raw)>16000{return nil,profiles.ErrInvalid}
 items:=[]profiles.Item{}
 for _,line:=range strings.Split(strings.ReplaceAll(raw,"\r\n","\n"),"\n"){
  line=strings.TrimSpace(line);if line==""{continue}
  parts:=strings.SplitN(line,"|",2)
  if len(parts)!=2{return nil,profiles.ErrInvalid}
  items=append(items,profiles.Item{Key:strings.TrimSpace(parts[0]),Title:strings.TrimSpace(parts[1]),Position:len(items)})
 }
 if err:=profiles.ValidateItems(items);err!=nil{return nil,err}
 return items,nil
}
func profileRows(p profiles.Profile)string{
 lines:=[]string{}
 for _,item:=range p.Items{lines=append(lines,item.Key+" | "+item.Title)}
 return strings.Join(lines,"\n")
}
func formToken(w http.ResponseWriter,r *http.Request,token string)bool{
 r.Body=http.MaxBytesReader(w,r.Body,20000)
 if err:=r.ParseForm();err!=nil{http.Error(w,"Некорректная форма.",http.StatusBadRequest);return false}
 if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")),[]byte(token))!=1{
  http.Error(w,"Запрос отклонён.",http.StatusForbidden);return false
 }
 return true
}
func parseID(raw string)(int64,error){
 id,err:=strconv.ParseInt(raw,10,64)
 if err!=nil||id<=0{return 0,profiles.ErrInvalid}
 return id,nil
}
func renderPage(w http.ResponseWriter,t *template.Template,data any){
 var out bytes.Buffer
 if err:=t.Execute(&out,data);err!=nil{http.Error(w,"Не удалось показать страницу.",500);return}
 w.Header().Set("Content-Type","text/html; charset=utf-8");_,_=w.Write(out.Bytes())
}
func profilesIndex(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 entries,err:=s.List(r.Context());if err!=nil{http.Error(w,"Ошибка профилей.",500);return}
 renderPage(w,profilePage,profilePageData{Profiles:entries,Rows:initialProfileRows,CSRF:csrf})
}
func createProfile(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 if !formToken(w,r,csrf){return}
 items,err:=parseProfileRows(r.PostForm.Get("items"))
 if err!=nil{http.Error(w,"Проверьте уникальность ключей и названий документов.",400);return}
 p,err:=s.Create(r.Context(),profiles.Profile{Name:r.PostForm.Get("name"),Description:r.PostForm.Get("description"),Items:items})
 if err!=nil{http.Error(w,"Не удалось создать профиль. Проверьте название.",400);return}
 http.Redirect(w,r,fmt.Sprintf("/profiles/%d",p.ID),http.StatusSeeOther)
}
func showProfile(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 id,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 p,err:=s.Get(r.Context(),id);if err!=nil{http.NotFound(w,r);return}
 renderPage(w,profileDetailPage,profilePageData{Profile:p,Rows:profileRows(p),CSRF:csrf})
}
func reviseProfile(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 if !formToken(w,r,csrf){return}
 id,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 rev,err:=strconv.Atoi(r.PostForm.Get("revision"));if err!=nil||rev<1{http.Error(w,"Неверная ревизия.",400);return}
 items,err:=parseProfileRows(r.PostForm.Get("items"));if err!=nil{http.Error(w,"Неверные документы профиля.",400);return}
 if _,err:=s.Revise(r.Context(),id,rev,items);err!=nil{
  if errors.Is(err,profiles.ErrStale){http.Error(w,"Профиль был изменён. Обновите страницу.",http.StatusConflict);return}
  http.Error(w,"Ошибка сохранения ревизии.",400);return
 }
 http.Redirect(w,r,fmt.Sprintf("/profiles/%d",id),http.StatusSeeOther)
}
func previewGroupProfile(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 g,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 pID,err:=parseID(r.URL.Query().Get("profile"));if err!=nil{http.Error(w,"Выберите профиль.",400);return}
 p,err:=s.Get(r.Context(),pID);if err!=nil{http.NotFound(w,r);return}
 plan,err:=s.Preview(r.Context(),g,pID);if err!=nil{http.Error(w,"Невозможно получить состав группы.",400);return}
 renderPage(w,groupPreviewPage,groupProfilePreview{Plan:plan,GroupID:g,Profile:p,CSRF:csrf})
}
func applyGroupProfile(w http.ResponseWriter,r *http.Request,s ProfileManager,csrf string){
 if !formToken(w,r,csrf){return}
 g,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 p,err:=parseID(r.PostForm.Get("profile"));if err!=nil{http.Error(w,"Неверный профиль.",400);return}
 _,err=s.Apply(r.Context(),g,p,r.PostForm.Get("preview_token"))
 if err!=nil{
  if errors.Is(err,profiles.ErrStale){http.Error(w,"Группа или профиль были изменены. Получите новый предпросмотр.",409);return}
  if errors.Is(err,profiles.ErrConflict){http.Error(w,"Конфликт с требованиями группы. Изменения не применены.",409);return}
  http.Error(w,"Ошибка применения профиля.",400);return
 }
 http.Redirect(w,r,fmt.Sprintf("/groups/%d",g),http.StatusSeeOther)
}
type profileSelectionPage struct{Profiles []profiles.Profile; GroupID int64}
var selectionBlock=template.Must(template.New("group-select-profile").Parse(`<!doctype html><html lang="ru">
<head><meta charset="utf-8"><title>Профиль группы — ThesisFlow</title><link rel="stylesheet" href="/static/theme.css"></head>
<body style="font:16px system-ui,sans-serif;max-width:750px;margin:40px auto">
<a href="/groups/{{.GroupID}}">← Группа</a><h1>Назначить профиль документов</h1>
<form action="/groups/{{.GroupID}}/profile-preview" method="get"><label>Профиль
<select name="profile">{{range .Profiles}}<option value="{{.ID}}">{{.Name}} — ревизия {{.Revision}}</option>{{end}}</select>
</label><button>Показать изменения перед применением</button></form>
<p><a href="/profiles">Создать профиль</a></p></body></html>`))
func selectGroupProfile(w http.ResponseWriter,r *http.Request,s ProfileManager){
 id,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 entries,err:=s.List(r.Context());if err!=nil{http.Error(w,"Ошибка профилей.",500);return}
 renderPage(w,selectionBlock,profileSelectionPage{Profiles:entries,GroupID:id})
}
var _=url.Values{}

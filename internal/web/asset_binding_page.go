package web

import (
 "context"
 "fmt"
 "html/template"
 "net/http"
 "strconv"

 "github.com/ZakirAlekperov/ThesisFlow/internal/userassets"
)
type assetBindingLibrary interface{
 List(context.Context)([]userassets.Asset,error)
 ListBindings(context.Context,int64)([]userassets.RequirementBinding,error)
 Bind(context.Context,int64,int64,int64)error
}
type bindingPageData struct{
 GroupID int64
 CSRF string
 Assets []userassets.Asset
 Rows []userassets.RequirementBinding
}
var bindingTemplate=template.Must(template.New("assign-template").Parse(`<!doctype html><html lang="ru"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Шаблоны для группы — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><style>
body{font:16px/1.55 system-ui,sans-serif;max-width:920px;margin:30px auto;padding:0 16px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px;padding:18px;margin:15px 0}
select,button{font:inherit;padding:9px}label{display:block;margin:8px 0}
</style></head><body>
<nav><a href="/groups/{{.GroupID}}">← Группа</a> · <a href="/assets">Библиотека файлов</a></nav>
<h1>Связать шаблоны с обязательными документами</h1>
<p>Файл-шаблон помогает в оформлении, но не влияет на оценку и не становится студенческим документом. Выбор можно изменить в любой момент.</p>
{{range $row:=.Rows}}
<section class="panel"><h2>{{$row.Title}}</h2>
<form method="post" action="/groups/{{$.GroupID}}/templates">
<input type="hidden" name="csrf_token" value="{{$.CSRF}}">
<input type="hidden" name="requirement" value="{{$row.RequirementID}}">
<label>Шаблон <select name="asset">
<option value="0">Не назначен</option>
{{range $.Assets}}<option value="{{.ID}}" {{if eq .ID $row.AssetID}}selected{{end}}>{{.Name}}</option>{{end}}
</select></label><button>Сохранить связь</button></form></section>
{{else}}<p>У группы пока нет обязательных документов.</p>{{end}}
</body></html>`))
func showGroupTemplateBindings(w http.ResponseWriter,r *http.Request,lib assetBindingLibrary,csrf string){
 g,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 rows,err:=lib.ListBindings(r.Context(),g)
 if err!=nil{http.Error(w,"Невозможно загрузить документы группы.",500);return}
 assets,err:=lib.List(r.Context())
 if err!=nil{http.Error(w,"Невозможно загрузить шаблоны.",500);return}
 renderPage(w,bindingTemplate,bindingPageData{GroupID:g,Rows:rows,Assets:assets,CSRF:csrf})
}
func saveGroupTemplateBinding(w http.ResponseWriter,r *http.Request,lib assetBindingLibrary,csrf string){
 if !formToken(w,r,csrf){return}
 g,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 req,err:=parseID(r.PostForm.Get("requirement"));if err!=nil{http.Error(w,"Неверный документ.",400);return}
 a,err:=strconv.ParseInt(r.PostForm.Get("asset"),10,64)
 if err!=nil||a<0{http.Error(w,"Неверный шаблон.",400);return}
 if err:=lib.Bind(r.Context(),g,req,a);err!=nil{
  http.Error(w,"Документ или шаблон не принадлежит текущей области.",400);return
 }
 http.Redirect(w,r,fmt.Sprintf("/groups/%d/templates",g),303)
}

package web

import (
 "bytes"
 "html/template"
 "net/http"
 "strings"

 "github.com/ZakirAlekperov/ThesisFlow/internal/profiles"
)
type portableProfileImport interface{ImportJSON(ctx context.Context,data []byte)(profiles.Profile,error)}
var profileImportPage=template.Must(template.New("profile-json-import").Parse(`<!doctype html><html lang="ru"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Импорт профиля — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css"><style>body{font:16px/1.5 system-ui,sans-serif;max-width:850px;margin:28px auto;padding:0 16px}
textarea{display:block;width:100%;min-height:240px;font:14px monospace}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:12px;padding:20px;margin-top:16px}
button{font:inherit;padding:9px}</style></head><body><a href="/profiles">← Профили</a>
<h1>Импорт переносимого JSON-профиля</h1><p>Данные не включают студентов, файлы или локальные ID. Импорт всегда создаёт <strong>новый</strong> профиль; существующий не заменяется.</p>
<section class="panel"><form method="post" action="/profiles/import/preview">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<label>Содержимое JSON<textarea required name="json" maxlength="65536">{{.JSON}}</textarea></label>
<button>Проверить перед импортом</button></form></section>
{{if .Ready}}<section class="panel"><h2>Подтверждение импорта</h2>
<p>Профиль: <strong>{{.Profile.Name}}</strong>. Документов: {{len .Profile.Items}}.</p>
<ol>{{range .Profile.Items}}<li>{{.Title}} ({{.Key}})</li>{{end}}</ol>
<form method="post" action="/profiles/import/commit">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<textarea name="json" readonly hidden>{{.JSON}}</textarea>
<button>Создать новый профиль</button></form></section>{{end}}
</body></html>`))
type profileImportData struct{CSRF,JSON string;Ready bool;Profile profiles.Profile}
func profileImportForm(w http.ResponseWriter,r *http.Request,csrf string){
 renderPage(w,profileImportPage,profileImportData{CSRF:csrf})
}
func profileImportPreview(w http.ResponseWriter,r *http.Request,csrf string){
 if !formToken(w,r,csrf){return}
 raw:=r.PostForm.Get("json")
 p,err:=profiles.DecodeJSON([]byte(raw))
 if err!=nil{http.Error(w,"JSON профиля неверен, слишком велик либо содержит дубликаты.",400);return}
 renderPage(w,profileImportPage,profileImportData{CSRF:csrf,JSON:raw,Ready:true,Profile:p})
}
func profileImportCommit(w http.ResponseWriter,r *http.Request,s portableProfileImport,csrf string){
 if !formToken(w,r,csrf){return}
 raw:=r.PostForm.Get("json")
 p,err:=s.ImportJSON(r.Context(),[]byte(raw))
 if err!=nil{http.Error(w,"Не удалось создать новый профиль. Проверьте уникальность названия.",400);return}
 http.Redirect(w,r,fmt.Sprintf("/profiles/%d",p.ID),303)
}
func profileExport(w http.ResponseWriter,r *http.Request,s ProfileManager){
 id,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 p,err:=s.Get(r.Context(),id);if err!=nil{http.NotFound(w,r);return}
 raw,err:=profiles.EncodeJSON(p);if err!=nil{http.Error(w,"Не удалось собрать профиль.",500);return}
 w.Header().Set("Content-Type","application/json; charset=utf-8")
 w.Header().Set("Content-Disposition",`attachment; filename="thesisflow-profile.json"`)
 w.Header().Set("X-Content-Type-Options","nosniff")
 _,_=w.Write(raw)
}
var _=bytes.Buffer{}
var _=strings.TrimSpace

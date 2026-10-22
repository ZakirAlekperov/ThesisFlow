package web

import (
 "bytes"
 "context"
 "crypto/subtle"
 "errors"
 "fmt"
 "html/template"
 "io"
 "mime"
 "net/http"
 "strconv"

 "github.com/ZakirAlekperov/ThesisFlow/internal/userassets"
)
type AssetLibrary interface{
 Create(context.Context,string,string,string)(userassets.Asset,error)
 List(context.Context)([]userassets.Asset,error)
 AddVersion(context.Context,int64,string,[]byte)(userassets.Version,error)
 Versions(context.Context,int64)([]userassets.Version,error)
 Read(context.Context,int64,int64)(userassets.Version,[]byte,error)
}
type assetDetail struct{userassets.Asset; Versions []userassets.Version}
type assetPageData struct{Items []assetDetail; CSRF string}
var assetPage=template.Must(template.New("user-templates").Parse(`<!doctype html><html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Шаблоны документов — ThesisFlow</title>
<link rel="stylesheet" href="/static/theme.css">
<style>body{font:16px/1.55 system-ui,sans-serif;max-width:1000px;margin:28px auto;padding:0 16px}
.panel{background:var(--tf-panel);border:1px solid var(--tf-border);border-radius:14px;padding:20px;margin:18px 0}
label{display:block;margin:8px 0}input,button{font:inherit;padding:9px;max-width:100%}li{margin:9px 0}
form{margin:12px 0}</style></head><body>
<nav><a href="/templates">← Встроенные образцы</a> · <a href="/profiles">Профили требований</a></nav>
<h1>Мои шаблоны документов</h1><p>Шаблоны помогают оформить ВКР, но сами по себе не влияют на статус сдачи. Старые версии доступны для скачивания.</p>
<section class="panel"><h2>Новый шаблон</h2><form action="/assets" method="post">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<label>Название <input type="text" required maxlength="120" name="name"></label>
<label>Описание <input type="text" maxlength="800" name="description"></label>
<label>Документ, для которого предназначен <input type="text" maxlength="120" name="requirement_title" placeholder="Глава 1"></label>
<button>Создать запись шаблона</button></form></section>
{{range .Items}}<section class="panel"><h2>{{.Name}}</h2><p>{{.Description}}</p>
<p>Связан с документом: {{.RequirementTitle}}</p>
<form action="/assets/{{.ID}}/upload" method="post" enctype="multipart/form-data">
<input type="hidden" name="csrf_token" value="{{$.CSRF}}">
<label>Добавить неизменяемую версию <input type="file" name="file" required accept=".txt,.pdf,.docx,.xlsx"></label>
<button>Загрузить файл</button></form>
<h3>История версий</h3><ol>{{range .Versions}}<li>
<a href="/assets/{{.AssetID}}/versions/{{.ID}}">{{.Filename}}</a> · версия {{.Number}} · {{.Size}} байт
</li>{{else}}<li>Файл ещё не загружен</li>{{end}}</ol></section>
{{else}}<p>Собственные шаблоны ещё не созданы.</p>{{end}}
</body></html>`))
func showAssets(w http.ResponseWriter,r *http.Request,s AssetLibrary,csrf string){
 assets,err:=s.List(r.Context());if err!=nil{http.Error(w,"Ошибка загрузки шаблонов.",500);return}
 data:=assetPageData{CSRF:csrf}
 for _,asset:=range assets{
  history,err:=s.Versions(r.Context(),asset.ID)
  if err!=nil{http.Error(w,"Ошибка версий.",500);return}
  data.Items=append(data.Items,assetDetail{Asset:asset,Versions:history})
 }
 renderPage(w,assetPage,data)
}
func createAsset(w http.ResponseWriter,r *http.Request,s AssetLibrary,csrf string){
 if !formToken(w,r,csrf){return}
 if _,err:=s.Create(r.Context(),r.PostForm.Get("name"),r.PostForm.Get("description"),r.PostForm.Get("requirement_title"));err!=nil{
  http.Error(w,"Некорректный шаблон.",400);return
 }
 http.Redirect(w,r,"/assets",303)
}
func uploadAsset(w http.ResponseWriter,r *http.Request,s AssetLibrary,csrf string){
 id,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 kind,_,err:=mime.ParseMediaType(r.Header.Get("Content-Type"))
 if err!=nil||kind!="multipart/form-data"{http.Error(w,"Ожидается файл.",415);return}
 r.Body=http.MaxBytesReader(w,r.Body,userassets.MaxBytes+(1<<20))
 if err:=r.ParseMultipartForm(1<<20);err!=nil{http.Error(w,"Файл превышает предел или форма повреждена.",400);return}
 if r.MultipartForm!=nil{defer r.MultipartForm.RemoveAll()}
 if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")),[]byte(csrf))!=1{http.Error(w,"Запрос отклонён.",403);return}
 file,header,err:=r.FormFile("file")
 if err!=nil{http.Error(w,"Выберите файл.",400);return}
 defer file.Close()
 raw,err:=io.ReadAll(io.LimitReader(file,userassets.MaxBytes+1))
 if err!=nil{http.Error(w,"Ошибка чтения.",400);return}
 if _,err:=s.AddVersion(r.Context(),id,header.Filename,raw);err!=nil{
  http.Error(w,"Разрешены корректные DOCX, XLSX, PDF и TXT не больше 8 МБ.",400);return
 }
 http.Redirect(w,r,"/assets",303)
}
func downloadAsset(w http.ResponseWriter,r *http.Request,s AssetLibrary){
 a,err:=parseID(r.PathValue("id"));if err!=nil{http.NotFound(w,r);return}
 v,err:=parseID(r.PathValue("version"));if err!=nil{http.NotFound(w,r);return}
 info,data,err:=s.Read(r.Context(),a,v)
 if err!=nil{http.NotFound(w,r);return}
 w.Header().Set("Content-Type","application/octet-stream")
 w.Header().Set("Content-Disposition",mime.FormatMediaType("attachment",map[string]string{"filename":info.Filename}))
 w.Header().Set("X-Content-Type-Options","nosniff")
 w.Header().Set("Cache-Control","no-store")
 w.Header().Set("Content-Length",strconv.Itoa(len(data)))
 _,_=w.Write(data)
}
var _=bytes.Buffer{}
var _=errors.Is
var _=fmt.Sprint

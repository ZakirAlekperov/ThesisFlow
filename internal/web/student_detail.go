package web

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// DetailService holds the local document workflow dependencies.
// All repositories share the same SQLite database, but own no network service.
// RU: DetailService объединяет локальную работу со статусами и файлами ВКР.
type DetailService struct {
	Requirements *requirements.Repository
	Materials    *materials.Repository
	Versions     *fileversions.Store
	Review       *review.Service
}

type materialRow struct {
	Requirement     requirements.Requirement
	Status          string
	LatestVersionID int64
	VersionCount    int
}

type detailPageData struct {
	Group     groups.Group
	Student   students.Student
	Rows      []materialRow
	CSRFToken string
}

var studentDetailTemplate = template.Must(template.New("student-detail").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Student.FullName}} — ThesisFlow</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, sans-serif; background: #f6f7fb; color: #202939; }
    * { box-sizing: border-box; }
    body { margin: 0; }
    main { width: min(900px, 100% - 32px); margin: 48px auto; }
    .back { display: inline-block; margin-bottom: 24px; color: #214eaf; }
    .subtitle { color: #687488; }
    .card { background: white; border: 1px solid #e4e8f0; border-radius: 16px; padding: 22px; margin: 20px 0; }
    .status { font-weight: 600; }
    form { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin: 16px 0 0; }
    input[type=file], select { min-width: 180px; max-width: 100%; padding: 8px; }
    button { border: 0; border-radius: 8px; background: #214eaf; color: white; padding: 10px 14px; font: inherit; cursor: pointer; }
    button.secondary { background: #596578; }
    .empty { color: #687488; }
    h2 { margin: 0; font-size: 1.25rem; }
    .meta { color: #687488; line-height: 1.6; }
  </style>
</head>
<body>
<main>
  <a class="back" href="/groups/{{.Group.ID}}">← {{.Group.Name}}</a>
  <h1>{{.Student.FullName}}</h1>
  <p class="subtitle">Учебный ID: {{.Student.EducationalID}}</p>
  <h2>Материалы ВКР</h2>
  {{if .Rows}}
  {{range .Rows}}
  <section class="card" data-requirement-id="{{.Requirement.ID}}">
    <h2>{{.Requirement.Title}}</h2>
    {{if .Requirement.DueDate}}<p class="meta">Срок сдачи: {{.Requirement.DueDate}}</p>{{end}}
    <p class="status">Статус: {{.Status}}</p>
    <p class="meta">Загруженных версий: {{.VersionCount}}</p>
    <form action="/groups/{{$.Group.ID}}/students/{{$.Student.ID}}/upload" method="post" enctype="multipart/form-data">
      <input type="hidden" name="requirement_id" value="{{.Requirement.ID}}">
      <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
      <label>Загрузить новую версию
        <input type="file" name="file" required accept=".docx,.pdf,.odt,.rtf,.txt,.xlsx">
      </label>
      <button type="submit">Загрузить</button>
    </form>
    <form action="/groups/{{$.Group.ID}}/students/{{$.Student.ID}}/mark" method="post">
      <input type="hidden" name="requirement_id" value="{{.Requirement.ID}}">
      <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
      <button class="secondary" type="submit">Отметить сдачу без файла</button>
    </form>
    {{if .LatestVersionID}}
    <form action="/groups/{{$.Group.ID}}/students/{{$.Student.ID}}/review" method="post">
      <input type="hidden" name="requirement_id" value="{{.Requirement.ID}}">
      <input type="hidden" name="version_id" value="{{.LatestVersionID}}">
      <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
      <button type="submit" name="action" value="accept">Принять версию</button>
      <button class="secondary" type="submit" name="action" value="reject">На доработку</button>
    </form>
    {{end}}
  </section>
  {{end}}
  {{else}}
  <p class="empty">Требования к документам пока не заданы.</p>
  {{end}}
</main>
</body></html>
`))

// detailContext validates route IDs and resolves the student in the selected
// group before the document workflow accesses any private record.
// RU: detailContext проверяет группу и студента до любой работы с файлами.
func detailContext(w http.ResponseWriter, r *http.Request, groupStore GroupStore, studentStore StudentStore) (groups.Group, students.Student, bool) {
	groupID, err := parseGroupPathID(r)
	if err != nil { http.NotFound(w,r);return groups.Group{},students.Student{},false }
	studentID, err := strconv.ParseInt(r.PathValue("student"), 10, 64)
	if err != nil || studentID<=0 {http.NotFound(w,r);return groups.Group{},students.Student{},false}
	group, err := findGroup(r.Context(),groupStore,groupID)
	if err!=nil {
		if errors.Is(err,students.ErrGroupNotFound){http.NotFound(w,r)}else{http.Error(w,"Не удалось загрузить группу.",500)}
		return groups.Group{},students.Student{},false
	}
	groupStudents,err:=studentStore.ListByGroup(r.Context(),groupID)
	if err!=nil {http.Error(w,"Не удалось загрузить студентов.",500);return groups.Group{},students.Student{},false}
	for _,student:=range groupStudents {
		if student.ID==studentID {return group,student,true}
	}
	http.NotFound(w,r)
	return groups.Group{},students.Student{},false
}

// statusTitle converts internal status keys to Russian labels.
// RU: statusTitle выдаёт русское название текущего состояния документа.
func statusTitle(status materials.Status) string {
	switch status {
	case materials.UnderReview:return "На проверке"
	case materials.NeedsRevision:return "На доработке"
	case materials.Accepted:return "Принят"
	default:return "Не сдан"
	}
}

// renderStudentDetail displays requirements, current statuses and uploaded
// versions for a single verified student. The templates escape all user data.
// RU: renderStudentDetail показывает документы и статусы одного студента.
func renderStudentDetail(w http.ResponseWriter,r *http.Request,groupStore GroupStore,studentStore StudentStore,detail *DetailService,token string) {
	group,student,ok:=detailContext(w,r,groupStore,studentStore)
	if !ok{return}
	reqs,err:=detail.Requirements.ListForStudent(r.Context(),group.ID,student.ID)
	if err!=nil {http.Error(w,"Не удалось загрузить документы студента.",500);return}
	rows:=make([]materialRow,0,len(reqs))
	for _,req:=range reqs {
		state,err:=detail.Materials.Get(r.Context(),student.ID,req.ID)
		if err!=nil {http.Error(w,"Не удалось загрузить статусы.",500);return}
		versions,err:=detail.Versions.List(r.Context(),student.ID,req.ID)
		if err!=nil {http.Error(w,"Не удалось загрузить версии документов.",500);return}
		row:=materialRow{Requirement:req,Status:statusTitle(state.Status),VersionCount:len(versions)}
		if len(versions)>0 {row.LatestVersionID=versions[len(versions)-1].ID}
		rows=append(rows,row)
	}
	var rendered bytes.Buffer
	if err:=studentDetailTemplate.Execute(&rendered,detailPageData{Group:group,Student:student,Rows:rows,CSRFToken:token});err!=nil {
		http.Error(w,"Не удалось отобразить страницу студента.",500);return
	}
	w.Header().Set("Content-Type","text/html; charset=utf-8")
	_,_=w.Write(rendered.Bytes())
}

// formDetail validates the CSRF-protected form and the student's group before
// permitting a write. The caller handles each operation's specific inputs.
// RU: formDetail проверяет защищённую форму и принадлежность студента группе.
func formDetail(w http.ResponseWriter,r *http.Request,groupStore GroupStore,studentStore StudentStore,token string,multipartForm bool) (students.Student,bool) {
	mediaType,_,err:=mime.ParseMediaType(r.Header.Get("Content-Type"))
	expected:="application/x-www-form-urlencoded"
	if multipartForm {expected="multipart/form-data"}
	if err!=nil || mediaType!=expected {http.Error(w,"Неподдерживаемый формат.",415);return students.Student{},false}
	maxSize:=int64(4096)
	if multipartForm {maxSize=fileversions.MaxFileSize+8192}
	r.Body=http.MaxBytesReader(w,r.Body,maxSize)
	if multipartForm {
		err=r.ParseMultipartForm(1<<20)
		if r.MultipartForm!=nil {defer r.MultipartForm.RemoveAll()}
	} else {err=r.ParseForm()}
	if err!=nil {
		var large *http.MaxBytesError
		if errors.As(err,&large){http.Error(w,"Размер файла или формы превышен.",413)}else{http.Error(w,"Некорректная форма.",400)}
		return students.Student{},false
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")),[]byte(token))!=1 {
		http.Error(w,"Недействительный запрос.",403);return students.Student{},false
	}
	_,student,ok:=detailContext(w,r,groupStore,studentStore)
	return student,ok
}

// validRequirement checks that the selected document belongs to the student.
// RU: validRequirement запрещает изменение чужого обязательного документа.
func validRequirement(ctx context.Context,detail *DetailService,student students.Student,requirementID int64) (bool,error) {
	entries,err:=detail.Requirements.ListForStudent(ctx,student.GroupID,student.ID)
	if err!=nil{return false,err}
	for _,entry:=range entries {if entry.ID==requirementID{return true,nil}}
	return false,nil
}

// uploadStudentDocument registers a newly received physical file version.
// RU: uploadStudentDocument сохраняет новую версию, полученную преподавателем.
func uploadStudentDocument(w http.ResponseWriter,r *http.Request,groupsStore GroupStore,studentsStore StudentStore,detail *DetailService,token string) {
	student,ok:=formDetail(w,r,groupsStore,studentsStore,token,true)
	if !ok{return}
	requirementID,err:=strconv.ParseInt(r.PostForm.Get("requirement_id"),10,64)
	if err!=nil || requirementID<=0 {http.Error(w,"Не выбран документ.",400);return}
	valid,err:=validRequirement(r.Context(),detail,student,requirementID)
	if err!=nil {http.Error(w,"Ошибка чтения требований.",500);return}
	if !valid {http.NotFound(w,r);return}
	file,header,err:=r.FormFile("file")
	if err!=nil {http.Error(w,"Выберите файл документа.",400);return}
	defer file.Close()
	data,err:=io.ReadAll(io.LimitReader(file,fileversions.MaxFileSize+1))
	if err!=nil {http.Error(w,"Не удалось прочитать файл.",400);return}
	_,err=detail.Versions.Save(r.Context(),student.ID,requirementID,header.Filename,data)
	if err!=nil {
		switch {
		case errors.Is(err,fileversions.ErrTooLarge):http.Error(w,"Файл превышает 20 МиБ.",413)
		case errors.Is(err,fileversions.ErrInvalidFile):http.Error(w,"Недопустимый тип или пустой файл.",400)
		case errors.Is(err,fileversions.ErrInvalidScope):http.NotFound(w,r)
		default:http.Error(w,"Не удалось сохранить документ.",500)
		}
		return
	}
	http.Redirect(w,r,fmt.Sprintf("/groups/%d/students/%d",student.GroupID,student.ID),http.StatusSeeOther)
}

// markStudentSubmission records the receipt of material sent through an
// external messenger or email without requiring an uploaded file.
// RU: markStudentSubmission фиксирует сдачу по почте/мессенджеру без файла.
func markStudentSubmission(w http.ResponseWriter,r *http.Request,groupsStore GroupStore,studentsStore StudentStore,detail *DetailService,token string) {
	student,ok:=formDetail(w,r,groupsStore,studentsStore,token,false)
	if !ok{return}
	requirementID,err:=strconv.ParseInt(r.PostForm.Get("requirement_id"),10,64)
	if err!=nil {http.Error(w,"Не выбран документ.",400);return}
	valid,err:=validRequirement(r.Context(),detail,student,requirementID)
	if err!=nil {http.Error(w,"Не удалось проверить документ.",500);return}
	if !valid {http.NotFound(w,r);return}
	if _,err:=detail.Materials.Set(r.Context(),student.ID,requirementID,materials.UnderReview);err!=nil {
		http.Error(w,"Не удалось отметить сдачу.",500);return
	}
	http.Redirect(w,r,fmt.Sprintf("/groups/%d/students/%d",student.GroupID,student.ID),http.StatusSeeOther)
}

// reviewStudentDocument accepts or rejects a version using the persistent
// review service and safeguards the last accepted physical document.
// RU: reviewStudentDocument принимает или отклоняет версию без потери истории.
func reviewStudentDocument(w http.ResponseWriter,r *http.Request,groupsStore GroupStore,studentsStore StudentStore,detail *DetailService,token string) {
	student,ok:=formDetail(w,r,groupsStore,studentsStore,token,false)
	if !ok{return}
	requirementID,err1:=strconv.ParseInt(r.PostForm.Get("requirement_id"),10,64)
	versionID,err2:=strconv.ParseInt(r.PostForm.Get("version_id"),10,64)
	if err1!=nil || err2!=nil || requirementID<=0 || versionID<=0 {http.Error(w,"Не выбран файл для проверки.",400);return}
	valid,err:=validRequirement(r.Context(),detail,student,requirementID)
	if err!=nil {http.Error(w,"Не удалось проверить документ.",500);return}
	if !valid {http.NotFound(w,r);return}
	switch r.PostForm.Get("action") {
	case "accept":
		_,err=detail.Review.Accept(r.Context(),student.ID,requirementID,versionID)
	case "reject":
		err=detail.Review.Reject(r.Context(),student.ID,requirementID,versionID)
	default:
		http.Error(w,"Неизвестное решение.",400);return
	}
	if err!=nil {
		if errors.Is(err,review.ErrInvalidVersion){http.NotFound(w,r)}else{http.Error(w,"Не удалось сохранить решение.",500)}
		return
	}
	http.Redirect(w,r,fmt.Sprintf("/groups/%d/students/%d",student.GroupID,student.ID),http.StatusSeeOther)
}

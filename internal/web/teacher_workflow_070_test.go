package web

import (
 "bytes"
 "context"
 "fmt"
 "mime/multipart"
 "net/http"
 "net/http/httptest"
 "net/url"
 "regexp"
 "strings"
 "testing"

 "github.com/ZakirAlekperov/ThesisFlow/internal/groups"
 "github.com/ZakirAlekperov/ThesisFlow/internal/profiles"
 "github.com/ZakirAlekperov/ThesisFlow/internal/schedule"
 "github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
 "github.com/ZakirAlekperov/ThesisFlow/internal/students"
 "github.com/ZakirAlekperov/ThesisFlow/internal/userassets"
)

func post070(h http.Handler,path string,form url.Values)*httptest.ResponseRecorder{
 req:=httptest.NewRequest("POST",path,strings.NewReader(form.Encode()))
 req.Header.Set("Content-Type","application/x-www-form-urlencoded")
 response:=httptest.NewRecorder();h.ServeHTTP(response,req);return response
}
func get070(h http.Handler,path string)*httptest.ResponseRecorder{
 res:=httptest.NewRecorder();h.ServeHTTP(res,httptest.NewRequest("GET",path,nil));return res
}
func token070(t *testing.T,body string)string{
 t.Helper()
 matches:=regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(body)
 if len(matches)!=2{t.Fatalf("CSRF missing in HTML: %.500s",body)}
 return matches[1]
}
func TestTeacherWorkflowEndToEndProfilesDeadlinesAndAssets(t *testing.T){
 _,groupID,studentID,reqID,_,versions:=documentFixture(t)
 ctx:=context.Background()
 if err:=migrate.Apply(ctx,versions.DB);err!=nil{t.Fatal(err)}
 profileService:=profiles.New(versions.DB)
 scheduleService:=schedule.New(versions.DB)
 assetService:=userassets.New(versions.DB,versions.Root)
 handler,err:=NewHandlerWithServices(groups.NewRepository(versions.DB),
  students.NewRepository(versions.DB),Dependencies{
   Profiles:profileService,Schedule:scheduleService,Assets:assetService,
  })
 if err!=nil{t.Fatal(err)}
 page:=get070(handler,"/profiles")
 if page.Code!=200||!strings.Contains(page.Body.String(),"Титульный лист"){t.Fatalf("profile page %d %s",page.Code,page.Body.String())}
 token:=token070(t,page.Body.String())
 form:=url.Values{"csrf_token":{token},"name":{"Обязательные документы ВКР"},
  "items":{"intro | Введение\nend | Заключение"}}
 wrong:=post070(handler,"/profiles",url.Values{"csrf_token":{"wrong"},"name":{"unsafe"},"items":{"x | X"}})
 if wrong.Code!=403{t.Fatalf("CSRF bypass HTTP %d",wrong.Code)}
 created:=post070(handler,"/profiles",form)
 if created.Code!=303{t.Fatalf("create profile HTTP %d: %s",created.Code,created.Body.String())}
 var pid int64
 if err:=versions.DB.QueryRowContext(ctx,"SELECT id FROM requirement_profiles WHERE name=?",form.Get("name")).Scan(&pid);err!=nil{t.Fatal(err)}
 preview:=get070(handler,fmt.Sprintf("/groups/%d/profile-preview?profile=%d",groupID,pid))
 if preview.Code!=200||!strings.Contains(preview.Body.String(),"Добавить"){t.Fatalf("profile plan: %d %s",preview.Code,preview.Body.String())}
 match:=regexp.MustCompile(`name="preview_token" value="([0-9a-f]+)"`).FindStringSubmatch(preview.Body.String())
 if len(match)!=2{t.Fatal("preview token missing")}
 apply:=post070(handler,fmt.Sprintf("/groups/%d/apply-profile",groupID),
  url.Values{"csrf_token":{token},"profile":{fmt.Sprint(pid)},"preview_token":{match[1]}})
 if apply.Code!=303{t.Fatalf("apply: %d %s",apply.Code,apply.Body.String())}
 var count int
 if err:=versions.DB.QueryRowContext(ctx,"SELECT COUNT(*) FROM requirements WHERE group_id=? AND student_id IS NULL",groupID).Scan(&count);err!=nil||count!=3{
  t.Fatalf("new group requirements: %d %v",count,err)
 }
 // Batch preview must not change deadlines until an explicit second POST.
 deadlinePage:=get070(handler,fmt.Sprintf("/groups/%d/deadlines",groupID))
 if deadlinePage.Code!=200{t.Fatalf("deadline GET %d",deadlinePage.Code)}
 dueForm:=url.Values{"csrf_token":{token},fmt.Sprintf("due-%d",reqID):{"2026-11-15"}}
 plan:=post070(handler,fmt.Sprintf("/groups/%d/deadlines/preview",groupID),dueForm)
 if plan.Code!=200||!strings.Contains(plan.Body.String(),"Подтвердить изменения сроков"){t.Fatalf("deadline plan HTTP %d %s",plan.Code,plan.Body.String())}
 dueToken:=regexp.MustCompile(`name="preview_token" value="([0-9a-f]+)"`).FindStringSubmatch(plan.Body.String())
 if len(dueToken)!=2{t.Fatal("deadline token absent")}
 dueForm.Set("preview_token",dueToken[1])
 commit:=post070(handler,fmt.Sprintf("/groups/%d/deadlines/commit",groupID),dueForm)
 if commit.Code!=303{t.Fatalf("deadline apply %d %s",commit.Code,commit.Body.String())}
 var currentDue string
 if err:=versions.DB.QueryRowContext(ctx,"SELECT due_date FROM requirements WHERE id=?",reqID).Scan(&currentDue);err!=nil||currentDue!="2026-11-15"{
  t.Fatalf("deadline %q %v",currentDue,err)
 }
 // Personal exception must be visible in the legacy student's document view.
 exception:=post070(handler,"/calendar/override",url.Values{
  "csrf_token":{token},"group":{fmt.Sprint(groupID)},"student":{fmt.Sprint(studentID)},
  "requirement":{fmt.Sprint(reqID)},"kind":{"none"},
 })
 if exception.Code!=303{t.Fatalf("personal override %d %s",exception.Code,exception.Body.String())}
 calendar:=get070(handler,fmt.Sprintf("/calendar?group=%d&category=%s",groupID,url.QueryEscape("без срока")))
 if calendar.Code!=200||!strings.Contains(calendar.Body.String(),"Глава 1"){t.Fatalf("calendar with no due %d",calendar.Code)}
 // Teacher file templates use the same session CSRF but a separate table.
 made:=post070(handler,"/assets",url.Values{"csrf_token":{token},"name":{"Образец главы 1"},"requirement_title":{"Глава 1"}})
 if made.Code!=303{t.Fatalf("asset create %d %s",made.Code,made.Body.String())}
 var assetID int64
 if err:=versions.DB.QueryRow("SELECT id FROM template_assets LIMIT 1").Scan(&assetID);err!=nil{t.Fatal(err)}
 var payload bytes.Buffer
 multipartWriter:=multipart.NewWriter(&payload)
 _=multipartWriter.WriteField("csrf_token",token)
 doc,err:=multipartWriter.CreateFormFile("file","Образец.txt");if err!=nil{t.Fatal(err)}
 _,_=doc.Write([]byte("Пример текста"))
 if err:=multipartWriter.Close();err!=nil{t.Fatal(err)}
 req:=httptest.NewRequest("POST",fmt.Sprintf("/assets/%d/upload",assetID),&payload)
 req.Header.Set("Content-Type",multipartWriter.FormDataContentType())
 uploaded:=httptest.NewRecorder();handler.ServeHTTP(uploaded,req)
 if uploaded.Code!=303{t.Fatalf("asset upload %d %s",uploaded.Code,uploaded.Body.String())}
 var versionID int64
 if err:=versions.DB.QueryRow("SELECT id FROM template_versions LIMIT 1").Scan(&versionID);err!=nil{t.Fatal(err)}
 downloaded:=get070(handler,fmt.Sprintf("/assets/%d/versions/%d",assetID,versionID))
 if downloaded.Code!=200||downloaded.Body.String()!="Пример текста"{t.Fatalf("download %d %s",downloaded.Code,downloaded.Body.String())}
 binding:=post070(handler,fmt.Sprintf("/groups/%d/templates",groupID),
  url.Values{"csrf_token":{token},"requirement":{fmt.Sprint(reqID)},"asset":{fmt.Sprint(assetID)}})
 if binding.Code!=303{t.Fatalf("template assignment %d %s",binding.Code,binding.Body.String())}
}

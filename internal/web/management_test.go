package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/management"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestManagementPageEditsAndRestores verifies the real browser workflow for
// reversible archival. It uses a temporary SQLite file, not the user's data.
//
// RU: Проверяет реальные HTTP-формы редактирования и восстановления групп
// на временной SQLite, не изменяя пользовательскую базу.
func TestManagementPageEditsAndRestores(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "data.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	groupRepo := groups.NewRepository(db)
	studentRepo := students.NewRepository(db)
	for _, init := range []func(context.Context) error{groupRepo.Init, studentRepo.Init} {
		if err := init(ctx); err != nil { t.Fatal(err) }
	}
	if err := migrate.Apply(ctx, db); err != nil { t.Fatal(err) }
	group, err := groupRepo.Create(ctx, "ИСП-401")
	if err != nil { t.Fatal(err) }
	person, err := studentRepo.Create(ctx, group.ID, "TF20-001", "Иванов Иван")
	if err != nil { t.Fatal(err) }
	handler, err := NewHandlerWithServices(groupRepo, studentRepo, Dependencies{Management: management.NewRepository(db)})
	if err != nil { t.Fatal(err) }
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/manage", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Иванов Иван") {
		t.Fatalf("management page = HTTP %d: %s", page.Code, page.Body.String())
	}
	match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 { t.Fatalf("missing CSRF token: %q", page.Body.String()) }
	submit := func(path string, fields url.Values) *httptest.ResponseRecorder {
		t.Helper()
		fields.Set("csrf_token", match[1])
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(fields.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	rename := submit(fmt.Sprintf("/manage/groups/%d/rename", group.ID),url.Values{"name":{"ИСП-500"}})
	if rename.Code != http.StatusSeeOther { t.Fatalf("rename: HTTP %d %s", rename.Code,rename.Body.String()) }
	archive := submit(fmt.Sprintf("/manage/students/%d/archive", person.ID), url.Values{})
	if archive.Code != http.StatusSeeOther { t.Fatalf("archive: HTTP %d",archive.Code) }
	people, err := studentRepo.ListByGroup(ctx, group.ID)
	if err != nil || len(people) != 0 { t.Fatalf("archived student visible: %#v %v",people,err) }
	restore := submit(fmt.Sprintf("/manage/students/%d/restore", person.ID), url.Values{})
	if restore.Code != http.StatusSeeOther { t.Fatalf("restore: HTTP %d",restore.Code) }
	people, err = studentRepo.ListByGroup(ctx, group.ID)
	if err != nil || len(people) != 1 || people[0].ID != person.ID { t.Fatalf("restored student missing: %#v %v",people,err) }
}

// TestManagementRejectsCSRF prevents cross-site modifications to student data.
// RU: Проверяет отказ от изменения данных при неверном CSRF-токене.
func TestManagementRejectsCSRF(t *testing.T) {
	ctx:=context.Background()
	db,err:=sqlite.Open(ctx,filepath.Join(t.TempDir(),"db.sqlite"));if err!=nil{t.Fatal(err)}
	defer db.Close()
	gr:=groups.NewRepository(db);st:=students.NewRepository(db)
	if err:=gr.Init(ctx);err!=nil{t.Fatal(err)}
	if err:=st.Init(ctx);err!=nil{t.Fatal(err)}
	if err:=migrate.Apply(ctx,db);err!=nil{t.Fatal(err)}
	group,err:=gr.Create(ctx,"ИСП-401");if err!=nil{t.Fatal(err)}
	handler,err:=NewHandlerWithServices(gr,st,Dependencies{Management:management.NewRepository(db)})
	if err!=nil{t.Fatal(err)}
	form:=url.Values{"name":{"ИСП-999"}}
	req:=httptest.NewRequest(http.MethodPost,fmt.Sprintf("/manage/groups/%d/rename",group.ID),strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type","application/x-www-form-urlencoded")
	rec:=httptest.NewRecorder();handler.ServeHTTP(rec,req)
	if rec.Code!=http.StatusForbidden{t.Fatalf("invalid CSRF status %d",rec.Code)}
	list,err:=gr.List(ctx);if err!=nil{t.Fatal(err)}
	if len(list)!=1||list[0].Name!="ИСП-401"{t.Fatal("forged request changed group")}
}

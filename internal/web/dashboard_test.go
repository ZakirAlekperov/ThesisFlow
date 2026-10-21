package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/readiness"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestDashboardShowsCurrentGroupProgress checks that the summary is computed
// from the injected repository and never fabricated by the HTML page.
// RU: Проверяет вывод реальных показателей готовности по группам.
func TestDashboardShowsCurrentGroupProgress(t *testing.T) {
	gs:=&fakeGroupLister{items:[]groups.Group{{ID:7,Name:"ИСП-401"}}}
	h,err:=NewHandlerWithServices(gs,&fakeStudentLister{},Dependencies{
		Report:fakeReadiness{summary:readiness.Summary{StudentCount:2,Required:10,Accepted:5,Overdue:2,Percent:50}},
	})
	if err!=nil{t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/dashboard",nil))
	if rec.Code!=http.StatusOK{t.Fatalf("HTTP %d: %s",rec.Code,rec.Body.String())}
	for _,expected:=range []string{"ИСП-401","50%","Просрочено","2","10"}{
		if !strings.Contains(rec.Body.String(),expected){t.Errorf("dashboard missing %q",expected)}
	}
}

// TestSearchFiltersStudentRecordsAndEscapesMarkup searches actual in-memory
// student records in their respective groups without XSS.
// RU: Проверяет поиск по ФИО, учебному номеру и безопасный HTML-вывод.
func TestSearchFiltersStudentRecordsAndEscapesMarkup(t *testing.T) {
	gs:=&fakeGroupLister{items:[]groups.Group{{ID:7,Name:"ИСП-401"}}}
	ss:=&fakeStudentLister{items:[]students.Student{
		{ID:1,GroupID:7,FullName:"Иванов Иван",EducationalID:"TF20-001"},
		{ID:2,GroupID:7,FullName:"<script>alert(1)</script>",EducationalID:"TF20-002"},
	}}
	h,err:=NewHandlerWithServices(gs,ss,Dependencies{})
	if err!=nil{t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/search?q=TF20-001",nil))
	if rec.Code!=http.StatusOK{t.Fatalf("search HTTP %d",rec.Code)}
	if !strings.Contains(rec.Body.String(),"Иванов Иван") || strings.Contains(rec.Body.String(),"TF20-002"){
		t.Errorf("search returned unrelated rows: %q",rec.Body.String())
	}
	rec=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/search?q=TF20-002",nil))
	if strings.Contains(rec.Body.String(),"<script>alert(1)</script>") || !strings.Contains(rec.Body.String(),"&lt;script&gt;"){
		t.Errorf("unescaped student name: %q",rec.Body.String())
	}
}

// TestLocalThemeAssetsServedWithoutNetwork verifies CSS and JS are embedded in
// the Go executable. No npm, React or external CDN is required.
// RU: Проверяет встроенные CSS/JS для переключения светлой и тёмной темы.
func TestLocalThemeAssetsServedWithoutNetwork(t *testing.T) {
	h,err:=NewHandler(&fakeGroupLister{},&fakeStudentLister{})
	if err!=nil{t.Fatal(err)}
	for _,path:=range []string{"/static/theme.css","/static/theme.js"}{
		rec:=httptest.NewRecorder()
		h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,path,nil))
		if rec.Code!=http.StatusOK || rec.Body.Len()==0 {t.Errorf("%s: HTTP %d",path,rec.Code)}
	}
}

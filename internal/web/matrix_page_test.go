package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/statusmatrix"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// fakeMatrixProvider reports fixed document statuses without database access.
// RU: fakeMatrixProvider выдаёт тестовую ведомость без SQLite.
type fakeMatrixProvider struct {
	result statusmatrix.Matrix
	err error
}
func (f fakeMatrixProvider) Group(context.Context,int64)(statusmatrix.Matrix,error) {
	return f.result,f.err
}

// TestMatrixPageDisplaysTenRequirementGrid validates the user-visible report.
// Scenario: one fictional student has a chapter marked Accepted.
// Expected: safe HTML table, requirement names and readable Russian status.
//
// RU: Проверяет HTML-ведомость с требованиями и статусом «Принят».
func TestMatrixPageDisplaysTenRequirementGrid(t *testing.T) {
	rows := []statusmatrix.Row{{Student:students.Student{ID:8,GroupID:17,FullName:"Иванов Иван",EducationalID:"TF-17"},Cells:[]statusmatrix.Cell{{RequirementID:9,Status:materials.Accepted}}}}
	data := statusmatrix.Matrix{GroupID:17,Requirements:[]requirements.Requirement{{ID:9,GroupID:17,Title:"Глава 1"}},Rows:rows}
	h,err:=NewHandlerWithServices(&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}},&fakeStudentLister{},Dependencies{Matrix:fakeMatrixProvider{result:data}})
	if err!=nil {t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/groups/17/matrix",nil))
	if rec.Code!=http.StatusOK {t.Fatalf("HTTP %d: %s",rec.Code,rec.Body.String())}
	for _,value:=range []string{"ИСП-401","Иванов Иван","TF-17","Глава 1","Принят",`<table`} {
		if !strings.Contains(rec.Body.String(),value) {t.Errorf("missing %q in matrix",value)}
	}
}

// TestMatrixPageRejectsMissingGroup checks that the report cannot be opened
// with a fabricated or invalid group ID.
// RU: Проверяет 404 для неизвестной группы или некорректного ID.
func TestMatrixPageRejectsMissingGroup(t *testing.T) {
	h,err:=NewHandlerWithServices(&fakeGroupLister{},&fakeStudentLister{},Dependencies{Matrix:fakeMatrixProvider{}})
	if err!=nil {t.Fatal(err)}
	for _,url:=range []string{"/groups/0/matrix","/groups/999/matrix","/groups/abc/matrix"} {
		rec:=httptest.NewRecorder()
		h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,url,nil))
		if rec.Code!=http.StatusNotFound {t.Errorf("GET %s: %d, want 404",url,rec.Code)}
	}
}

// TestMatrixPageHidesStorageErrors ensures private database information is
// not returned to a client when the status provider fails.
// RU: Проверяет отсутствие утечки внутренних ошибок SQLite.
func TestMatrixPageHidesStorageErrors(t *testing.T) {
	h,err:=NewHandlerWithServices(&fakeGroupLister{items:[]groups.Group{{ID:17,Name:"ИСП-401"}}},&fakeStudentLister{},Dependencies{Matrix:fakeMatrixProvider{err:errors.New("secret sqlite path")}})
	if err!=nil {t.Fatal(err)}
	rec:=httptest.NewRecorder()
	h.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/groups/17/matrix",nil))
	if rec.Code!=http.StatusInternalServerError || strings.Contains(rec.Body.String(),"secret") {
		t.Errorf("HTTP %d body=%q",rec.Code,rec.Body.String())
	}
}

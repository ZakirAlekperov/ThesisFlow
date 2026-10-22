package web

import (
	"fmt"
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

// TestMatrixStatusFilters reports only students with any matching group-level
// requirement; it never filters away the underlying database records.
// RU: Фильтрация работает в памяти, исходная ведомость не изменяется.
func TestMatrixStatusFilters(t *testing.T) {
	data := statusmatrix.Matrix{
		GroupID: 17,
		Requirements: []requirements.Requirement{
			{ID: 9, Title: "Глава 1", DueDate: "2020-01-01"},
			{ID: 10, Title: "Глава 2", DueDate: "2099-01-01"},
		},
		Rows: []statusmatrix.Row{
			{Student: students.Student{ID: 1, FullName: "Иванов Иван"}, Cells: []statusmatrix.Cell{
				{RequirementID: 9, Status: materials.Missing},
				{RequirementID: 10, Status: materials.UnderReview},
			}},
			{Student: students.Student{ID: 2, FullName: "Петров Пётр"}, Cells: []statusmatrix.Cell{
				{RequirementID: 9, Status: materials.Accepted},
				{RequirementID: 10, Status: materials.NeedsRevision},
			}},
			{Student: students.Student{ID: 3, FullName: "Сидоров Сергей"}, Cells: []statusmatrix.Cell{
				{RequirementID: 9, Status: materials.Accepted},
				{RequirementID: 10, Status: materials.Accepted},
			}},
		},
	}
	h, err := NewHandlerWithServices(
		&fakeGroupLister{items: []groups.Group{{ID: 17, Name: "ИСП-401"}}},
		&fakeStudentLister{}, Dependencies{Matrix: fakeMatrixProvider{result: data}},
	)
	if err != nil { t.Fatal(err) }
	cases := []struct{ filter, included, excluded string; count string }{
		{"under_review", "Иванов Иван", "Петров Пётр", "1 из 3"},
		{"needs_revision", "Петров Пётр", "Иванов Иван", "1 из 3"},
		{"overdue", "Иванов Иван", "Петров Пётр", "1 из 3"},
		{"accepted", "Сидоров Сергей", "", "2 из 3"},
		{"", "Петров Пётр", "", "3 из 3"},
	}
	for _, tt := range cases {
		t.Run(fmt.Sprintf("filter-%s", tt.filter), func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/groups/17/matrix?status="+tt.filter, nil))
			if rec.Code != http.StatusOK { t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String()) }
			body := rec.Body.String()
			if !strings.Contains(body, tt.included) || !strings.Contains(body, tt.count) {
				t.Errorf("filter %s missing included student/count in %q", tt.filter, body)
			}
			if tt.excluded != "" && strings.Contains(body, tt.excluded) {
				t.Errorf("filter %s leaked excluded student", tt.filter)
			}
			if !strings.Contains(body, "name=\"status\"") {
				t.Errorf("filter form absent")
			}
		})
	}
	for _, bad := range []string{"unknown", "<script>", "UnderReview"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/groups/17/matrix?status="+bad, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("unknown filter %q got %d, want 400", bad, rec.Code)
		}
	}
}

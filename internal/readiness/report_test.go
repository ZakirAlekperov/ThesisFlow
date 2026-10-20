package readiness

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestGroupSummaryCountsOnlyAcceptedDocuments checks the readiness formula.
// Scenario: two students each require two documents; only one is accepted.
// Expected: accepted/total = 1/4 and percentage = 25.
//
// RU: Проверяет расчёт готовности только по принятым документам.
// Сценарий: у двух студентов по два требования, один документ принят.
// Ожидается: 1/4 принятых, готовность 25%.
func TestGroupSummaryCountsOnlyAcceptedDocuments(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "data.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	req := requirements.NewRepository(db)
	mat := materials.NewRepository(db)
	for _, init := range []func(context.Context) error{gr.Init, st.Init, req.Init, mat.Init} {
		if err := init(ctx); err != nil { t.Fatal(err) }
	}
	group, err := gr.Create(ctx, "ИСП-401")
	if err != nil { t.Fatal(err) }
	first, err := st.Create(ctx, group.ID, "100", "Иванов Иван")
	if err != nil { t.Fatal(err) }
	if _, err := st.Create(ctx, group.ID, "101", "Петров Пётр"); err != nil { t.Fatal(err) }
	one, err := req.Create(ctx, group.ID, 0, "Глава 1", "2026-10-10")
	if err != nil { t.Fatal(err) }
	if _, err := req.Create(ctx, group.ID, 0, "Глава 2", "2026-11-10"); err != nil { t.Fatal(err) }
	if _, err := mat.Set(ctx, first.ID, one.ID, materials.Accepted); err != nil { t.Fatal(err) }

	service := New(st, req, mat)
	got, err := service.Group(ctx, group.ID, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	if err != nil { t.Fatal(err) }
	if got.StudentCount != 2 || got.Required != 4 || got.Accepted != 1 || got.Percent != 25 || got.Overdue != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
}

// TestGroupSummaryWithoutRequirementsDoesNotClaimCompletion verifies that
// no defined requirements yields 0 percent rather than false 100% readiness.
//
// RU: Проверяет отсутствие ложной стопроцентной готовности, если
// для студентов ещё не определены обязательные документы.
func TestGroupSummaryWithoutRequirementsDoesNotClaimCompletion(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "data.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	req := requirements.NewRepository(db)
	mat := materials.NewRepository(db)
	for _, init := range []func(context.Context) error{gr.Init, st.Init, req.Init, mat.Init} {
		if err := init(ctx); err != nil { t.Fatal(err) }
	}
	group, err := gr.Create(ctx, "ИСП-401")
	if err != nil { t.Fatal(err) }
	if _, err := st.Create(ctx, group.ID, "100", "Иванов Иван"); err != nil { t.Fatal(err) }
	got, err := New(st, req, mat).Group(ctx, group.ID, time.Now())
	if err != nil { t.Fatal(err) }
	if got.StudentCount != 1 || got.Required != 0 || got.Percent != 0 {
		t.Fatalf("unexpected empty-requirements report: %+v", got)
	}
}

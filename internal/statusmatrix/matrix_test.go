package statusmatrix

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestBuildCombinesStudentsAndGroupRequirements verifies the two-dimensional
// academic group report against a real temporary SQLite database.
//
// RU: Проверяет матрицу «студент × требование» на временной базе SQLite.
func TestBuildCombinesStudentsAndGroupRequirements(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
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
	first, err := st.Create(ctx, group.ID, "TF-001", "Алексеев Алексей")
	if err != nil { t.Fatal(err) }
	second, err := st.Create(ctx, group.ID, "TF-002", "Иванов Иван")
	if err != nil { t.Fatal(err) }
	chapter, err := req.Create(ctx, group.ID, 0, "Глава 1", "")
	if err != nil { t.Fatal(err) }
	title, err := req.Create(ctx, group.ID, 0, "Титульный лист", "")
	if err != nil { t.Fatal(err) }
	if _, err := mat.Set(ctx, first.ID, chapter.ID, materials.Accepted); err != nil { t.Fatal(err) }
	if _, err := mat.Set(ctx, second.ID, title.ID, materials.UnderReview); err != nil { t.Fatal(err) }

	matrix, err := New(st, req, mat).Group(ctx, group.ID)
	if err != nil { t.Fatal(err) }
	if matrix.GroupID != group.ID || len(matrix.Requirements) != 2 || len(matrix.Rows) != 2 {
		t.Fatalf("unexpected matrix dimensions: %+v", matrix)
	}
	if matrix.Rows[0].Student.ID != first.ID || matrix.Rows[1].Student.ID != second.ID {
		t.Errorf("student order changed: %+v", matrix.Rows)
	}
	byID := func(row Row, id int64) materials.Status {
		for _, cell := range row.Cells {
			if cell.RequirementID == id { return cell.Status }
		}
		return "unknown"
	}
	if byID(matrix.Rows[0], chapter.ID) != materials.Accepted ||
		byID(matrix.Rows[0], title.ID) != materials.Missing ||
		byID(matrix.Rows[1], chapter.ID) != materials.Missing ||
		byID(matrix.Rows[1], title.ID) != materials.UnderReview {
		t.Errorf("incorrect cells: %+v", matrix.Rows)
	}
}

// TestBuildEmptyGroupGivesEmptyRows verifies a valid group with no students.
// RU: Проверяет пустую ведомость для существующей группы без студентов.
func TestBuildEmptyGroupGivesEmptyRows(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
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
	got, err := New(st, req, mat).Group(ctx, group.ID)
	if err != nil { t.Fatal(err) }
	if got.Rows == nil || len(got.Rows) != 0 { t.Errorf("rows=%#v, want empty non-nil slice", got.Rows) }
}

// TestBuildRejectsUnknownGroup preserves explicit missing-group diagnostics.
// RU: Проверяет ошибку для несуществующей группы вместо пустой ведомости.
func TestBuildRejectsUnknownGroup(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	req := requirements.NewRepository(db)
	mat := materials.NewRepository(db)
	for _, init := range []func(context.Context) error{gr.Init, st.Init, req.Init, mat.Init} {
		if err := init(ctx); err != nil { t.Fatal(err) }
	}
	if _, err := New(st, req, mat).Group(ctx, 999); !errors.Is(err, students.ErrGroupNotFound) {
		t.Errorf("error=%v, want ErrGroupNotFound", err)
	}
}

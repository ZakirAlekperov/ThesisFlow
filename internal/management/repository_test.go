package management

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// fixture upgrades a real older SQLite workspace; users' rows are untouched.
// RU: fixture обновляет прежнюю SQLite и сохраняет исходные записи.
func fixture(t *testing.T) (*Repository, *sql.DB, int64, int64, int64) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "data.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	if err := gr.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := st.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	a, err := gr.Create(context.Background(), "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	b, err := gr.Create(context.Background(), "ИСП-402")
	if err != nil {
		t.Fatal(err)
	}
	person, err := st.Create(context.Background(), a.ID, "TF-001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	return NewRepository(db), db, a.ID, b.ID, person.ID
}

// TestRenameGroupKeepsIDsAndRejectsDuplicates ensures rename does not break
// students' relationships and normalized name uniqueness is preserved.
// RU: Переименование не меняет ID группы/студента и отклоняет дубликаты.
func TestRenameGroupKeepsIDsAndRejectsDuplicates(t *testing.T) {
	r, db, a, _, person := fixture(t)
	ctx := context.Background()
	if err := r.RenameGroup(ctx, a, " ИСП-401А "); err != nil {
		t.Fatal(err)
	}
	var id int64
	var name string
	if err := db.QueryRow("SELECT group_id FROM students WHERE id = ?", person).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != a {
		t.Fatal("group reference changed")
	}
	if err := db.QueryRow("SELECT name FROM student_groups WHERE id = ?", a).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "ИСП-401А" {
		t.Fatalf("name = %q", name)
	}
	if err := r.RenameGroup(ctx, a, " исп-402 "); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate error = %v", err)
	}
}

// TestArchiveAndRestoreRecordsNeverDeleteData checks reversible archival.
// RU: Архивирование групп и студентов сохраняет записи и позволяет восстановить.
func TestArchiveAndRestoreRecordsNeverDeleteData(t *testing.T) {
	r, db, a, _, person := fixture(t)
	ctx := context.Background()
	if err := r.SetStudentArchived(ctx, person, true); err != nil {
		t.Fatal(err)
	}
	if err := r.SetGroupArchived(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM students WHERE id = ?", person).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("student removed")
	}
	if err := r.SetGroupArchived(ctx, a, false); err != nil {
		t.Fatal(err)
	}
	if err := r.SetStudentArchived(ctx, person, false); err != nil {
		t.Fatal(err)
	}
	var archived sql.NullString
	if err := db.QueryRow("SELECT archived_at FROM students WHERE id = ?", person).Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if archived.Valid {
		t.Fatal("student not restored")
	}
}

// TestUpdateStudentAndMoveRequireExistingTarget tests student editing and
// global uniqueness without copying/recreating its permanent primary key.
// RU: Редактирование и перевод сохраняют постоянный ID и уникальность номера.
func TestUpdateStudentAndMoveRequireExistingTarget(t *testing.T) {
	r, db, _, b, id := fixture(t)
	ctx := context.Background()
	if err := r.UpdateStudent(ctx, id, " TF-002 ", "Петров Пётр"); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveStudent(ctx, id, b); err != nil {
		t.Fatal(err)
	}
	var name, edu string
	var groupID int64
	if err := db.QueryRow("SELECT full_name,educational_id,group_id FROM students WHERE id=?", id).Scan(&name, &edu, &groupID); err != nil {
		t.Fatal(err)
	}
	if name != "Петров Пётр" || edu != "TF-002" || groupID != b {
		t.Fatalf("wrong record: %q %q %d", name, edu, groupID)
	}
	if err := r.MoveStudent(ctx, id, 99999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing target error: %v", err)
	}
	if err := r.UpdateStudent(ctx, id, " ", "Name"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty ID error: %v", err)
	}
}

// TestDuplicateEducationalIDIsRejected checks conflict across all groups.
// RU: Нельзя назначить одинаковый учебный номер двум разным студентам.
func TestDuplicateEducationalIDIsRejected(t *testing.T) {
	r, db, a, _, id := fixture(t)
	other, err := students.NewRepository(db).Create(context.Background(), a, "TF-002", "Сидоров")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateStudent(context.Background(), id, "tf-002", "Иванов"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate error = %v (existing student %d)", err, other.ID)
	}
}

// TestCanceledContextDoesNotEdit verifies cancellation before SQL changes.
// RU: При отменённом контексте никаких изменений не производится.
func TestCanceledContextDoesNotEdit(t *testing.T) {
	r, db, a, _, _ := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.RenameGroup(ctx, a, "changed"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	var name string
	_ = db.QueryRow("SELECT name FROM student_groups WHERE id=?", a).Scan(&name)
	if name != "ИСП-401" {
		t.Fatal("cancelled rename was applied")
	}
}

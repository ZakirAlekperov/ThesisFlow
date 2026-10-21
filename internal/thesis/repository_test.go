package thesis

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestThesisRequiresOrderToChangeTopic confirms the agreed order-reference
// restriction. Existing topics cannot be silently overwritten without an order.
// RU: Проверяет запрет смены утверждённой темы ВКР без нового приказа.
func TestThesisRequiresOrderToChangeTopic(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "thesis.sqlite"))
	if err != nil {t.Fatal(err)}
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	if err := gr.Init(ctx); err != nil {t.Fatal(err)}
	if err := st.Init(ctx); err != nil {t.Fatal(err)}
	if err := migrate.Apply(ctx, db); err != nil {t.Fatal(err)}
	group, err := gr.Create(ctx, "ИСП-401")
	if err != nil {t.Fatal(err)}
	student, err := st.Create(ctx, group.ID, "TF20-001", "Иванов Иван")
	if err != nil {t.Fatal(err)}
	repo := NewRepository(db)
	first, err := repo.Set(ctx, student.ID, "Локальная система учёта", "Приказ № 123")
	if err != nil {t.Fatal(err)}
	if first.StudentID != student.ID || first.Topic != "Локальная система учёта" {
		t.Fatalf("unexpected record %+v", first)
	}
	if _, err := repo.Set(ctx, student.ID, "Другая тема", ""); !errors.Is(err, ErrOrderRequired) {
		t.Fatalf("topic changed without order: %v", err)
	}
	got, err := repo.Get(ctx, student.ID)
	if err != nil {t.Fatal(err)}
	if got.Topic != first.Topic || got.OrderReference != first.OrderReference {
		t.Fatal("rejected change modified stored data")
	}
	updated, err := repo.Set(ctx, student.ID, "Другая тема", "Приказ № 124")
	if err != nil {t.Fatal(err)}
	if updated.Topic != "Другая тема" || updated.OrderReference != "Приказ № 124" {
		t.Fatalf("topic update failed: %+v", updated)
	}
}

// TestThesisRequiresStudentAndNonblankTopic prevents orphan thesis records.
// RU: Проверяет запрет ВКР без студента или без темы.
func TestThesisRequiresStudentAndNonblankTopic(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "thesis.sqlite"))
	if err != nil {t.Fatal(err)}
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	if err := gr.Init(ctx); err != nil {t.Fatal(err)}
	if err := st.Init(ctx); err != nil {t.Fatal(err)}
	if err := migrate.Apply(ctx, db); err != nil {t.Fatal(err)}
	repo := NewRepository(db)
	if _, err := repo.Set(ctx, 99999, "Тема", "Приказ № 1"); !errors.Is(err, ErrStudentNotFound) {
		t.Fatalf("orphan record error %v", err)
	}
	if _, err := repo.Set(ctx, 99999, " ", "Приказ № 1"); !errors.Is(err, ErrInvalidTopic) {
		t.Fatalf("invalid topic error %v", err)
	}
}

// TestMissingThesisReturnsEmptyRecord allows a student to exist before topic
// approval. RU: Проверяет отсутствие фиктивной темы у нового студента.
func TestMissingThesisReturnsEmptyRecord(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "thesis.sqlite"))
	if err != nil {t.Fatal(err)}
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	if err := gr.Init(ctx); err != nil {t.Fatal(err)}
	if err := st.Init(ctx); err != nil {t.Fatal(err)}
	if err := migrate.Apply(ctx, db); err != nil {t.Fatal(err)}
	record, err := NewRepository(db).Get(ctx, 19)
	if err != nil {t.Fatal(err)}
	if record.Topic != "" || record.StudentID != 19 {t.Fatalf("unexpected empty record %+v", record)}
}

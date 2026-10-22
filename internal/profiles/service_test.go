package profiles

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

func fixtures(t *testing.T) (*Service, int64, int64) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	g := groups.NewRepository(db)
	s := students.NewRepository(db)
	if err := g.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := requirements.NewRepository(db).Init(ctx); err != nil {
		t.Fatal(err)
	}
	group, err := g.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := s.Create(ctx, group.ID, "ABC-001", "Петров Пётр")
	if err != nil {
		t.Fatal(err)
	}
	return New(db), group.ID, student.ID
}
func sample() Profile {
	return Profile{Name: "Документы ВКР", Items: []Item{
		{Key: "intro", Title: "Введение", Position: 0}, {Key: "chapter1", Title: "Глава 1", Position: 1},
	}}
}
func TestApplyProfileAtomicRepeatAndInherited(t *testing.T) {
	ctx := context.Background()
	s, g, student := fixtures(t)
	p, err := s.Create(ctx, sample())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview(ctx, g, p.ID)
	if err != nil || len(plan.Actions) != 2 {
		t.Fatalf("%+v %v", plan, err)
	}
	if _, err := s.Apply(ctx, g, p.ID, plan.Token); err != nil {
		t.Fatal(err)
	}
	rules, err := requirements.NewRepository(s.DB).ListForStudent(ctx, g, student)
	if err != nil || len(rules) != 2 {
		t.Fatalf("inherit %+v %v", rules, err)
	}
	again, err := s.Preview(ctx, g, p.ID)
	if err != nil || again.Conflict || again.Actions[0].Kind != "unchanged" {
		t.Fatalf("repeat %+v %v", again, err)
	}
	if _, err := s.Apply(ctx, g, p.ID, again.Token); err != nil {
		t.Fatal(err)
	}
	rules, err = requirements.NewRepository(s.DB).ListForGroup(ctx, g)
	if err != nil || len(rules) != 2 {
		t.Fatalf("duplicate group rows %+v %v", rules, err)
	}
	// Add a third requirement through a revision; previous IDs survive.
	updated := sample().Items
	updated = append(updated, Item{Key: "conclusion", Title: "Заключение", Position: 2})
	if _, err := s.Revise(ctx, p.ID, 1, updated); err != nil {
		t.Fatal(err)
	}
	newPlan, err := s.Preview(ctx, g, p.ID)
	if err != nil || newPlan.Actions[2].Kind != "add" {
		t.Fatalf("new revision %+v %v", newPlan, err)
	}
	if _, err := s.Apply(ctx, g, p.ID, newPlan.Token); err != nil {
		t.Fatal(err)
	}
	newest, err := requirements.NewRepository(s.DB).ListForGroup(ctx, g)
	if err != nil || len(newest) != 3 || newest[0].ID != rules[1].ID && newest[1].ID != rules[1].ID {
		t.Fatalf("existing requirement changed %+v versus %+v %v", newest, rules, err)
	}
}
func TestConflictingGroupAndStalePreviewRejectWithoutWrites(t *testing.T) {
	ctx := context.Background()
	s, g, _ := fixtures(t)
	if _, err := requirements.NewRepository(s.DB).Create(ctx, g, 0, "Введение", ""); err != nil {
		t.Fatal(err)
	}
	p, err := s.Create(ctx, sample())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.Preview(ctx, g, p.ID)
	if err != nil || !preview.Conflict {
		t.Fatalf("expected conflict: %+v %v", preview, err)
	}
	if _, err := s.Apply(ctx, g, p.ID, preview.Token); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict accepted %v", err)
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_profile_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	p2, err := s.Create(ctx, Profile{Name: "Другой профиль", Items: []Item{{Key: "x", Title: "Заключение", Position: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := s.Preview(ctx, g, p2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requirements.NewRepository(s.DB).Create(ctx, g, 0, "Список источников", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(ctx, g, p2.ID, valid.Token); !errors.Is(err, ErrStale) {
		t.Fatalf("stale preview accepted %v", err)
	}
}
func TestValidateRejectsDuplicateAndRevisionConflict(t *testing.T) {
	s, _, _ := fixtures(t)
	ctx := context.Background()
	if err := ValidateItems([]Item{{Key: "x", Title: "Глава 1", Position: 0}, {Key: "y", Title: " глава 1 ", Position: 1}}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	p, err := s.Create(ctx, sample())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revise(ctx, p.ID, 0, sample().Items); !errors.Is(err, ErrStale) {
		t.Fatal(err)
	}
}

package schedule

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

func setup(t *testing.T) (*Service, int64, int64, int64, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gr := groups.NewRepository(db)
	sr := students.NewRepository(db)
	if err := gr.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sr.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	req := requirements.NewRepository(db)
	if err := req.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := materials.NewRepository(db).Init(ctx); err != nil {
		t.Fatal(err)
	}
	g, err := gr.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := sr.Create(ctx, g.ID, "A-1", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := req.Create(ctx, g.ID, 0, "Глава 1", "2026-10-20")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db)
	svc.Now = func() time.Time { return time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC) }
	return svc, g.ID, student.ID, rule.ID, db
}
func TestGroupDeadlinePreviewAndAudit(t *testing.T) {
	ctx := context.Background()
	s, g, _, req, db := setup(t)
	proposed := map[int64]string{req: "2026-10-25"}
	before, token, err := s.Preview(ctx, g, proposed)
	if err != nil || len(before) != 1 || before[0].Next != "2026-10-25" {
		t.Fatalf("plan %+v %v", before, err)
	}
	if err := s.ApplyDeadlines(ctx, g, proposed, token); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyDeadlines(ctx, g, proposed, token); !errors.Is(err, ErrStale) {
		t.Fatalf("old preview accepted %v", err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM requirement_changes WHERE kind='group_deadline'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("events %d %v", count, err)
	}
	var statusEvents int
	if err := db.QueryRow("SELECT COUNT(*) FROM material_events").Scan(&statusEvents); err != nil || statusEvents != 0 {
		t.Fatalf("deadline fabricated submission: %d %v", statusEvents, err)
	}
}
func TestIndividualDeadlineOverridesAndCalendar(t *testing.T) {
	ctx := context.Background()
	s, g, student, req, db := setup(t)
	rows, err := s.Calendar(ctx, g)
	if err != nil || len(rows) != 1 || rows[0].Category != "просрочено" || rows[0].OverdueDays != 2 {
		t.Fatalf("initial calendar %+v %v", rows, err)
	}
	if err := s.SetOverride(ctx, g, student, req, "date", "2026-10-23"); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Calendar(ctx, g)
	if err != nil || rows[0].Category != "7 дней" {
		t.Fatalf("individual date %+v %v", rows, err)
	}
	effective, err := requirements.NewRepository(db).ListForStudent(ctx, g, student)
	if err != nil || effective[0].DueDate != "2026-10-23" {
		t.Fatalf("requirement view %+v %v", effective, err)
	}
	if err := s.SetOverride(ctx, g, student, req, "none", ""); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Calendar(ctx, g)
	if err != nil || rows[0].Category != "без срока" {
		t.Fatalf("no deadline %+v %v", rows, err)
	}
	if err := s.SetOverride(ctx, g, student, req, "inherit", ""); err != nil {
		t.Fatal(err)
	}
	rows, err = s.Calendar(ctx, g)
	if err != nil || rows[0].Category != "просрочено" {
		t.Fatalf("inherit %+v %v", rows, err)
	}
	if err := s.SetOverride(ctx, g, student+99, req, "date", "2026-10-30"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign student date %v", err)
	}
}

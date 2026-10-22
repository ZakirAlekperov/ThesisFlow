package analytics

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

type reportFixture struct {
	db        *sql.DB
	groups    *groups.Repository
	people    *students.Repository
	req       *requirements.Repository
	states    *materials.Repository
	versions  *fileversions.Store
	reviews   *review.Service
	groupID   int64
	studentID int64
	ruleID    int64
	now       time.Time
}

func newReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := &reportFixture{db: db, groups: groups.NewRepository(db), people: students.NewRepository(db),
		req: requirements.NewRepository(db), states: materials.NewRepository(db),
		versions: fileversions.NewStore(db, filepath.Join(t.TempDir(), "storage")),
		now:      time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)}
	f.reviews = review.NewService(db, f.versions)
	f.versions.Clock = func() time.Time { return f.now }
	f.states.Clock = func() time.Time { return f.now }
	f.reviews.Clock = func() time.Time { return f.now }
	for _, init := range []func(context.Context) error{f.groups.Init, f.people.Init, f.req.Init, f.states.Init, f.versions.Init, f.reviews.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	g, err := f.groups.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.people.Create(ctx, g.ID, "И-0001", "Иванова Мария")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := f.req.Create(ctx, g.ID, 0, "Глава 1", "2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	f.groupID = g.ID
	f.studentID = st.ID
	f.ruleID = rule.ID
	return f
}

func TestBuildCountsInheritedAndPersonalRequirementsAndArchived(t *testing.T) {
	f := newReportFixture(t)
	ctx := context.Background()
	person, err := f.req.Create(ctx, f.groupID, f.studentID, "Приложение 1", "2026-10-10")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.people.Create(ctx, f.groupID, "И-0002", "Петров Иван")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.states.Set(ctx, f.studentID, person.ID, materials.NeedsRevision)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(f.db).Build(ctx, Filter{}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Totals.Students != 2 || r.Totals.Required != 3 || r.Totals.Missing != 2 ||
		r.Totals.NeedsRevision != 1 || r.Totals.Overdue != 3 {
		t.Fatalf("invalid metrics: %+v", r.Totals)
	}
	if len(r.Documents) != 3 || len(r.Groups) != 1 {
		t.Fatalf("document scope: %+v", r)
	}
	if percent, ok := r.Totals.Percent(); !ok || percent != 0 {
		t.Fatalf("readiness %d %t", percent, ok)
	}
	overdue, err := New(f.db).Build(ctx, Filter{OverdueOnly: true, Status: "needs_revision"}, f.now)
	if err != nil || overdue.Totals.Required != 1 || overdue.Totals.NeedsRevision != 1 {
		t.Fatalf("filters mismatch: %+v %v", overdue.Totals, err)
	}
	if _, err := f.db.ExecContext(ctx, "UPDATE students SET archived_at='2026-10-22' WHERE educational_id='И-0002'"); err != nil {
		t.Fatal(err)
	}
	current, err := New(f.db).Build(ctx, Filter{}, f.now)
	if err != nil || current.Totals.Students != 1 || current.Totals.Required != 2 {
		t.Fatalf("archived student still counted: %+v %v", current.Totals, err)
	}
}

func TestBuildKeepsOldApprovedCopyAfterReplacement(t *testing.T) {
	f := newReportFixture(t)
	ctx := context.Background()
	version, err := f.versions.Save(ctx, f.studentID, f.ruleID, "Глава 1.docx", []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.reviews.Accept(ctx, f.studentID, f.ruleID, version.ID); err != nil {
		t.Fatal(err)
	}
	r, err := New(f.db).Build(ctx, Filter{}, f.now)
	if err != nil || r.Totals.Accepted != 1 || r.Totals.WithApprovedCopy != 1 {
		t.Fatalf("approved %+v %v", r.Totals, err)
	}
	if _, err := f.versions.Save(ctx, f.studentID, f.ruleID, "Глава 1 новая.docx", []byte("second")); err != nil {
		t.Fatal(err)
	}
	r, err = New(f.db).Build(ctx, Filter{}, f.now)
	if err != nil || r.Totals.UnderReview != 1 || r.Totals.WithApprovedCopy != 1 || r.Totals.Accepted != 0 {
		t.Fatalf("stale approval masked new state: %+v %v", r.Totals, err)
	}
	if r.Documents[0].Status != "under_review" || !r.Documents[0].ApprovedCopy {
		t.Fatalf("separate status/copy lost: %+v", r.Documents[0])
	}
}

func TestBuildHandlesEmptyGroupsAndInvalidFilters(t *testing.T) {
	f := newReportFixture(t)
	ctx := context.Background()
	no, err := f.groups.Create(ctx, "БЕЗ СТУДЕНТОВ")
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(f.db).Build(ctx, Filter{GroupID: no.ID}, f.now)
	if err != nil || len(r.Groups) != 1 || r.Totals.Required != 0 || len(r.Students) != 0 {
		t.Fatalf("empty group: %+v %v", r, err)
	}
	if _, valid := r.Totals.Percent(); valid {
		t.Fatal("empty readiness must be undefined")
	}
	if _, err := New(f.db).Build(ctx, Filter{GroupID: 123456789}, f.now); !errors.Is(err, ErrUnknownGroup) {
		t.Fatalf("missing group: %v", err)
	}
	for _, filter := range []Filter{{Status: "xyz"}, {Sort: "injected"}, {GroupID: -1}, {Search: string(make([]byte, 121))}} {
		if err := CheckFilter(filter); !errors.Is(err, ErrInvalidFilter) {
			t.Fatalf("invalid accepted: %+v %v", filter, err)
		}
	}
}

func TestActivityCountsOnlyActualEventsInWeek(t *testing.T) {
	f := newReportFixture(t)
	ctx := context.Background()
	v1, err := f.versions.Save(ctx, f.studentID, f.ruleID, "chapter.docx", []byte("first"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.reviews.Accept(ctx, f.studentID, f.ruleID, v1.ID); err != nil {
		t.Fatal(err)
	}
	v2, err := f.versions.Save(ctx, f.studentID, f.ruleID, "chapter-new.docx", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.reviews.RejectWithReason(ctx, f.studentID, f.ruleID, v2.ID, "Исправьте содержание"); err != nil {
		t.Fatal(err)
	}
	beginning := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	ending := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	weeks, err := New(f.db).Activity(ctx, beginning, ending, f.groupID)
	if err != nil || len(weeks) != 1 {
		t.Fatalf("weekly data %+v %v", weeks, err)
	}
	if w := weeks[0]; w.Start != "2026-10-19" || w.Uploaded != 2 || w.Accepted != 1 || w.Returned != 1 {
		t.Errorf("wrong event facts %+v", w)
	}
}

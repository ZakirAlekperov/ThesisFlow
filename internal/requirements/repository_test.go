package requirements

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// testDatabase starts a new SQLite workspace with one academic group.
// RU: testDatabase создаёт временную SQLite с учебной группой.
func testDatabase(t *testing.T) (*Repository, *sql.DB, int64, int64) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gr := groups.NewRepository(db)
	if err := gr.Init(ctx); err != nil {
		t.Fatal(err)
	}
	sr := students.NewRepository(db)
	if err := sr.Init(ctx); err != nil {
		t.Fatal(err)
	}
	req := NewRepository(db)
	if err := req.Init(ctx); err != nil {
		t.Fatal(err)
	}
	group, err := gr.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := sr.Create(ctx, group.ID, "ЗК-001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	return req, db, group.ID, student.ID
}

// TestGroupRequirementsApplyToAllStudents verifies the default requirement
// profile is shared by all students of the academic group.
// Scenario: create a group requirement and read one student's requirements.
// Expected: the new requirement is visible without copying any rows.
//
// RU: Проверяет единые требования группы для всех её студентов.
// Сценарий: добавить требование группе, затем открыть требования студента.
// Ожидается: документ виден без копирования записи для каждого студента.
func TestGroupRequirementsApplyToAllStudents(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	ctx := context.Background()
	saved, err := req.Create(ctx, groupID, 0, "Глава 1", "2026-11-10")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID <= 0 || saved.GroupID != groupID || saved.StudentID != 0 || saved.Title != "Глава 1" || saved.DueDate != "2026-11-10" {
		t.Errorf("created requirement: %+v", saved)
	}
	items, err := req.ListForStudent(ctx, groupID, studentID)
	if err != nil || len(items) != 1 || items[0] != saved {
		t.Errorf("student requirements = %+v, err=%v; want %+v", items, err, saved)
	}
	shared, err := req.ListForGroup(ctx, groupID)
	if err != nil || len(shared) != 1 || shared[0] != saved {
		t.Errorf("group requirements = %+v, err=%v", shared, err)
	}
}

// TestPersonalRequirementsAreNotLeaked checks optional per-student items.
// Scenario: add a group-wide document and a personal document.
// Expected: the student sees both; group-wide list excludes personal records.
//
// RU: Проверяет дополнительные индивидуальные требования.
// Сценарий: создать общее и персональное требование.
// Ожидается: студент видит оба; общий список не содержит персональное.
func TestPersonalRequirementsAreNotLeaked(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	ctx := context.Background()
	if _, err := req.Create(ctx, groupID, 0, "Введение", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := req.Create(ctx, groupID, studentID, "Индивидуальное задание", ""); err != nil {
		t.Fatal(err)
	}
	studentItems, err := req.ListForStudent(ctx, groupID, studentID)
	if err != nil || len(studentItems) != 2 {
		t.Errorf("student items = %+v, err=%v", studentItems, err)
	}
	groupItems, err := req.ListForGroup(ctx, groupID)
	if err != nil || len(groupItems) != 1 {
		t.Errorf("group items = %+v, err=%v", groupItems, err)
	}
}

// TestRequirementsRejectInvalidInputs preserves referential and semantic
// integrity for nonexistent groups, mismatched students and invalid dates.
// RU: Проверяет целостность группы/студента и корректность сроков.
func TestRequirementsRejectInvalidInputs(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	ctx := context.Background()
	cases := []struct {
		group, student int64
		title, due string
		want error
	}{
		{groupID, 0, "  ", "", ErrInvalidTitle},
		{groupID, 0, "Глава 1", "2026-99-99", ErrInvalidDate},
		{9999, 0, "Глава 1", "", ErrInvalidScope},
		{groupID, 9999, "Глава 1", "", ErrInvalidScope},
		{groupID, -1, "Глава 1", "", ErrInvalidScope},
		{0, studentID, "Глава 1", "", ErrInvalidScope},
	}
	for i, tc := range cases {
		if _, err := req.Create(ctx, tc.group, tc.student, tc.title, tc.due); !errors.Is(err, tc.want) {
			t.Errorf("case %d returned %v, want %v", i, err, tc.want)
		}
	}
}

// TestRequirementNamesAreUniqueWithinScope avoids accidental duplicates but
// permits the same title for another academic group or a personal override.
// RU: Проверяет уникальность документов в пределах группы или студента.
func TestRequirementNamesAreUniqueWithinScope(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	if _, err := req.Create(context.Background(), groupID, 0, "Глава 1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := req.Create(context.Background(), groupID, 0, " глава 1 ", ""); !errors.Is(err, ErrDuplicateTitle) {
		t.Errorf("duplicate title returned %v", err)
	}
	if _, err := req.Create(context.Background(), groupID, studentID, "Глава 1", ""); err != nil {
		t.Errorf("personal requirement with same title rejected: %v", err)
	}
}

// TestRequirementListingRespectsCancellation forbids partial results from
// cancelled requests.
// RU: Проверяет отмену запроса требований без выдачи частичного списка.
func TestRequirementListingRespectsCancellation(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := req.ListForStudent(ctx, groupID, studentID); !errors.Is(err, context.Canceled) || result != nil {
		t.Errorf("result=%v, error=%v", result, err)
	}
}

package requirements

import (
	"context"
	"errors"
	"testing"
)

// TestUpdateGroupRequirement changes both title and deadline in place;
// permanent ID and student inheritance must remain unchanged.
// RU: Исправление названия/срока сохраняет ID и наследование группой.
func TestUpdateGroupRequirement(t *testing.T) {
	req, _, groupID, studentID := testDatabase(t)
	ctx := context.Background()
	original, err := req.Create(ctx, groupID, 0, "Введение", "2026-10-25")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := req.Update(ctx, groupID, original.ID, "Новое введение", "2026-11-12")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != original.ID || updated.Title != "Новое введение" || updated.DueDate != "2026-11-12" {
		t.Fatalf("update changed ID or content: %+v", updated)
	}
	inherited, err := req.ListForStudent(ctx, groupID, studentID)
	if err != nil || len(inherited) != 1 || inherited[0].Title != updated.Title {
		t.Fatalf("student inheritance failed: %+v, %v", inherited, err)
	}
}

func TestUpdateGroupRequirementValidationAndScope(t *testing.T) {
	req, _, groupID, _ := testDatabase(t)
	ctx := context.Background()
	a, _ := req.Create(ctx, groupID, 0, "Глава 1", "")
	_, _ = req.Create(ctx, groupID, 0, "Глава 2", "")
	cases := []struct {
		group, id  int64
		title, due string
		want       error
	}{
		{groupID, a.ID, "", "", ErrInvalidTitle},
		{groupID, a.ID, "Глава 1", "2026-02-30", ErrInvalidDate},
		{groupID, a.ID, " глава 2 ", "", ErrDuplicateTitle},
		{groupID + 100, a.ID, "Чужая глава", "", ErrInvalidScope},
		{groupID, a.ID + 100, "Чужая глава", "", ErrInvalidScope},
	}
	for _, tc := range cases {
		_, err := req.Update(ctx, tc.group, tc.id, tc.title, tc.due)
		if !errors.Is(err, tc.want) {
			t.Errorf("update %+v returned %v", tc, err)
		}
	}
	row, err := req.ListForGroup(ctx, groupID)
	if err != nil || len(row) != 2 || row[0].Title != "Глава 1" {
		t.Errorf("invalid edit mutated records: %+v / %v", row, err)
	}
}

// Rename of an accepted requirement would orphan the existing user-readable
// file projection. Deadline-only edits remain safe.
// RU: При принятом файле запрещаем переименование, но не перенос срока.
func TestUpdateAcceptedRequirementPreservesReadableFileName(t *testing.T) {
	req, db, groupID, _ := testDatabase(t)
	ctx := context.Background()
	a, err := req.Create(ctx, groupID, 0, "Глава 1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE accepted_files(requirement_id INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO accepted_files(requirement_id) VALUES(?)", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := req.Update(ctx, groupID, a.ID, "Глава другая", ""); !errors.Is(err, ErrAcceptedRename) {
		t.Fatalf("accepted rename err = %v, want ErrAcceptedRename", err)
	}
	updated, err := req.Update(ctx, groupID, a.ID, "Глава 1", "2026-12-05")
	if err != nil || updated.DueDate != "2026-12-05" {
		t.Fatalf("due date edit: %+v, %v", updated, err)
	}
}

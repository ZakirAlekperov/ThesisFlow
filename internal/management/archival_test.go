package management

import (
	"context"
	"errors"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestArchivingHidesRecordsButRestorationReturnsThem proves soft deletion.
// Scenario: archive a group and a student; list their active records and
// restore them. Expected: hidden while archived, same permanent IDs afterward.
//
// RU: Проверяет скрытие архивных записей из активных списков без удаления.
// Сценарий: архивировать и восстановить группу/студента.
// Ожидается: исходные постоянные ID после восстановления.
func TestArchivingHidesRecordsButRestorationReturnsThem(t *testing.T) {
	repo, db, groupID, _, studentID := fixture(t)
	ctx := context.Background()
	if err := repo.SetStudentArchived(ctx, studentID, true); err != nil {
		t.Fatal(err)
	}
	people, err := students.NewRepository(db).ListByGroup(ctx, groupID)
	if err != nil || len(people) != 0 {
		t.Fatalf("archived student still in active list: %#v %v", people, err)
	}
	if err := repo.SetStudentArchived(ctx, studentID, false); err != nil {
		t.Fatal(err)
	}
	people, err = students.NewRepository(db).ListByGroup(ctx, groupID)
	if err != nil || len(people) != 1 || people[0].ID != studentID {
		t.Fatalf("restored student missing: %#v %v", people, err)
	}
	if err := repo.SetGroupArchived(ctx, groupID, true); err != nil {
		t.Fatal(err)
	}
	list, err := groups.NewRepository(db).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range list {
		if group.ID == groupID {
			t.Fatal("archived group still in active list")
		}
	}
	if err := repo.SetGroupArchived(ctx, groupID, false); err != nil {
		t.Fatal(err)
	}
	list, err = groups.NewRepository(db).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, group := range list {
		if group.ID == groupID {
			found = true
		}
	}
	if !found {
		t.Fatal("restored group is missing")
	}
}

// TestAcceptedFilePathsBlockIdentityChanges prevents renaming/moving a group
// or student while accepted document paths still reference old readable names.
// Scenario: the student has an approved file projection.
// Expected: typed error and original group/student identities unchanged.
//
// RU: Проверяет защиту принятых файлов от переименования путей без переноса.
// Сценарий: у студента есть принятый документ.
// Ожидается: изменение имени/группы запрещено до согласованного переноса.
func TestAcceptedFilePathsBlockIdentityChanges(t *testing.T) {
	repo, db, groupID, otherGroup, studentID := fixture(t)
	if _, err := db.Exec("CREATE TABLE accepted_files (student_id INTEGER NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO accepted_files (student_id) VALUES (?)", studentID); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, action := range []struct {
		name string
		run  func() error
	}{
		{"rename group", func() error { return repo.RenameGroup(ctx, groupID, "ИСП-500") }},
		{"rename student", func() error { return repo.UpdateStudent(ctx, studentID, "TF-011", "Сидоров") }},
		{"move student", func() error { return repo.MoveStudent(ctx, studentID, otherGroup) }},
	} {
		t.Run(action.name, func(t *testing.T) {
			if err := action.run(); !errors.Is(err, ErrAcceptedFiles) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

package students

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// newTestRepository prepares independent SQLite tables for groups and students.
// RU: newTestRepository создаёт независимые таблицы групп и студентов для теста.
func newTestRepository(t *testing.T) (*Repository, *groups.Repository, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "thesisflow.sqlite"))
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	groupRepository := groups.NewRepository(db)
	if err := groupRepository.Init(ctx); err != nil {
		t.Fatalf("initialize groups: %v", err)
	}
	studentRepository := NewRepository(db)
	if err := studentRepository.Init(ctx); err != nil {
		t.Fatalf("initialize students: %v", err)
	}
	return studentRepository, groupRepository, db
}

// createTestGroup inserts a real parent group for student creation tests.
// RU: createTestGroup добавляет настоящую учебную группу для теста студента.
func createTestGroup(t *testing.T, repository *groups.Repository, name string) groups.Group {
	t.Helper()
	group, err := repository.Create(context.Background(), name)
	if err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	return group
}

// TestCreateStudentStoresNormalizedFields verifies durable student identity.
// Scenario: create a student with whitespace around the name and educational ID.
// Expected: trimmed values, a positive permanent ID and the chosen group ID.
//
// RU: Проверяет сохранение студента с постоянным идентификатором.
// Сценарий: ФИО и учебный ID имеют пробелы по краям.
// Ожидается: очищенные строки, положительный ID и правильная учебная группа.
func TestCreateStudentStoresNormalizedFields(t *testing.T) {
	repo, groupRepo, db := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")
	saved, err := repo.Create(context.Background(), group.ID, "  ЗК-0017 ", " Иванов Иван Иванович ")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if saved.ID <= 0 || saved.GroupID != group.ID || saved.EducationalID != "ЗК-0017" || saved.FullName != "Иванов Иван Иванович" {
		t.Errorf("created student = %+v", saved)
	}

	var groupID int64
	var educationalID, fullName string
	if err := db.QueryRow("SELECT group_id, educational_id, full_name FROM students WHERE id = ?", saved.ID).
		Scan(&groupID, &educationalID, &fullName); err != nil {
		t.Fatalf("read persisted student: %v", err)
	}
	if groupID != saved.GroupID || educationalID != saved.EducationalID || fullName != saved.FullName {
		t.Errorf("persisted = (%d, %q, %q); created = %+v", groupID, educationalID, fullName, saved)
	}
}

// TestCreateStudentRejectsMissingFields prevents incomplete academic records.
// Scenario: blank educational ID, blank full name, or nonpositive group ID.
// Expected: typed validation errors and no inserted students.
//
// RU: Проверяет запрет неполных записей о студентах.
// Сценарий: пустой учебный ID, пустое ФИО или неположительный ID группы.
// Ожидается: конкретные ошибки валидации и отсутствие записей.
func TestCreateStudentRejectsMissingFields(t *testing.T) {
	repo, groupRepo, db := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")
	cases := []struct {
		name          string
		groupID       int64
		educationalID string
		fullName      string
		want          error
	}{
		{"blank ID", group.ID, " \n ", "Иванов Иван", ErrInvalidEducationalID},
		{"blank name", group.ID, "ЗК-0017", " \t ", ErrInvalidFullName},
		{"zero group", 0, "ЗК-0017", "Иванов Иван", ErrGroupNotFound},
		{"negative group", -1, "ЗК-0017", "Иванов Иван", ErrGroupNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.Create(context.Background(), tc.groupID, tc.educationalID, tc.fullName); !errors.Is(err, tc.want) {
				t.Errorf("Create() error = %v, want %v", err, tc.want)
			}
		})
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&count); err != nil {
		t.Fatalf("count students: %v", err)
	}
	if count != 0 {
		t.Errorf("student count = %d, want 0", count)
	}
}

// TestCreateStudentRequiresExistingGroup verifies the group foreign-key rule.
// Scenario: create a student under a positive but unknown group ID.
// Expected: ErrGroupNotFound and no saved record.
//
// RU: Проверяет обязательную принадлежность существующей учебной группе.
// Сценарий: студент ссылается на положительный, но неизвестный ID группы.
// Ожидается: ErrGroupNotFound и отсутствие записи.
func TestCreateStudentRequiresExistingGroup(t *testing.T) {
	repo, _, db := newTestRepository(t)
	if _, err := repo.Create(context.Background(), 9999, "ЗК-0017", "Иванов Иван"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatalf("Create() error = %v, want ErrGroupNotFound", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&count); err != nil {
		t.Fatalf("count students: %v", err)
	}
	if count != 0 {
		t.Errorf("student count = %d, want 0", count)
	}
}

// TestCreateStudentRejectsDuplicateIDAcrossGroups enforces global educational
// ID uniqueness, even when the student is assigned to a different group.
// Scenario: create matching IDs with different case and surrounding whitespace.
// Expected: ErrDuplicateEducationalID, original group/name unchanged.
//
// RU: Проверяет уникальность учебного ID между всеми группами.
// Сценарий: тот же ID в другой группе с отличиями регистра и пробелов.
// Ожидается: ErrDuplicateEducationalID, исходная запись не изменяется.
func TestCreateStudentRejectsDuplicateIDAcrossGroups(t *testing.T) {
	repo, groupRepo, db := newTestRepository(t)
	first := createTestGroup(t, groupRepo, "ИСП-401")
	second := createTestGroup(t, groupRepo, "ПР-301")
	original, err := repo.Create(context.Background(), first.ID, "abc-017", "Иванов Иван")
	if err != nil {
		t.Fatalf("create original: %v", err)
	}
	if _, err := repo.Create(context.Background(), second.ID, " ABC-017 ", "Петров Пётр"); !errors.Is(err, ErrDuplicateEducationalID) {
		t.Fatalf("duplicate ID error = %v, want ErrDuplicateEducationalID", err)
	}
	var count int
	var groupID int64
	var name string
	if err := db.QueryRow("SELECT count(*), group_id, full_name FROM students").Scan(&count, &groupID, &name); err != nil {
		t.Fatalf("query existing student: %v", err)
	}
	if count != 1 || groupID != original.GroupID || name != original.FullName {
		t.Errorf("existing student changed: count=%d, group=%d, name=%q", count, groupID, name)
	}
}

// TestCreateStudentSurvivesReopen checks that student identity, educational ID
// and group association remain unchanged across restarts and repeated Init.
// Scenario: create a student, close SQLite, reopen and call Init again.
// Expected: identical row including permanent student and group IDs.
//
// RU: Проверяет сохранность студента после перезапуска и повторного Init.
// Сценарий: закрыть SQLite после создания студента и открыть заново.
// Ожидается: прежние ID, ФИО, учебный идентификатор и группа.
func TestCreateStudentSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	firstDB, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open first DB: %v", err)
	}
	firstGroupRepo := groups.NewRepository(firstDB)
	if err := firstGroupRepo.Init(ctx); err != nil {
		firstDB.Close()
		t.Fatalf("initialize first groups: %v", err)
	}
	firstRepo := NewRepository(firstDB)
	if err := firstRepo.Init(ctx); err != nil {
		firstDB.Close()
		t.Fatalf("initialize first students: %v", err)
	}
	group := createTestGroup(t, firstGroupRepo, "ИСП-401")
	student, err := firstRepo.Create(ctx, group.ID, "ЗК-0017", "Иванов Иван")
	if err != nil {
		firstDB.Close()
		t.Fatalf("create student: %v", err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatalf("close first DB: %v", err)
	}

	secondDB, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen DB: %v", err)
	}
	defer secondDB.Close()
	if err := NewRepository(secondDB).Init(ctx); err != nil {
		t.Fatalf("repeat Init(): %v", err)
	}
	var got Student
	if err := secondDB.QueryRow("SELECT id, group_id, educational_id, full_name FROM students WHERE id = ?", student.ID).
		Scan(&got.ID, &got.GroupID, &got.EducationalID, &got.FullName); err != nil {
		t.Fatalf("load reopened student: %v", err)
	}
	if got != student {
		t.Errorf("reopened student = %+v, want %+v", got, student)
	}
}

// TestStudentsTableEnforcesForeignKey verifies the SQLite-level integrity
// constraint even when SQL is executed outside the Go repository.
// Scenario: direct INSERT references a missing parent group.
// Expected: SQLite rejects the row and does not create an orphan student.
//
// RU: Проверяет внешний ключ непосредственно на уровне SQLite.
// Сценарий: прямой INSERT с несуществующей группой вне Go-репозитория.
// Ожидается: СУБД отклоняет сиротскую запись.
func TestStudentsTableEnforcesForeignKey(t *testing.T) {
	_, _, db := newTestRepository(t)
	_, err := db.Exec(
		"INSERT INTO students(group_id, educational_id, educational_id_key, full_name) VALUES (?, ?, ?, ?)",
		9999, "ЗК-0017", "зк-0017", "Иванов Иван",
	)
	if err == nil {
		t.Fatal("SQLite accepted a student with a nonexistent group")
	}
}

// TestCreateStudentRespectsCancellation ensures canceled operations never
// write data, and cancellation errors propagate to callers.
// Scenario: cancel context before calling Create.
// Expected: context.Canceled and no student row.
//
// RU: Проверяет отмену операции без записи в SQLite.
// Сценарий: контекст отменён до Create.
// Ожидается: context.Canceled и отсутствие студента.
func TestCreateStudentRespectsCancellation(t *testing.T) {
	repo, groupRepo, db := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Create(ctx, group.ID, "ЗК-0017", "Иванов Иван"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() error = %v, want context.Canceled", err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&count); err != nil {
		t.Fatalf("count students: %v", err)
	}
	if count != 0 {
		t.Errorf("student count = %d, want 0", count)
	}
}

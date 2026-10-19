package groups

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// newTestRepository opens a separate SQLite database for each test.
// RU: newTestRepository открывает отдельную базу SQLite для каждого теста.
func newTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "thesisflow.sqlite"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := NewRepository(db)
	if err := repo.Init(context.Background()); err != nil {
		t.Fatalf("initialize schema: %v", err)
	}
	return repo, db
}

// TestCreateGroupStoresNormalizedNameAndID verifies persistent creation.
// Scenario: create a group with surrounding whitespace.
// Expected: trimmed name, positive ID and matching row in SQLite.
//
// RU: Проверяет сохранение группы с постоянным идентификатором.
// Сценарий: название содержит пробелы по краям.
// Ожидается: очищенное название, положительный ID и запись в SQLite.
func TestCreateGroupStoresNormalizedNameAndID(t *testing.T) {
	repo, db := newTestRepository(t)
	group, err := repo.Create(context.Background(), "  ИСП-401 \t")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if group.ID <= 0 || group.Name != "ИСП-401" {
		t.Fatalf("created group = %+v, want positive ID and ИСП-401", group)
	}
	var stored string
	if err := db.QueryRow("SELECT name FROM student_groups WHERE id = ?", group.ID).Scan(&stored); err != nil {
		t.Fatalf("query group: %v", err)
	}
	if stored != group.Name {
		t.Errorf("stored name = %q, want %q", stored, group.Name)
	}
}

// TestCreateGroupRejectsEmptyName protects the group-name invariant.
// Scenario: try empty or whitespace-only names.
// Expected: ErrInvalidName, and no rows are inserted.
//
// RU: Проверяет запрет пустого названия.
// Сценарий: пустые строки или только пробельные символы.
// Ожидается: ErrInvalidName и отсутствие новых записей.
func TestCreateGroupRejectsEmptyName(t *testing.T) {
	repo, db := newTestRepository(t)
	for _, name := range []string{"", "   ", "\n\t"} {
		if _, err := repo.Create(context.Background(), name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("Create(%q) error = %v, want ErrInvalidName", name, err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM student_groups").Scan(&count); err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if count != 0 {
		t.Errorf("group count = %d, want 0", count)
	}
}

// TestCreateGroupRejectsDuplicateName prevents accidental repeated imports.
// Scenario: save the same name with different case and surrounding whitespace.
// Expected: ErrDuplicateName; the original group remains unchanged.
//
// RU: Проверяет защиту от повторного добавления группы.
// Сценарий: одинаковые названия с отличиями в регистре и пробелах.
// Ожидается: ErrDuplicateName, исходная группа не меняется.
func TestCreateGroupRejectsDuplicateName(t *testing.T) {
	repo, db := newTestRepository(t)
	first, err := repo.Create(context.Background(), "ИСП-401")
	if err != nil {
		t.Fatalf("create original group: %v", err)
	}
	if _, err := repo.Create(context.Background(), " исп-401 "); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("duplicate error = %v, want ErrDuplicateName", err)
	}
	var id int64
	var name string
	if err := db.QueryRow("SELECT id, name FROM student_groups").Scan(&id, &name); err != nil {
		t.Fatalf("query original group: %v", err)
	}
	if id != first.ID || name != first.Name {
		t.Errorf("stored = (%d, %q), want (%d, %q)", id, name, first.ID, first.Name)
	}
}

// TestCreateGroupKeepsIDAfterReopen verifies durable identity across restarts.
// Scenario: create a group, close the DB, reopen and initialize the schema.
// Expected: old ID survives; the next group receives a different ID.
//
// RU: Проверяет постоянство ID после перезапуска.
// Сценарий: группа создана, база закрыта, вновь открыта и инициализирована.
// Ожидается: прежний ID сохраняется; следующей группе назначается другой ID.
func TestCreateGroupKeepsIDAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	firstDB, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	firstRepo := NewRepository(firstDB)
	if err := firstRepo.Init(context.Background()); err != nil {
		firstDB.Close()
		t.Fatalf("first initialize: %v", err)
	}
	first, err := firstRepo.Create(context.Background(), "ИСП-401")
	if err != nil {
		firstDB.Close()
		t.Fatalf("create first group: %v", err)
	}
	if err := firstDB.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
	}
	secondDB, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer secondDB.Close()
	secondRepo := NewRepository(secondDB)
	if err := secondRepo.Init(context.Background()); err != nil {
		t.Fatalf("reinitialize: %v", err)
	}
	var restored string
	if err := secondDB.QueryRow("SELECT name FROM student_groups WHERE id = ?", first.ID).Scan(&restored); err != nil {
		t.Fatalf("read restored group: %v", err)
	}
	if restored != first.Name {
		t.Errorf("restored name = %q, want %q", restored, first.Name)
	}
	second, err := secondRepo.Create(context.Background(), "ИСП-402")
	if err != nil {
		t.Fatalf("create second group: %v", err)
	}
	if second.ID == first.ID {
		t.Errorf("second group reused ID %d", first.ID)
	}
}

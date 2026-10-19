package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenCreatesDatabase verifies initialization of a fresh local workspace.
// Scenario: the database path points into a directory that does not exist.
// Expected: Open creates the directory, creates a usable SQLite file and enables
// foreign-key enforcement.
//
// RU: Проверяет инициализацию нового локального рабочего пространства.
// Сценарий: путь к базе указывает на ещё не существующий каталог.
// Ожидается: создаются каталог и рабочая база SQLite; проверка внешних
// ключей включена.
func TestOpenCreatesDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "thesisflow.sqlite")

	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open new database: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file must exist: %v", err)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d; want 1", foreignKeys)
	}
}

// TestOpenPreservesData verifies reopening an existing SQLite database.
// Scenario: write a record, close the database, then reopen the same file.
// Expected: the previously stored record remains available.
//
// RU: Проверяет сохранность данных после повторного открытия базы.
// Сценарий: записать строку, закрыть соединение и открыть тот же файл.
// Ожидается: сохранённая строка доступна без потери данных.
func TestOpenPreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := first.Exec("CREATE TABLE sample (value TEXT NOT NULL)"); err != nil {
		first.Close()
		t.Fatalf("create sample table: %v", err)
	}
	if _, err := first.Exec("INSERT INTO sample(value) VALUES (?)", "Глава 1"); err != nil {
		first.Close()
		t.Fatalf("save sample record: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer second.Close()

	var got string
	if err := second.QueryRow("SELECT value FROM sample").Scan(&got); err != nil {
		t.Fatalf("read saved record: %v", err)
	}
	if got != "Глава 1" {
		t.Errorf("saved record = %q, want %q", got, "Глава 1")
	}
}

// TestOpenRejectsInvalidPath verifies clear filesystem error propagation.
// Scenario: a parent path component is a file rather than a directory.
// Expected: initialization fails instead of creating another database.
//
// RU: Проверяет возврат ошибки при недопустимом пути к базе.
// Сценарий: компонент родительского пути является обычным файлом.
// Ожидается: инициализация завершается ошибкой без создания другой базы.
func TestOpenRejectsInvalidPath(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("occupied"), 0600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}
	if db, err := Open(context.Background(), filepath.Join(parent, "thesisflow.sqlite")); err == nil {
		db.Close()
		t.Fatal("Open() on invalid path: want error")
	}
}

// TestOpenRespectsCancellation verifies the caller controls initialization.
// Scenario: the incoming context has already been cancelled.
// Expected: Open returns context.Canceled and creates no database file.
//
// RU: Проверяет отмену инициализации вызывающей стороной.
// Сценарий: входной контекст уже отменён.
// Ожидается: context.Canceled и отсутствие созданного файла базы.
func TestOpenRespectsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace", "thesisflow.sqlite")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db, err := Open(ctx, path)
	if db != nil {
		db.Close()
		t.Fatal("Open() returned non-nil database after cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("unexpected database file after cancellation: %v", err)
	}
}

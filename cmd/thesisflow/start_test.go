package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestStartInitializesLocalDatabase verifies that the normal startup path
// initializes persistent storage before opening the browser.
// Scenario: use a fresh temporary workspace, then cancel at browser opening.
// Expected: SQLite exists, the server URL is printed, and startup exits cleanly.
//
// RU: Проверяет создание постоянного хранилища при обычном запуске приложения.
// Сценарий: новый временный каталог, отмена после обращения к браузеру.
// Ожидается: база SQLite создана, локальный URL выведен, остановка штатная.
func TestStartInitializesLocalDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dbPath := filepath.Join(t.TempDir(), "workspace", "thesisflow.sqlite")
	var output bytes.Buffer
	var opened bool

	err := start(ctx, &output, "0", dbPath, func(address string) error {
		opened = true
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("start() error = %v", err)
	}
	if !opened {
		t.Fatal("browser opener was not invoked")
	}
	if !strings.Contains(output.String(), "ThesisFlow: http://127.0.0.1:") {
		t.Errorf("startup output = %q, want local URL", output.String())
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database not created during startup: %v", err)
	}
	db, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("cannot reopen database after shutdown: %v", err)
	}
	defer db.Close()
}

// TestStartInitializesGroupsSchema checks application startup integration.
// Scenario: start the local application with a fresh database path.
// Expected: after shutdown, the database contains an empty groups table.
//
// RU: Проверяет подключение таблицы групп при запуске ThesisFlow.
// Сценарий: локальный запуск с новым файлом базы данных.
// Ожидается: после остановки в базе есть пустая таблица учебных групп.
func TestStartInitializesGroupsSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dbPath := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	err := start(ctx, io.Discard, "0", dbPath, func(string) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	db, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT count(*) FROM student_groups").Scan(&count); err != nil {
		t.Fatalf("groups table missing after startup: %v", err)
	}
	if count != 0 {
		t.Errorf("groups table row count = %d, want 0", count)
	}
}

// TestStartInitializesStudentsSchema verifies table creation before the HTTP
// interface is opened. The students table must reference academic groups.
// Scenario: start a fresh workspace and cancel from the browser opener.
// Expected: the students table is present and initially empty.
//
// RU: Проверяет создание таблицы студентов до запуска HTTP-интерфейса.
// Сценарий: новый файл SQLite и отмена из обработчика открытия браузера.
// Ожидается: таблица студентов существует и не содержит записей.
func TestStartInitializesStudentsSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	dbPath := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	err := start(ctx, io.Discard, "0", dbPath, func(string) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	db, err := sqlite.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&count); err != nil {
		t.Fatalf("students table missing after startup: %v", err)
	}
	if count != 0 {
		t.Errorf("student count = %d, want 0", count)
	}
}

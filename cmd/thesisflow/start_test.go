package main

import (
	"bytes"
	"context"
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

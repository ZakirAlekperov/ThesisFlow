package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestBackupCLIExportsAndRestores checks the user-facing command using
// temporary workspaces with a real SQLite record.
// Scenario: export a populated workspace, restore into a fresh destination.
// Expected: the archived record survives and the CLI confirms completion.
//
// RU: Проверяет команды резервного копирования на временной рабочей папке.
// Сценарий: экспортировать SQLite, восстановить в новый каталог.
// Ожидается: запись сохранилась и команда сообщила об успешном завершении.
func TestBackupCLIExportsAndRestores(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	workspace := filepath.Join(root, "work")
	db, err := sqlite.Open(ctx, filepath.Join(workspace, "thesisflow.sqlite"))
	if err != nil { t.Fatal(err) }
	if _, err := db.Exec("CREATE TABLE demo (value TEXT)"); err != nil { t.Fatal(err) }
	if _, err := db.Exec("INSERT INTO demo(value) VALUES ('saved')"); err != nil { t.Fatal(err) }
	if err := db.Close(); err != nil { t.Fatal(err) }

	archive := filepath.Join(root, "snapshot.zip")
	var output bytes.Buffer
	if err := runBackupCommand(ctx, workspace, []string{"backup", "export", archive}, &output); err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(output.String(), "создана") { t.Errorf("unexpected message: %q", output.String()) }
	target := filepath.Join(root, "recovered")
	output.Reset()
	if err := runBackupCommand(ctx, workspace, []string{"backup", "restore", archive, target}, &output); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, err := sqlite.Open(ctx, filepath.Join(target, "thesisflow.sqlite"))
	if err != nil { t.Fatal(err) }
	defer restored.Close()
	var value string
	if err := restored.QueryRow("SELECT value FROM demo").Scan(&value); err != nil { t.Fatal(err) }
	if value != "saved" { t.Errorf("record = %q", value) }
}

// TestBackupCLIRejectsMissingSource prevents silently archiving a newly
// created empty database instead of an existing user workspace.
//
// RU: Проверяет отказ создавать пустую базу вместо отсутствующих данных.
func TestBackupCLIRejectsMissingSource(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "missing")
	err := runBackupCommand(context.Background(), workspace,
		[]string{"backup", "export", filepath.Join(root, "snapshot.zip")}, &bytes.Buffer{})
	if err == nil { t.Fatal("exported missing source") }
	if _, statErr := os.Stat(filepath.Join(workspace, "thesisflow.sqlite")); !os.IsNotExist(statErr) {
		t.Errorf("command created a new database: %v", statErr)
	}
}

// TestBackupCLIRefusesRestoreOverExistingWorkspace verifies that command
// arguments cannot overwrite an existing local workspace.
//
// RU: Проверяет защиту от перезаписи существующего рабочего каталога.
func TestBackupCLIRefusesRestoreOverExistingWorkspace(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "existing")
	if err := os.Mkdir(existing, 0700); err != nil { t.Fatal(err) }
	err := runBackupCommand(context.Background(), filepath.Join(root, "work"),
		[]string{"backup", "restore", "anything.zip", existing}, &bytes.Buffer{})
	if !errors.Is(err, backup.ErrDestinationExists) {
		t.Errorf("restore error = %v, want ErrDestinationExists", err)
	}
}

// TestBackupCLIRejectsInvalidArguments prevents ambiguous destructive actions.
// Scenario: incomplete or unrecognized backup subcommands.
// Expected: error before accessing the filesystem.
//
// RU: Проверяет отказ при неполных или неизвестных аргументах CLI.
// Ожидается: понятная ошибка без файловых операций.
func TestBackupCLIRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"backup"}, {"backup", "export"}, {"backup", "restore"}, {"backup", "restore", "archive.zip"},
		{"backup", "destroy", "file"}, {"backup", "export", "file", "extra"},
	} {
		if err := runBackupCommand(context.Background(), t.TempDir(), args, &bytes.Buffer{}); err == nil {
			t.Errorf("runBackupCommand(%q) should fail", args)
		}
	}
}

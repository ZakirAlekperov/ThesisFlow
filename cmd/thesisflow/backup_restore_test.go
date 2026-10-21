package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestWorkspaceRestoreRecoversToNewDirectory checks the actual restore service.
// Scenario: export a real SQLite record and restore to an isolated folder.
// Expected: recovered DB contains the same row, active DB is unaffected.
//
// RU: Проверяет настоящее восстановление в отдельный каталог.
// Сценарий: экспорт записи SQLite, восстановление без изменения текущей базы.
func TestWorkspaceRestoreRecoversToNewDirectory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "live", "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE sample(value TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sample(value) VALUES('saved')"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "export.zip")
	if err := backup.Export(ctx, db, filepath.Join(root, "live", "storage"), archive); err != nil {
		t.Fatal(err)
	}
	svc := workspaceRestore{root: filepath.Join(root, "restores")}
	dest, err := svc.Restore(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	if dest == filepath.Join(root, "live") {
		t.Fatal("restore replaced active directory")
	}
	if info, err := os.Stat(dest); err != nil || !info.IsDir() {
		t.Fatalf("missing restored directory: %v", err)
	}
	recovered, err := sqlite.Open(ctx, filepath.Join(dest, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	var value string
	if err := recovered.QueryRow("SELECT value FROM sample").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "saved" {
		t.Errorf("record=%q", value)
	}
}

// TestWorkspaceRestoreRejectsInvalidZIP checks that a failed attempt never
// populates a recovered workspace with unverified data.
// RU: Проверяет отклонение повреждённого ZIP и отсутствие готовой копии.
func TestWorkspaceRestoreRejectsInvalidZIP(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "broken.zip")
	if err := os.WriteFile(path, []byte("not-a-ZIP"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := workspaceRestore{root: filepath.Join(root, "restored")}
	if _, err := svc.Restore(context.Background(), path); err == nil {
		t.Fatal("invalid archive accepted")
	}
	items, err := os.ReadDir(filepath.Join(root, "restored"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("invalid restore left %d directories", len(items))
	}
}

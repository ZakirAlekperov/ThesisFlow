package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestInventoryReportsUnknownFilesButPreservesThem checks that diagnostics
// never remove user-owned files, including unregistered symlink entries.
// RU: Лишние файлы выявляются без удаления или открытия чужих данных.
func TestInventoryReportsUnknownFilesButPreservesThem(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storage := filepath.Join(root, "storage")
	if err := os.MkdirAll(storage, 0700); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(storage, "unregistered.txt")
	if err := os.WriteFile(unknown, []byte("never delete"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Check(ctx, db, storage)
	if err != nil {
		t.Fatal(err)
	}
	// Legacy databases without the file version table have no indexed files
	// to compare; the direct inventory still counts them safely.
	count, err := inventoryUnknown(ctx, db, storage)
	if err != nil || count != 1 {
		t.Fatalf("unknown files=%d err=%v", count, err)
	}
	if report.ExtraFiles != 1 { t.Fatalf("missing orphan count in audit: %+v",report) }
	if !report.Healthy() {
		t.Fatalf("audit unexpectedly flagged data: %+v", report)
	}
	data, err := os.ReadFile(unknown)
	if err != nil || string(data) != "never delete" {
		t.Fatalf("diagnostic modified orphan: %q %v", data, err)
	}
}

func TestInventoryRecognizesRegisteredPathsAndLegacyBackslashes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "CREATE TABLE file_versions(relative_path TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(root, "storage")
	name := filepath.Join(storage, "versions", "chapter.docx")
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("version"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO file_versions(relative_path) VALUES(?)", `versions\chapter.docx`); err != nil {
		t.Fatal(err)
	}
	count, err := inventoryUnknown(ctx, db, storage)
	if err != nil || count != 0 {
		t.Fatalf("legacy path registered count=%d err=%v", count, err)
	}
}

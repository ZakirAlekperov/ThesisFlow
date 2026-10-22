package fileversions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestReadRejectsModifiedBytes protects downloads and approvals from reading
// corrupted immutable version files without warning.
// RU: Изменённые байты версии нельзя незаметно выдать преподавателю.
func TestReadRejectsModifiedBytes(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	saved, err := store.Save(context.Background(), studentID, requirementID, "test.docx", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(store.Root, saved.RelativePath)
	if err := os.WriteFile(target, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(context.Background(), saved.ID); !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("corrupted read error %v, want ErrInvalidFile", err)
	}
}

// TestReadRejectsSymlink prevents a path inside .internal from redirecting
// the version reader to a file outside the user's managed workspace.
// RU: Символическая ссылка внутри хранилища не должна читаться.
func TestReadRejectsSymlink(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	saved, err := store.Save(context.Background(), studentID, requirementID, "test.docx", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(store.Root, saved.RelativePath)
	outside := filepath.Join(t.TempDir(), "outside.docx")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := store.Read(context.Background(), saved.ID); !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("symlink read error %v, want ErrInvalidFile", err)
	}
}

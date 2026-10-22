package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestExportPreflightRefusesUnfinishedApprovalsAndDamagedPublicCopies
// verifies that a ZIP is never advertised as valid when SQLite and public
// files are already inconsistent.
// RU: Нельзя создать успешный ZIP при аварийной приёмке или порче файла.
func TestExportPreflightRefusesUnfinishedApprovalsAndDamagedPublicCopies(t *testing.T) {
	root, storage, service, studentID, requirementID, versionID := reviewBackupFixture(t)
	ctx := context.Background()
	var acceptedPath string
	if err := service.DB.QueryRowContext(ctx,
		"SELECT relative_path FROM accepted_files WHERE student_id=? AND requirement_id=?",
		studentID, requirementID).Scan(&acceptedPath); err != nil {
		t.Fatal(err)
	}
	_, err := service.DB.ExecContext(ctx, `INSERT INTO approval_operations(
	token,student_id,requirement_id,version_id,previous_version_id,
	previous_path,target_path,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		"abcdef123456abcdef123456", studentID, requirementID, versionID, versionID,
		acceptedPath, acceptedPath, "2026-10-22T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "should-not-exist.zip")
	if err := Export(ctx, service.DB, storage, archive); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("accepted pending journal for backup: %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("backup published with pending approval: %v", err)
	}
	if _, err := service.DB.ExecContext(ctx, "DELETE FROM approval_operations"); err != nil {
		t.Fatal(err)
	}
	publicFile := filepath.Join(storage, filepath.FromSlash(acceptedPath))
	if err := os.WriteFile(publicFile, []byte("changed outside ThesisFlow"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Export(ctx, service.DB, storage, archive); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("accepted corrupt public file for backup: %v", err)
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("backup published with corrupt accepted file: %v", err)
	}
}

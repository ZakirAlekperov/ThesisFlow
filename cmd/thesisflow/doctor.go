package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)

// ErrUnhealthyWorkspace marks a failed, read-only integrity check.
// RU: ErrUnhealthyWorkspace обозначает нарушения целостности данных.
var ErrUnhealthyWorkspace = errors.New("ThesisFlow workspace integrity failed")

// workspaceDoctor exposes the same audit to CLI and the local settings page.
// RU: Диагностика CLI и GUI использует один аудит SQLite и файлов.
type workspaceDoctor struct {
	db         *sql.DB
	storageDir string
}

// Check never performs automatic repair or removes unknown files.
// RU: Проверка данных без удаления файлов и скрытого исправления записей.
func (d workspaceDoctor) Check(ctx context.Context) (workspace.Report, error) {
	return workspace.Check(ctx, d.db, d.storageDir)
}

// runDoctorCommand audits only a pre-existing workspace and prints no
// student names or file paths. It must not create an empty replacement DB.
// RU: Команда doctor не создаёт пустую БД вместо отсутствующей.
func runDoctorCommand(ctx context.Context, root string, out io.Writer) error {
	dbPath := filepath.Join(root, "thesisflow.sqlite")
	info, err := os.Lstat(dbPath)
	if err != nil {
		return fmt.Errorf("database is unavailable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("database is not a regular file")
	}
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open existing SQLite: %w", err)
	}
	defer db.Close()
	report, err := (workspaceDoctor{db: db, storageDir: filepath.Join(root, "storage")}).Check(ctx)
	if err != nil {
		return fmt.Errorf("diagnostic query failed: %w", err)
	}
	if _, err := fmt.Fprintf(out, "SQLite schema: %d. Проверено версий: %d, целых: %d. Принятых копий: %d, целых: %d.\n",
		report.SchemaVersion, report.VersionsChecked, report.VersionsIntact,
		report.AcceptedChecked, report.AcceptedIntact); err != nil {
		return err
	}
	for _, issue := range report.Issues {
		if _, err := fmt.Fprintf(out, "Проблема: %s (версия %d, требование %d).\n",
			issue.Code, issue.VersionID, issue.RequirementID); err != nil {
			return err
		}
	}
	if !report.Healthy() {
		return ErrUnhealthyWorkspace
	}
	_, err = fmt.Fprintln(out, "Проверка завершена: критических нарушений не обнаружено.")
	return err
}

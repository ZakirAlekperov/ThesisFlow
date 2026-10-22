package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspacelock"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspaceselect"
)

// ErrUnsafeWorkspace rejects unverified, missing, damaged, or unsupported
// restored workspaces without overwriting the current default installation.
// RU: Подключаем только существующие и проверенные рабочие пространства.
var ErrUnsafeWorkspace = errors.New("restored workspace is unsafe or incompatible")

// validatedWorkspaceDB refuses nonexistent databases, symlinked roots,
// redirected storage or future DB schema before SQLite is opened.
// RU: Не создаём новую пустую SQLite вместо восстановленной копии.
func validatedWorkspaceDB(root string) (string, error) {
	if root == "" {
		return "", ErrUnsafeWorkspace
	}
	full, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", fmt.Errorf("%w: workspace folder: %v", ErrUnsafeWorkspace, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrUnsafeWorkspace
	}
	for _, entry := range []string{"thesisflow.sqlite", "storage"} {
		path := filepath.Join(full, entry)
		st, err := os.Lstat(path)
		if err != nil {
			if entry == "storage" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("%w: %s unavailable", ErrUnsafeWorkspace, entry)
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", ErrUnsafeWorkspace
		}
		if entry == "storage" && !st.IsDir() {
			return "", ErrUnsafeWorkspace
		}
		if entry == "thesisflow.sqlite" && !st.Mode().IsRegular() {
			return "", ErrUnsafeWorkspace
		}
	}
	return filepath.Join(full, "thesisflow.sqlite"), nil
}

// preflightWorkspace checks SQLite structural health and compatibility. Full
// accepted-file auditing takes place after crash-journal replay in start().
// RU: До восстановления журнала проверяем SQL; после — все файлы.
func preflightWorkspace(ctx context.Context, root string) error {
	dbPath, err := validatedWorkspaceDB(root)
	if err != nil {
		return err
	}
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("%w: open SQLite: %v", ErrUnsafeWorkspace, err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > migrate.CurrentVersion {
		return fmt.Errorf("%w: database has future schema %d", ErrUnsafeWorkspace, version)
	}
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master
 WHERE type='table' AND name IN ('student_groups','students')`).Scan(&tables); err != nil {
		return err
	}
	if tables != 2 {
		return fmt.Errorf("%w: missing ThesisFlow tables", ErrUnsafeWorkspace)
	}
	return checkDatabasePragmas(ctx, db)
}

func checkDatabasePragmas(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("%w: SQLite check failed", ErrUnsafeWorkspace)
	}
	var broken int
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		broken++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if broken != 0 {
		return fmt.Errorf("%w: SQLite foreign keys", ErrUnsafeWorkspace)
	}
	return nil
}

// runWorkspaceCommand inspects or opens an already restored workspace.
// No default workspace files or global settings are overwritten. Explicit
// "run" affects ONLY this invocation and terminates on Ctrl+C.
// RU: Выбор другого пространства не перезаписывает основную базу.
func runWorkspaceCommand(ctx context.Context, args []string, out io.Writer, openBrowser func(string) error) error {
	if len(args) == 2 && args[0] == "workspace" && args[1] == "reset" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		return resetWorkspaceChoice(ctx, filepath.Join(configDir, "ThesisFlow"), out)
	}
	if len(args) != 3 || args[0] != "workspace" || args[2] == "" {
		return fmt.Errorf("usage: thesisflow workspace inspect <directory> | workspace run <directory>")
	}
	root := args[2]
	switch args[1] {
	case "inspect":
		dbPath, err := validatedWorkspaceDB(root)
		if err != nil {
			return err
		}
		lock, err := workspacelock.Acquire(filepath.Dir(dbPath))
		if err != nil {
			return err
		}
		defer lock.Close()
		if err := preflightWorkspace(ctx, filepath.Dir(dbPath)); err != nil {
			return err
		}
		db, err := sqlite.Open(ctx, dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		report, err := workspace.Check(ctx, db, filepath.Join(filepath.Dir(dbPath), "storage"))
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "SQLite: schema %d. Files %d/%d, accepted %d/%d, unregistered %d.\n",
			report.SchemaVersion, report.VersionsIntact, report.VersionsChecked,
			report.AcceptedIntact, report.AcceptedChecked, report.ExtraFiles); err != nil {
			return err
		}
		if !report.Healthy() {
			return ErrUnsafeWorkspace
		}
		_, err = fmt.Fprintln(out, "Рабочее пространство прошло проверку целостности.")
		return err
	case "run":
		return startWithPreflight(ctx, out, localPort, filepath.Join(root, "thesisflow.sqlite"), openBrowser, true, nil)
	default:
		return fmt.Errorf("usage: thesisflow workspace inspect <directory> | workspace run <directory>")
	}
}

// resetWorkspaceChoice recovers from a missing/damaged selected workspace
// without launching the UI or overwriting database files.
// RU: Аварийный возврат к основной папке без удаления документов.
func resetWorkspaceChoice(ctx context.Context, root string, out io.Writer) error {
	if err := workspaceselect.New(root).Choose(ctx, workspaceselect.Default); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, "При следующем запуске будет открыто основное рабочее пространство.")
	return err
}

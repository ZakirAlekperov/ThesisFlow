package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspacelock"
)

// TestAlternateWorkspaceMustExistAndNeverReplacesDefault ensures the CLI
// cannot create an empty database at a misspelled restore location.
// RU: Ошибка пути к ZIP-копии не создаёт пустое рабочее пространство.
func TestAlternateWorkspaceMustExistAndNeverReplacesDefault(t *testing.T) {
	ctx := context.Background()
	selected := filepath.Join(t.TempDir(), "restored")
	var out bytes.Buffer
	if err := runWorkspaceCommand(ctx, []string{"workspace", "inspect", selected}, &out, nil); !errors.Is(err, ErrUnsafeWorkspace) {
		t.Fatalf("missing workspace accepted: %v", err)
	}
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Errorf("workspace inspect created a directory: %v", err)
	}
	if err := os.MkdirAll(selected, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(selected, "thesisflow.sqlite"), []byte("not SQLite"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runWorkspaceCommand(ctx, []string{"workspace", "inspect", selected}, &out, nil); err == nil {
		t.Fatal("invalid SQLite accepted as workspace")
	}
}

func TestRestoredWorkspaceInspectAndRun(t *testing.T) {
	root := filepath.Join(t.TempDir(), "restored")
	database := filepath.Join(root, "thesisflow.sqlite")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Create the working environment using the REAL application startup path.
	if err := start(ctx, io.Discard, "0", database, func(string) error { cancel(); return nil }); err != nil {
		t.Fatalf("initialize real workspace: %v", err)
	}
	var output bytes.Buffer
	if err := runWorkspaceCommand(context.Background(), []string{"workspace", "inspect", root}, &output, nil); err != nil {
		t.Fatalf("cannot inspect healthy restored workspace: %v", err)
	}
	if !strings.Contains(output.String(), "прошло проверку целостности") {
		t.Errorf("unexpected report: %s", output.String())
	}
	freshCtx, freshCancel := context.WithCancel(context.Background())
	defer freshCancel()
	output.Reset()
	if err := runWorkspaceCommand(freshCtx, []string{"workspace", "run", root}, &output, func(string) error {
		freshCancel()
		return nil
	}); err != nil {
		t.Fatalf("cannot launch selected workspace: %v", err)
	}
	if !strings.Contains(output.String(), "ThesisFlow: http://127.0.0.1:") {
		t.Errorf("workspace run did not launch browser server: %q", output.String())
	}
}

func TestWorkspaceLockGuardsRunningServerAndExternalBackup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	database := filepath.Join(root, "thesisflow.sqlite")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	running := make(chan error, 1)
	ready := make(chan struct{})
	go func() {
		running <- start(ctx, io.Discard, "0", database, func(string) error {
			close(ready)
			return nil
		})
	}()
	select {
	case <-ready:
	case err := <-running:
		t.Fatalf("server died before opening: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("startup timeout")
	}
	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	err := start(secondCtx, io.Discard, "0", database, func(string) error {
		t.Error("second instance opened browser")
		return nil
	})
	if !errors.Is(err, workspacelock.ErrInUse) {
		t.Fatalf("second instance not blocked: %v", err)
	}
	var backupOutput bytes.Buffer
	path := filepath.Join(t.TempDir(), "archive.zip")
	if err := runBackupCommand(context.Background(), root, []string{"backup", "export", path}, &backupOutput); !errors.Is(err, workspacelock.ErrInUse) {
		t.Errorf("external backup allowed while workspace running: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("blocked export created an archive")
	}
	cancel()
	select {
	case err := <-running:
		if err != nil {
			t.Fatalf("server shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server shutdown timeout")
	}
	// The OS lock is released automatically when the application exits.
	lock, err := workspacelock.Acquire(root)
	if err != nil {
		t.Fatalf("workspace lock remained held: %v", err)
	}
	_ = lock.Close()
}

func TestWorkspaceRejectsFutureSchemaBeforeMigration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "restored")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := start(ctx, io.Discard, "0", filepath.Join(root, "thesisflow.sqlite"), func(string) error {
		cancel()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=999"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	var output bytes.Buffer
	err = runWorkspaceCommand(context.Background(), []string{"workspace", "inspect", root}, &output, nil)
	if !errors.Is(err, ErrUnsafeWorkspace) {
		t.Fatalf("future schema accepted: %v", err)
	}
}

func TestWorkspaceRejectsSymlinkedDatabase(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.sqlite")
	if err := os.WriteFile(target, []byte("anything"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "thesisflow.sqlite")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := validatedWorkspaceDB(root); !errors.Is(err, ErrUnsafeWorkspace) {
		t.Fatalf("symlink database accepted: %v", err)
	}
}

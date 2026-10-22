package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestApprovalCrashChild is run only by TestApprovalSurvivesActualProcessExit.
// It terminates the process at one exact approval stage with os.Exit,
// bypassing deferred application cleanup.
// RU: Дочерний процесс завершается аварийно посреди приёмки.
func TestApprovalCrashChild(t *testing.T) {
	if os.Getenv("THESISFLOW_CRASH_CHILD") != "1" {
		t.Skip("subprocess helper")
	}
	ctx := context.Background()
	databasePath := os.Getenv("THESISFLOW_CRASH_DB")
	storageRoot := os.Getenv("THESISFLOW_CRASH_STORAGE")
	stage := os.Getenv("THESISFLOW_CRASH_STAGE")
	sid, err := strconv.ParseInt(os.Getenv("THESISFLOW_CRASH_STUDENT"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	rid, err := strconv.ParseInt(os.Getenv("THESISFLOW_CRASH_REQUIREMENT"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	vid, err := strconv.ParseInt(os.Getenv("THESISFLOW_CRASH_VERSION"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db, fileversions.NewStore(db, storageRoot))
	if err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	service.interruptApproval = func(at string) error {
		if at == stage {
			os.Exit(82)
		}
		return nil
	}
	if _, err := service.Accept(ctx, sid, rid, vid); err != nil {
		t.Fatalf("unexpected child approval error: %v", err)
	}
	t.Fatal("process exit was not reached")
}

// TestApprovalSurvivesActualProcessExit demonstrates that the durable
// intent is readable after the program really exits before/after SQLite
// commit, not merely after a synthetic returned error.
// RU: Тест проверяет восстановление после реального os.Exit дочернего Go-процесса.
func TestApprovalSurvivesActualProcessExit(t *testing.T) {
	for _, stage := range []string{"after_old_renamed", "after_publish", "after_commit"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			svc, sid, rid := testReview(t)
			old, err := svc.Versions.Save(ctx, sid, rid, "chapter.docx", []byte("old approved bytes"))
			if err != nil {
				t.Fatal(err)
			}
			approved, err := svc.Accept(ctx, sid, rid, old.ID)
			if err != nil {
				t.Fatal(err)
			}
			next, err := svc.Versions.Save(ctx, sid, rid, "new.docx", []byte("new approved bytes"))
			if err != nil {
				t.Fatal(err)
			}
			databasePath := filepath.Join(filepath.Dir(svc.Versions.Root), "thesisflow.sqlite")
			cmd := exec.Command(os.Args[0], "-test.run=^TestApprovalCrashChild$")
			cmd.Env = append(os.Environ(),
				"THESISFLOW_CRASH_CHILD=1",
				"THESISFLOW_CRASH_DB="+databasePath,
				"THESISFLOW_CRASH_STORAGE="+svc.Versions.Root,
				"THESISFLOW_CRASH_STAGE="+stage,
				"THESISFLOW_CRASH_STUDENT="+strconv.FormatInt(sid, 10),
				"THESISFLOW_CRASH_REQUIREMENT="+strconv.FormatInt(rid, 10),
				"THESISFLOW_CRASH_VERSION="+strconv.FormatInt(next.ID, 10),
			)
			output, err := cmd.CombinedOutput()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 82 {
				t.Fatalf("child did not crash at %q: %v / %s", stage, err, output)
			}
			if err := svc.DB.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sqlite.Open(ctx, databasePath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			restarted := NewService(db, fileversions.NewStore(db, svc.Versions.Root))
			if err := restarted.Init(ctx); err != nil {
				t.Fatal(err)
			}
			n, err := restarted.RecoverPending(ctx)
			if err != nil || n != 1 {
				t.Fatalf("real-crash replay recovered %d, error %v", n, err)
			}
			want := "old approved bytes"
			if stage == "after_commit" {
				want = "new approved bytes"
			}
			data, err := os.ReadFile(filepath.Join(svc.Versions.Root, filepath.FromSlash(approved.RelativePath)))
			if err != nil || string(data) != want {
				t.Fatalf("after %s got %q instead of %q: %v", stage, data, want, err)
			}
			var count int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM approval_operations").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal(fmt.Sprintf("left %d unfinished intent rows", count))
			}
		})
	}
}

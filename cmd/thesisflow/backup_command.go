package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// runBackupCommand exports or restores a complete local workspace. The caller
// provides the workspace directory so tests never access personal user files.
// Restore always targets a separate, previously nonexistent directory.
//
// RU: runBackupCommand экспортирует или восстанавливает локальное хранилище.
// Каталог передаётся явно для безопасного тестирования. Восстановление
// допускается только в новый, ещё не существующий каталог.
func runBackupCommand(ctx context.Context, workspace string, args []string, output io.Writer) error {
	if len(args) < 2 || args[0] != "backup" {
		return fmt.Errorf("usage: thesisflow backup export <archive.zip> | backup restore <archive.zip> <new-directory>")
	}

	switch args[1] {
	case "export":
		if len(args) != 3 || args[2] == "" {
			return fmt.Errorf("usage: thesisflow backup export <archive.zip>")
		}
		databasePath := filepath.Join(workspace, "thesisflow.sqlite")
		if _, err := os.Stat(databasePath); err != nil {
			return fmt.Errorf("database is absent: %w", err)
		}
		db, err := sqlite.Open(ctx, databasePath)
		if err != nil {
			return fmt.Errorf("open workspace database: %w", err)
		}
		defer db.Close()
		if err := backup.Export(ctx, db, filepath.Join(workspace, "storage"), args[2]); err != nil {
			return fmt.Errorf("export workspace: %w", err)
		}
		_, err = fmt.Fprintf(output, "Резервная копия создана: %s\n", args[2])
		return err
	case "restore":
		if len(args) != 4 || args[2] == "" || args[3] == "" {
			return fmt.Errorf("usage: thesisflow backup restore <archive.zip> <new-directory>")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := backup.Restore(args[2], args[3]); err != nil {
			return fmt.Errorf("restore workspace: %w", err)
		}
		_, err := fmt.Fprintf(output, "Копия восстановлена в отдельный каталог: %s\n", args[3])
		return err
	default:
		return fmt.Errorf("unknown backup operation %q (use export or restore)", args[1])
	}
}

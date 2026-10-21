package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
)

// workspaceRestore restores checked archives exclusively to a fresh folder
// inside the user's private ThesisFlow recovery area.
// RU: workspaceRestore восстанавливает проверенный архив в новую папку
// внутри защищённого каталога восстановления пользователя.
type workspaceRestore struct{ root string }

// Restore creates a random destination name and never replaces active SQLite.
// RU: Restore использует случайное имя и не заменяет рабочую базу SQLite.
func (s workspaceRestore) Restore(ctx context.Context, archivePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return "", fmt.Errorf("create recovery folder: %w", err)
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate recovery path: %w", err)
	}
	path := filepath.Join(s.root, "restore-"+hex.EncodeToString(random[:]))
	if err := backup.Restore(archivePath, path); err != nil {
		return "", fmt.Errorf("verify backup archive: %w", err)
	}
	return path, nil
}

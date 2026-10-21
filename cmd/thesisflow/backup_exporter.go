package main

import (
	"context"
	"database/sql"

	"github.com/ZakirAlekperov/ThesisFlow/internal/backup"
)

// workspaceBackup adapts the existing verified backup service to the
// localhost HTTP interface without exposing database files via URLs.
//
// RU: workspaceBackup подключает резервное копирование к локальному GUI
// без публикации путей базы данных через HTTP.
type workspaceBackup struct {
	db         *sql.DB
	storageDir string
}

// Export creates an archive in a new temporary destination provided by web.
// RU: Export создаёт резервную копию по временному пути, заданному web.
func (s workspaceBackup) Export(ctx context.Context, path string) error {
	return backup.Export(ctx, s.db, s.storageDir, path)
}

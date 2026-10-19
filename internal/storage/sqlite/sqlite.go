// Package sqlite opens the local persistent database used by ThesisFlow.
// Пакет sqlite открывает локальную постоянную базу данных ThesisFlow.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open creates or opens the SQLite database at path. It creates parent
// directories when necessary, restricts permissions on a newly created file,
// and enables SQLite foreign-key checks on its single pooled connection.
//
// RU: Open создаёт или открывает базу SQLite по указанному пути. Функция
// создаёт необходимые каталоги, ограничивает доступ к новому файлу и включает
// проверку внешних ключей для единственного соединения в пуле.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// A new database should not be world-readable on Unix systems.
	// RU: Новый файл базы не должен быть доступен посторонним пользователям Unix.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("prepare database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close prepared database file: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	// PRAGMA foreign_keys applies per connection; keep the initial pool simple.
	// RU: PRAGMA foreign_keys действует на отдельное соединение.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect SQLite: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable SQLite foreign keys: %w", err)
	}
	return db, nil
}

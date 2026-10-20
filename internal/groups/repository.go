// Package groups stores academic groups in the local ThesisFlow database.
// Пакет groups хранит учебные группы в локальной базе ThesisFlow.
package groups

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidName reports an empty group name after trimming whitespace.
// RU: ErrInvalidName означает пустое название после удаления пробелов.
var ErrInvalidName = errors.New("group name must not be empty")

// ErrDuplicateName reports that a group with the same normalized name exists.
// RU: ErrDuplicateName означает, что название группы уже используется.
var ErrDuplicateName = errors.New("group name already exists")

// Group represents a group with an immutable database identity.
// RU: Group представляет группу с постоянным идентификатором в базе.
type Group struct {
	ID   int64
	Name string
}

// Repository persists academic groups in SQLite.
// RU: Repository сохраняет учебные группы в SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository constructs a group repository using an existing database.
// The caller owns the database connection and closes it at application exit.
//
// RU: NewRepository создаёт хранилище групп на существующей базе.
// Закрытием соединения управляет вызывающая сторона.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Init creates the minimal group table without deleting existing data.
// The operation is safe to repeat at each application startup.
//
// RU: Init создаёт таблицу групп без удаления существующих данных.
// Вызывать метод можно при каждом запуске приложения.
func (r *Repository) Init(ctx context.Context) error {
	const schema = `CREATE TABLE IF NOT EXISTS student_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		name_key TEXT NOT NULL UNIQUE
	)`
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize groups table: %w", err)
	}
	return nil
}

// Create adds a new group and returns its permanent ID and display name.
// It trims surrounding whitespace and treats Unicode case variants as
// duplicates via a normalized name key. It never silently overwrites a group.
//
// RU: Create добавляет группу и возвращает постоянный ID и название.
// Пробелы по краям удаляются; регистр Unicode нормализуется для проверки
// повторов. Существующие группы не перезаписываются.
func (r *Repository) Create(ctx context.Context, name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, ErrInvalidName
	}

	const query = `INSERT INTO student_groups (name, name_key)
		VALUES (?, ?) ON CONFLICT(name_key) DO NOTHING`
	result, err := r.db.ExecContext(ctx, query, name, strings.ToLower(name))
	if err != nil {
		return Group{}, fmt.Errorf("insert group: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Group{}, fmt.Errorf("check group insertion: %w", err)
	}
	if affected == 0 {
		return Group{}, ErrDuplicateName
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Group{}, fmt.Errorf("read group ID: %w", err)
	}
	return Group{ID: id, Name: name}, nil
}

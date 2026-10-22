// Package migrate upgrades existing ThesisFlow SQLite databases in place.
// Пакет migrate обновляет существующую схему SQLite без потери данных.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CurrentVersion is the highest SQLite user_version understood by this build.
// RU: CurrentVersion — последняя версия схемы, поддерживаемая приложением.
const CurrentVersion = 2

// ErrFutureVersion prevents opening a database made by a newer application.
// RU: ErrFutureVersion защищает базы, созданные более новой версией.
var ErrFutureVersion = errors.New("database schema is newer than this application")

// Apply runs all schema upgrades in one transaction. It must be called after
// the existing group and student tables have been initialized; it preserves
// existing records and commits the version change with the new columns.
//
// RU: Apply выполняет миграции атомарно после создания базовых таблиц групп
// и студентов. Сведения пользователей сохраняются, номер схемы обновляется
// в той же транзакции, что и структура таблиц.
func Apply(ctx context.Context, db *sql.DB) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("start database migration: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read SQLite schema version: %w", err)
	}
	if version > CurrentVersion {
		return fmt.Errorf("%w: got %d, supports %d", ErrFutureVersion, version, CurrentVersion)
	}
	if version == 0 {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS thesis_records (
       student_id INTEGER PRIMARY KEY REFERENCES students(id) ON DELETE RESTRICT,
       topic TEXT NOT NULL DEFAULT '',
       order_reference TEXT NOT NULL DEFAULT '',
       updated_at TEXT NOT NULL DEFAULT (datetime('now'))
   )`,
			"PRAGMA user_version = 1",
		}
		// Previously created and freshly initialized schemas may differ.
		// RU: Добавляем поля только если их ещё нет в старой базе.
		for _, table := range []string{"student_groups", "students"} {
			query := "SELECT COUNT(*) FROM pragma_table_info('" + table + "') WHERE name='archived_at'"
			var present int
			if err := tx.QueryRowContext(ctx, query).Scan(&present); err != nil {
				return fmt.Errorf("inspect archive field: %w", err)
			}
			if present == 0 {
				if _, err := tx.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN archived_at TEXT"); err != nil {
					return fmt.Errorf("extend archive fields: %w", err)
				}
			}
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply SQLite migration to version 1: %w", err)
			}
		}
	}
	// Version 2 adds teacher-managed configuration only. Current requirements
	// and material events remain the single source of truth for submissions.
	// RU: Миграция создаёт профили/сроки/шаблоны, не меняя историю ВКР.
	if version < 2 {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS requirement_profiles (
				id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE,
				description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`,
			`CREATE TABLE IF NOT EXISTS profile_revisions (
				id INTEGER PRIMARY KEY, profile_id INTEGER NOT NULL REFERENCES requirement_profiles(id),
				number INTEGER NOT NULL, created_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(profile_id, number)
			)`,
			`CREATE TABLE IF NOT EXISTS profile_items (
				revision_id INTEGER NOT NULL REFERENCES profile_revisions(id),
				item_key TEXT NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
				position INTEGER NOT NULL CHECK(position >= 0),
				PRIMARY KEY(revision_id,item_key), UNIQUE(revision_id,position)
			)`,
			`CREATE TABLE IF NOT EXISTS group_profile_applications (
				id INTEGER PRIMARY KEY, group_id INTEGER NOT NULL REFERENCES student_groups(id),
				profile_id INTEGER NOT NULL REFERENCES requirement_profiles(id),
				revision_id INTEGER NOT NULL REFERENCES profile_revisions(id),
				applied_at TEXT NOT NULL DEFAULT (datetime('now')),
				added_count INTEGER NOT NULL DEFAULT 0,
				UNIQUE(group_id,revision_id)
			)`,
			`CREATE TABLE IF NOT EXISTS requirement_asset_bindings (
				requirement_id INTEGER PRIMARY KEY REFERENCES requirements(id),
				asset_id INTEGER NOT NULL REFERENCES template_assets(id)
			)`,
			`CREATE TABLE IF NOT EXISTS group_profile_bindings (
				group_id INTEGER NOT NULL REFERENCES student_groups(id),
				profile_id INTEGER NOT NULL REFERENCES requirement_profiles(id),
				item_key TEXT NOT NULL, requirement_id INTEGER NOT NULL REFERENCES requirements(id),
				PRIMARY KEY(group_id,profile_id,item_key),
				UNIQUE(group_id,requirement_id)
			)`,
			`CREATE TABLE IF NOT EXISTS group_profile_state (
				group_id INTEGER PRIMARY KEY REFERENCES student_groups(id),
				profile_id INTEGER NOT NULL REFERENCES requirement_profiles(id),
				revision_id INTEGER NOT NULL REFERENCES profile_revisions(id),
				applied_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`,
			`CREATE TABLE IF NOT EXISTS due_overrides (
				student_id INTEGER NOT NULL REFERENCES students(id),
				requirement_id INTEGER NOT NULL REFERENCES requirements(id),
				kind TEXT NOT NULL CHECK(kind IN ('date','none')),
				due_date TEXT NOT NULL DEFAULT '',
				PRIMARY KEY(student_id,requirement_id)
			)`,
			`CREATE TABLE IF NOT EXISTS requirement_changes (
				id INTEGER PRIMARY KEY, group_id INTEGER NOT NULL REFERENCES student_groups(id),
				requirement_id INTEGER NOT NULL REFERENCES requirements(id),
				student_id INTEGER REFERENCES students(id),
				old_value TEXT NOT NULL, new_value TEXT NOT NULL,
				kind TEXT NOT NULL, actor TEXT NOT NULL DEFAULT 'local-teacher',
				changed_at TEXT NOT NULL DEFAULT (datetime('now'))
			)`,
			`CREATE TABLE IF NOT EXISTS template_assets (
				id INTEGER PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
				requirement_title TEXT NOT NULL DEFAULT '', current_version INTEGER NOT NULL DEFAULT 0
			)`,
			`CREATE TABLE IF NOT EXISTS template_versions (
				id INTEGER PRIMARY KEY, asset_id INTEGER NOT NULL REFERENCES template_assets(id),
				number INTEGER NOT NULL, filename TEXT NOT NULL, relative_path TEXT NOT NULL UNIQUE,
				size INTEGER NOT NULL, sha256 TEXT NOT NULL,
				created_at TEXT NOT NULL DEFAULT (datetime('now')),
				UNIQUE(asset_id,number)
			)`,
			"PRAGMA user_version = 2",
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply SQLite migration to version 2: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite migration: %w", err)
	}
	return nil
}

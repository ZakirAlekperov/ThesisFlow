// Package management updates existing academic records without changing IDs.
// Пакет management обновляет записи без удаления и изменения постоянных ID.
package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Typed errors let HTTP handlers distinguish invalid data, duplicates and
// missing archived/active records without exposing internal SQLite details.
// RU: Типизированные ошибки используются для безопасной проверки форм.
var (
	ErrInvalid   = errors.New("invalid record data")
	ErrDuplicate = errors.New("normalized identifier already exists")
	ErrNotFound  = errors.New("record does not exist")
	ErrAcceptedFiles = errors.New("accepted documents require file-path reconciliation before renaming")
)

// Repository operates on records introduced by the version-one migration.
// RU: Repository работает с записями схемы после миграции v1.
type Repository struct{ db *sql.DB }

// NewRepository wraps an existing application-owned SQLite connection.
// RU: NewRepository использует соединение, которым владеет приложение.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func exists(ctx context.Context, db *sql.DB, statement string, args ...any) (bool, error) {
	var present int
	if err := db.QueryRowContext(ctx, statement, args...).Scan(&present); err != nil {
		return false, err
	}
	return present != 0, nil
}
func changed(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// hasAcceptedFiles blocks path-sensitive edits while approved files retain
// human-readable names. No check is required on a legacy database without this table.
// RU: hasAcceptedFiles защищает пути принятых документов от рассинхронизации.
func (r *Repository) hasAcceptedFiles(ctx context.Context, recordID int64, byGroup bool) (bool, error) {
	ready, err := exists(ctx, r.db, "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='accepted_files')")
	if err != nil { return false, err }
	if !ready { return false, nil }
	if byGroup {
		return exists(ctx, r.db, `SELECT EXISTS(
			SELECT 1 FROM accepted_files af JOIN students st ON af.student_id=st.id
			WHERE st.group_id=? )`, recordID)
	}
	return exists(ctx, r.db, "SELECT EXISTS(SELECT 1 FROM accepted_files WHERE student_id=?)", recordID)
}

// RenameGroup updates display and case-folded index while preserving ID.
// RU: RenameGroup меняет название группы, сохраняя постоянный ID.
func (r *Repository) RenameGroup(ctx context.Context, id int64, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if id <= 0 || name == "" {
		return ErrInvalid
	}
	key := strings.ToLower(name)
	duplicate, err := exists(ctx, r.db, "SELECT EXISTS(SELECT 1 FROM student_groups WHERE name_key=? AND id<>?)", key, id)
	if err != nil {
		return fmt.Errorf("check duplicate group: %w", err)
	}
	if duplicate {
		return ErrDuplicate
	}
	approved, err := r.hasAcceptedFiles(ctx, id, true)
	if err != nil { return fmt.Errorf("check accepted group files: %w", err) }
	if approved { return ErrAcceptedFiles }
	result, err := r.db.ExecContext(ctx, "UPDATE student_groups SET name=?, name_key=? WHERE id=?", name, key, id)
	if err != nil {
		return fmt.Errorf("rename academic group: %w", err)
	}
	return changed(result, nil)
}

// SetGroupArchived hides/restores a group without deleting members or files.
// RU: SetGroupArchived архивирует или восстанавливает группу без удаления данных.
func (r *Repository) SetGroupArchived(ctx context.Context, id int64, archived bool) error {
	if id <= 0 {
		return ErrInvalid
	}
	result, err := r.db.ExecContext(ctx, "UPDATE student_groups SET archived_at=CASE WHEN ? THEN datetime('now') ELSE NULL END WHERE id=?", archived, id)
	if err != nil {
		return fmt.Errorf("archive academic group: %w", err)
	}
	return changed(result, nil)
}

// UpdateStudent safely corrects a student's ID and full name in place.
// RU: UpdateStudent исправляет ФИО и номер, сохраняя внутренний ID и файлы.
func (r *Repository) UpdateStudent(ctx context.Context, id int64, educationalID, fullName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	educationalID = strings.TrimSpace(educationalID)
	fullName = strings.TrimSpace(fullName)
	if id <= 0 || educationalID == "" || fullName == "" {
		return ErrInvalid
	}
	key := strings.ToLower(educationalID)
	duplicate, err := exists(ctx, r.db, "SELECT EXISTS(SELECT 1 FROM students WHERE educational_id_key=? AND id<>?)", key, id)
	if err != nil {
		return fmt.Errorf("check duplicate student: %w", err)
	}
	if duplicate {
		return ErrDuplicate
	}
	approved, err := r.hasAcceptedFiles(ctx, id, false)
	if err != nil { return fmt.Errorf("check accepted student files: %w", err) }
	if approved { return ErrAcceptedFiles }
	result, err := r.db.ExecContext(ctx, "UPDATE students SET educational_id=?, educational_id_key=?, full_name=? WHERE id=?", educationalID, key, fullName, id)
	if err != nil {
		return fmt.Errorf("update student: %w", err)
	}
	return changed(result, nil)
}

// MoveStudent transfers a student to an existing active group in place.
// RU: MoveStudent переводит студента только в существующую активную группу.
func (r *Repository) MoveStudent(ctx context.Context, id, target int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id <= 0 || target <= 0 {
		return ErrInvalid
	}
	ok, err := exists(ctx, r.db, "SELECT EXISTS(SELECT 1 FROM student_groups WHERE id=? AND archived_at IS NULL)", target)
	if err != nil {
		return fmt.Errorf("check target group: %w", err)
	}
	if !ok {
		return ErrNotFound
	}
	approved, err := r.hasAcceptedFiles(ctx, id, false)
	if err != nil { return fmt.Errorf("check accepted student files: %w", err) }
	if approved { return ErrAcceptedFiles }
	result, err := r.db.ExecContext(ctx, "UPDATE students SET group_id=? WHERE id=?", target, id)
	if err != nil {
		return fmt.Errorf("move student: %w", err)
	}
	return changed(result, nil)
}

// SetStudentArchived is a reversible soft-delete without touching revisions.
// RU: SetStudentArchived архивирует запись без удаления версий документов.
func (r *Repository) SetStudentArchived(ctx context.Context, id int64, archived bool) error {
	if id <= 0 {
		return ErrInvalid
	}
	result, err := r.db.ExecContext(ctx, "UPDATE students SET archived_at=CASE WHEN ? THEN datetime('now') ELSE NULL END WHERE id=?", archived, id)
	if err != nil {
		return fmt.Errorf("archive student: %w", err)
	}
	return changed(result, nil)
}

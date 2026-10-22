// Package students persists students and their academic group membership.
// Пакет students сохраняет студентов и принадлежность учебным группам.
package students

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidEducationalID reports an empty student educational identifier.
// RU: ErrInvalidEducationalID означает пустой учебный идентификатор студента.
var ErrInvalidEducationalID = errors.New("student educational ID must not be empty")

// ErrInvalidFullName reports an empty student name after trimming whitespace.
// RU: ErrInvalidFullName означает пустое ФИО после удаления пробелов по краям.
var ErrInvalidFullName = errors.New("student full name must not be empty")

// ErrGroupNotFound reports an absent or invalid parent academic group.
// RU: ErrGroupNotFound означает отсутствующую или недопустимую учебную группу.
var ErrGroupNotFound = errors.New("student academic group does not exist")

// ErrDuplicateEducationalID reports an identifier already used by any student.
// RU: ErrDuplicateEducationalID означает учебный ID, занятый другим студентом.
var ErrDuplicateEducationalID = errors.New("student educational ID already exists")

// Student holds a permanent database ID, one group and an educational identity.
// RU: Student содержит постоянный ID записи, одну группу и учебный идентификатор.
type Student struct {
	ID            int64
	GroupID       int64
	EducationalID string
	FullName      string
}

// Repository manages student records in the application's SQLite database.
// RU: Repository управляет записями студентов в локальной базе SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a student repository; the caller owns the database.
// RU: NewRepository создаёт хранилище; соединением управляет вызывающая сторона.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Init creates the minimal student table and its group lookup index without
// deleting existing rows. It requires the academic groups table to exist.
// The educational ID is unique across all groups; the foreign key ensures that
// every student references exactly one existing academic group.
//
// RU: Init создаёт таблицу студентов и индекс группы, не удаляя записи.
// Таблица групп должна существовать. Учебный ID уникален для всех групп,
// внешний ключ обеспечивает принадлежность существующей группе.
func (r *Repository) Init(ctx context.Context) error {
	const schema = `CREATE TABLE IF NOT EXISTS students (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES student_groups(id) ON DELETE RESTRICT,
		educational_id TEXT NOT NULL,
		educational_id_key TEXT NOT NULL UNIQUE,
		full_name TEXT NOT NULL,
		archived_at TEXT,
		CHECK (group_id > 0),
		CHECK (length(trim(educational_id)) > 0),
		CHECK (length(trim(full_name)) > 0)
	)`
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize students table: %w", err)
	}
	const index = `CREATE INDEX IF NOT EXISTS idx_students_group_id ON students (group_id)`
	if _, err := r.db.ExecContext(ctx, index); err != nil {
		return fmt.Errorf("initialize students group index: %w", err)
	}
	return nil
}

// Create persists a student in exactly one existing academic group.
// It trims the name and educational ID, and compares IDs case-insensitively
// using a normalized database key. Full names need not be unique.
// Existing records are not overwritten when an educational ID is duplicated.
//
// RU: Create сохраняет студента в одной существующей учебной группе.
// ФИО и учебный ID очищаются от пробелов по краям; идентификаторы проверяются
// на повторы без учёта регистра. Совпадающие ФИО допустимы.
// При повторе учебного ID существующие данные не перезаписываются.
func (r *Repository) Create(ctx context.Context, groupID int64, educationalID, fullName string) (Student, error) {
	if err := ctx.Err(); err != nil {
		return Student{}, err
	}
	educationalID = strings.TrimSpace(educationalID)
	fullName = strings.TrimSpace(fullName)
	if educationalID == "" {
		return Student{}, ErrInvalidEducationalID
	}
	if fullName == "" {
		return Student{}, ErrInvalidFullName
	}
	if groupID <= 0 {
		return Student{}, ErrGroupNotFound
	}

	var exists int
	if err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM student_groups WHERE id = ?)", groupID,
	).Scan(&exists); err != nil {
		return Student{}, fmt.Errorf("check student group: %w", err)
	}
	if exists == 0 {
		return Student{}, ErrGroupNotFound
	}

	const query = `INSERT INTO students (group_id, educational_id, educational_id_key, full_name)
		VALUES (?, ?, ?, ?) ON CONFLICT(educational_id_key) DO NOTHING`
	result, err := r.db.ExecContext(ctx, query, groupID, educationalID, strings.ToLower(educationalID), fullName)
	if err != nil {
		return Student{}, fmt.Errorf("insert student: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Student{}, fmt.Errorf("check student insertion: %w", err)
	}
	if affected == 0 {
		return Student{}, ErrDuplicateEducationalID
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Student{}, fmt.Errorf("read student ID: %w", err)
	}
	return Student{
		ID: id, GroupID: groupID, EducationalID: educationalID, FullName: fullName,
	}, nil
}

// ListByGroup returns the students of one existing academic group in a
// repeatable order: full name (SQLite binary text order), then permanent ID.
// A valid but empty group returns a non-nil empty slice. Missing groups return
// ErrGroupNotFound. Failed queries never expose partially loaded records.
//
// RU: ListByGroup возвращает студентов одной существующей учебной группы.
// Порядок стабилен: ФИО (бинарная сортировка SQLite), затем постоянный ID.
// Для пустой существующей группы возвращается пустой (не nil) срез.
// Отсутствующая группа даёт ErrGroupNotFound; при ошибке частичных данных нет.
func (r *Repository) ListByGroup(ctx context.Context, groupID int64) ([]Student, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, ErrGroupNotFound
	}

	var exists int
	if err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM student_groups WHERE id = ?)", groupID,
	).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check students group: %w", err)
	}
	if exists == 0 {
		return nil, ErrGroupNotFound
	}

	const query = `SELECT id, group_id, educational_id, full_name
		FROM students
		WHERE group_id = ? AND archived_at IS NULL
		ORDER BY full_name COLLATE BINARY, id`
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group students: %w", err)
	}
	defer rows.Close()

	result := make([]Student, 0)
	for rows.Next() {
		var student Student
		if err := rows.Scan(&student.ID, &student.GroupID, &student.EducationalID, &student.FullName); err != nil {
			return nil, fmt.Errorf("scan group student: %w", err)
		}
		result = append(result, student)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group students: %w", err)
	}
	return result, nil
}

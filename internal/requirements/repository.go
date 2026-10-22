// Package requirements manages documents expected from thesis students.
// Пакет requirements хранит требования к документам студентов ВКР.
package requirements

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidTitle reports a document requirement without a meaningful name.
// RU: ErrInvalidTitle означает пустое название обязательного документа.
var ErrInvalidTitle = errors.New("requirement title is empty")

// ErrInvalidScope reports a missing group or student/group mismatch.
// RU: ErrInvalidScope означает неизвестную группу или чужого студента.
var ErrInvalidScope = errors.New("requirement group/student scope invalid")

// ErrInvalidDate reports a deadline not in valid YYYY-MM-DD form.
// RU: ErrInvalidDate означает некорректный срок сдачи YYYY-MM-DD.
var ErrInvalidDate = errors.New("requirement deadline is invalid")

// ErrDuplicateTitle reports a repeated requirement within the same scope.
// RU: ErrDuplicateTitle означает повторное название в одной области действия.
var ErrDuplicateTitle = errors.New("requirement title already exists")

// Requirement specifies one mandatory or student-specific thesis document.
// StudentID == 0 means that the requirement belongs to the whole group.
//
// RU: Requirement описывает обязательный документ ВКР. StudentID == 0
// означает требование для всей группы.
type Requirement struct {
	ID        int64
	GroupID   int64
	StudentID int64
	Title     string
	DueDate   string
}

// Repository persists requirements in SQLite without duplicating group-wide
// requirements for every student.
// RU: Repository хранит требования группы один раз, без копий у студентов.
type Repository struct{ db *sql.DB }

// NewRepository uses the caller-owned initialized SQLite connection.
// RU: NewRepository использует соединение SQLite вызывающей стороны.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Init creates the requirement table and two scope-specific unique indexes.
// It does not erase or replace any existing user data.
//
// RU: Init создаёт таблицу требований и индексы уникальности отдельно
// для группы и студента. Существующие данные не удаляются.
func (r *Repository) Init(ctx context.Context) error {
	const schema = `CREATE TABLE IF NOT EXISTS requirements (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_id INTEGER NOT NULL REFERENCES student_groups(id) ON DELETE RESTRICT,
		student_id INTEGER REFERENCES students(id) ON DELETE RESTRICT,
		title TEXT NOT NULL,
		title_key TEXT NOT NULL,
		due_date TEXT NOT NULL DEFAULT '',
		CHECK (length(trim(title)) > 0)
	)`
	if _, err := r.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize requirements: %w", err)
	}
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_requirements_group_title ON requirements(group_id, title_key) WHERE student_id IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_requirements_student_title ON requirements(student_id, title_key) WHERE student_id IS NOT NULL`,
	}
	for _, statement := range indexes {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize requirement indexes: %w", err)
		}
	}
	// Older unit-test workspaces initialize requirements directly without
	// invoking the app migration. Keep this optional override table available.
	// RU: Сроки студентов доступны и в автономном тестовом хранилище.
	if _, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS due_overrides (
		student_id INTEGER NOT NULL REFERENCES students(id),
		requirement_id INTEGER NOT NULL REFERENCES requirements(id),
		kind TEXT NOT NULL CHECK(kind IN ('date','none')),
		due_date TEXT NOT NULL DEFAULT '',
		PRIMARY KEY(student_id,requirement_id))`); err != nil {
		return fmt.Errorf("initialize due overrides: %w", err)
	}
	return nil
}

// Create adds a group-wide requirement when studentID == 0 or an individual
// requirement for a student belonging to groupID. The optional deadline is
// validated as a real ISO calendar day. Existing requirements are untouched.
//
// RU: Create создаёт общее (studentID == 0) либо персональное требование.
// Студент должен принадлежать группе, дата должна быть реальной датой ISO.
// Уже созданные документы не меняются.
func (r *Repository) Create(ctx context.Context, groupID, studentID int64, title, dueDate string) (Requirement, error) {
	if err := ctx.Err(); err != nil {
		return Requirement{}, err
	}
	title = strings.TrimSpace(title)
	dueDate = strings.TrimSpace(dueDate)
	if title == "" {
		return Requirement{}, ErrInvalidTitle
	}
	if dueDate != "" {
		parsed, err := time.Parse("2006-01-02", dueDate)
		if err != nil || parsed.Format("2006-01-02") != dueDate {
			return Requirement{}, ErrInvalidDate
		}
	}
	if groupID <= 0 || studentID < 0 {
		return Requirement{}, ErrInvalidScope
	}
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM student_groups WHERE id = ?)", groupID).Scan(&exists); err != nil {
		return Requirement{}, fmt.Errorf("check requirement group: %w", err)
	}
	if exists == 0 {
		return Requirement{}, ErrInvalidScope
	}
	var databaseStudentID any
	if studentID > 0 {
		if err := r.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM students WHERE id = ? AND group_id = ?)", studentID, groupID,
		).Scan(&exists); err != nil {
			return Requirement{}, fmt.Errorf("check requirement student: %w", err)
		}
		if exists == 0 {
			return Requirement{}, ErrInvalidScope
		}
		databaseStudentID = studentID
	}

	result, err := r.db.ExecContext(ctx,
		`INSERT INTO requirements(group_id, student_id, title, title_key, due_date)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		groupID, databaseStudentID, title, strings.ToLower(title), dueDate,
	)
	if err != nil {
		return Requirement{}, fmt.Errorf("create requirement: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Requirement{}, fmt.Errorf("check requirement insertion: %w", err)
	}
	if affected == 0 {
		return Requirement{}, ErrDuplicateTitle
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Requirement{}, fmt.Errorf("read requirement ID: %w", err)
	}
	return Requirement{ID: id, GroupID: groupID, StudentID: studentID, Title: title, DueDate: dueDate}, nil
}

// ListForGroup returns requirements shared by the entire group, ordered by
// normalized title and permanent ID; personal requirements remain private.
//
// RU: ListForGroup выдаёт только общие требования группы в постоянном порядке.
func (r *Repository) ListForGroup(ctx context.Context, groupID int64) ([]Requirement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, ErrInvalidScope
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,group_id,title,due_date FROM requirements
		WHERE group_id=? AND student_id IS NULL ORDER BY title_key,id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group requirements: %w", err)
	}
	defer rows.Close()
	result := make([]Requirement, 0)
	for rows.Next() {
		var x Requirement
		if err := rows.Scan(&x.ID, &x.GroupID, &x.Title, &x.DueDate); err != nil {
			return nil, fmt.Errorf("scan requirement: %w", err)
		}
		result = append(result, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read group requirements: %w", err)
	}
	return result, nil
}

// ListForStudent reads inherited group requirements and personal overrides,
// without copying rows when the group-level profile changes.
//
// RU: ListForStudent читает требования группы и личные требования студента.
// Изменение профиля группы видно всем студентам без копирования записей.
func (r *Repository) ListForStudent(ctx context.Context, groupID, studentID int64) ([]Requirement, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID <= 0 || studentID <= 0 {
		return nil, ErrInvalidScope
	}
	var exists int
	if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM students WHERE id=? AND group_id=?)", studentID, groupID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check student requirement scope: %w", err)
	}
	if exists == 0 {
		return nil, ErrInvalidScope
	}
	rows, err := r.db.QueryContext(ctx, `SELECT r.id,r.group_id,r.student_id,r.title,
		CASE WHEN o.kind='none' THEN '' WHEN o.kind='date' THEN o.due_date ELSE r.due_date END
		FROM requirements r LEFT JOIN due_overrides o ON o.student_id=? AND o.requirement_id=r.id
		WHERE r.group_id=? AND (r.student_id IS NULL OR r.student_id=?)
		ORDER BY r.title_key,r.id`, studentID, groupID, studentID)
	if err != nil {
		return nil, fmt.Errorf("list student requirements: %w", err)
	}
	defer rows.Close()
	result := make([]Requirement, 0)
	for rows.Next() {
		var x Requirement
		var personal sql.NullInt64
		if err := rows.Scan(&x.ID, &x.GroupID, &personal, &x.Title, &x.DueDate); err != nil {
			return nil, fmt.Errorf("scan student requirement: %w", err)
		}
		if personal.Valid {
			x.StudentID = personal.Int64
		}
		result = append(result, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read student requirements: %w", err)
	}
	return result, nil
}

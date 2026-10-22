// Package materials tracks document submission status and event history.
// Пакет materials хранит состояния сдачи документов и историю событий.
package materials

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Status is an internal stable key for a teacher-visible document state.
// RU: Status — постоянный ключ состояния документа для интерфейса.
type Status string

const (
	Missing       Status = "missing"
	UnderReview   Status = "under_review"
	NeedsRevision Status = "needs_revision"
	Accepted      Status = "accepted"
)

// ErrInvalidScope reports an unrelated student and document requirement.
// RU: ErrInvalidScope означает чужого студента или неподходящее требование.
var ErrInvalidScope = errors.New("material student/requirement scope invalid")

// ErrInvalidStatus prevents storing unknown arbitrary workflow states.
// RU: ErrInvalidStatus запрещает неизвестные значения состояния сдачи.
var ErrInvalidStatus = errors.New("material status is invalid")

// Event is one immutable registered status change with a system timestamp.
// RU: Event — неизменяемое событие изменения состояния с системным временем.
type Event struct {
	ID            int64
	StudentID     int64
	RequirementID int64
	Status        Status
	At            time.Time
}

// Entry is the current status for a student's expected document.
// RU: Entry — текущее состояние ожидаемого документа студента.
type Entry struct {
	StudentID     int64
	RequirementID int64
	Status        Status
	At            time.Time
}

// Repository owns the SQL operations and an optional deterministic test clock.
// The application should not accept arbitrary user-supplied submission dates.
//
// RU: Repository выполняет SQL-операции; Clock внедряется только для тестов.
// Пользователь не должен произвольно менять время регистрации сдачи.
type Repository struct {
	db    *sql.DB
	Clock func() time.Time
}

// NewRepository uses the application's existing SQLite connection.
// RU: NewRepository использует общую SQLite-базу ThesisFlow.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, Clock: time.Now}
}

// Init creates the current status and append-only event history tables.
// RU: Init создаёт текущие состояния и журнал событий без удаления записей.
func (r *Repository) Init(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS material_status (
			student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
			requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
			status TEXT NOT NULL CHECK (status IN ('missing','under_review','needs_revision','accepted')),
			changed_at TEXT NOT NULL,
			PRIMARY KEY(student_id, requirement_id)
		)`,
		`CREATE TABLE IF NOT EXISTS material_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
			requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
			status TEXT NOT NULL,
			occurred_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_material_events_scope ON material_events(student_id,requirement_id,id)`,
	}
	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize material history: %w", err)
		}
	}
	return nil
}

// validStatus checks supported states before any SQL write.
// RU: validStatus проверяет состояния до выполнения SQL-записи.
func validStatus(status Status) bool {
	switch status {
	case Missing, UnderReview, NeedsRevision, Accepted:
		return true
	}
	return false
}

// readTimestamp decodes timestamps created by the application clock.
// RU: readTimestamp читает системные отметки времени из SQLite.
func readTimestamp(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

// Get returns the persisted current status or Missing when no material has
// ever been submitted. A missing status does not generate a history event.
//
// RU: Get возвращает текущее состояние или «Не сдан» при отсутствии записи.
// Чтение состояния не создаёт фиктивных событий.
func (r *Repository) Get(ctx context.Context, studentID, requirementID int64) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if studentID <= 0 || requirementID <= 0 {
		return Entry{}, ErrInvalidScope
	}
	out := Entry{StudentID: studentID, RequirementID: requirementID, Status: Missing}
	var changed string
	err := r.db.QueryRowContext(ctx, `SELECT status,changed_at FROM material_status WHERE student_id=? AND requirement_id=?`, studentID, requirementID).Scan(&out.Status, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Entry{}, fmt.Errorf("read material status: %w", err)
	}
	out.At, err = readTimestamp(changed)
	if err != nil {
		return Entry{}, fmt.Errorf("decode status time: %w", err)
	}
	return out, nil
}

// Set atomically updates current material status and adds an immutable event.
// Scope is verified in SQL: the requirement must belong to the same group
// and must either be group-wide or specifically assigned to the student.
// Repeating an unchanged state is a no-op and does not duplicate history.
//
// RU: Set атомарно обновляет состояние и добавляет событие истории.
// Документ должен относиться к группе студента. Повтор неизменного
// состояния не создаёт дублирующих событий.
func (r *Repository) Set(ctx context.Context, studentID, requirementID int64, next Status) (Entry, error) {
	if err := ctx.Err(); err != nil {
		return Entry{}, err
	}
	if !validStatus(next) {
		return Entry{}, ErrInvalidStatus
	}
	if studentID <= 0 || requirementID <= 0 {
		return Entry{}, ErrInvalidScope
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, fmt.Errorf("begin material status transaction: %w", err)
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM students s
		JOIN requirements r ON r.group_id=s.group_id
		WHERE s.id=? AND r.id=? AND (r.student_id IS NULL OR r.student_id=s.id)
	)`, studentID, requirementID).Scan(&exists)
	if err != nil {
		return Entry{}, fmt.Errorf("check material scope: %w", err)
	}
	if exists == 0 {
		return Entry{}, ErrInvalidScope
	}
	var current Status
	var changed string
	err = tx.QueryRowContext(ctx, `SELECT status,changed_at FROM material_status
		WHERE student_id=? AND requirement_id=?`, studentID, requirementID).Scan(&current, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		current = Missing
	} else if err != nil {
		return Entry{}, fmt.Errorf("read current material status: %w", err)
	}
	if current == next {
		previousTime, err := readTimestamp(changed)
		if err != nil {
			return Entry{}, fmt.Errorf("decode material status time: %w", err)
		}
		return Entry{StudentID: studentID, RequirementID: requirementID, Status: next, At: previousTime}, nil
	}
	now := r.Clock().UTC()
	timestamp := now.Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO material_status(student_id,requirement_id,status,changed_at)
		VALUES(?,?,?,?)
		ON CONFLICT(student_id,requirement_id) DO UPDATE SET status=excluded.status,changed_at=excluded.changed_at`,
		studentID, requirementID, string(next), timestamp)
	if err != nil {
		return Entry{}, fmt.Errorf("update material status: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO material_events(student_id,requirement_id,status,occurred_at)
		VALUES(?,?,?,?)`, studentID, requirementID, string(next), timestamp)
	if err != nil {
		return Entry{}, fmt.Errorf("record material status history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, fmt.Errorf("commit material status: %w", err)
	}
	return Entry{StudentID: studentID, RequirementID: requirementID, Status: next, At: now}, nil
}

// History returns the ordered, append-only record of status changes.
// RU: History возвращает неизменяемую историю изменений по времени записи.
func (r *Repository) History(ctx context.Context, studentID, requirementID int64) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,student_id,requirement_id,status,occurred_at
		FROM material_events WHERE student_id=? AND requirement_id=? ORDER BY id`, studentID, requirementID)
	if err != nil {
		return nil, fmt.Errorf("read material history: %w", err)
	}
	defer rows.Close()
	result := make([]Event, 0)
	for rows.Next() {
		var e Event
		var timestamp string
		if err := rows.Scan(&e.ID, &e.StudentID, &e.RequirementID, &e.Status, &timestamp); err != nil {
			return nil, fmt.Errorf("scan material event: %w", err)
		}
		e.At, err = readTimestamp(timestamp)
		if err != nil {
			return nil, fmt.Errorf("decode material history time: %w", err)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate material events: %w", err)
	}
	return result, nil
}

// GroupStatuses reads all material statuses for one academic group with a
// single indexed query. Missing pairs are omitted and interpreted as Missing
// by the caller; no query runs for every student/requirement combination.
// RU: Все статусы группы читаются одним запросом без N+1.
func (r *Repository) GroupStatuses(ctx context.Context, groupID int64) (map[[2]int64]Status, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, ErrInvalidScope
	}
	const query = `SELECT m.student_id,m.requirement_id,m.status
		FROM material_status m
		JOIN students s ON s.id=m.student_id AND s.group_id=?
		JOIN requirements req ON req.id=m.requirement_id
			AND req.group_id=s.group_id
			AND (req.student_id IS NULL OR req.student_id=s.id)`
	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("query group statuses: %w", err)
	}
	defer rows.Close()
	result := make(map[[2]int64]Status)
	for rows.Next() {
		var studentID, requirementID int64
		var status Status
		if err := rows.Scan(&studentID, &requirementID, &status); err != nil {
			return nil, fmt.Errorf("scan group statuses: %w", err)
		}
		if !validStatus(status) {
			return nil, fmt.Errorf("unknown stored material status: %q", status)
		}
		result[[2]int64{studentID, requirementID}] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate group statuses: %w", err)
	}
	return result, nil
}

// Package thesis stores one approved research topic per student.
// Пакет thesis хранит одну утверждённую тему ВКР на студента.
package thesis

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidTopic rejects an empty graduation thesis title.
// RU: ErrInvalidTopic означает пустую тему ВКР.
var ErrInvalidTopic = errors.New("thesis topic must not be empty")

// ErrOrderRequired prevents a topic change without a new order reference.
// RU: ErrOrderRequired запрещает смену темы без нового приказа.
var ErrOrderRequired = errors.New("new order reference is required")

// ErrStudentNotFound reports that a thesis cannot be assigned to an unknown student.
// RU: ErrStudentNotFound — студент с таким ID не существует.
var ErrStudentNotFound = errors.New("thesis student does not exist")

// Record stores one student's thesis topic and approval reference.
// RU: Record хранит тему ВКР и реквизиты приказа об утверждении.
type Record struct {
	StudentID      int64
	Topic          string
	OrderReference string
}

// Repository accesses thesis_records, introduced by migration version one.
// RU: Repository работает с таблицей thesis_records после миграции v1.
type Repository struct{ db *sql.DB }

// NewRepository uses the application's SQLite connection.
// RU: NewRepository использует соединение с SQLite приложения.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Get returns a blank topic when the student has no thesis assignment yet.
// RU: Get возвращает пустую тему, если ВКР ещё не утверждена.
func (r *Repository) Get(ctx context.Context, studentID int64) (Record, error) {
	result := Record{StudentID: studentID}
	err := r.db.QueryRowContext(ctx,
		"SELECT topic, order_reference FROM thesis_records WHERE student_id = ?", studentID,
	).Scan(&result.Topic, &result.OrderReference)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return Record{}, fmt.Errorf("read student's thesis: %w", err)
	}
	return result, nil
}

// Set assigns a topic only with an order reference. A changed topic must use
// a different order reference than the previously stored approval. Existing
// student identity is preserved. The update and checks share one transaction.
//
// RU: Set сохраняет тему только с приказом. Изменение темы требует новых
// реквизитов приказа; проверки и запись выполняются одной транзакцией.
func (r *Repository) Set(ctx context.Context, studentID int64, topic, order string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	topic = strings.TrimSpace(topic)
	order = strings.TrimSpace(order)
	if topic == "" {
		return Record{}, ErrInvalidTopic
	}
	if order == "" {
		return Record{}, ErrOrderRequired
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, fmt.Errorf("begin thesis update: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM students WHERE id=?)", studentID).Scan(&exists); err != nil {
		return Record{}, fmt.Errorf("check thesis student: %w", err)
	}
	if exists == 0 {
		return Record{}, ErrStudentNotFound
	}
	var previousTopic, previousOrder string
	err = tx.QueryRowContext(ctx, "SELECT topic, order_reference FROM thesis_records WHERE student_id=?", studentID).
		Scan(&previousTopic, &previousOrder)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Record{}, fmt.Errorf("read approved thesis: %w", err)
	}
	if err == nil && previousTopic != topic && previousOrder == order {
		return Record{}, ErrOrderRequired
	}
	const query = `INSERT INTO thesis_records(student_id, topic, order_reference)
		VALUES (?, ?, ?) ON CONFLICT(student_id) DO UPDATE SET
		topic=excluded.topic, order_reference=excluded.order_reference,
		updated_at=datetime('now')`
	if _, err := tx.ExecContext(ctx, query, studentID, topic, order); err != nil {
		return Record{}, fmt.Errorf("save thesis approval: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, fmt.Errorf("commit thesis approval: %w", err)
	}
	return Record{StudentID: studentID, Topic: topic, OrderReference: order}, nil
}

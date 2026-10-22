// Package reviewqueue provides read-only teacher review lists and document
// completeness statistics directly from persisted SQLite state.
// Пакет reviewqueue рассчитывает очередь проверки и комплектность по SQLite.
package reviewqueue

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	// PageSize bounds review queue UI pages for predictable memory usage.
	// RU: Ограничиваем размер очереди в интерфейсе.
	PageSize = 50
)

// Pending represents the latest version of a document awaiting review.
// RU: Pending содержит только актуальную версию, ожидающую проверки.
type Pending struct {
	GroupID          int64
	GroupName        string
	StudentID        int64
	StudentName      string
	EducationalID    string
	RequirementID    int64
	RequirementTitle string
	VersionID        int64
	OriginalName     string
	UploadedAt       string
}

// Completeness summarizes applicable group-wide and personal requirements.
// A zero Total is a non-applicable state, not 100% complete.
// RU: Комплектность включает общие и персональные требования.
type Completeness struct {
	StudentID     int64
	FullName      string
	EducationalID string
	Total         int
	Accepted      int
	UnderReview   int
	NeedsRevision int
	Missing       int
	Overdue       int
}

// Repository reads the existing tables and never stores derived metrics.
// RU: Repository не создаёт таблиц и не меняет статусы документов.
type Repository struct{ db *sql.DB }

// New creates a read-only teacher workflow projection.
// RU: New создаёт слой чтения из существующей SQLite.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// ListPending returns active students' most recent uploaded revisions in the
// under_review state. Offset is bounded and results are ordered oldest first.
// No stale older version can be submitted for review from this list.
// RU: Выдаёт только последнюю версию у активных студентов/групп.
func (r *Repository) ListPending(ctx context.Context, groupID int64, offset int) ([]Pending, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID < 0 || offset < 0 || offset > 100000 {
		return nil, fmt.Errorf("invalid queue filters")
	}
	const query = `SELECT g.id,g.name,s.id,s.full_name,s.educational_id,
		r.id,r.title,v.id,v.original_name,v.uploaded_at
	FROM material_status m
	JOIN students s ON s.id=m.student_id
	JOIN student_groups g ON g.id=s.group_id
	JOIN requirements r ON r.id=m.requirement_id AND r.group_id=s.group_id
	JOIN file_versions v ON v.student_id=m.student_id
		AND v.requirement_id=m.requirement_id
		AND v.id=(SELECT MAX(v2.id) FROM file_versions v2
			WHERE v2.student_id=m.student_id AND v2.requirement_id=m.requirement_id)
	WHERE m.status='under_review' AND s.archived_at IS NULL
		AND g.archived_at IS NULL AND (r.student_id IS NULL OR r.student_id=s.id)
		AND (?=0 OR g.id=?)
	ORDER BY v.uploaded_at,v.id LIMIT ? OFFSET ?`
	// Fetch one extra row to show a next-page affordance without COUNT queries.
	rows, err := r.db.QueryContext(ctx, query, groupID, groupID, PageSize+1, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending revisions: %w", err)
	}
	defer rows.Close()
	list := make([]Pending, 0)
	for rows.Next() {
		var entry Pending
		if err := rows.Scan(&entry.GroupID, &entry.GroupName, &entry.StudentID,
			&entry.StudentName, &entry.EducationalID, &entry.RequirementID,
			&entry.RequirementTitle, &entry.VersionID, &entry.OriginalName, &entry.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan review queue: %w", err)
		}
		list = append(list, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review queue: %w", err)
	}
	return list, nil
}

// GroupCompleteness counts every applicable requirement for each active
// student, including personal requirements. A LEFT JOIN keeps students with
// zero requirements visible. Deadlines are compared as ISO local dates.
// RU: Один SQL-запрос даёт комплектность, включая пустые наборы документов.
func (r *Repository) GroupCompleteness(ctx context.Context, groupID int64, today time.Time) ([]Completeness, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, fmt.Errorf("invalid group identifier")
	}
	const query = `SELECT s.id,s.full_name,s.educational_id,
		COUNT(r.id),
		COALESCE(SUM(CASE WHEN r.id IS NOT NULL AND m.status='accepted' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN r.id IS NOT NULL AND m.status='under_review' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN r.id IS NOT NULL AND m.status='needs_revision' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN r.id IS NOT NULL AND r.due_date<>'' AND r.due_date<?
			AND COALESCE(m.status,'missing')<>'accepted' THEN 1 ELSE 0 END),0)
	FROM students s
	JOIN student_groups g ON g.id=s.group_id AND g.archived_at IS NULL
	LEFT JOIN requirements r ON r.group_id=s.group_id AND (r.student_id IS NULL OR r.student_id=s.id)
	LEFT JOIN material_status m ON m.student_id=s.id AND m.requirement_id=r.id
	WHERE s.group_id=? AND s.archived_at IS NULL
	GROUP BY s.id,s.full_name,s.educational_id
	ORDER BY s.full_name,s.id`
	rows, err := r.db.QueryContext(ctx, query, today.Format("2006-01-02"), groupID)
	if err != nil {
		return nil, fmt.Errorf("load group completeness: %w", err)
	}
	defer rows.Close()
	list := make([]Completeness, 0)
	for rows.Next() {
		var c Completeness
		if err := rows.Scan(&c.StudentID, &c.FullName, &c.EducationalID,
			&c.Total, &c.Accepted, &c.UnderReview, &c.NeedsRevision, &c.Overdue); err != nil {
			return nil, fmt.Errorf("scan completeness: %w", err)
		}
		c.Missing = c.Total - c.Accepted - c.UnderReview - c.NeedsRevision
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completeness: %w", err)
	}
	return list, nil
}

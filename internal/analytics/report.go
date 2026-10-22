// Package analytics calculates read-only teacher reports from active SQLite
// students, requirements, current statuses and the accepted-files index.
// Пакет analytics формирует актуальные отчёты из SQLite без кеширования.
package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrInvalidFilter is returned for unsupported or malformed report criteria.
// RU: ErrInvalidFilter означает недопустимые параметры отчёта.
var ErrInvalidFilter = errors.New("invalid report filters")

// ErrUnknownGroup marks a requested archived/nonexistent academic group.
// RU: ErrUnknownGroup означает недоступную активную учебную группу.
var ErrUnknownGroup = errors.New("group not found")

// Filter describes the report selection. GroupID == 0 includes every active
// group; Status == "" includes all applicable documents.
// RU: Filter определяет группу, поиск, статус и просрочки.
type Filter struct {
	GroupID     int64
	Search      string
	Status      string
	OverdueOnly bool
	Sort        string
}

// Document is one applicable requirement and its live workflow status.
// ApprovedCopy records the accepted projection independently from Status.
// RU: Document хранит текущий статус и факт прежней принятой копии отдельно.
type Document struct {
	GroupID       int64
	GroupName     string
	StudentID     int64
	StudentName   string
	EducationalID string
	RequirementID int64
	Title         string
	DueDate       string
	Status        string
	ApprovedCopy  bool
	OverdueDays   int
}

// Student provides an aggregate for one active student.
// Percent is undefined when Required == 0; check HasRequirements.
// RU: Студент без требований не получает ложную готовность 100%.
type Student struct {
	GroupID       int64
	GroupName     string
	ID            int64
	Name          string
	EducationalID string
	Metrics
	NearestDue string
}

// Metrics counts current, applicable requirements rather than upload events.
// Required == Accepted + UnderReview + NeedsRevision + Missing.
// RU: Показатели описывают текущий срез обязательных документов.
type Metrics struct {
	Students         int
	Required         int
	Accepted         int
	UnderReview      int
	NeedsRevision    int
	Missing          int
	Overdue          int
	WithApprovedCopy int
}

// Percent computes accepted share only for nonempty sets.
// RU: Нулевой знаменатель возвращает false вместо 100%.
func (m Metrics) Percent() (int, bool) {
	if m.Required == 0 {
		return 0, false
	}
	return m.Accepted * 100 / m.Required, true
}

// Group is the comparative summary of one active group.
// RU: Group содержит показатели учебной группы.
type Group struct {
	ID   int64
	Name string
	Metrics
}

// Report is one consistent, read-only database snapshot for HTML/CSV/XLSX.
// RU: Report передаёт один набор данных всем экспортным форматам.
type Report struct {
	Filter      Filter
	GeneratedAt time.Time
	Groups      []Group
	Students    []Student
	Documents   []Document
	Totals      Metrics
}

// Repository owns report queries. No schema changes are required.
// RU: Repository только читает существующую SQLite-базу.
type Repository struct{ DB *sql.DB }

// New constructs analytics over the application-owned connection.
// RU: New использует открытую БД приложения.
func New(db *sql.DB) *Repository { return &Repository{DB: db} }

// CheckFilter validates bounded filters, avoiding unbounded UI input.
// RU: Запрещаем неизвестные статусы, сортировки и слишком длинный поиск.
func CheckFilter(f Filter) error {
	if f.GroupID < 0 || len([]rune(f.Search)) > 120 {
		return ErrInvalidFilter
	}
	switch f.Status {
	case "", "missing", "accepted", "under_review", "needs_revision":
	default:
		return ErrInvalidFilter
	}
	switch f.Sort {
	case "", "name", "ready", "overdue", "deadline":
	default:
		return ErrInvalidFilter
	}
	return nil
}

// Build queries all active group/student/requirement/status/approval rows in
// one joined statement (no student-by-requirement N+1 requests), then applies
// the same filters to report rows and metrics. now determines local deadlines.
// RU: Build формирует отчёт одним пакетным SQL-запросом.
func (r *Repository) Build(ctx context.Context, f Filter, now time.Time) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	f.Search = strings.TrimSpace(f.Search)
	if err := CheckFilter(f); err != nil {
		return Report{}, err
	}
	result := Report{Filter: f, GeneratedAt: now, Groups: make([]Group, 0), Students: make([]Student, 0), Documents: make([]Document, 0)}
	const query = `SELECT g.id,g.name,s.id,COALESCE(s.full_name,''),COALESCE(s.educational_id,''),
 req.id,COALESCE(req.title,''),CASE WHEN o.kind='none' THEN '' WHEN o.kind='date' THEN o.due_date ELSE COALESCE(req.due_date,'') END,COALESCE(m.status,'missing'),
 CASE WHEN af.version_id IS NULL THEN 0 ELSE 1 END
 FROM student_groups g
 LEFT JOIN students s ON s.group_id=g.id AND s.archived_at IS NULL
 LEFT JOIN requirements req ON req.group_id=g.id
  AND s.id IS NOT NULL AND (req.student_id IS NULL OR req.student_id=s.id)
 LEFT JOIN due_overrides o ON o.student_id=s.id AND o.requirement_id=req.id
 LEFT JOIN material_status m ON m.student_id=s.id AND m.requirement_id=req.id
 LEFT JOIN accepted_files af ON af.student_id=s.id AND af.requirement_id=req.id
 WHERE g.archived_at IS NULL AND (?=0 OR g.id=?)
 ORDER BY g.name,g.id,s.full_name,s.id,req.title_key,req.id`
	rows, err := r.DB.QueryContext(ctx, query, f.GroupID, f.GroupID)
	if err != nil {
		return Report{}, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()
	groups := make(map[int64]*Group)
	students := make(map[int64]*Student)
	shownStudents := make(map[int64]bool)
	var activeGroups int
	today := now.Format("2006-01-02")
	for rows.Next() {
		var gID int64
		var gName, stName, stEdu, ruleName, due, status string
		var stID, ruleID sql.NullInt64
		var approved int
		if err := rows.Scan(&gID, &gName, &stID, &stName, &stEdu, &ruleID, &ruleName, &due, &status, &approved); err != nil {
			return Report{}, fmt.Errorf("scan analytics: %w", err)
		}
		if groups[gID] == nil {
			groups[gID] = &Group{ID: gID, Name: gName}
			activeGroups++
		}
		if !stID.Valid {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(stName+" "+stEdu), strings.ToLower(f.Search)) {
			continue
		}
		if students[stID.Int64] == nil {
			students[stID.Int64] = &Student{GroupID: gID, GroupName: gName, ID: stID.Int64, Name: stName, EducationalID: stEdu}
		}
		if !ruleID.Valid {
			// A student without requirements is visible, not 100% ready.
			if f.Status == "" && !f.OverdueOnly {
				shownStudents[stID.Int64] = true
			}
			continue
		}
		if status != "missing" && status != "under_review" && status != "needs_revision" && status != "accepted" {
			return Report{}, fmt.Errorf("unknown persisted status %q", status)
		}
		days := 0
		if due != "" && due < today && status != "accepted" {
			deadline, parseErr := time.Parse("2006-01-02", due)
			if parseErr != nil {
				return Report{}, fmt.Errorf("invalid stored deadline: %w", parseErr)
			}
			// UTC civil-day subtraction is independent of daylight-saving shifts.
			y, m, d := now.Date()
			current := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			days = int(current.Sub(deadline).Hours() / 24)
		}
		if f.Status != "" && status != f.Status {
			continue
		}
		if f.OverdueOnly && days == 0 {
			continue
		}
		doc := Document{GroupID: gID, GroupName: gName, StudentID: stID.Int64, StudentName: stName,
			EducationalID: stEdu, RequirementID: ruleID.Int64, Title: ruleName, DueDate: due,
			Status: status, ApprovedCopy: approved == 1, OverdueDays: days}
		result.Documents = append(result.Documents, doc)
		shownStudents[stID.Int64] = true
		st := students[stID.Int64]
		addDocument(&st.Metrics, doc)
		if status != "accepted" && due != "" && (st.NearestDue == "" || due < st.NearestDue) {
			st.NearestDue = due
		}
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("read analytics rows: %w", err)
	}
	if f.GroupID != 0 && activeGroups == 0 {
		return Report{}, ErrUnknownGroup
	}
	ids := make([]int64, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := groups[ids[i]], groups[ids[j]]
		if a.Name == b.Name {
			return a.ID < b.ID
		}
		return a.Name < b.Name
	})
	for _, id := range ids {
		result.Groups = append(result.Groups, *groups[id])
	}
	// Build student and group totals from the *same* filtered documents.
	studentIDs := make([]int64, 0, len(students))
	for id := range shownStudents {
		studentIDs = append(studentIDs, id)
	}
	sort.Slice(studentIDs, func(i, j int) bool {
		a, b := students[studentIDs[i]], students[studentIDs[j]]
		if a.GroupName != b.GroupName {
			return a.GroupName < b.GroupName
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	for _, id := range studentIDs {
		student := *students[id]
		student.Students = 1
		result.Students = append(result.Students, student)
		g := groups[student.GroupID]
		g.Students++
		addMetrics(&g.Metrics, student.Metrics)
		result.Totals.Students++
		addMetrics(&result.Totals, student.Metrics)
	}
	for _, id := range ids { // Update summaries after student aggregation.
		for i := range result.Groups {
			if result.Groups[i].ID == id {
				result.Groups[i] = *groups[id]
				break
			}
		}
	}
	switch f.Sort {
	case "ready":
		sort.SliceStable(result.Students, func(i, j int) bool {
			a, b := result.Students[i], result.Students[j]
			pa, oka := a.Metrics.Percent()
			pb, okb := b.Metrics.Percent()
			if oka != okb {
				return !oka
			}
			return pa < pb
		})
	case "overdue":
		sort.SliceStable(result.Students, func(i, j int) bool { return result.Students[i].Overdue > result.Students[j].Overdue })
	case "deadline":
		sort.SliceStable(result.Students, func(i, j int) bool {
			a, b := result.Students[i].NearestDue, result.Students[j].NearestDue
			if a == "" {
				return false
			}
			if b == "" {
				return true
			}
			return a < b
		})
	}
	return result, nil
}

func addDocument(m *Metrics, d Document) {
	m.Required++
	switch d.Status {
	case "accepted":
		m.Accepted++
	case "under_review":
		m.UnderReview++
	case "needs_revision":
		m.NeedsRevision++
	default:
		m.Missing++
	}
	if d.OverdueDays > 0 {
		m.Overdue++
	}
	if d.ApprovedCopy {
		m.WithApprovedCopy++
	}
}
func addMetrics(total *Metrics, m Metrics) {
	total.Required += m.Required
	total.Accepted += m.Accepted
	total.UnderReview += m.UnderReview
	total.NeedsRevision += m.NeedsRevision
	total.Missing += m.Missing
	total.Overdue += m.Overdue
	total.WithApprovedCopy += m.WithApprovedCopy
}

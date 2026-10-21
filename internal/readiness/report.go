// Package readiness calculates academic-group progress from accepted documents.
// Пакет readiness вычисляет готовность группы по принятым документам.
package readiness

import (
	"context"
	"fmt"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// Summary reports document completion and overdue requirements in one group.
// RU: Summary показывает принятие документов и просрочки одной группы.
type Summary struct {
	GroupID      int64
	StudentCount int
	Required     int
	Accepted     int
	Overdue      int
	Percent      int
}

// Service reads existing student, requirement and material repositories.
// RU: Service использует существующие репозитории студентов, требований и сдач.
type Service struct {
	Students     *students.Repository
	Requirements *requirements.Repository
	Materials    *materials.Repository
}

// New composes readiness calculation without introducing a new database table.
// RU: New собирает расчёт готовности без новых таблиц SQLite.
func New(st *students.Repository, req *requirements.Repository, mat *materials.Repository) *Service {
	return &Service{Students: st, Requirements: req, Materials: mat}
}

// Group calculates percent as accepted documents divided by applicable
// requirements across all students; a zero denominator means 0%, not 100%.
// Deadlines are overdue only when strictly earlier than the given calendar day
// and the corresponding document has not been accepted.
//
// RU: Group делит число принятых документов на число требований всех студентов.
// Если требований нет, готовность равна 0%. Просрочка возникает, когда срок
// строго раньше текущего календарного дня, а документ ещё не принят.
func (s *Service) Group(ctx context.Context, groupID int64, today time.Time) (Summary, error) {
	result := Summary{GroupID: groupID}
	people, err := s.Students.ListByGroup(ctx, groupID)
	if err != nil {
		return Summary{}, fmt.Errorf("read students for readiness: %w", err)
	}
	result.StudentCount = len(people)
	day := today.Format("2006-01-02")
	for _, student := range people {
		expected, err := s.Requirements.ListForStudent(ctx, groupID, student.ID)
		if err != nil {
			return Summary{}, fmt.Errorf("read document requirements: %w", err)
		}
		for _, document := range expected {
			result.Required++
			entry, err := s.Materials.Get(ctx, student.ID, document.ID)
			if err != nil {
				return Summary{}, fmt.Errorf("read material status: %w", err)
			}
			if entry.Status == materials.Accepted {
				result.Accepted++
			} else if document.DueDate != "" && document.DueDate < day {
				result.Overdue++
			}
		}
	}
	if result.Required > 0 {
		result.Percent = result.Accepted * 100 / result.Required
	}
	return result, nil
}

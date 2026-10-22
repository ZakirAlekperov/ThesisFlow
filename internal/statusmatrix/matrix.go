// Package statusmatrix builds the teacher's read-only group document grid.
// Пакет statusmatrix формирует ведомость документов студентов группы.
package statusmatrix

import (
	"context"
	"fmt"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// Cell contains the current material status for one student/requirement pair.
// RU: Cell хранит статус обязательного документа одного студента.
type Cell struct {
	RequirementID int64
	Status        materials.Status
}

// Row groups one student's statuses without changing stored records.
// RU: Row содержит студента и статусы его документов без изменений SQLite.
type Row struct {
	Student students.Student
	Cells   []Cell
}

// Matrix is a complete group-wide requirement grid in stable database order.
// Only group-wide requirements become columns; individual requirements remain
// visible on the student's own document page.
//
// RU: Matrix — ведомость общих требований учебной группы.
// Индивидуальные требования остаются на странице конкретного студента.
type Matrix struct {
	GroupID      int64
	Requirements []requirements.Requirement
	Rows         []Row
}

// Service composes the already-tested student, requirement and status readers.
// RU: Service использует действующие репозитории студентов, требований и статусов.
type Service struct {
	Students     *students.Repository
	Requirements *requirements.Repository
	Materials    *materials.Repository
}

// New creates the read-only matrix service without adding SQLite tables.
// RU: New создаёт сервис ведомости без новых таблиц базы.
func New(st *students.Repository, req *requirements.Repository, mat *materials.Repository) *Service {
	return &Service{Students: st, Requirements: req, Materials: mat}
}

// Group reads current statuses for all students and group-wide requirements.
// The student list verifies group existence; a missing group returns its typed
// error, while other failures propagate with context and no partial matrix.
//
// RU: Group читает текущие статусы всех студентов и общих требований группы.
// Несуществующая группа вызывает типизированную ошибку, другие ошибки
// передаются без выдачи неполной ведомости.
func (s *Service) Group(ctx context.Context, groupID int64) (Matrix, error) {
	people, err := s.Students.ListByGroup(ctx, groupID)
	if err != nil {
		return Matrix{}, fmt.Errorf("list matrix students: %w", err)
	}
	required, err := s.Requirements.ListForGroup(ctx, groupID)
	if err != nil {
		return Matrix{}, fmt.Errorf("list matrix requirements: %w", err)
	}
	// A single status read replaces N×M individual SQLite queries.
	// RU: На всю группу выполняется один запрос статусов.
	statuses, err := s.Materials.GroupStatuses(ctx, groupID)
	if err != nil {
		return Matrix{}, fmt.Errorf("read matrix group statuses: %w", err)
	}
	result := Matrix{GroupID: groupID, Requirements: required, Rows: make([]Row, 0, len(people))}
	for _, person := range people {
		row := Row{Student: person, Cells: make([]Cell, 0, len(required))}
		for _, rule := range required {
			status, ok := statuses[[2]int64{person.ID, rule.ID}]
			if !ok {
				status = materials.Missing
			}
			row.Cells = append(row.Cells, Cell{RequirementID: rule.ID, Status: status})
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

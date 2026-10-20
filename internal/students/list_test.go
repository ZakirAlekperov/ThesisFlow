package students

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestListByGroupReturnsEmptySlice checks an existing group with no students.
// Scenario: create an academic group, but do not add student records.
// Expected: a non-nil empty slice and no error.
//
// RU: Проверяет чтение существующей группы без студентов.
// Сценарий: учебная группа создана, но студенты в неё не добавлены.
// Ожидается: пустой (не nil) срез и отсутствие ошибки.
func TestListByGroupReturnsEmptySlice(t *testing.T) {
	repo, groupRepo, _ := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")

	got, err := repo.ListByGroup(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("ListByGroup() error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("ListByGroup() = %#v, want non-nil empty slice", got)
	}
}

// TestListByGroupReturnsOnlyRequestedGroupInNameOrder ensures correct filtering,
// stable student IDs and repeatable ordering for identical full names.
// Scenario: interleave students from two groups, including same-name students.
// Expected: only the requested group; ordered by full_name and then student ID.
//
// RU: Проверяет фильтрацию по группе и стабильный порядок при одинаковых ФИО.
// Сценарий: студенты двух групп создаются вперемешку, два ФИО совпадают.
// Ожидается: только нужная группа с сортировкой по ФИО, затем по ID студента.
func TestListByGroupReturnsOnlyRequestedGroupInNameOrder(t *testing.T) {
	repo, groupRepo, _ := newTestRepository(t)
	ctx := context.Background()
	first := createTestGroup(t, groupRepo, "ИСП-401")
	other := createTestGroup(t, groupRepo, "ИСП-402")
	last, err := repo.Create(ctx, first.ID, "ЗК-004", "Яковлев Яков")
	if err != nil {
		t.Fatalf("create last student: %v", err)
	}
	if _, err := repo.Create(ctx, other.ID, "ЗК-003", "Алексеев Алексей"); err != nil {
		t.Fatalf("create student in other group: %v", err)
	}
	middle1, err := repo.Create(ctx, first.ID, "ЗК-002", "Иванов Иван")
	if err != nil {
		t.Fatalf("create first same-name student: %v", err)
	}
	firstStudent, err := repo.Create(ctx, first.ID, "ЗК-001", "Андреев Андрей")
	if err != nil {
		t.Fatalf("create first student: %v", err)
	}
	middle2, err := repo.Create(ctx, first.ID, "ЗК-005", "Иванов Иван")
	if err != nil {
		t.Fatalf("create second same-name student: %v", err)
	}

	want := []Student{firstStudent, middle1, middle2, last}
	for i := 0; i < 2; i++ {
		got, err := repo.ListByGroup(ctx, first.ID)
		if err != nil {
			t.Fatalf("ListByGroup() iteration %d: %v", i, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ListByGroup() iteration %d = %#v, want %#v", i, got, want)
		}
	}
}

// TestListByGroupRejectsUnknownGroup distinguishes an absent group from an
// existing group with no students.
// Scenario: request zero, negative, and unknown positive group IDs.
// Expected: ErrGroupNotFound and no student list.
//
// RU: Проверяет отличие несуществующей группы от существующей пустой.
// Сценарий: запрос с нулевым, отрицательным и неизвестным положительным ID.
// Ожидается: ErrGroupNotFound и отсутствие списка.
func TestListByGroupRejectsUnknownGroup(t *testing.T) {
	repo, _, _ := newTestRepository(t)
	for _, groupID := range []int64{0, -1, 9999} {
		got, err := repo.ListByGroup(context.Background(), groupID)
		if !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("ListByGroup(%d) error = %v, want ErrGroupNotFound", groupID, err)
		}
		if got != nil {
			t.Errorf("ListByGroup(%d) = %#v, want nil on failure", groupID, got)
		}
	}
}

// TestListByGroupSurvivesReopen verifies durable reads after restarting.
// Scenario: persist a student, close the database, reopen and list their group.
// Expected: all fields and both permanent IDs remain unchanged.
//
// RU: Проверяет сохранность чтения списка после перезапуска приложения.
// Сценарий: сохранить студента, закрыть базу, снова открыть и запросить группу.
// Ожидается: все поля, ID студента и ID группы остались прежними.
func TestListByGroupSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	groupRepo := groups.NewRepository(db)
	if err := groupRepo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("initialize groups: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("initialize students: %v", err)
	}
	group := createTestGroup(t, groupRepo, "ИСП-401")
	saved, err := repo.Create(ctx, group.ID, "ЗК-0017", "Иванов Иван")
	if err != nil {
		db.Close()
		t.Fatalf("create student: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()
	repo = NewRepository(reopened)
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("reinitialize students: %v", err)
	}
	got, err := repo.ListByGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("list students after restart: %v", err)
	}
	if !reflect.DeepEqual(got, []Student{saved}) {
		t.Errorf("ListByGroup() = %#v, want %#v", got, []Student{saved})
	}
}

// TestListByGroupRespectsCancellation checks the query cancellation contract.
// Scenario: cancel context before requesting an existing group's students.
// Expected: context.Canceled and no partial list.
//
// RU: Проверяет отмену чтения без выдачи частичного результата.
// Сценарий: контекст отменён до запроса студентов существующей группы.
// Ожидается: context.Canceled и nil вместо частичного списка.
func TestListByGroupRespectsCancellation(t *testing.T) {
	repo, groupRepo, _ := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := repo.ListByGroup(ctx, group.ID)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ListByGroup() error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("ListByGroup() = %#v, want nil on failure", got)
	}
}

// TestListByGroupPropagatesDatabaseErrors ensures failures are observable.
// Scenario: close the SQLite connection before listing students.
// Expected: a non-nil error and no misleading empty result.
//
// RU: Проверяет возврат реальных ошибок SQLite вместо пустого списка.
// Сценарий: закрыть соединение до получения списка студентов.
// Ожидается: ошибка и nil, а не пустой список.
func TestListByGroupPropagatesDatabaseErrors(t *testing.T) {
	repo, groupRepo, db := newTestRepository(t)
	group := createTestGroup(t, groupRepo, "ИСП-401")
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	got, err := repo.ListByGroup(context.Background(), group.ID)
	if err == nil {
		t.Fatal("ListByGroup() returned no error for a closed database")
	}
	if got != nil {
		t.Errorf("ListByGroup() = %#v, want nil on error", got)
	}
}

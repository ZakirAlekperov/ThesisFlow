package groups

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestListReturnsEmptySlice verifies that a new workspace has no groups.
// Scenario: query the initialized group repository before creating records.
// Expected: a non-nil, empty slice and no error.
//
// RU: Проверяет список групп в новом рабочем пространстве.
// Сценарий: чтение сразу после инициализации, до создания групп.
// Ожидается: пустой, но не nil-срез и отсутствие ошибки.
func TestListReturnsEmptySlice(t *testing.T) {
	repo, _ := newTestRepository(t)

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("List() = %#v, want non-nil empty slice", got)
	}
}

// TestListReturnsGroupsInStableNameOrder verifies ordering and identity.
// Scenario: insert three groups in an order different from their names.
// Expected: all original names and IDs ordered by normalized name.
//
// RU: Проверяет порядок списка и сохранение идентификаторов.
// Сценарий: три группы создаются не в алфавитном порядке.
// Ожидается: все исходные названия и ID, сортировка по нормализованному имени.
func TestListReturnsGroupsInStableNameOrder(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	last, err := repo.Create(ctx, "ПР-301")
	if err != nil {
		t.Fatalf("create last group: %v", err)
	}
	first, err := repo.Create(ctx, "  ИСП-401  ")
	if err != nil {
		t.Fatalf("create first group: %v", err)
	}
	middle, err := repo.Create(ctx, "ИСП-402")
	if err != nil {
		t.Fatalf("create middle group: %v", err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []Group{first, middle, last}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}

	again, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}
	if !reflect.DeepEqual(again, want) {
		t.Errorf("second List() = %#v, want %#v", again, want)
	}
}

// TestListPersistsAcrossReopen verifies that listing reads durable storage.
// Scenario: write a group, close SQLite and reopen the same database file.
// Expected: List returns the original group with its original ID.
//
// RU: Проверяет получение сохранённых групп после перезапуска.
// Сценарий: создать группу, закрыть SQLite и повторно открыть тот же файл.
// Ожидается: List возвращает исходные название и постоянный ID.
func TestListPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	repo := NewRepository(db)
	if err := repo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("initialize groups: %v", err)
	}
	saved, err := repo.Create(ctx, "ИСП-401")
	if err != nil {
		db.Close()
		t.Fatalf("create group: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close SQLite: %v", err)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen SQLite: %v", err)
	}
	defer reopened.Close()
	repo = NewRepository(reopened)
	if err := repo.Init(ctx); err != nil {
		t.Fatalf("reinitialize groups: %v", err)
	}
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() after reopening: %v", err)
	}
	if !reflect.DeepEqual(got, []Group{saved}) {
		t.Errorf("List() after reopening = %#v, want %#v", got, []Group{saved})
	}
}

// TestListRespectsCanceledContext verifies query cancellation.
// Scenario: pass a context canceled before reading the groups table.
// Expected: context.Canceled is preserved and no partial list is returned.
//
// RU: Проверяет отмену чтения списка групп.
// Сценарий: контекст отменён до выполнения запроса к SQLite.
// Ожидается: ошибка context.Canceled и отсутствие частичного списка.
func TestListRespectsCanceledContext(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := repo.List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("List() error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("List() = %#v, want nil on failure", got)
	}
}

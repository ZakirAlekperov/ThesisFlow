package importxlsx

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// setupImportDB creates one SQLite workspace and one academic group.
// RU: setupImportDB готовит отдельную SQLite и учебную группу для тестов.
func setupImportDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "thesisflow.sqlite"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _ = db.Close() })
	groupsRepo := groups.NewRepository(db)
	if err := groupsRepo.Init(context.Background()); err != nil { t.Fatal(err) }
	if err := students.NewRepository(db).Init(context.Background()); err != nil { t.Fatal(err) }
	group, err := groupsRepo.Create(context.Background(), "ИСП-401")
	if err != nil { t.Fatal(err) }
	return db, group.ID
}

// TestImportStudentsSelectsColumnsAndSkipsHeader validates mapping.
// Scenario: name and educational ID appear in non-adjacent XLSX columns.
// Expected: both students inserted with correct values and group IDs.
//
// RU: Проверяет выбор несоседних колонок и пропуск заголовка.
// Сценарий: ФИО и учебный номер находятся в колонках A и C.
// Ожидается: два студента с правильными полями и группой.
func TestImportStudentsSelectsColumnsAndSkipsHeader(t *testing.T) {
	db, groupID := setupImportDB(t)
	rows := [][]string{{"ФИО", "Курс", "Номер"}, {"Иванов Иван", "4", "ЗК-001"}, {"Петров Пётр", "4", "ЗК-002"}}
	count, err := ImportStudents(context.Background(), db, groupID, rows, 0, 2, true)
	if err != nil || count != 2 { t.Fatalf("import = %d, %v, want 2", count, err) }
	items, err := students.NewRepository(db).ListByGroup(context.Background(), groupID)
	if err != nil { t.Fatal(err) }
	if len(items)!=2 || items[0].EducationalID!="ЗК-001" || items[1].FullName!="Петров Пётр" {
		t.Errorf("got imported rows %#v", items)
	}
}

// TestImportStudentsRollsBackOnDuplicate protects atomicity.
// Scenario: import a new student followed by a globally duplicated ID.
// Expected: ErrDuplicateEducationalID and the new student not committed.
//
// RU: Проверяет полную отмену импорта при повторном учебном ID.
// Сценарий: сначала новый номер, затем уже занятый другой группой.
// Ожидается: ошибка дубликата без частично сохранённых строк.
func TestImportStudentsRollsBackOnDuplicate(t *testing.T) {
	db, groupID := setupImportDB(t)
	if _, err := students.NewRepository(db).Create(context.Background(), groupID, "ЗК-001", "Ранее Добавлен"); err != nil { t.Fatal(err) }
	rows := [][]string{{"Новый Студент", "ЗК-002"}, {"Повтор Номера", "зк-001"}}
	count, err := ImportStudents(context.Background(), db, groupID, rows, 0, 1, false)
	if !errors.Is(err, students.ErrDuplicateEducationalID) || count != 0 { t.Fatalf("count=%d, err=%v", count, err) }
	var exists int
	if err := db.QueryRow("SELECT count(*) FROM students WHERE educational_id_key = 'зк-002'").Scan(&exists); err != nil { t.Fatal(err) }
	if exists != 0 { t.Errorf("partially committed %d rows", exists) }
}

// TestImportStudentsRejectsBadMappingsAndMissingData disallows invalid
// column indexes and blank required values without modifying the database.
// RU: Проверяет индексы столбцов и обязательные поля без изменения SQLite.
func TestImportStudentsRejectsBadMappingsAndMissingData(t *testing.T) {
	db, groupID := setupImportDB(t)
	cases := []struct {
		rows [][]string
		nameCol int
		idCol int
	}{
		{[][]string{{"Имя", "ЗК-001"}}, -1, 1},
		{[][]string{{"Имя", "ЗК-001"}}, 0, 0},
		{[][]string{{"Имя", "ЗК-001"}}, 0, 7},
		{[][]string{{" ", "ЗК-001"}}, 0, 1},
		{[][]string{{"Иванов", " "}}, 0, 1},
	}
	for i, tc := range cases {
		if n, err := ImportStudents(context.Background(), db, groupID, tc.rows, tc.nameCol, tc.idCol, false); err == nil || n != 0 {
			t.Errorf("case %d: count=%d, err=%v", i, n, err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&count); err != nil { t.Fatal(err) }
	if count != 0 { t.Errorf("unexpected persisted students: %d", count) }
}

// TestImportStudentsRejectsUnknownGroup ensures a batch cannot target a
// nonexistent academic group.
// RU: Проверяет запрет импорта в несуществующую учебную группу.
func TestImportStudentsRejectsUnknownGroup(t *testing.T) {
	db, _ := setupImportDB(t)
	_, err := ImportStudents(context.Background(), db, 9999, [][]string{{"Иванов", "ЗК-001"}}, 0, 1, false)
	if !errors.Is(err, students.ErrGroupNotFound) { t.Fatalf("want ErrGroupNotFound, got %v", err) }
}

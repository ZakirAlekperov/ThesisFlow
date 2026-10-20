package importxlsx

import (
	"context"
	"errors"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestImportFileMapsHeadersAndSavesStudents verifies the user-facing header
// mapping and that accepted Excel rows persist in the chosen academic group.
// RU: Проверяет сопоставление названий колонок и сохранение студентов.
func TestImportFileMapsHeadersAndSavesStudents(t *testing.T) {
	db, groupID := setupImportDB(t)
	data := xlsxFixture(t, "", `<worksheet><sheetData>
		<row r="1"><c r="A1" t="inlineStr"><is><t>Номер зачётки</t></is></c><c r="B1" t="inlineStr"><is><t>ФИО</t></is></c></row>
		<row r="2"><c r="A2" t="inlineStr"><is><t>ЗК-001</t></is></c><c r="B2" t="inlineStr"><is><t>Иванов Иван</t></is></c></row>
		</sheetData></worksheet>`)
	count, err := (Service{DB: db}).ImportFile(context.Background(), groupID, data, " фио ", "НОМЕР ЗАЧЁТКИ")
	if err != nil || count != 1 {
		t.Fatalf("import count=%d error=%v", count, err)
	}
	items, err := students.NewRepository(db).ListByGroup(context.Background(), groupID)
	if err != nil || len(items) != 1 || items[0].EducationalID != "ЗК-001" {
		t.Fatalf("imported students=%v error=%v", items, err)
	}
}

// TestImportFileMissingHeaderKeepsDatabaseUntouched verifies that a mistaken
// heading does not import rows into the wrong student fields.
// RU: Проверяет отсутствие записи при ошибочном названии колонки.
func TestImportFileMissingHeaderKeepsDatabaseUntouched(t *testing.T) {
	db, groupID := setupImportDB(t)
	data := xlsxFixture(t, "", `<worksheet><sheetData><row><c r="A1" t="inlineStr"><is><t>ФИО</t></is></c></row></sheetData></worksheet>`)
	count, err := (Service{DB: db}).ImportFile(context.Background(), groupID, data, "ФИО", "Учебный ID")
	if count != 0 || !errors.Is(err, ErrMissingColumn) {
		t.Fatalf("import count=%d error=%v, want missing column", count, err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM students").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("unexpected imported students: %d", n)
	}
}

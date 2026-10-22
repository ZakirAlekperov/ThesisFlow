package importxlsx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// workbookWithSheets creates a real XLSX structure where visible sheet order
// differs from worksheet filenames. No Excel or mock parser is involved.
// RU: workbookWithSheets создаёт настоящий ZIP/XLSX с двумя листами.
func workbookWithSheets(t *testing.T) []byte {
	t.Helper()
	files := map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Архив" sheetId="4" r:id="rId2"/><sheet name="Студенты" sheetId="2" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>ФИО</t></is></c><c r="B1" t="inlineStr"><is><t>Учебный ID</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>Иванов Иван</t></is></c><c r="B2" t="inlineStr"><is><t>TF20-001</t></is></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml":   `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Устаревший</t></is></c></row></sheetData></worksheet>`,
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for path, contents := range files {
		file, err := z.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestSheetsAndNamedRead uses workbook relationships instead of guessing
// sheet1.xml from the position of a user-visible sheet.
// RU: Проверяет выбор листа по имени с разрешением связей XLSX.
func TestSheetsAndNamedRead(t *testing.T) {
	data := workbookWithSheets(t)
	sheets, err := Sheets(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sheets, []string{"Архив", "Студенты"}) {
		t.Fatalf("names %q", sheets)
	}
	rows, err := ReadNamed(data, "Студенты")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][0] != "Иванов Иван" || rows[1][1] != "TF20-001" {
		t.Fatalf("rows %#v", rows)
	}
	archive, err := ReadNamed(data, "Архив")
	if err != nil {
		t.Fatal(err)
	}
	if archive[0][0] != "Устаревший" {
		t.Fatalf("wrong sheet %#v", archive)
	}
	if _, err := ReadNamed(data, "Несуществующий"); !errors.Is(err, ErrSheetNotFound) {
		t.Fatalf("error %v", err)
	}
}

// TestImportSelectedSheetUsesActualHeaders validates the interactive import
// contract: a teacher chooses a sheet and column headings before commit.
// RU: Проверяет импорт только выбранного листа и заголовков.
func TestImportSelectedSheetUsesActualHeaders(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "student.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	gr := groups.NewRepository(db)
	st := students.NewRepository(db)
	if err := gr.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.Init(ctx); err != nil {
		t.Fatal(err)
	}
	group, err := gr.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	count, err := Service{DB: db}.ImportSelected(ctx, group.ID, workbookWithSheets(t), "Студенты", "ФИО", "Учебный ID")
	if err != nil || count != 1 {
		t.Fatalf("count %d err %v", count, err)
	}
	people, err := st.ListByGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].EducationalID != "TF20-001" {
		t.Fatalf("people %#v", people)
	}
}

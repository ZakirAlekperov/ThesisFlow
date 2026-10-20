package importxlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

// xlsxFixture builds a small real ZIP-based XLSX file without external
// packages, so tests are deterministic and work without Microsoft Excel.
// RU: xlsxFixture создаёт XLSX как ZIP без внешних библиотек и Excel.
func xlsxFixture(t *testing.T, stringsXML, worksheetXML string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	parts := map[string]string{
		"xl/worksheets/sheet1.xml": worksheetXML,
	}
	if stringsXML != "" {
		parts["xl/sharedStrings.xml"] = stringsXML
	}
	for name, body := range parts {
		file, err := writer.Create(name)
		if err != nil { t.Fatal(err) }
		if _, err := file.Write([]byte(body)); err != nil { t.Fatal(err) }
	}
	if err := writer.Close(); err != nil { t.Fatal(err) }
	return data.Bytes()
}

// TestReadFirstSheetHandlesSharedAndInlineStrings verifies XLSX cell formats.
// Scenario: headers and students use shared strings, inline strings and numbers;
// cells are sparse and reordered by their Excel cell references.
// Expected: a rectangular table preserving column positions and Unicode.
//
// RU: Проверяет общие и встроенные строки, числа и пропущенные ячейки.
// Сценарий: смешанные типы и разреженные ссылки на столбцы.
// Ожидается: прямоугольная таблица с правильными колонками и кириллицей.
func TestReadFirstSheetHandlesSharedAndInlineStrings(t *testing.T) {
	ss := `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>ФИО</t></si><si><t>Учебный ID</t></si><si><r><t>Иванов </t></r><r><t>Иван</t></r></si></sst>`
	sheet := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row><row r="2"><c r="C2" t="inlineStr"><is><t>ЗК-001</t></is></c><c r="A2" t="s"><v>2</v></c></row><row r="3"><c r="A3" t="inlineStr"><is><t>Петров Пётр</t></is></c><c r="C3"><v>202</v></c></row></sheetData></worksheet>`
	data := xlsxFixture(t, ss, sheet)
	rows, err := Read(data)
	if err != nil { t.Fatal(err) }
	if len(rows) != 3 { t.Fatalf("got %d rows, want 3: %#v", len(rows), rows) }
	want := [][]string{
		{"ФИО", "", "Учебный ID"},
		{"Иванов Иван", "", "ЗК-001"},
		{"Петров Пётр", "", "202"},
	}
	for i := range want {
		if len(rows[i]) != len(want[i]) { t.Fatalf("row %d cols = %v, want %v", i, rows[i], want[i]) }
		for j := range want[i] {
			if rows[i][j] != want[i][j] { t.Errorf("row %d col %d = %q, want %q", i, j, rows[i][j], want[i][j]) }
		}
	}
}

// TestReadRejectsInvalidOrIncompleteXLSX ensures invalid archives and missing
// sheets fail explicitly instead of appearing as empty student lists.
// RU: Проверяет ошибки повреждённого архива и отсутствующей первой страницы.
func TestReadRejectsInvalidOrIncompleteXLSX(t *testing.T) {
	for _, data := range [][]byte{[]byte("not a zip"), xlsxFixture(t, "", "")} {
		if _, err := Read(data); err == nil { t.Fatal("Read accepted an invalid XLSX") }
	}
}

// TestReadRejectsInvalidSharedStringReference protects against corrupt files.
// Scenario: sheet references string number 99 while only string 0 exists.
// Expected: an explicit parsing error, not a panic or omitted row.
//
// RU: Проверяет неверную ссылку на общую строку.
// Сценарий: индекс ячейки не существует в таблице строк.
// Ожидается: ошибка, а не паника или потерянная запись.
func TestReadRejectsInvalidSharedStringReference(t *testing.T) {
	data := xlsxFixture(t, `<sst><si><t>A</t></si></sst>`, `<worksheet><sheetData><row><c r="A1" t="s"><v>99</v></c></row></sheetData></worksheet>`)
	if _, err := Read(data); err == nil { t.Fatal("expected invalid shared string error") }
}

// TestReadCapsUploadSize prevents processing unusually large uploads.
// Scenario: provide byte payload beyond the XLSX input limit.
// Expected: ErrTooLarge without parsing the archive.
//
// RU: Проверяет ограничение объёма XLSX.
// Сценарий: файл превышает предел загрузки.
// Ожидается: ErrTooLarge без попытки распаковки.
func TestReadCapsUploadSize(t *testing.T) {
	data := make([]byte, MaxFileSize+1)
	if _, err := Read(data); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Read error = %v, want ErrTooLarge", err)
	}
}

// TestReadRejectsUnsafeColumnIndex protects against sparse-cell memory abuse.
// Scenario: worksheet contains an extremely large column label.
// Expected: an error before allocating an unbounded number of cells.
//
// RU: Проверяет ограничение номера столбца.
// Сценарий: ячейка с чрезмерно большим адресом.
// Ожидается: ошибка без неограниченного выделения памяти.
func TestReadRejectsUnsafeColumnIndex(t *testing.T) {
	data := xlsxFixture(t, "", `<worksheet><sheetData><row><c r="ZZZZ999" t="inlineStr"><is><t>bad</t></is></c></row></sheetData></worksheet>`)
	if _, err := Read(data); err == nil { t.Fatal("expected invalid spreadsheet column error") }
}

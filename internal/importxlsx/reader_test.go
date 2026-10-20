package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

// workbook creates a tiny XLSX archive without requiring Excel or test
// fixtures. Each file is owned by the test and entirely in memory.
//
// RU: workbook собирает минимальный XLSX в памяти без Excel и внешних файлов.
func workbook(t *testing.T, sheet, shared string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	files := map[string]string{"xl/worksheets/sheet1.xml": sheet}
	if shared != "" {
		files["xl/sharedStrings.xml"] = shared
	}
	for name, contents := range files {
		entry, err := writer.Create(name)
		if err != nil { t.Fatal(err) }
		if _, err := entry.Write([]byte(contents)); err != nil { t.Fatal(err) }
	}
	if err := writer.Close(); err != nil { t.Fatal(err) }
	return buf.Bytes()
}

// TestReadFirstSheetResolvesSharedAndInlineStrings checks ordinary Excel
// string storage while preserving actual cell coordinates and missing columns.
// Scenario: names are stored as shared strings, IDs as inline strings.
// Expected: two complete rows with values mapped by A/B cell references.
//
// RU: Проверяет общие и встроенные строки вместе с пропущенными ячейками.
// Сценарий: ФИО — shared strings, номера — inline strings.
// Ожидается: строки с сохранением координат колонок A и B.
func TestReadFirstSheetResolvesSharedAndInlineStrings(t *testing.T) {
	shared := `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>ФИО</t></si><si><t>Иванов Иван</t></si><si><r><t>Петров</t></r><r><t> Пётр</t></r></si></sst>`
	sheet := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="inlineStr"><is><t>Учебный ID</t></is></c></row><row r="2"><c r="B2" t="inlineStr"><is><t>0017</t></is></c><c r="A2" t="s"><v>1</v></c></row><row r="3"><c r="A3" t="s"><v>2</v></c><c r="B3"><v>18</v></c></row></sheetData></worksheet>`
	got, err := ReadFirstSheet(workbook(t, sheet, shared))
	if err != nil { t.Fatal(err) }
	if len(got) != 3 || len(got[1]) != 2 || got[1][0] != "Иванов Иван" || got[1][1] != "0017" || got[2][0] != "Петров Пётр" || got[2][1] != "18" {
		t.Fatalf("unexpected rows: %#v", got)
	}
}

// TestReadFirstSheetRetainsGaps checks sparse XLSX worksheets.
// Scenario: only column C contains a value.
// Expected: columns A/B remain empty to keep field mapping stable.
//
// RU: Проверяет пустые столбцы внутри листа.
// Сценарий: заполнен только столбец C.
// Ожидается: A и B пустые, значение находится в позиции C.
func TestReadFirstSheetRetainsGaps(t *testing.T) {
	sheet := `<worksheet><sheetData><row r="1"><c r="C1" t="inlineStr"><is><t>ЗК-001</t></is></c></row></sheetData></worksheet>`
	got, err := ReadFirstSheet(workbook(t, sheet, ""))
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || len(got[0]) != 3 || got[0][0] != "" || got[0][1] != "" || got[0][2] != "ЗК-001" {
		t.Fatalf("cell coordinates lost: %#v", got)
	}
}

// TestReadFirstSheetRejectsInvalidArchives covers invalid ZIP, missing sheets
// and inconsistent shared string references.
// RU: Проверяет отклонение повреждённого XLSX, пустого листа и ошибочного
// индекса общей строки без возврата вымышленных данных.
func TestReadFirstSheetRejectsInvalidArchives(t *testing.T) {
	cases := []struct { name string; file []byte }{
		{"not zip", []byte("not an XLSX")},
		{"missing sheet", workbook(t, "", "<sst/>")},
		{"bad shared index", workbook(t, `<worksheet><sheetData><row r="1"><c r="A1" t="s"><v>42</v></c></row></sheetData></worksheet>`, `<sst><si><t>Only one</t></si></sst>`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadFirstSheet(tc.file); err == nil {
				t.Fatal("expected malformed workbook to be rejected")
			}
		})
	}
}

// TestReadFirstSheetLimitsUploadSize refuses archives with extreme input size.
// RU: Проверяет, что слишком большой входной XLSX отклоняется заранее.
func TestReadFirstSheetLimitsUploadSize(t *testing.T) {
	_, err := ReadFirstSheet(make([]byte, MaxFileBytes+1))
	if !errors.Is(err, ErrTooLarge) { t.Fatalf("error = %v, want ErrTooLarge", err) }
}

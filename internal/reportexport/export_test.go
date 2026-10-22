package reportexport

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/analytics"
)

// TestCSVEscapesSpreadsheetFormulasAndRussianText verifies that real
// spreadsheet apps cannot interpret a student's name as an Excel formula.
// RU: Проверяем защиту от формул в экспорте CSV.
func TestCSVEscapesSpreadsheetFormulasAndRussianText(t *testing.T) {
	r := analytics.Report{Documents: []analytics.Document{
		{GroupName: "ИС-401", StudentName: "=HYPERLINK(\"evil\")", EducationalID: " +SUM(1,2)",
			Title: "Глава 1", Status: "missing"},
		{GroupName: "ИС-401", StudentName: "Иванов Иван", EducationalID: "ЗК-01",
			Title: "Введение", Status: "accepted", ApprovedCopy: true},
	}}
	var b bytes.Buffer
	if err := WriteCSV(&b, r); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b.Bytes(), []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal("UTF-8 BOM not found")
	}
	reader := csv.NewReader(bytes.NewReader(b.Bytes()[3:]))
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || len(rows[0]) != 8 {
		t.Fatalf("CSV shape %v", rows)
	}
	if !strings.HasPrefix(rows[1][1], "'=") || !strings.HasPrefix(rows[1][2], "' ") {
		t.Errorf("CSV formula unescaped: %q %q", rows[1][1], rows[1][2])
	}
	if rows[2][1] != "Иванов Иван" || rows[2][7] != "Да" {
		t.Errorf("CSV Cyrillic corrupted: %v", rows[2])
	}
}

// TestWriteXLSXProducesValidOOXMLWorkbook checks the internal ZIP/relationships
// and parses every generated XML part. Inline literal text never emits <f>.
// RU: XLSX — подлинный OOXML с тремя листами, без макросов и формул.
func TestWriteXLSXProducesValidOOXMLWorkbook(t *testing.T) {
	report := analytics.Report{Documents: []analytics.Document{{
		GroupName: "ИС-401", StudentName: "=1+1", EducationalID: "ЗК-001",
		Title: "Глава 1", Status: "under_review", DueDate: "2026-10-23",
	}}}
	var b bytes.Buffer
	if err := WriteXLSX(&b, Sheets(report)); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	xmlFiles := map[string]string{}
	for _, entry := range archive.File {
		if !strings.HasSuffix(entry.Name, ".xml") && !strings.HasSuffix(entry.Name, ".rels") {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid XML %s: %v", entry.Name, err)
			}
		}
		xmlFiles[entry.Name] = string(data)
	}
	for _, required := range []string{"xl/workbook.xml", "xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml", "xl/worksheets/sheet3.xml", "xl/_rels/workbook.xml.rels"} {
		if xmlFiles[required] == "" {
			t.Errorf("missing Excel part %s", required)
		}
	}
	if !strings.Contains(xmlFiles["xl/workbook.xml"], "Документы") {
		t.Error("documents worksheet name lost")
	}
	body := xmlFiles["xl/worksheets/sheet3.xml"]
	if !strings.Contains(body, "=1+1") || strings.Contains(body, "<f>") || strings.Contains(body, "<f ") {
		t.Fatal("user data is not safe literal text in XLSX")
	}
	if !strings.Contains(body, `<c r="G2" s="0"><v>0</v></c>`) {
		t.Error("overdue count should be numeric Excel cell, not text")
	}
	if !strings.Contains(body, "autoFilter") || !strings.Contains(body, "pane") {
		t.Error("styles or frozen header missing")
	}
}

func TestWriteXLSXRejectsInvalidRowsAndNames(t *testing.T) {
	tests := [][]Sheet{
		{{Name: "Sheet", Headers: []string{"X"}, Rows: [][]string{{"x", "y"}}}},
		{{Name: "Sheet", Headers: []string{"X"}}, {Name: "sheet", Headers: []string{"X"}}},
		{{Name: "", Headers: []string{"X"}}},
	}
	for i, sheets := range tests {
		var b bytes.Buffer
		if err := WriteXLSX(&b, sheets); err == nil {
			t.Errorf("invalid workbook %d passed", i)
		}
	}
}

// TestEscapeTextRejectsInvalidXMLCharacters checks damaged imported labels
// cannot corrupt exported workbooks even when they are otherwise valid UTF-8.
// RU: Недопустимые кодовые точки не попадают в XML-файлы Excel.
func TestEscapeTextRejectsInvalidXMLCharacters(t *testing.T) {
	value := escapeText("ВКР\x00\uFFFE\uFFFF <глава>")
	if strings.Contains(value, "\x00") || strings.Contains(value, "\uFFFE") ||
		strings.Contains(value, "\uFFFF") {
		t.Fatalf("illegal XML codepoint escaped into workbook: %q", value)
	}
	if !strings.Contains(value, "&lt;глава&gt;") {
		t.Fatalf("user title not escaped: %q", value)
	}
}

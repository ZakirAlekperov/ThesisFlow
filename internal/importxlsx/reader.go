// Package importxlsx reads rows from a local XLSX workbook without CGO or
// external spreadsheet libraries.
// Пакет importxlsx читает строки XLSX без CGO и внешних библиотек.
package importxlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MaxFileSize caps the uploaded XLSX ZIP before it is parsed.
// RU: MaxFileSize ограничивает размер исходного ZIP-файла XLSX.
const MaxFileSize = 10 << 20

const maxXMLPart = 16 << 20
const maxColumns = 256
const maxRows = 10000

// ErrTooLarge reports a workbook or XML part exceeding safe resource limits.
// RU: ErrTooLarge означает превышение лимита XLSX или его XML-части.
var ErrTooLarge = errors.New("XLSX exceeds the allowed size")

type worksheet struct {
	Rows []sheetRow `xml:"sheetData>row"`
}

type sheetRow struct {
	Cells []sheetCell `xml:"c"`
}

type sheetCell struct {
	Ref    string       `xml:"r,attr"`
	Type   string       `xml:"t,attr"`
	Value  string       `xml:"v"`
	Inline inlineString `xml:"is"`
}

type inlineString struct {
	Text string `xml:"t"`
	Runs []run  `xml:"r"`
}

type run struct {
	Text string `xml:"t"`
}

type sharedStrings struct {
	Items []inlineString `xml:"si"`
}

// cellColumn parses Excel references such as A1 and AB42. Column 0 represents
// a missing reference, with row-local fallback ordering handled by the caller.
// RU: cellColumn разбирает адрес вида AB42 с ограничением числа столбцов.
func cellColumn(reference string) (int, error) {
	column := 0
	for _, char := range reference {
		if char >= '0' && char <= '9' {
			break
		}
		if char < 'A' || char > 'Z' {
			return 0, fmt.Errorf("invalid cell reference %q", reference)
		}
		column = column*26 + int(char-'A'+1)
		if column > maxColumns {
			return 0, fmt.Errorf("column out of bounds: %q", reference)
		}
	}
	if column == 0 && reference != "" {
		return 0, fmt.Errorf("invalid cell reference %q", reference)
	}
	return column, nil
}

// readPart bounds decompression of one XLSX member to prevent ZIP bombs.
// RU: readPart ограничивает распаковку одного компонента XLSX.
func readPart(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > maxXMLPart {
		return nil, ErrTooLarge
	}
	r, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxXMLPart+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxXMLPart {
		return nil, ErrTooLarge
	}
	return data, nil
}

// text joins ordinary or rich formatted XLSX text fragments.
// RU: text объединяет обычные и форматированные фрагменты текста XLSX.
func (s inlineString) text() string {
	if len(s.Runs) == 0 {
		return s.Text
	}
	var builder strings.Builder
	builder.WriteString(s.Text)
	for _, part := range s.Runs {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

// Read returns a rectangular table from the first XLSX worksheet.
// It supports shared strings, inline strings, sparse cells and scalar values.
// Formulas are represented only by their cached values if present. Headers
// and column selection are interpreted by the importer in a later step.
// The first worksheet is currently expected at xl/worksheets/sheet1.xml.
//
// RU: Read возвращает таблицу первой страницы XLSX. Поддерживаются общие
// строки, встроенные строки, разреженные ячейки и числовые значения.
// Формулы читаются по сохранённому результату, если он присутствует.
// Выбор нужных столбцов реализуется отдельно. Первая страница пока должна
// находиться по адресу xl/worksheets/sheet1.xml.
func Read(data []byte) ([][]string, error) {
	return readSelected(data, "xl/worksheets/sheet1.xml")
}

// readSelected applies the existing bounded parser to a resolved XLSX member.
// RU: readSelected разбирает выбранный XML-лист с прежними ограничениями.
func readSelected(data []byte, selectedPart string) ([][]string, error) {
	if len(data) > MaxFileSize {
		return nil, ErrTooLarge
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open XLSX: %w", err)
	}
	var sheetXML, sharedXML []byte
	for _, file := range archive.File {
		if file.Name != selectedPart && file.Name != "xl/sharedStrings.xml" {
			continue
		}
		part, err := readPart(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Name, err)
		}
		switch file.Name {
		case selectedPart:
			sheetXML = part
		case "xl/sharedStrings.xml":
			sharedXML = part
		}
	}
	if len(sheetXML) == 0 {
		return nil, errors.New("XLSX first worksheet is missing")
	}

	var stringsTable sharedStrings
	if len(sharedXML) > 0 {
		if err := xml.Unmarshal(sharedXML, &stringsTable); err != nil {
			return nil, fmt.Errorf("parse XLSX shared strings: %w", err)
		}
	}
	var sheet worksheet
	if err := xml.Unmarshal(sheetXML, &sheet); err != nil {
		return nil, fmt.Errorf("parse XLSX worksheet: %w", err)
	}
	if len(sheet.Rows) > maxRows {
		return nil, ErrTooLarge
	}

	rows := make([][]string, 0, len(sheet.Rows))
	width := 0
	for rowIndex, source := range sheet.Rows {
		row := make([]string, 0)
		fallbackColumn := 0
		for _, cell := range source.Cells {
			column, err := cellColumn(cell.Ref)
			if err != nil {
				return nil, fmt.Errorf("row %d: %w", rowIndex+1, err)
			}
			if column == 0 {
				column = fallbackColumn + 1
			}
			fallbackColumn = column
			for len(row) < column {
				row = append(row, "")
			}
			var value string
			switch cell.Type {
			case "s":
				index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
				if err != nil || index < 0 || index >= len(stringsTable.Items) {
					return nil, fmt.Errorf("row %d: invalid shared string index %q", rowIndex+1, cell.Value)
				}
				value = stringsTable.Items[index].text()
			case "inlineStr":
				value = cell.Inline.text()
			default:
				value = cell.Value
			}
			row[column-1] = value
		}
		if len(row) > width {
			width = len(row)
		}
		rows = append(rows, row)
	}
	for i := range rows {
		for len(rows[i]) < width {
			rows[i] = append(rows[i], "")
		}
	}
	return rows, nil
}

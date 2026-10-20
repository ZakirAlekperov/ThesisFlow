// Package xlsx reads a bounded set of tabular values from an XLSX worksheet.
// Пакет xlsx читает ограниченный набор табличных данных листа XLSX.
package xlsx

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

const (
	// MaxFileBytes bounds uploaded compressed XLSX data.
	// RU: MaxFileBytes ограничивает сжатый входной файл.
	MaxFileBytes = 8 << 20
	maxEntryBytes = 16 << 20
	maxRows = 10000
	maxColumns = 256
)

// ErrTooLarge reports an XLSX file or XML entry above the supported limit.
// RU: ErrTooLarge означает превышение допустимого размера файла или XML.
var ErrTooLarge = errors.New("XLSX exceeds size limit")

type sharedStrings struct {
	Items []richText `xml:"si"`
}

type richText struct {
	Text string `xml:"t"`
	Runs []struct { Text string `xml:"t"` } `xml:"r"`
}

func (s richText) value() string {
	var b strings.Builder
	b.WriteString(s.Text)
	for _, run := range s.Runs { b.WriteString(run.Text) }
	return b.String()
}

type worksheet struct {
	Rows []sheetRow `xml:"sheetData>row"`
}

type sheetRow struct {
	Cells []sheetCell `xml:"c"`
}

type sheetCell struct {
	Ref string `xml:"r,attr"`
	Kind string `xml:"t,attr"`
	Raw string `xml:"v"`
	Inline richText `xml:"is"`
}

// readZipEntry enforces an upper bound before and during XML decompression.
// RU: readZipEntry проверяет размер XML до и во время распаковки.
func readZipEntry(files []*zip.File, name string) ([]byte, error) {
	for _, file := range files {
		if file.Name != name { continue }
		if file.UncompressedSize64 > maxEntryBytes { return nil, ErrTooLarge }
		reader, err := file.Open()
		if err != nil { return nil, fmt.Errorf("open %s: %w", name, err) }
		defer reader.Close()
		data, err := io.ReadAll(io.LimitReader(reader, maxEntryBytes+1))
		if err != nil { return nil, fmt.Errorf("read %s: %w", name, err) }
		if len(data) > maxEntryBytes { return nil, ErrTooLarge }
		return data, nil
	}
	return nil, fmt.Errorf("missing XLSX entry %s", name)
}

// cellColumn extracts the zero-based column from an Excel coordinate.
// RU: cellColumn вычисляет нулевой индекс столбца из координаты Excel.
func cellColumn(ref string) (int, error) {
	if ref == "" { return 0, errors.New("cell reference missing") }
	column := 0
	digits := false
	for _, ch := range ref {
		switch {
		case ch >= 'A' && ch <= 'Z' && !digits:
			column = column*26 + int(ch-'A'+1)
		case ch >= '0' && ch <= '9':
			digits = true
		default:
			return 0, fmt.Errorf("invalid cell reference %q", ref)
		}
		if column > maxColumns { return 0, ErrTooLarge }
	}
	if !digits || column == 0 { return 0, fmt.Errorf("invalid cell reference %q", ref) }
	return column-1, nil
}

// ReadFirstSheet reads visible cell values from a minimal XLSX worksheet.
// It preserves column positions for future column mapping, supports shared
// strings, rich/inline text and numeric values, and rejects oversized content.
// This initial reader uses xl/worksheets/sheet1.xml, not Excel macros.
//
// RU: ReadFirstSheet читает значения первого листа XLSX с сохранением позиции
// столбцов. Поддерживает общие строки, встроенный текст и числовые значения,
// ограничивает размер и не выполняет макросы.
func ReadFirstSheet(data []byte) ([][]string, error) {
	if len(data) > MaxFileBytes { return nil, ErrTooLarge }
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil { return nil, fmt.Errorf("open XLSX archive: %w", err) }
	sheetData, err := readZipEntry(archive.File, "xl/worksheets/sheet1.xml")
	if err != nil { return nil, err }
	var stringsTable []string
	for _, file := range archive.File {
		if file.Name != "xl/sharedStrings.xml" { continue }
		raw, err := readZipEntry(archive.File, file.Name)
		if err != nil { return nil, err }
		var s sharedStrings
		if err := xml.Unmarshal(raw, &s); err != nil { return nil, fmt.Errorf("parse shared strings: %w", err) }
		for _, item := range s.Items { stringsTable = append(stringsTable, item.value()) }
		break
	}
	var sheet worksheet
	if err := xml.Unmarshal(sheetData, &sheet); err != nil { return nil, fmt.Errorf("parse worksheet: %w", err) }
	if len(sheet.Rows) > maxRows { return nil, ErrTooLarge }

	rows := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := make([]string, 0)
		for _, cell := range row.Cells {
			column, err := cellColumn(cell.Ref)
			if err != nil { return nil, err }
			for len(values) <= column { values = append(values, "") }
			var value string
			switch cell.Kind {
			case "s":
				index, err := strconv.Atoi(strings.TrimSpace(cell.Raw))
				if err != nil || index < 0 || index >= len(stringsTable) {
					return nil, fmt.Errorf("invalid shared string index %q", cell.Raw)
				}
				value = stringsTable[index]
			case "inlineStr":
				value = cell.Inline.value()
			default:
				value = cell.Raw
			}
			values[column] = value
		}
		rows = append(rows, values)
	}
	return rows, nil
}

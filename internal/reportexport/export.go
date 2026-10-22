// Package reportexport serializes current analytics into offline CSV/XLSX.
// Пакет reportexport формирует автономные отчёты CSV и настоящие XLSX.
package reportexport

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/ZakirAlekperov/ThesisFlow/internal/analytics"
)

// Sheet is a report tab. NumericColumns marks only trusted generated metric
// cells as numbers; all user-controlled values remain literal text.
// RU: Только вычисленные показатели — числа, ввод пользователя — строки.
type Sheet struct {
	Name           string
	Headers        []string
	Rows           [][]string
	NumericColumns map[int]bool
}

// Sheets constructs three consistently filtered views of one report.
// RU: Одинаковые фильтры используются для экрана и всех листов Excel.
func Sheets(r analytics.Report) []Sheet {
	summary := Sheet{Name: "Сводка", NumericColumns: map[int]bool{1: true,2: true,3: true,4: true,5: true,6: true,7: true,8: true}, Headers: []string{"Учебная группа", "Студентов", "Всего требований", "Принято", "На проверке", "На доработке", "Не сдано", "Просрочено", "Есть принятая копия", "Готовность"}}
	for _, g := range r.Groups {
		p, valid := g.Metrics.Percent()
		summary.Rows = append(summary.Rows, []string{g.Name, fmt.Sprint(g.Students), fmt.Sprint(g.Required),
			fmt.Sprint(g.Accepted), fmt.Sprint(g.UnderReview), fmt.Sprint(g.NeedsRevision), fmt.Sprint(g.Missing),
			fmt.Sprint(g.Overdue), fmt.Sprint(g.WithApprovedCopy), percent(p, valid)})
	}
	totals := r.Totals
	p, ok := totals.Percent()
	summary.Rows = append(summary.Rows, []string{"ИТОГО", fmt.Sprint(totals.Students), fmt.Sprint(totals.Required),
		fmt.Sprint(totals.Accepted), fmt.Sprint(totals.UnderReview), fmt.Sprint(totals.NeedsRevision),
		fmt.Sprint(totals.Missing), fmt.Sprint(totals.Overdue), fmt.Sprint(totals.WithApprovedCopy), percent(p, ok)})
	students := Sheet{Name: "Студенты", NumericColumns: map[int]bool{3: true,4: true,5: true,6: true,7: true,8: true}, Headers: []string{"Группа", "ФИО", "Учебный ID", "Всего", "Принято", "На проверке", "На доработке", "Не сдано", "Просрочено", "Готовность", "Ближайший срок"}}
	for _, s := range r.Students {
		p, ok := s.Metrics.Percent()
		students.Rows = append(students.Rows, []string{s.GroupName, s.Name, s.EducationalID,
			fmt.Sprint(s.Required), fmt.Sprint(s.Accepted), fmt.Sprint(s.UnderReview),
			fmt.Sprint(s.NeedsRevision), fmt.Sprint(s.Missing), fmt.Sprint(s.Overdue),
			percent(p, ok), s.NearestDue})
	}
	docs := Sheet{Name: "Документы", NumericColumns: map[int]bool{6: true}, Headers: []string{"Группа", "ФИО", "Учебный ID", "Требование", "Статус", "Срок", "Дней просрочки", "Ранее принятая копия"}}
	for _, d := range r.Documents {
		had := "Нет"
		if d.ApprovedCopy {
			had = "Да"
		}
		docs.Rows = append(docs.Rows, []string{d.GroupName, d.StudentName, d.EducationalID,
			d.Title, statusText(d.Status), d.DueDate, fmt.Sprint(d.OverdueDays), had})
	}
	return []Sheet{summary, students, docs}
}
func percent(n int, ok bool) string {
	if !ok {
		return "Нет данных"
	}
	return fmt.Sprintf("%d%%", n)
}
func statusText(s string) string {
	switch s {
	case "accepted":
		return "Принят"
	case "under_review":
		return "На проверке"
	case "needs_revision":
		return "На доработке"
	default:
		return "Не сдан"
	}
}

// SafeCSVCell shields CSV opened in Excel from interpreting user-controlled
// names or identifiers as formulas, even after leading whitespace.
// RU: Префикс-апостроф предотвращает запуск CSV-формул в Excel.
func SafeCSVCell(value string) string {
	v := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
	if v != "" && strings.ContainsRune("=+-@", []rune(v)[0]) {
		return "'" + value
	}
	return value
}

// WriteCSV streams BOM-prefixed UTF-8 CSV for Windows Excel. Documents share
// the same filtered data as the HTML table and XLSX "Документы" worksheet.
// RU: CSV экранирует поля и Excel-формулы, не теряя русские буквы.
func WriteCSV(w io.Writer, report analytics.Report) error {
	if _, err := io.WriteString(w, "\xef\xbb\xbf"); err != nil {
		return err
	}
	tab := Sheets(report)[2]
	cw := csv.NewWriter(w)
	if err := cw.Write(tab.Headers); err != nil {
		return err
	}
	for _, row := range tab.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = SafeCSVCell(v)
		}
		if err := cw.Write(cells); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func escapeText(value string) string {
	// Limit Excel strings to 32,767 Unicode characters and remove XML 1.0
	// forbidden code points, including U+FFFE and U+FFFF.
	// RU: Исправляем недопустимые символы XML, сохраняя кириллицу.
	var clean strings.Builder
	count := 0
	for _, r := range value {
		if count >= 32767 {
			break
		}
		if r != 9 && r != 10 && r != 13 &&
			!((r >= 0x20 && r <= 0xD7FF) ||
				(r >= 0xE000 && r <= 0xFFFD) ||
				(r >= 0x10000 && r <= 0x10FFFF)) {
			continue
		}
		clean.WriteRune(r)
		count++
	}
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(clean.String()))
	return out.String()
}

func colName(n int) string {
	n++
	var b string
	for n > 0 {
		n--
		b = string(rune('A'+n%26)) + b
		n /= 26
	}
	return b
}
func sheetXML(s Sheet) ([]byte, error) {
	if len(s.Headers) == 0 {
		return nil, errors.New("empty sheet headings")
	}
	if len(s.Rows) > 1048575 {
		return nil, errors.New("Excel row limit exceeded")
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><sheetFormatPr defaultRowHeight="18"/><cols>`)
	for i := range s.Headers {
		width := 19
		if i <= 3 {
			width = 27
		}
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, width)
	}
	b.WriteString(`</cols><sheetData>`)
	values := make([][]string, 0, len(s.Rows)+1)
	values = append(values, s.Headers)
	values = append(values, s.Rows...)
	for index, row := range values {
		if len(row) != len(s.Headers) {
			return nil, errors.New("mismatched report columns")
		}
		fmt.Fprintf(&b, `<row r="%d">`, index+1)
		for c, v := range row {
			style := 0
			if index == 0 {
				style = 1
			} else if index%2 == 0 {
				style = 2
			}
			if index > 0 && s.NumericColumns[c] {
				if _, err := strconv.ParseInt(v, 10, 64); err != nil {
					return nil, fmt.Errorf("invalid generated report number: %w", err)
				}
				fmt.Fprintf(&b, `<c r="%s%d" s="%d"><v>%s</v></c>`, colName(c), index+1, style, v)
			} else {
				fmt.Fprintf(&b, `<c r="%s%d" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, colName(c), index+1, style, escapeText(v))
			}
		}
		b.WriteString("</row>")
	}
	b.WriteString(`</sheetData>`)
	fmt.Fprintf(&b, `<autoFilter ref="A1:%s%d"/>`, colName(len(s.Headers)-1), len(values))
	b.WriteString(`<pageMargins left="0.5" right="0.5" top="0.7" bottom="0.7" header="0.3" footer="0.3"/></worksheet>`)
	return []byte(b.String()), nil
}

// WriteXLSX creates a real OOXML ZIP workbook using only Go's standard
// library. The writer is intentionally limited to formatted literal text
// and a fixed set of sheets, rather than implementing arbitrary Excel.
// RU: Экспорт настоящего XLSX без зависимостей, макросов и формул.
func WriteXLSX(w io.Writer, sheets []Sheet) error {
	if len(sheets) == 0 || len(sheets) > 10 {
		return errors.New("invalid worksheet count")
	}
	names := map[string]bool{}
	parts := make([][]byte, len(sheets))
	for i, s := range sheets {
		if strings.TrimSpace(s.Name) == "" || len([]rune(s.Name)) > 31 || names[strings.ToLower(s.Name)] {
			return errors.New("invalid or duplicate worksheet name")
		}
		names[strings.ToLower(s.Name)] = true
		var err error
		parts[i], err = sheetXML(s)
		if err != nil {
			return err
		}
	}
	z := zip.NewWriter(w)
	add := func(name, body string) error {
		file, err := z.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(file, body)
		return err
	}
	if err := add("[Content_Types].xml", contentTypes(len(sheets))); err != nil {
		return err
	}
	if err := add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`); err != nil {
		return err
	}
	var workbook strings.Builder
	workbook.WriteString(`<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	var rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i, s := range sheets {
		fmt.Fprintf(&workbook, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escapeText(s.Name), i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
	}
	workbook.WriteString(`</sheets></workbook>`)
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, len(sheets)+1)
	rels.WriteString("</Relationships>")
	if err := add("xl/workbook.xml", workbook.String()); err != nil {
		return err
	}
	if err := add("xl/_rels/workbook.xml.rels", rels.String()); err != nil {
		return err
	}
	if err := add("xl/styles.xml", stylesXML); err != nil {
		return err
	}
	for i, bytes := range parts {
		file, err := z.Create("xl/worksheets/sheet" + strconv.Itoa(i+1) + ".xml")
		if err != nil {
			return err
		}
		if _, err := file.Write(bytes); err != nil {
			return err
		}
	}
	return z.Close()
}
func contentTypes(n int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i)
	}
	b.WriteString("</Types>")
	return b.String()
}

const stylesXML = `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2"><font><sz val="11"/><name val="Calibri"/><color rgb="FF25344D"/></font><font><b/><sz val="11"/><name val="Calibri"/><color rgb="FFFFFFFF"/></font></fonts>
<fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF214F9B"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFF0F4FA"/><bgColor indexed="64"/></patternFill></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1"/><xf numFmtId="0" fontId="0" fillId="3" borderId="0" xfId="0" applyFill="1"/></cellXfs>
<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`

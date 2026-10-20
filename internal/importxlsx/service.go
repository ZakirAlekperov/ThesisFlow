package importxlsx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrMissingColumn reports a column header that is not present in the XLSX.
// RU: ErrMissingColumn означает отсутствие выбранного заголовка столбца.
var ErrMissingColumn = errors.New("required XLSX column not found")

// Service connects the XLSX reader to the atomic SQLite batch importer.
// The connection is owned by the parent application.
// RU: Service связывает XLSX и атомарный импорт в SQLite приложения.
type Service struct { DB *sql.DB }

// ImportFile matches the chosen name and educational ID columns by the text
// in the first worksheet row, then saves students atomically. Column heading
// comparison ignores case and surrounding whitespace.
//
// RU: ImportFile находит указанные колонки по заголовкам первой строки листа
// и атомарно сохраняет студентов. Регистр и краевые пробелы не учитываются.
func (s Service) ImportFile(ctx context.Context, groupID int64, data []byte, nameHeader, idHeader string) (int, error) {
	rows, err := Read(data)
	if err != nil { return 0, err }
	if len(rows) == 0 { return 0, ErrMissingColumn }
	nameHeader = strings.ToLower(strings.TrimSpace(nameHeader))
	idHeader = strings.ToLower(strings.TrimSpace(idHeader))
	if nameHeader == "" || idHeader == "" || nameHeader == idHeader {
		return 0, ErrMissingColumn
	}
	nameCol, idCol := -1, -1
	for i, heading := range rows[0] {
		key := strings.ToLower(strings.TrimSpace(heading))
		if key == nameHeader { nameCol = i }
		if key == idHeader { idCol = i }
	}
	if nameCol < 0 || idCol < 0 || nameCol == idCol {
		return 0, fmt.Errorf("%w: name %q, ID %q", ErrMissingColumn, nameHeader, idHeader)
	}
	return ImportStudents(ctx, s.DB, groupID, rows, nameCol, idCol, true)
}

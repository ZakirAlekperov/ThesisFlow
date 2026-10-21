package importxlsx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// ImportStudents atomically saves students from a parsed spreadsheet table.
// nameColumn and idColumn are zero-based positions in each row; skipHeader
// omits the first row. The caller must use the application's initialized DB.
// On any error the entire transaction is rolled back and count is zero.
//
// RU: ImportStudents сохраняет студентов из таблицы Excel атомарно.
// Колонки nameColumn и idColumn нумеруются с нуля; skipHeader пропускает
// первую строку. При любой ошибке откатывается вся транзакция.
func ImportStudents(ctx context.Context, db *sql.DB, groupID int64, rows [][]string, nameColumn, idColumn int, skipHeader bool) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if nameColumn < 0 || idColumn < 0 || nameColumn == idColumn || nameColumn >= maxColumns || idColumn >= maxColumns {
		return 0, fmt.Errorf("invalid column mapping")
	}
	if groupID <= 0 {
		return 0, students.ErrGroupNotFound
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin student import: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM student_groups WHERE id = ?)", groupID).Scan(&exists); err != nil {
		return 0, fmt.Errorf("check import group: %w", err)
	}
	if exists == 0 {
		return 0, students.ErrGroupNotFound
	}
	const insert = `INSERT INTO students (group_id, educational_id, educational_id_key, full_name)
		VALUES (?, ?, ?, ?) ON CONFLICT(educational_id_key) DO NOTHING`
	first := 0
	if skipHeader {
		first = 1
	}
	imported := 0
	for index := first; index < len(rows); index++ {
		row := rows[index]
		if nameColumn >= len(row) || idColumn >= len(row) {
			return 0, fmt.Errorf("Excel row %d: column does not exist", index+1)
		}
		name := strings.TrimSpace(row[nameColumn])
		educationalID := strings.TrimSpace(row[idColumn])
		if name == "" && educationalID == "" {
			continue
		}
		if name == "" {
			return 0, fmt.Errorf("Excel row %d: %w", index+1, students.ErrInvalidFullName)
		}
		if educationalID == "" {
			return 0, fmt.Errorf("Excel row %d: %w", index+1, students.ErrInvalidEducationalID)
		}
		result, err := tx.ExecContext(ctx, insert, groupID, educationalID, strings.ToLower(educationalID), name)
		if err != nil {
			return 0, fmt.Errorf("Excel row %d: insert: %w", index+1, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("Excel row %d: check insert: %w", index+1, err)
		}
		if affected == 0 {
			return 0, fmt.Errorf("Excel row %d: %w", index+1, students.ErrDuplicateEducationalID)
		}
		imported++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit student import: %w", err)
	}
	return imported, nil
}

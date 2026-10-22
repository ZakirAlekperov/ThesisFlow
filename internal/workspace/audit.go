// Package workspace checks local database and file invariants without
// modifying, deleting or attempting to repair any user data.
// Пакет workspace проверяет SQLite и файлы, ничего не исправляя автоматически.
package workspace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/integrity"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

const maxProjectedSize = 64 << 20

// Issue identifies a failed invariant without exposing students' names
// or absolute paths in routine diagnostics.
// RU: Issue не раскрывает ФИО и полные пути в диагностике.
type Issue struct {
	Code          string
	VersionID     int64
	RequirementID int64
	StudentID     int64
	Detail        string
}

// Report distinguishes blocking corruption from non-blocking stray files.
// A nonempty Issues slice must never be silently treated as healthy.
// RU: Отчёт разделяет серьёзные повреждения и посторонние файлы.
type Report struct {
	SchemaVersion   int
	VersionsChecked int
	VersionsIntact  int
	AcceptedChecked int
	AcceptedIntact  int
	ExtraFiles      int
	Issues          []Issue
}

// Healthy reports whether all SQLite and document checks passed.
// RU: Healthy означает отсутствие нарушений, блокирующих восстановление.
func (r Report) Healthy() bool { return len(r.Issues) == 0 }

// add records a non-sensitive identifier and problem category.
// RU: Фиксируем идентификаторы, но не содержимое документа.
func (r *Report) add(code string, studentID, requirementID, versionID int64, detail string) {
	r.Issues = append(r.Issues, Issue{
		Code: code, StudentID: studentID, RequirementID: requirementID,
		VersionID: versionID, Detail: detail,
	})
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)", table).Scan(&exists)
	return exists == 1, err
}

// canonicalPath requires a platform-relative name and prohibits absolute
// paths, traversal, backslashes, colons and symbolic links in every component.
// RU: Проверяем путь и все компоненты на символические ссылки.
func canonicalPath(root, relative string) (string, error) {
	normalized, err := storagepath.Normalize(relative)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", errors.New("unsafe storage root")
	}
	current := root
	for _, part := range strings.Split(normalized, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlink component")
		}
	}
	info, err := os.Lstat(current)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	return current, nil
}

func hashFile(path string, limit int64) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hasher := sha256.New()
	count, err := io.Copy(hasher, io.LimitReader(file, limit+1))
	if err != nil {
		return 0, "", err
	}
	if count > limit {
		return count, "", errors.New("file over limit")
	}
	return count, hex.EncodeToString(hasher.Sum(nil)), nil
}

// Check performs SQLite quick_check, foreign_key_check, immutable-version
// verification, accepted-projection verification and scope/link validation.
// Corrupt user files are reported; I/O failures and malformed SQL abort the
// operation with an error instead of returning false success.
// RU: Check сверяет SQL, версии и принятые копии и не меняет файлы.
func Check(ctx context.Context, db *sql.DB, storageRoot string) (Report, error) {
	report := Report{Issues: make([]Issue, 0)}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&report.SchemaVersion); err != nil {
		return Report{}, fmt.Errorf("read SQLite version: %w", err)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return Report{}, fmt.Errorf("SQLite integrity query: %w", err)
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return Report{}, err
		}
		if value != "ok" {
			report.add("sqlite_integrity", 0, 0, 0, "SQLite quick_check failed")
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, err
	}
	if err := rows.Close(); err != nil {
		return Report{}, err
	}
	rows, err = db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return Report{}, fmt.Errorf("SQLite foreign keys: %w", err)
	}
	for rows.Next() {
		var table string
		var rowID sql.NullInt64
		var parent string
		var fkID int
		if err := rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			rows.Close()
			return Report{}, err
		}
		report.add("foreign_key", 0, 0, 0, "SQLite foreign-key relation is broken")
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Report{}, err
	}
	if err := rows.Close(); err != nil {
		return Report{}, err
	}

	versionsPresent, err := tableExists(ctx, db, "file_versions")
	if err != nil {
		return Report{}, err
	}
	acceptedPresent, err := tableExists(ctx, db, "accepted_files")
	if err != nil {
		return Report{}, err
	}
	if acceptedPresent && !versionsPresent {
		report.add("missing_metadata", 0, 0, 0, "accepted files table exists without versions table")
		return report, nil
	}
	if !versionsPresent {
		return report, nil
	} // legacy SQLite ZIP without document records

	versions, err := integrity.Check(ctx, db, storageRoot)
	if err != nil {
		return Report{}, fmt.Errorf("check version files: %w", err)
	}
	report.VersionsChecked = versions.Checked
	report.VersionsIntact = versions.Intact
	for _, issue := range versions.Issues {
		report.add("version_"+strings.ReplaceAll(issue.Problem, " ", "_"), 0, 0, issue.VersionID,
			"immutable file version failed integrity check")
	}
	if !acceptedPresent {
		return report, nil
	}
	// The joined version must belong to this student and requirement, even
	// if SQLite foreign keys allow referencing another valid version ID.
	const query = `SELECT a.student_id,a.requirement_id,a.version_id,a.relative_path,
		v.student_id,v.requirement_id,v.file_size,v.sha256
		FROM accepted_files a LEFT JOIN file_versions v ON v.id=a.version_id
		ORDER BY a.student_id,a.requirement_id`
	links, err := db.QueryContext(ctx, query)
	if err != nil {
		return Report{}, fmt.Errorf("read accepted projections: %w", err)
	}
	for links.Next() {
		if err := ctx.Err(); err != nil {
			links.Close()
			return Report{}, err
		}
		var studentID, requirementID, versionID int64
		var vStudent, vRequirement, vSize sql.NullInt64
		var relative string
		var digest sql.NullString
		if err := links.Scan(&studentID, &requirementID, &versionID, &relative,
			&vStudent, &vRequirement, &vSize, &digest); err != nil {
			links.Close()
			return Report{}, err
		}
		report.AcceptedChecked++
		if !vStudent.Valid || vStudent.Int64 != studentID ||
			!vRequirement.Valid || vRequirement.Int64 != requirementID {
			report.add("accepted_scope", studentID, requirementID, versionID,
				"accepted version does not belong to requirement and student")
			continue
		}
		if !vSize.Valid || vSize.Int64 < 0 || vSize.Int64 > maxProjectedSize ||
			!digest.Valid || len(digest.String) != 64 {
			report.add("accepted_metadata", studentID, requirementID, versionID, "invalid source metadata")
			continue
		}
		source, err := canonicalPath(storageRoot, relative)
		if errors.Is(err, os.ErrNotExist) {
			report.add("accepted_missing", studentID, requirementID, versionID, "approved projection is missing")
			continue
		}
		if err != nil {
			report.add("accepted_unsafe_path", studentID, requirementID, versionID, "approved projection path is unsafe")
			continue
		}
		size, hash, err := hashFile(source, maxProjectedSize)
		if err != nil {
			report.add("accepted_read", studentID, requirementID, versionID, "cannot read approved projection")
			continue
		}
		if size != vSize.Int64 || !strings.EqualFold(hash, digest.String) {
			report.add("accepted_corrupt", studentID, requirementID, versionID, "approved projection differs from accepted version")
			continue
		}
		report.AcceptedIntact++
	}
	if err := links.Err(); err != nil {
		links.Close()
		return Report{}, err
	}
	if err := links.Close(); err != nil {
		return Report{}, err
	}
	return report, nil
}

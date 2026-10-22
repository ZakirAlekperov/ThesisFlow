// Package integrity audits local ThesisFlow document files without changing
// stored records. Пакет integrity проверяет файлы без изменения данных.
package integrity

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

	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

const maxAuditedFileSize = 20 << 20

// Issue contains the identity and reason for one broken stored file.
// RU: Issue описывает ID файла и причину нарушения целостности.
type Issue struct {
	VersionID    int64
	RelativePath string
	Problem      string
}

// Report summarizes examined, intact, lost, corrupt and unsafe versions.
// RU: Report содержит итог проверки документов, включая проблемы.
type Report struct {
	Checked int
	Intact  int
	Missing int
	Corrupt int
	Unsafe  int
	Issues  []Issue
}

// safeRegularPath rejects traversal, absolute paths and symlinks in every
// component, not only the final filename. It never opens outside root.
// RU: safeRegularPath запрещает обход каталогов и символические ссылки.
func safeRegularPath(root, relative string) (string, error) {
	normalized, err := storagepath.Normalize(relative)
	if err != nil {return "",err}
	current := root
	for _, part := range strings.Split(normalized, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {return "",err}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic link not allowed")
		}
	}
	info, err := os.Lstat(current)
	if err != nil {return "",err}
	if !info.Mode().IsRegular() {return "",errors.New("not a regular file")}
	return current, nil
}

// Check scans immutable file_versions and compares bytes to each stored SHA-256
// and size. Problems become report issues; SQL/read cancellations are errors.
// Nothing is repaired or written, so user data remains unchanged.
//
// RU: Check сравнивает файлы с контрольными суммами SHA-256 и размерами
// из SQLite. Проблемы перечисляются, но файлы и БД не изменяются.
func Check(ctx context.Context, db *sql.DB, root string) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	rows, err := db.QueryContext(ctx, "SELECT id,relative_path,file_size,sha256 FROM file_versions ORDER BY id")
	if err != nil {
		return Report{}, fmt.Errorf("list document versions: %w", err)
	}
	defer rows.Close()
	result := Report{Issues: make([]Issue, 0)}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		var id, size int64
		var relative, digest string
		if err := rows.Scan(&id, &relative, &size, &digest); err != nil {
			return Report{}, fmt.Errorf("read version metadata: %w", err)
		}
		result.Checked++
		record := func(problem string) {
			result.Issues = append(result.Issues, Issue{VersionID: id, RelativePath: relative, Problem: problem})
		}
		// Metadata or paths may be corrupted; never follow potentially
		// malicious entries even on a local single-user installation.
		if _, pathErr := storagepath.Normalize(relative); pathErr != nil {
			result.Unsafe++
			record("unsafe path")
			continue
		}
		full, err := safeRegularPath(root, relative)
		if errors.Is(err, os.ErrNotExist) {
			result.Missing++
			record("missing file")
			continue
		}
		if err != nil {
			result.Unsafe++
			record("unsafe filesystem entry")
			continue
		}
		if size < 0 || size > maxAuditedFileSize {
			result.Corrupt++
			record("invalid stored file size")
			continue
		}
		handle, err := os.Open(full)
		if err != nil {
			return Report{}, fmt.Errorf("open audited version %d: %w", id, err)
		}
		hash := sha256.New()
		length, copyErr := io.Copy(hash, io.LimitReader(handle, maxAuditedFileSize+1))
		closeErr := handle.Close()
		if copyErr != nil {
			return Report{}, fmt.Errorf("read audited version: %w", copyErr)
		}
		if closeErr != nil {
			return Report{}, fmt.Errorf("close audited version: %w", closeErr)
		}
		if length != size || length > maxAuditedFileSize || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), digest) {
			result.Corrupt++
			record("size or SHA-256 mismatch")
			continue
		}
		result.Intact++
	}
	if err := rows.Err(); err != nil {
		return Report{}, fmt.Errorf("iterate file metadata: %w", err)
	}
	return result, nil
}

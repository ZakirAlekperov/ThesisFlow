// Package backup exports a consistent SQLite snapshot and its document files
// into a verifiable ZIP archive and restores it into a new workspace.
//
// Пакет backup сохраняет согласованную копию SQLite и документов в проверяемом
// ZIP-архиве и восстанавливает их только в новый рабочий каталог.
package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)

const (
	// MaxArchiveBytes bounds a ZIP uploaded for recovery.
	// RU: MaxArchiveBytes ограничивает размер ZIP для восстановления.
	MaxArchiveBytes int64 = 512 << 20
	maxEntryBytes   int64 = 64 << 20
	maxTotalBytes   int64 = 1 << 30
	maxFiles              = 10000
)

var (
	// ErrDestinationExists prevents overwriting user data.
	// RU: ErrDestinationExists запрещает перезапись пользовательских файлов.
	ErrDestinationExists = errors.New("backup destination already exists")
	// ErrUnsafeArchive identifies untrusted paths, links or missing checksums.
	// RU: ErrUnsafeArchive означает опасные пути, ссылки или неверные хеши.
	ErrUnsafeArchive = errors.New("unsafe or corrupt backup archive")
	// ErrTooLarge reports that the backup exceeds the permitted size.
	// RU: ErrTooLarge означает превышение лимита размера.
	ErrTooLarge = errors.New("backup exceeds size limit")
)

type manifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

// checkNewPath ensures that an output path does not already exist.
// RU: checkNewPath проверяет отсутствие уже существующего назначения.
func checkNewPath(target string) error {
	_, err := os.Lstat(target)
	if err == nil {
		return ErrDestinationExists
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check backup destination: %w", err)
	}
	return nil
}

// addFile streams one source file to ZIP while calculating its SHA-256.
// RU: addFile записывает файл в ZIP и вычисляет SHA-256.
func addFile(writer *zip.Writer, source, name string, hashes map[string]string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrUnsafeArchive
	}
	if info.Size() > maxEntryBytes {
		return ErrTooLarge
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	entry, err := writer.Create(name)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	count, err := io.Copy(io.MultiWriter(entry, hasher), io.LimitReader(file, maxEntryBytes+1))
	if err != nil {
		return err
	}
	if count > maxEntryBytes {
		return ErrTooLarge
	}
	hashes[name] = hex.EncodeToString(hasher.Sum(nil))
	return nil
}

// Export writes a SQLite VACUUM INTO snapshot and all regular document files
// under storageDir to a new ZIP archive. The final archive is renamed only
// after writing succeeds; existing destinations are never overwritten.
//
// RU: Export создаёт согласованный снимок SQLite через VACUUM INTO, добавляет
// обычные файлы хранилища и завершает запись атомарным переименованием ZIP.
// Существующий архив никогда не перезаписывается.
func Export(ctx context.Context, db *sql.DB, storageDir, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkNewPath(destination); err != nil {
		return err
	}
	// Do not claim that a ZIP is a valid backup when its source workspace
	// already contains unresolved intents or corrupted approved bytes.
	// RU: Перед снимком проверяем файлы и незавершённые приёмки.
	audit, err := workspace.Check(ctx, db, storageDir)
	if err != nil {
		return fmt.Errorf("backup preflight: %w", err)
	}
	if !audit.Healthy() {
		return fmt.Errorf("%w: %d preflight integrity issues", ErrUnsafeArchive, len(audit.Issues))
	}
	workdir, err := os.MkdirTemp("", "thesisflow-snapshot-*")
	if err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	defer os.RemoveAll(workdir)
	snapshot := filepath.Join(workdir, "thesisflow.sqlite")
	escaped := strings.ReplaceAll(snapshot, "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("snapshot SQLite: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(destination), ".thesisflow-backup-*.zip")
	if err != nil {
		return fmt.Errorf("create temporary archive: %w", err)
	}
	defer os.Remove(output.Name())
	defer output.Close()

	writer := zip.NewWriter(output)
	hashes := make(map[string]string)
	if err := addFile(writer, snapshot, "thesisflow.sqlite", hashes); err != nil {
		writer.Close()
		return fmt.Errorf("archive SQLite snapshot: %w", err)
	}
	var walkErr error
	if info, err := os.Lstat(storageDir); err == nil && info.IsDir() {
		walkErr = filepath.WalkDir(storageDir, func(name string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if name == storageDir {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return ErrUnsafeArchive
			}
			relative, err := filepath.Rel(storageDir, name)
			if err != nil {
				return err
			}
			archivePath := path.Join("storage", filepath.ToSlash(relative))
			return addFile(writer, name, archivePath, hashes)
		})
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		walkErr = err
	} else if err == nil && !info.IsDir() {
		walkErr = ErrUnsafeArchive
	}
	if walkErr != nil {
		writer.Close()
		return fmt.Errorf("archive document files: %w", walkErr)
	}
	data, err := json.Marshal(manifest{Version: 1, Files: hashes})
	if err != nil {
		writer.Close()
		return err
	}
	entry, err := writer.Create("manifest.json")
	if err != nil {
		writer.Close()
		return err
	}
	if _, err := entry.Write(data); err != nil {
		writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := checkNewPath(destination); err != nil {
		return err
	}
	if err := os.Rename(output.Name(), destination); err != nil {
		return fmt.Errorf("finish backup: %w", err)
	}
	return nil
}

// safeEntry permits only the database, manifest, and regular storage files.
// It rejects absolute paths, traversal, Windows separators and drive names.
//
// RU: safeEntry разрешает лишь базу, манифест и обычные файлы storage;
// абсолютные пути, переходы ../ и пути Windows запрещены.
func safeEntry(name string) bool {
	if name == "" || strings.Contains(name, "\\") || strings.Contains(name, ":") || strings.HasPrefix(name, "/") {
		return false
	}
	if path.Clean(name) != name || strings.HasPrefix(name, "../") {
		return false
	}
	if !(name == "manifest.json" || name == "thesisflow.sqlite" || strings.HasPrefix(name, "storage/")) {
		return false
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return false
		}
		reserved := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		if reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" ||
			(len(reserved) == 4 && (strings.HasPrefix(reserved, "COM") || strings.HasPrefix(reserved, "LPT")) &&
				reserved[3] >= '1' && reserved[3] <= '9') {
			return false
		}
	}
	return true
}

// Restore verifies file hashes and safely restores a backup into a completely
// new directory. It refuses existing destinations and unsafe ZIP entries.
// A temporary directory is renamed to the final destination only after the
// full archive has been verified.
//
// RU: Restore сверяет SHA-256 и восстанавливает данные только в новый каталог.
// Существующие данные не затрагиваются; опасные пути отклоняются.
// До полной проверки распаковка остаётся во временном каталоге.
func Restore(archivePath, destination string) error {
	if err := checkNewPath(destination); err != nil {
		return err
	}
	stat, err := os.Stat(archivePath)
	if err != nil {
		return err
	}
	if stat.Size() > MaxArchiveBytes {
		return ErrTooLarge
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxFiles+2 {
		return ErrTooLarge
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(destination), ".thesisflow-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	found := make(map[string]string)
	casefold := make(map[string]struct{})
	var expected manifest
	manifestFound := false
	var total int64
	for _, entry := range reader.File {
		name := entry.Name
		if !safeEntry(name) || entry.FileInfo().Mode()&os.ModeSymlink != 0 || !entry.FileInfo().Mode().IsRegular() {
			return fmt.Errorf("%w: invalid entry %q", ErrUnsafeArchive, name)
		}
		// Windows and many macOS volumes are case-insensitive. Reject
		// archives that would overwrite different files on those systems.
		// RU: Архив не должен содержать коллизии имён без учёта регистра.
		key := strings.ToLower(name)
		if _, duplicate := casefold[key]; duplicate {
			return fmt.Errorf("%w: case-insensitive path collision", ErrUnsafeArchive)
		}
		casefold[key] = struct{}{}

		if entry.UncompressedSize64 > uint64(maxEntryBytes) {
			return ErrTooLarge
		}
		total += int64(entry.UncompressedSize64)
		if total > maxTotalBytes {
			return ErrTooLarge
		}
		if _, duplicate := found[name]; duplicate {
			return ErrUnsafeArchive
		}
		stream, err := entry.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(stream, maxEntryBytes+1))
		closeErr := stream.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if int64(len(data)) > maxEntryBytes {
			return ErrTooLarge
		}
		if name == "manifest.json" {
			if manifestFound {
				return ErrUnsafeArchive
			}
			manifestFound = true
			if err := json.Unmarshal(data, &expected); err != nil {
				return ErrUnsafeArchive
			}
			continue
		}
		digest := sha256.Sum256(data)
		found[name] = hex.EncodeToString(digest[:])
		target := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
	}
	if !manifestFound || expected.Version != 1 || len(found) != len(expected.Files) {
		return ErrUnsafeArchive
	}
	if _, present := found["thesisflow.sqlite"]; !present {
		return ErrUnsafeArchive
	}
	for name, digest := range found {
		if expected.Files[name] != digest {
			return fmt.Errorf("%w: hash mismatch %s", ErrUnsafeArchive, name)
		}
	}
	if err := checkRestoredWorkspace(staging); err != nil {
		return fmt.Errorf("%w: %v", ErrUnsafeArchive, err)
	}
	if err := checkNewPath(destination); err != nil {
		return err
	}
	if err := os.Rename(staging, destination); err != nil {
		return fmt.Errorf("complete restore: %w", err)
	}
	return nil
}

// checkRestoredWorkspace validates the staged SQLite snapshot and document
// references without opening the user's active database.
// RU: Проверка связей и файлов до публикации восстановленного каталога.
func checkRestoredWorkspace(destination string) error {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(destination, "thesisflow.sqlite"))
	if err != nil {
		return fmt.Errorf("open staged database: %w", err)
	}
	defer db.Close()
	report, err := workspace.Check(ctx, db, filepath.Join(destination, "storage"))
	if err != nil {
		return err
	}
	if report.SchemaVersion > migrate.CurrentVersion {
		return fmt.Errorf("unsupported SQLite schema %d", report.SchemaVersion)
	}
	if !report.Healthy() {
		return fmt.Errorf("staged workspace has %d integrity issues", len(report.Issues))
	}
	return nil
}

// Verify validates a ZIP in an isolated temporary folder, then removes it.
// It never overwrites the active workspace.
// RU: Проверяем архив без изменения действующих документов.
func Verify(archivePath string) error {
	temp, err := os.MkdirTemp("", "thesisflow-verify-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	return Restore(archivePath, filepath.Join(temp, "verified"))
}

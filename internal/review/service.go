// Package review accepts or rejects submitted document versions and maintains
// a human-readable projection of the latest approved files.
// Пакет review принимает и отклоняет версии документов и поддерживает
// человекочитаемую папку последних принятых файлов.
package review

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

// ErrInvalidVersion means that a version does not belong to the requested
// student and requirement, or does not exist.
// RU: ErrInvalidVersion означает отсутствующую или чужую версию файла.
var ErrInvalidVersion = errors.New("document version is not in the selected scope")

// Accepted describes a committed human-readable approved-file projection.
// RU: Accepted — зафиксированная принятая версия с понятным путём.
type Accepted struct {
	StudentID     int64
	RequirementID int64
	VersionID     int64
	RelativePath  string
}

// Service controls approval and the accepted filesystem projection.
// RU: Service управляет приёмкой и пользовательскими папками документов.
type Service struct {
	DB       *sql.DB
	Versions *fileversions.Store
	Clock    func() time.Time
}

// NewService uses an existing database and version store.
// RU: NewService работает на общей SQLite и хранилище версий.
func NewService(db *sql.DB, versions *fileversions.Store) *Service {
	return &Service{DB: db, Versions: versions, Clock: time.Now}
}

// Init creates an index of currently accepted files. The accepted user folder
// is intentionally not part of the internal versions directory.
// RU: Init создаёт индекс принятых документов отдельно от истории версий.
func (s *Service) Init(ctx context.Context) error {
	const query = `CREATE TABLE IF NOT EXISTS accepted_files (
		student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
		requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
		version_id INTEGER NOT NULL REFERENCES file_versions(id) ON DELETE RESTRICT,
		relative_path TEXT NOT NULL UNIQUE,
		accepted_at TEXT NOT NULL,
		PRIMARY KEY(student_id,requirement_id)
	)`
	if _, err := s.DB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("initialize accepted files index: %w", err)
	}
	return s.initFeedback(ctx)
}

// safeName removes characters forbidden by major desktop filesystems while
// preserving readable Cyrillic text and whitespace.
// RU: safeName сохраняет читаемую кириллицу, удаляя опасные символы имени.
func safeName(value string) string {
	var b strings.Builder
	for _, char := range strings.TrimSpace(value) {
		switch char {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			if unicode.IsControl(char) {
				b.WriteRune('_')
			} else {
				b.WriteRune(char)
			}
		}
	}
	name := strings.Trim(b.String(), ". ")
	if name == "" {
		return "Без названия"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	// Windows device names cannot be used as filenames without a prefix.
	// RU: Избегаем зарезервированных имён Windows.
	upper := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name
}

type versionInfo struct {
	studentID, requirementID, groupID                    int64
	name, educationalID, groupName, title, relative, sha string
}

// versionInfo verifies the scope and retrieves human-readable names before
// opening the SQL transaction for file projection.
// RU: versionInfo проверяет владельца версии и получает читаемые названия.
func (s *Service) versionInfo(ctx context.Context, studentID, requirementID, versionID int64) (versionInfo, error) {
	if studentID <= 0 || requirementID <= 0 || versionID <= 0 {
		return versionInfo{}, ErrInvalidVersion
	}
	const query = `SELECT s.id,r.id,g.id,s.full_name,s.educational_id,g.name,r.title,fv.relative_path,fv.sha256
		FROM file_versions fv JOIN students s ON fv.student_id=s.id
		JOIN requirements r ON fv.requirement_id=r.id
		JOIN student_groups g ON s.group_id=g.id AND r.group_id=g.id
		WHERE fv.id=? AND fv.student_id=? AND fv.requirement_id=?
		AND (r.student_id IS NULL OR r.student_id=s.id)`
	var result versionInfo
	err := s.DB.QueryRowContext(ctx, query, versionID, studentID, requirementID).
		Scan(&result.studentID, &result.requirementID, &result.groupID,
			&result.name, &result.educationalID, &result.groupName, &result.title, &result.relative, &result.sha)
	if errors.Is(err, sql.ErrNoRows) {
		return versionInfo{}, ErrInvalidVersion
	}
	if err != nil {
		return versionInfo{}, fmt.Errorf("look up document version: %w", err)
	}
	return result, nil
}

// projectionPath produces the readable group/student/document hierarchy.
// The internal student ID guarantees folder uniqueness even after sanitizing.
// RU: projectionPath формирует понятные папки с уникальным ID студента.
func projectionPath(info versionInfo, versionExt string) string {
	groupFolder := fmt.Sprintf("%s [%d]", safeName(info.groupName), info.groupID)
	studentFolder := fmt.Sprintf("%s [%s-%d]", safeName(info.name), safeName(info.educationalID), info.studentID)
	return path.Join(groupFolder, studentFolder, safeName(info.title)+versionExt)
}

// findProjection resolves collisions with previously approved documents.
// RU: findProjection исключает случайное совпадение имён двух требований.
func (s *Service) findProjection(ctx context.Context, wanted string, studentID, requirementID int64) (string, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM accepted_files
		WHERE relative_path=? AND NOT(student_id=? AND requirement_id=?)`, wanted, studentID, requirementID).Scan(&count)
	if err != nil {
		return "", fmt.Errorf("check accepted document path: %w", err)
	}
	if count == 0 {
		return wanted, nil
	}
	ext := filepath.Ext(wanted)
	return strings.TrimSuffix(wanted, ext) + " [R" + strconv.FormatInt(requirementID, 10) + "]" + ext, nil
}

// Accept approves one immutable version and projects its bytes into a clean
// group/student/document path. The previous accepted copy stays intact until
// the new bytes are ready; in case of a database failure, the old copy is
// restored. The complete history remains in .internal/versions.
//
// RU: Accept принимает выбранную версию и копирует её в понятную папку.
// Старая принятая копия защищена до подготовки новой; при ошибке SQLite
// выполняется возврат старого файла. История версий остаётся нетронутой.
func (s *Service) Accept(ctx context.Context, studentID, requirementID, versionID int64) (Accepted, error) {
	if err := ctx.Err(); err != nil {
		return Accepted{}, err
	}
	info, err := s.versionInfo(ctx, studentID, requirementID, versionID)
	if err != nil {
		return Accepted{}, err
	}
	data, err := s.Versions.Read(ctx, versionID)
	if err != nil {
		return Accepted{}, fmt.Errorf("read approved source: %w", err)
	}
	checksum := sha256.Sum256(data)
	if hex.EncodeToString(checksum[:]) != info.sha {
		return Accepted{}, errors.New("document version checksum mismatch")
	}
	extension := filepath.Ext(info.relative)
	relative, err := s.findProjection(ctx, projectionPath(info, extension), studentID, requirementID)
	if err != nil {
		return Accepted{}, err
	}

	var previous string
	err = s.DB.QueryRowContext(ctx, `SELECT relative_path FROM accepted_files
		WHERE student_id=? AND requirement_id=?`, studentID, requirementID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Accepted{}, fmt.Errorf("find previous approval: %w", err)
	}
	// Legacy Windows versions stored '\\' separators in SQLite. Normalize
	// them for comparison but do not rewrite or delete old user data.
	// RU: Читаем исторические пути Windows как переносимые.
	if previous != "" {
		previous, err = storagepath.Normalize(previous)
		if err != nil {
			return Accepted{}, fmt.Errorf("unsafe previously accepted path: %w", err)
		}
	}
	// An identical accepted version is idempotent if the visible file exists.
	// RU: Повторная приёмка той же версии не изменяет файл и историю.
	var currentVersionID int64
	_ = s.DB.QueryRowContext(ctx, `SELECT version_id FROM accepted_files
		WHERE student_id=? AND requirement_id=?`, studentID, requirementID).Scan(&currentVersionID)
	if currentVersionID == versionID && previous == relative {
		if _, err := os.Stat(filepath.Join(s.Versions.Root, filepath.FromSlash(relative))); err == nil {
			return Accepted{StudentID: studentID, RequirementID: requirementID, VersionID: versionID, RelativePath: relative}, nil
		}
	}
	target := filepath.Join(s.Versions.Root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return Accepted{}, fmt.Errorf("create accepted folder: %w", err)
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return Accepted{}, fmt.Errorf("prepare approval file: %w", err)
	}
	token := hex.EncodeToString(random)
	temporary := target + ".tmp-" + token
	handle, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Accepted{}, fmt.Errorf("create approval temporary file: %w", err)
	}
	defer os.Remove(temporary)
	if _, err := handle.Write(data); err != nil {
		handle.Close()
		return Accepted{}, fmt.Errorf("write approval file: %w", err)
	}
	if err := handle.Sync(); err != nil {
		handle.Close()
		return Accepted{}, fmt.Errorf("sync approval file: %w", err)
	}
	if err := handle.Close(); err != nil {
		return Accepted{}, fmt.Errorf("close approval file: %w", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Accepted{}, fmt.Errorf("begin approval transaction: %w", err)
	}
	defer tx.Rollback()
	backup := ""
	if _, err := os.Stat(target); err == nil {
		// Never overwrite an untracked user file or another requirement's file.
		// RU: Не заменяем посторонний файл, не принадлежащий этому требованию.
		if previous != relative {
			return Accepted{}, errors.New("accepted path already contains a different file")
		}
		backup = target + ".backup-" + token
		if err := os.Rename(target, backup); err != nil {
			return Accepted{}, fmt.Errorf("protect previous accepted file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return Accepted{}, fmt.Errorf("check approval target: %w", err)
	}
	if err := os.Rename(temporary, target); err != nil {
		if backup != "" {
			_ = os.Rename(backup, target)
		}
		return Accepted{}, fmt.Errorf("replace approved file: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(target)
			if backup != "" {
				_ = os.Rename(backup, target)
			}
		}
	}()
	now := s.Clock().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO accepted_files(student_id,requirement_id,version_id,relative_path,accepted_at)
		VALUES(?,?,?,?,?) ON CONFLICT(student_id,requirement_id) DO UPDATE SET
		version_id=excluded.version_id,relative_path=excluded.relative_path,accepted_at=excluded.accepted_at`,
		studentID, requirementID, versionID, relative, now)
	if err != nil {
		return Accepted{}, fmt.Errorf("save accepted file: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE file_versions SET accepted=1 WHERE id=?", versionID); err != nil {
		return Accepted{}, fmt.Errorf("mark accepted version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO material_status(student_id,requirement_id,status,changed_at)
		VALUES(?,?,?,?) ON CONFLICT(student_id,requirement_id) DO UPDATE SET
		status=excluded.status,changed_at=excluded.changed_at`, studentID, requirementID, string(materials.Accepted), now); err != nil {
		return Accepted{}, fmt.Errorf("update accepted status: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO material_events(student_id,requirement_id,status,occurred_at)
		VALUES(?,?,?,?)`, studentID, requirementID, string(materials.Accepted), now); err != nil {
		return Accepted{}, fmt.Errorf("record accepted event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Accepted{}, fmt.Errorf("commit approval: %w", err)
	}
	committed = true
	if backup != "" {
		_ = os.Remove(backup)
	}
	if previous != "" && previous != relative && previous != "" {
		_ = os.Remove(filepath.Join(s.Versions.Root, filepath.FromSlash(previous)))
	}
	return Accepted{StudentID: studentID, RequirementID: requirementID, VersionID: versionID, RelativePath: relative}, nil
}

// Reject marks a submitted version for revision. Old accepted files and all
// internal version contents are intentionally preserved.
//
// RU: Reject отправляет документ на доработку, не удаляя принятые версии.
func (s *Service) Reject(ctx context.Context, studentID, requirementID, versionID int64) error {
	if _, err := s.versionInfo(ctx, studentID, requirementID, versionID); err != nil {
		return err
	}
	_, err := materials.NewRepository(s.DB).Set(ctx, studentID, requirementID, materials.NeedsRevision)
	return err
}

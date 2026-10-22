// Package fileversions stores immutable submitted document files off the
// teacher's accepted-files directory, preserving every uploaded revision.
// Пакет fileversions хранит неизменяемые версии документов ВКР отдельно
// от пользовательской папки принятых документов.
package fileversions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
)

// MaxFileSize caps individual submitted files at 20 MiB in MVP.
// RU: MaxFileSize ограничивает размер отдельного документа 20 МиБ.
const MaxFileSize = 20 << 20

// ErrInvalidFile reports a disallowed extension or empty uploaded document.
// RU: ErrInvalidFile означает пустой файл или запрещённое расширение.
var ErrInvalidFile = errors.New("submitted document file is invalid")

// ErrTooLarge reports that a document exceeds the allowed file size.
// RU: ErrTooLarge означает превышение максимального размера документа.
var ErrTooLarge = errors.New("submitted document exceeds size limit")

// ErrInvalidScope reports a student/requirement ownership mismatch.
// RU: ErrInvalidScope означает, что документ не принадлежит студенту.
var ErrInvalidScope = errors.New("document student/requirement scope invalid")

// ErrVersionNotFound reports a missing saved document version.
// RU: ErrVersionNotFound означает отсутствие указанной версии файла.
var ErrVersionNotFound = errors.New("document version not found")

// Version describes immutable metadata and storage location of one upload.
// RU: Version описывает неизменяемую запись одной загруженной версии.
type Version struct {
	ID            int64
	StudentID     int64
	RequirementID int64
	OriginalName  string
	RelativePath  string
	Size          int64
	SHA256        string
	UploadedAt    string
}

// Store owns internal document files and uses the application's SQLite DB.
// The Root is a dedicated application-managed storage directory.
//
// RU: Store управляет внутренними файлами и использует базу приложения.
// Root — отдельный каталог служебного хранилища.
type Store struct {
	DB    *sql.DB
	Root  string
	Clock func() time.Time
}

// NewStore uses an existing SQLite connection and dedicated data root.
// RU: NewStore получает общую SQLite и корень внутреннего хранилища.
func NewStore(db *sql.DB, root string) *Store {
	return &Store{DB: db, Root: root, Clock: time.Now}
}

// Init creates an additive version metadata table without touching files.
// RU: Init создаёт таблицу версий, не изменяя уже сохранённые файлы.
func (s *Store) Init(ctx context.Context) error {
	const query = `CREATE TABLE IF NOT EXISTS file_versions(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE RESTRICT,
		requirement_id INTEGER NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
		original_name TEXT NOT NULL,
		relative_path TEXT NOT NULL UNIQUE,
		file_size INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		uploaded_at TEXT NOT NULL,
		accepted INTEGER NOT NULL DEFAULT 0 CHECK (accepted IN (0,1))
	)`
	if _, err := s.DB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("initialize document versions: %w", err)
	}
	return nil
}

// permittedExtension is intentionally limited to common thesis documents.
// RU: permittedExtension разрешает привычные форматы документов ВКР.
func permittedExtension(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".docx", ".pdf", ".odt", ".rtf", ".txt", ".xlsx":
		return extension, true
	default:
		return "", false
	}
}

// fileScope checks that a requirement applies to the specified student.
// RU: fileScope проверяет принадлежность документа конкретному студенту.
func fileScope(ctx context.Context, tx *sql.Tx, studentID, requirementID int64) error {
	if studentID <= 0 || requirementID <= 0 {
		return ErrInvalidScope
	}
	var valid int
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM students s JOIN requirements r ON r.group_id=s.group_id
		WHERE s.id=? AND r.id=? AND (r.student_id IS NULL OR r.student_id=s.id)
	)`, studentID, requirementID).Scan(&valid)
	if err != nil {
		return fmt.Errorf("validate document ownership: %w", err)
	}
	if valid == 0 {
		return ErrInvalidScope
	}
	return nil
}

// Save writes a new immutable file under .internal/versions and creates its
// SQLite row in a transaction. It sets the material back to UnderReview and
// records the submission time; earlier versions and any previously accepted
// user-visible file projection are never overwritten here.
//
// RU: Save записывает новую версию в .internal/versions, атомарно регистрирует
// её в SQLite и переводит документ в «На проверке». Старые версии и ранее
// принятый файл в пользовательской папке не перезаписываются.
func (s *Store) Save(ctx context.Context, studentID, requirementID int64, originalName string, data []byte) (Version, error) {
	if err := ctx.Err(); err != nil {
		return Version{}, err
	}
	if len(data) == 0 {
		return Version{}, ErrInvalidFile
	}
	if len(data) > MaxFileSize {
		return Version{}, ErrTooLarge
	}
	extension, valid := permittedExtension(originalName)
	if !valid {
		return Version{}, ErrInvalidFile
	}
	if s.Root == "" {
		return Version{}, errors.New("document storage root is empty")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Version{}, fmt.Errorf("begin file-version transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fileScope(ctx, tx, studentID, requirementID); err != nil {
		return Version{}, err
	}

	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return Version{}, fmt.Errorf("generate version name: %w", err)
	}
	relative := filepath.Join(".internal", "versions", strconv.FormatInt(studentID, 10),
		strconv.FormatInt(requirementID, 10), hex.EncodeToString(token)+extension)
	target := filepath.Join(s.Root, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return Version{}, fmt.Errorf("create document versions directory: %w", err)
	}
	temporary := target + ".tmp"
	handle, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Version{}, fmt.Errorf("create temporary document: %w", err)
	}
	defer os.Remove(temporary)
	if _, err := handle.Write(data); err != nil {
		handle.Close()
		return Version{}, fmt.Errorf("write document version: %w", err)
	}
	if err := handle.Sync(); err != nil {
		handle.Close()
		return Version{}, fmt.Errorf("sync document version: %w", err)
	}
	if err := handle.Close(); err != nil {
		return Version{}, fmt.Errorf("close document version: %w", err)
	}
	if err := os.Rename(temporary, target); err != nil {
		return Version{}, fmt.Errorf("finish document version: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(target)
		}
	}()

	originalName = filepath.Base(strings.ReplaceAll(originalName, "\\", "/"))
	uploadedAt := s.Clock().UTC().Format(time.RFC3339Nano)
	digest := sha256.Sum256(data)
	version := Version{
		StudentID: studentID, RequirementID: requirementID, OriginalName: originalName,
		RelativePath: relative, Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]),
		UploadedAt: uploadedAt,
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO file_versions(
		student_id,requirement_id,original_name,relative_path,file_size,sha256,uploaded_at
	) VALUES(?,?,?,?,?,?,?)`, studentID, requirementID, version.OriginalName,
		version.RelativePath, version.Size, version.SHA256, version.UploadedAt)
	if err != nil {
		return Version{}, fmt.Errorf("save file version metadata: %w", err)
	}
	version.ID, err = result.LastInsertId()
	if err != nil {
		return Version{}, fmt.Errorf("read file version ID: %w", err)
	}
	// Register each upload as a new review event, even for a resubmission.
	// RU: Каждая новая версия — отдельное событие сдачи на проверку.
	if _, err := tx.ExecContext(ctx, `INSERT INTO material_status(student_id,requirement_id,status,changed_at)
		VALUES (?,?,?,?)
		ON CONFLICT(student_id,requirement_id) DO UPDATE SET
		status=excluded.status,changed_at=excluded.changed_at`,
		studentID, requirementID, string(materials.UnderReview), uploadedAt); err != nil {
		return Version{}, fmt.Errorf("mark material for review: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO material_events(student_id,requirement_id,status,occurred_at)
		VALUES(?,?,?,?)`, studentID, requirementID, string(materials.UnderReview), uploadedAt); err != nil {
		return Version{}, fmt.Errorf("record document upload event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Version{}, fmt.Errorf("commit document version: %w", err)
	}
	committed = true
	return version, nil
}

// List returns all immutable versions for one student's requirement.
// RU: List возвращает все загруженные версии одного документа студента.
func (s *Store) List(ctx context.Context, studentID, requirementID int64) ([]Version, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,student_id,requirement_id,original_name,
		relative_path,file_size,sha256,uploaded_at FROM file_versions
		WHERE student_id=? AND requirement_id=? ORDER BY id`, studentID, requirementID)
	if err != nil {
		return nil, fmt.Errorf("list document versions: %w", err)
	}
	defer rows.Close()
	result := make([]Version, 0)
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.StudentID, &v.RequirementID, &v.OriginalName, &v.RelativePath, &v.Size, &v.SHA256, &v.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan document version: %w", err)
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read version list: %w", err)
	}
	return result, nil
}

// Read returns immutable version bytes, capped by MaxFileSize and resolved
// only beneath the configured application storage root.
// RU: Read читает файл версии только внутри служебного каталога приложения.
func (s *Store) Read(ctx context.Context, versionID int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var relative, expectedHash string
	var expectedSize int64
	err := s.DB.QueryRowContext(ctx,
		"SELECT relative_path,file_size,sha256 FROM file_versions WHERE id=?", versionID).
		Scan(&relative, &expectedSize, &expectedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find file version: %w", err)
	}
	if !filepath.IsLocal(relative) || strings.ContainsAny(relative, "\\\\:") {
		return nil, ErrInvalidFile
	}
	// Ensure the recorded path still contains only actual directories and one
	// regular file. Plain os.Open follows symlinks and must not be used alone.
	// RU: Запрещаем символические ссылки на всех уровнях относительного пути.
	rootInfo, err := os.Lstat(s.Root)
	if err != nil {
		return nil, fmt.Errorf("inspect version store: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidFile
	}
	target := s.Root
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for i, part := range parts {
		target = filepath.Join(target, part)
		info, err := os.Lstat(target)
		if err != nil {
			return nil, fmt.Errorf("inspect stored version path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalidFile
		}
		if i < len(parts)-1 && !info.IsDir() {
			return nil, ErrInvalidFile
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, ErrInvalidFile
		}
	}
	handle, err := os.Open(target)
	if err != nil {
		return nil, fmt.Errorf("open stored version: %w", err)
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read stored version: %w", err)
	}
	if len(data) > MaxFileSize {
		return nil, ErrTooLarge
	}
	if int64(len(data)) != expectedSize {
		return nil, ErrInvalidFile
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != expectedHash {
		return nil, ErrInvalidFile
	}
	return data, nil
}

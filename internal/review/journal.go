package review

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
)

// approvalMu serializes review file publications from different Service
// instances within one process. This is NOT a cross-process file lock.
// RU: Мьютекс защищает приёмку в одном процессе, не между процессами.
var approvalMu sync.Mutex

// ErrPendingApproval means recovery must run before further publications.
// RU: Неоконченная приёмка должна быть разрешена до новых изменений.
var ErrPendingApproval = errors.New("unfinished approved-file operation requires recovery")

// ErrRecoveryConflict means neither the old nor intended bytes can be
// safely identified. Such files are never deleted or overwritten.
// RU: При неизвестных байтах автоматическое восстановление запрещено.
var ErrRecoveryConflict = errors.New("approved-file recovery conflict")

type approvalIntent struct {
	Token             string
	StudentID         int64
	RequirementID     int64
	VersionID         int64
	PreviousVersionID int64
	PreviousPath      string
	TargetPath        string
	CreatedAt         string
}

// initJournal creates a durable committed intent *before* any accepted file
// is renamed. A separate SQLite transaction then changes accepted_files.
// RU: Намерение записывается до переименования пользовательского файла.
func (s *Service) initJournal(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS approval_operations(
  token TEXT PRIMARY KEY,
  student_id INTEGER NOT NULL,
  requirement_id INTEGER NOT NULL,
  version_id INTEGER NOT NULL,
  previous_version_id INTEGER NOT NULL,
  previous_path TEXT NOT NULL,
  target_path TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(student_id,requirement_id)
 )`)
	if err != nil {
		return fmt.Errorf("initialize approval operation journal: %w", err)
	}
	return nil
}

func (s *Service) insertIntent(ctx context.Context, op approvalIntent) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO approval_operations(
 token,student_id,requirement_id,version_id,previous_version_id,
 previous_path,target_path,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		op.Token, op.StudentID, op.RequirementID, op.VersionID,
		op.PreviousVersionID, op.PreviousPath, op.TargetPath, op.CreatedAt)
	if err != nil {
		return fmt.Errorf("%w: cannot record durable intent: %v", ErrPendingApproval, err)
	}
	return nil
}

func (s *Service) deleteIntent(ctx context.Context, token string) error {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM approval_operations WHERE token=?", token)
	if err != nil {
		return fmt.Errorf("complete approval journal: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrPendingApproval
	}
	return nil
}

// secureTarget refuses symlinks in the workspace and all existing relative
// components. Missing directories can be created only for publication.
// RU: Проверяем корень и всю цепочку каталогов до любого открытия файла.
func secureTarget(root, relative string, createDirs bool) (string, error) {
	normal, err := storagepath.Normalize(relative)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", ErrRecoveryConflict
	}
	parts := strings.Split(normal, "/")
	at := root
	for _, component := range parts[:len(parts)-1] {
		at = filepath.Join(at, component)
		info, err := os.Lstat(at)
		if errors.Is(err, os.ErrNotExist) && createDirs {
			if err := os.Mkdir(at, 0700); err != nil {
				return "", err
			}
			info, err = os.Lstat(at)
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrRecoveryConflict
		}
	}
	target := filepath.Join(at, parts[len(parts)-1])
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", ErrRecoveryConflict
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return target, nil
}

func fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// existingHash deliberately treats a missing file differently from unknown
// or symlinked content. Never remove a file with unexpected bytes.
// RU: Перед удалением/заменой сверяем байты конкретного файла.
func existingHash(filename string) (hash string, present bool, err error) {
	st, err := os.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 {
		return "", false, ErrRecoveryConflict
	}
	if st.Size() > int64(64<<20) {
		return "", false, ErrRecoveryConflict
	}
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", false, err
	}
	return fingerprint(bytes), true, nil
}

// ensureProjection uses a synced temporary file in the same directory,
// refuses unknown existing bytes and atomically replaces known old/new data.
// Its caller has already validated immutable version source bytes.
// RU: Заменяем только известную проекцию; непонятные файлы не трогаем.
func ensureProjection(target string, intended []byte, known ...string) error {
	current, present, err := existingHash(target)
	if err != nil {
		return err
	}
	wanted := fingerprint(intended)
	if present && current == wanted {
		return nil
	}
	if present {
		safe := false
		for _, expected := range known {
			if expected != "" && current == expected {
				safe = true
			}
		}
		if !safe {
			return ErrRecoveryConflict
		}
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".thesisflow-repair-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(intended); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		return err
	}
	return storagepath.SyncParent(target)
}

func removeKnown(filename, expected string) error {
	hash, present, err := existingHash(filename)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if expected == "" || hash != expected {
		return ErrRecoveryConflict
	}
	if err := os.Remove(filename); err != nil {return err}
	return storagepath.SyncParent(filename)
}

// RecoverPending resolves every durable approval intent using the committed
// accepted_files row as authority. A crash before SQLite commit restores the
// previous version; a crash after commit confirms the newly accepted bytes.
// It is idempotent on repeat start and never trusts a guessed file name.
// RU: Восстановление строго следует подтверждённой SQLite и хешам версий.
func (s *Service) RecoverPending(ctx context.Context) (int, error) {
	approvalMu.Lock()
	defer approvalMu.Unlock()
	return s.recoverPending(ctx)
}

func (s *Service) recoverPending(ctx context.Context) (int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT token,student_id,requirement_id,version_id,
 previous_version_id,previous_path,target_path,created_at
 FROM approval_operations ORDER BY created_at,token LIMIT 1001`)
	if err != nil {
		return 0, fmt.Errorf("read approval intents: %w", err)
	}
	var intents []approvalIntent
	for rows.Next() {
		var op approvalIntent
		if err := rows.Scan(&op.Token, &op.StudentID, &op.RequirementID, &op.VersionID,
			&op.PreviousVersionID, &op.PreviousPath, &op.TargetPath, &op.CreatedAt); err != nil {
			rows.Close()
			return 0, err
		}
		intents = append(intents, op)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(intents) > 1000 {
		return 0, ErrRecoveryConflict
	}
	count := 0
	for _, op := range intents {
		if err := s.resolveIntent(ctx, op); err != nil {
			return count, fmt.Errorf("recover approval %s: %w", op.Token, err)
		}
		count++
	}
	return count, nil
}

func (s *Service) resolveIntent(ctx context.Context, op approvalIntent) error {
	if len(op.Token) != 24 || op.StudentID <= 0 || op.RequirementID <= 0 || op.VersionID <= 0 {
		return ErrRecoveryConflict
	}
	if _, err := storagepath.Normalize(op.TargetPath); err != nil {
		return ErrRecoveryConflict
	}
	if op.PreviousPath != "" {
		if _, err := storagepath.Normalize(op.PreviousPath); err != nil {
			return ErrRecoveryConflict
		}
	}
	if (op.PreviousVersionID == 0) != (op.PreviousPath == "") {
		return ErrRecoveryConflict
	}
	// The immutable source is verified by Read against size and SHA-256.
	newData, err := s.Versions.Read(ctx, op.VersionID)
	if err != nil {
		return fmt.Errorf("read new immutable source: %w", err)
	}
	oldData := []byte(nil)
	oldHash := ""
	if op.PreviousVersionID > 0 {
		oldData, err = s.Versions.Read(ctx, op.PreviousVersionID)
		if err != nil {
			return fmt.Errorf("read prior immutable source: %w", err)
		}
		oldHash = fingerprint(oldData)
	}
	newHash := fingerprint(newData)
	var currentID int64
	var currentPath string
	err = s.DB.QueryRowContext(ctx, `SELECT version_id,relative_path FROM accepted_files
 WHERE student_id=? AND requirement_id=?`, op.StudentID, op.RequirementID).
		Scan(&currentID, &currentPath)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		currentID = 0
		currentPath = ""
	}
	if currentPath != "" {
		currentPath, err = storagepath.Normalize(currentPath)
		if err != nil {
			return ErrRecoveryConflict
		}
	}
	committed := currentID == op.VersionID && currentPath == op.TargetPath
	rolledBack := currentID == op.PreviousVersionID && currentPath == op.PreviousPath
	if !committed && !rolledBack {
		return ErrRecoveryConflict
	}
	target, err := secureTarget(s.Versions.Root, op.TargetPath, true)
	if err != nil {
		return err
	}
	backup := target + ".backup-" + op.Token
	if committed {
		if err := ensureProjection(target, newData, oldHash); err != nil {
			return err
		}
		if err := removeKnown(backup, oldHash); err != nil {
			return err
		}
		if op.PreviousPath != "" && op.PreviousPath != op.TargetPath {
			prior, err := secureTarget(s.Versions.Root, op.PreviousPath, false)
			if err != nil {
				return err
			}
			if err := removeKnown(prior, oldHash); err != nil {
				return err
			}
		}
	} else {
		if op.PreviousPath == op.TargetPath {
			if err := ensureProjection(target, oldData, newHash); err != nil {
				return err
			}
		} else {
			if err := removeKnown(target, newHash); err != nil {
				return err
			}
			if op.PreviousPath != "" {
				prior, err := secureTarget(s.Versions.Root, op.PreviousPath, true)
				if err != nil {
					return err
				}
				if err := ensureProjection(prior, oldData); err != nil {
					return err
				}
			}
		}
		if err := removeKnown(backup, oldHash); err != nil {
			return err
		}
	}
	return s.deleteIntent(ctx, op.Token)
}

// PendingCount is a read-only startup/doctor indicator.
// RU: Количество незавершённых операций для диагностики.
func (s *Service) PendingCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM approval_operations").Scan(&n)
	return n, err
}

// The journal timestamp is informational; SQLite alone selects the
// authoritative accepted version after an interrupted operation.
// RU: Время намерения не используется как основание для удаления файлов.
func approvalTimestamp(clock func() time.Time) string {
	return clock().UTC().Format(time.RFC3339Nano)
}

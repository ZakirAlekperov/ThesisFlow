// Package workspaceselect stores the user's next-launch choice without
// replacing or modifying the workspace currently opened by ThesisFlow.
// Пакет workspaceselect выбирает каталог только для следующего запуска.
package workspaceselect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/migrate"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storagepath"
	"github.com/ZakirAlekperov/ThesisFlow/internal/workspace"
)

const Default = "default"
const selectedFile = ".selected-workspace"

var restoreID = regexp.MustCompile(`^restore-[0-9a-f]{24}$`)

// ErrInvalidChoice rejects unknown folders and tampered selection metadata.
// RU: ErrInvalidChoice не допускает подмену пути или выбор чужой папки.
var ErrInvalidChoice = errors.New("invalid or unsafe workspace selection")

// Option describes a recognized recovered workspace under restores.
// RU: Option не содержит персональных данных.
type Option struct {
	ID      string
	Created time.Time
}

// Service keeps selection state outside recovered workspaces, in the
// original per-user ThesisFlow directory.
// RU: Service не меняет содержимое восстановленных каталогов.
type Service struct{ Root string }

// New binds selection to the default application data directory.
// RU: New привязывает выбор к исходному каталогу приложения.
func New(root string) *Service { return &Service{Root: root} }

func (s *Service) path() string { return filepath.Join(s.Root, selectedFile) }
func validID(id string) bool    { return id == Default || restoreID.MatchString(id) }
func secureDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidChoice
	}
	return nil
}
func secureFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidChoice
	}
	return nil
}

// Candidate returns an existing restored directory and refuses symlinks and
// unknown/ambiguous identifiers. It does not create any directory or DB.
// RU: Нельзя указать произвольный путь с клиентской формы.
func (s *Service) Candidate(id string) (string, error) {
	if !restoreID.MatchString(id) {
		return "", ErrInvalidChoice
	}
	if err := secureDir(s.Root); err != nil {
		return "", fmt.Errorf("%w: home: %v", ErrInvalidChoice, err)
	}
	restores := filepath.Join(s.Root, "restores")
	if err := secureDir(restores); err != nil {
		return "", fmt.Errorf("%w: restores: %v", ErrInvalidChoice, err)
	}
	dir := filepath.Join(restores, id)
	if err := secureDir(dir); err != nil {
		return "", fmt.Errorf("%w: candidate: %v", ErrInvalidChoice, err)
	}
	if err := secureFile(filepath.Join(dir, "thesisflow.sqlite")); err != nil {
		return "", fmt.Errorf("%w: database: %v", ErrInvalidChoice, err)
	}
	store := filepath.Join(dir, "storage")
	if err := secureDir(store); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: storage: %v", ErrInvalidChoice, err)
	}
	return dir, nil
}

// List returns safe-looking restore directories, newest first. Corrupt DBs
// remain visible but cannot be selected until their verification succeeds.
// RU: Каталог виден в списке, однако выбор требует полной проверки.
func (s *Service) List(ctx context.Context) ([]Option, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	folder := filepath.Join(s.Root, "restores")
	entries, err := os.ReadDir(folder)
	if errors.Is(err, os.ErrNotExist) {
		return []Option{}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := secureDir(folder); err != nil {
		return nil, err
	}
	options := make([]Option, 0)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !restoreID.MatchString(entry.Name()) || !entry.IsDir() {
			continue
		}
		full := filepath.Join(folder, entry.Name())
		if err := secureDir(full); err != nil {
			continue
		}
		if err := secureFile(filepath.Join(full, "thesisflow.sqlite")); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		options = append(options, Option{ID: entry.Name(), Created: info.ModTime()})
	}
	// Natural order by timestamp, and ID for deterministic ties.
	sortOptions(options)
	return options, nil
}
func sortOptions(items []Option) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && (items[j].Created.After(items[j-1].Created) ||
			items[j].Created.Equal(items[j-1].Created) && items[j].ID > items[j-1].ID); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// Current reports the persisted next-launch choice. Corrupt/symlinked state
// is an error, never an implicit fallback to a new empty database.
// RU: Повреждённый выбор не сбрасывается молча.
func (s *Service) Current() (string, error) {
	file := s.path()
	if err := secureFile(file); errors.Is(err, os.ErrNotExist) {
		return Default, nil
	} else if err != nil {
		return "", fmt.Errorf("%w: metadata: %v", ErrInvalidChoice, err)
	}
	fd, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer fd.Close()
	data, err := io.ReadAll(io.LimitReader(fd, 129))
	if err != nil {
		return "", err
	}
	if len(data) > 128 {
		return "", ErrInvalidChoice
	}
	id := strings.TrimSuffix(string(data), "\n")
	if !validID(id) {
		return "", ErrInvalidChoice
	}
	return id, nil
}

// Validate checks a candidate's current SQLite, schema and document SHA-256,
// without migration, file deletion or journal replay.
// RU: Подтверждение выбора требует успешного read-only аудита.
func (s *Service) Validate(ctx context.Context, id string) error {
	if id == Default {
		return secureDir(s.Root)
	}
	dir, err := s.Candidate(id)
	if err != nil {
		return err
	}
	db, err := sqlite.Open(ctx, filepath.Join(dir, "thesisflow.sqlite"))
	if err != nil {
		return fmt.Errorf("%w: database open: %v", ErrInvalidChoice, err)
	}
	defer db.Close()
	report, err := workspace.Check(ctx, db, filepath.Join(dir, "storage"))
	if err != nil {
		return fmt.Errorf("%w: audit: %v", ErrInvalidChoice, err)
	}
	if !report.Healthy() || report.SchemaVersion > migrate.CurrentVersion {
		return ErrInvalidChoice
	}
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table'
  AND name IN ('student_groups','students')`).Scan(&tables); err != nil {
		return err
	}
	if tables != 2 {
		return ErrInvalidChoice
	}
	return nil
}

// Choose atomically persists the user's choice for the NEXT launch.
// Current DB connections and browser sessions are never switched in place.
// RU: Файл настройки меняется атомарно после успешной проверки.
func (s *Service) Choose(ctx context.Context, id string) error {
	if !validID(id) {
		return ErrInvalidChoice
	}
	if err := s.Validate(ctx, id); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Root, ".workspace-choice-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.WriteString(tmp, id+"\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, s.path()); err != nil {
		return err
	}
	return storagepath.SyncParent(s.path())
}

// Resolve returns the selected database path for the next launch. An invalid
// selection must fail loudly instead of silently opening the default store.
// RU: Сломанный выбор не приводит к запуску пустой базы.
func (s *Service) Resolve() (string, bool, error) {
	id, err := s.Current()
	if err != nil {
		return "", false, err
	}
	if id == Default {
		return filepath.Join(s.Root, "thesisflow.sqlite"), false, nil
	}
	dir, err := s.Candidate(id)
	if err != nil {
		return "", false, err
	}
	return filepath.Join(dir, "thesisflow.sqlite"), true, nil
}

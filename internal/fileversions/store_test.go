package fileversions

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// testVersionStore initializes a full minimal workspace for file-version tests.
// RU: testVersionStore готовит временную SQLite и хранилище версий файлов.
func testVersionStore(t *testing.T) (*Store, int64, int64) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	g := groups.NewRepository(db)
	s := students.NewRepository(db)
	req := requirements.NewRepository(db)
	m := materials.NewRepository(db)
	for _, init := range []func(context.Context) error{g.Init, s.Init, req.Init, m.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	group, err := g.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := s.Create(ctx, group.ID, "ЗК-001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	required, err := req.Create(ctx, group.ID, 0, "Глава 1", "")
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db, filepath.Join(root, "storage"))
	if err := store.Init(ctx); err != nil {
		t.Fatal(err)
	}
	return store, student.ID, required.ID
}

// TestSaveStoresMultipleDistinctVersions verifies that a new upload keeps the
// earlier stored bytes and assigns a separate immutable version identifier.
// RU: Проверяет сохранность старых файлов при загрузке новых версий.
func TestSaveStoresMultipleDistinctVersions(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	ctx := context.Background()
	first, err := store.Save(ctx, studentID, requirementID, "Первый вариант.docx", []byte("draft one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Save(ctx, studentID, requirementID, "Второй вариант.docx", []byte("draft two"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.RelativePath == second.RelativePath {
		t.Errorf("versions are not distinct: %+v %+v", first, second)
	}
	versions, err := store.List(ctx, studentID, requirementID)
	if err != nil || len(versions) != 2 {
		t.Fatalf("versions=%+v err=%v", versions, err)
	}
	for _, tc := range []struct {
		version Version
		data    []byte
	}{{first, []byte("draft one")}, {second, []byte("draft two")}} {
		data, err := store.Read(ctx, tc.version.ID)
		if err != nil || !bytes.Equal(data, tc.data) {
			t.Errorf("Read(%d)=%q,%v", tc.version.ID, data, err)
		}
	}
}

// TestSaveDoesNotTrustOriginalFilename prevents path traversal by upload names.
// Scenario: original filename contains parent-directory components.
// Expected: storage is under internal version root only.
//
// RU: Проверяет запрет обхода каталогов через имя исходного документа.
// Сценарий: имя содержит ../ и пытается выйти из папки.
// Ожидается: содержимое остаётся внутри внутреннего хранилища.
func TestSaveDoesNotTrustOriginalFilename(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	version, err := store.Save(context.Background(), studentID, requirementID, "../../evil.docx", []byte("protected"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(version.RelativePath) || filepath.Clean(version.RelativePath) != version.RelativePath {
		t.Errorf("unsafe path %q", version.RelativePath)
	}
	if _, err := os.Stat(filepath.Join(store.Root, version.RelativePath)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.Root, "..", "evil.docx")); !os.IsNotExist(err) {
		t.Errorf("file escaped storage root: %v", err)
	}
}

// TestSaveRejectsInvalidFilesAndWrongScope ensures files are only accepted for
// applicable student requirements and known document formats.
// RU: Проверяет запрет неподходящих файлов и чужих требований.
func TestSaveRejectsInvalidFilesAndWrongScope(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	cases := []struct {
		student, requirement int64
		name                 string
		data                 []byte
		want                 error
	}{
		{studentID, requirementID, "file.exe", []byte("bad"), ErrInvalidFile},
		{studentID, requirementID, "file.docx", nil, ErrInvalidFile},
		{studentID, 9999, "file.docx", []byte("data"), ErrInvalidScope},
		{9999, requirementID, "file.docx", []byte("data"), ErrInvalidScope},
	}
	for _, tc := range cases {
		if _, err := store.Save(context.Background(), tc.student, tc.requirement, tc.name, tc.data); !errors.Is(err, tc.want) {
			t.Errorf("Save(%q) err=%v want=%v", tc.name, err, tc.want)
		}
	}
}

// TestSaveMovesStatusBackToReviewAfterAcceptance ensures the next revision
// requires re-approval while preserving older accepted bytes and version rows.
// RU: Проверяет возврат к «На проверке» при замене принятого документа.
func TestSaveMovesStatusBackToReviewAfterAcceptance(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	ctx := context.Background()
	old, err := store.Save(ctx, studentID, requirementID, "Глава 1.docx", []byte("accepted draft"))
	if err != nil {
		t.Fatal(err)
	}
	status := materials.NewRepository(store.DB)
	if _, err := status.Set(ctx, studentID, requirementID, materials.Accepted); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, studentID, requirementID, "Глава 1.docx", []byte("new draft")); err != nil {
		t.Fatal(err)
	}
	current, err := status.Get(ctx, studentID, requirementID)
	if err != nil || current.Status != materials.UnderReview {
		t.Errorf("status=%+v error=%v", current, err)
	}
	data, err := store.Read(ctx, old.ID)
	if err != nil || string(data) != "accepted draft" {
		t.Errorf("old accepted bytes=%q error=%v", data, err)
	}
}

// TestSaveRejectsTooLargePayload prevents unbounded memory writes.
// RU: Проверяет максимальный размер загружаемого документа.
func TestSaveRejectsTooLargePayload(t *testing.T) {
	store, studentID, requirementID := testVersionStore(t)
	if _, err := store.Save(context.Background(), studentID, requirementID, "large.pdf", make([]byte, MaxFileSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("large file error = %v", err)
	}
}

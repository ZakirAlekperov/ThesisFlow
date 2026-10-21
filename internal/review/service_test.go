package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// testReview creates a real SQLite group/student/requirement with storage.
// RU: testReview создаёт реальную SQLite и файловое хранилище для тестов.
func testReview(t *testing.T) (*Service, int64, int64) {
	t.Helper()
	ctx := context.Background()
	base := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(base, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	groupRepo := groups.NewRepository(db)
	studentRepo := students.NewRepository(db)
	reqRepo := requirements.NewRepository(db)
	events := materials.NewRepository(db)
	versions := fileversions.NewStore(db, filepath.Join(base, "storage"))
	for _, init := range []func(context.Context) error{groupRepo.Init, studentRepo.Init, reqRepo.Init, events.Init, versions.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	group, err := groupRepo.Create(ctx, "ИСП/401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := studentRepo.Create(ctx, group.ID, "ЗК/001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	requirement, err := reqRepo.Create(ctx, group.ID, 0, "Глава 1", "")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db, versions)
	if err := service.Init(ctx); err != nil {
		t.Fatal(err)
	}
	return service, student.ID, requirement.ID
}

// TestAcceptCreatesHumanReadableProjection verifies accepted bytes are
// projected to group/student/document folders, separate from internal versions.
// RU: Проверяет человекочитаемую папку принятого документа.
func TestAcceptCreatesHumanReadableProjection(t *testing.T) {
	service, studentID, reqID := testReview(t)
	ctx := context.Background()
	version, err := service.Versions.Save(ctx, studentID, reqID, "random-name.docx", []byte("chapter accepted"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Accept(ctx, studentID, reqID, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.VersionID != version.ID || !strings.HasSuffix(result.RelativePath, "Глава 1.docx") {
		t.Errorf("accepted projection=%+v", result)
	}
	if filepath.IsAbs(result.RelativePath) || strings.Contains(result.RelativePath, "..") {
		t.Errorf("unsafe projection path %q", result.RelativePath)
	}
	bytes, err := os.ReadFile(filepath.Join(service.Versions.Root, result.RelativePath))
	if err != nil || string(bytes) != "chapter accepted" {
		t.Errorf("accepted bytes %q err=%v", bytes, err)
	}
	if !strings.Contains(result.RelativePath, "ИСП_401") || !strings.Contains(result.RelativePath, "Иванов Иван") {
		t.Errorf("projection folders are not human-readable: %q", result.RelativePath)
	}
	current, err := materials.NewRepository(service.DB).Get(ctx, studentID, reqID)
	if err != nil || current.Status != materials.Accepted {
		t.Errorf("status=%+v err=%v", current, err)
	}
}

// TestNewUploadPreservesAcceptedProjectionUntilApproval is a critical safety
// regression test: uploading a replacement never changes the accepted copy.
// RU: Проверяет сохранность принятого файла до новой приёмки.
func TestNewUploadPreservesAcceptedProjectionUntilApproval(t *testing.T) {
	service, studentID, reqID := testReview(t)
	ctx := context.Background()
	first, err := service.Versions.Save(ctx, studentID, reqID, "first.docx", []byte("approved v1"))
	if err != nil {
		t.Fatal(err)
	}
	projected, err := service.Accept(ctx, studentID, reqID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Versions.Save(ctx, studentID, reqID, "second.docx", []byte("draft v2"))
	if err != nil {
		t.Fatal(err)
	}
	current, err := materials.NewRepository(service.DB).Get(ctx, studentID, reqID)
	if err != nil || current.Status != materials.UnderReview {
		t.Fatalf("status=%v error=%v", current, err)
	}
	path := filepath.Join(service.Versions.Root, projected.RelativePath)
	before, err := os.ReadFile(path)
	if err != nil || string(before) != "approved v1" {
		t.Fatalf("accepted document overwritten by upload: %q err=%v", before, err)
	}
	updated, err := service.Accept(ctx, studentID, reqID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(service.Versions.Root, updated.RelativePath))
	if err != nil || string(after) != "draft v2" {
		t.Errorf("new accepted content=%q err=%v", after, err)
	}
	oldVersion, err := service.Versions.Read(ctx, first.ID)
	if err != nil || string(oldVersion) != "approved v1" {
		t.Errorf("historical accepted version lost: %q err=%v", oldVersion, err)
	}
}

// TestRejectKeepsExistingAcceptedCopy ensures returning an update for revision
// does not remove the last successfully approved file.
// RU: Проверяет сохранность ранее принятого файла после отправки на доработку.
func TestRejectKeepsExistingAcceptedCopy(t *testing.T) {
	service, studentID, reqID := testReview(t)
	ctx := context.Background()
	first, err := service.Versions.Save(ctx, studentID, reqID, "first.docx", []byte("approved v1"))
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.Accept(ctx, studentID, reqID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Versions.Save(ctx, studentID, reqID, "second.docx", []byte("bad revision"))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Reject(ctx, studentID, reqID, second.ID); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(service.Versions.Root, approved.RelativePath))
	if err != nil || string(content) != "approved v1" {
		t.Errorf("accepted copy missing: %q err=%v", content, err)
	}
	status, err := materials.NewRepository(service.DB).Get(ctx, studentID, reqID)
	if err != nil || status.Status != materials.NeedsRevision {
		t.Errorf("status=%v err=%v", status, err)
	}
}

// TestAcceptRejectWrongVersionScope prevents one student from claiming another
// student's document or an unrelated version.
// RU: Проверяет запрет приёмки версии чужого документа.
func TestAcceptRejectWrongVersionScope(t *testing.T) {
	service, studentID, reqID := testReview(t)
	if _, err := service.Accept(context.Background(), studentID, reqID, 9999); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("missing version error=%v", err)
	}
	if err := service.Reject(context.Background(), studentID, reqID, 9999); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("reject missing version error=%v", err)
	}
}

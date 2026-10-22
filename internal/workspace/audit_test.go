package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/fileversions"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/materials"
	"github.com/ZakirAlekperov/ThesisFlow/internal/requirements"
	"github.com/ZakirAlekperov/ThesisFlow/internal/review"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestCheckDetectsAcceptedAndImmutableCorruption checks actual saved revisions
// and the human-readable accepted projection without modifying any user data.
// RU: Проверяем реальные файлы, недостающие и повреждённые копии.
func TestCheckDetectsAcceptedAndImmutableCorruption(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := filepath.Join(root, "storage")
	g := groups.NewRepository(db)
	st := students.NewRepository(db)
	req := requirements.NewRepository(db)
	mat := materials.NewRepository(db)
	versions := fileversions.NewStore(db, store)
	reviews := review.NewService(db, versions)
	for _, init := range []func(context.Context) error{g.Init, st.Init, req.Init, mat.Init, versions.Init, reviews.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	group, err := g.Create(ctx, "ИСП-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := st.Create(ctx, group.ID, "301001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := req.Create(ctx, group.ID, 0, "Глава 1", "")
	if err != nil {
		t.Fatal(err)
	}
	version, err := versions.Save(ctx, student.ID, rule.ID, "Глава 1.docx", []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := reviews.Accept(ctx, student.ID, rule.ID, version.ID)
	if err != nil {
		t.Fatal(err)
	}

	first, err := Check(ctx, db, store)
	if err != nil || !first.Healthy() || first.VersionsChecked != 1 || first.VersionsIntact != 1 ||
		first.AcceptedChecked != 1 || first.AcceptedIntact != 1 {
		t.Fatalf("healthy report %+v err %v", first, err)
	}
	// Simulate metadata written by an older Windows build. Both the audit
	// and ordinary version reader must accept safe legacy separators.
	// RU: Проверяем перенос без переписывания существующих файлов.
	legacyVersion := strings.ReplaceAll(version.RelativePath, "/", string(rune(92)))
	legacyApproved := strings.ReplaceAll(accepted.RelativePath, "/", string(rune(92)))
	if _, err := db.ExecContext(ctx, "UPDATE file_versions SET relative_path=? WHERE id=?", legacyVersion, version.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE accepted_files SET relative_path=? WHERE student_id=? AND requirement_id=?", legacyApproved, student.ID, rule.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := versions.Read(ctx, version.ID); err != nil {
		t.Fatalf("legacy Windows upload path unreadable: %v", err)
	}
	oldPathAudit, err := Check(ctx, db, store)
	if err != nil || !oldPathAudit.Healthy() {
		t.Fatalf("legacy paths rejected: %+v %v", oldPathAudit, err)
	}
	if _, err := reviews.Accept(ctx, student.ID, rule.ID, version.ID); err != nil {
		t.Fatalf("idempotent approval of legacy path failed: %v", err)
	}
	projection := filepath.Join(store, accepted.RelativePath)
	if err := os.WriteFile(projection, []byte("TAMPERED"), 0600); err != nil {
		t.Fatal(err)
	}
	damaged, err := Check(ctx, db, store)
	if err != nil || damaged.Healthy() || len(damaged.Issues) != 1 || damaged.Issues[0].Code != "accepted_corrupt" {
		t.Fatalf("undetected accepted corruption %+v, %v", damaged, err)
	}
	// The diagnostic must not repair or silently overwrite the damaged file.
	data, err := os.ReadFile(projection)
	if err != nil || string(data) != "TAMPERED" {
		t.Fatalf("read-only audit changed data %q %v", data, err)
	}
	if err := os.Remove(projection); err != nil {
		t.Fatal(err)
	}
	missing, err := Check(ctx, db, store)
	if err != nil || missing.Healthy() || missing.Issues[0].Code != "accepted_missing" {
		t.Fatalf("accepted file lost %+v %v", missing, err)
	}
	source := filepath.Join(store, version.RelativePath)
	if err := os.WriteFile(source, []byte("WRONG"), 0600); err != nil {
		t.Fatal(err)
	}
	corrupt, err := Check(ctx, db, store)
	if err != nil || corrupt.Healthy() || corrupt.VersionsIntact != 0 {
		t.Fatalf("immutable file corruption missed %+v %v", corrupt, err)
	}
}

func TestCheckFlagsCrossScopeAndUnsafePath(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := t.TempDir()
	g := groups.NewRepository(db)
	s := students.NewRepository(db)
	req := requirements.NewRepository(db)
	m := materials.NewRepository(db)
	v := fileversions.NewStore(db, root)
	rev := review.NewService(db, v)
	for _, init := range []func(context.Context) error{g.Init, s.Init, req.Init, m.Init, v.Init, rev.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	group, err := g.Create(ctx, "ИС-401")
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Create(ctx, group.ID, "300001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Create(ctx, group.ID, "300002", "Петров Пётр")
	if err != nil {
		t.Fatal(err)
	}
	rule, err := req.Create(ctx, group.ID, 0, "Введение", "")
	if err != nil {
		t.Fatal(err)
	}
	upload, err := v.Save(ctx, first.ID, rule.ID, "Введение.txt", []byte("text"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO accepted_files(student_id,requirement_id,version_id,relative_path,accepted_at)
 VALUES(?,?,?,?,?)`, other.ID, rule.ID, upload.ID, "../../bad.txt", "2026-10-22T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	report, err := Check(ctx, db, root)
	if err != nil || report.Healthy() || report.Issues[0].Code != "accepted_scope" {
		t.Fatalf("accepted foreign version %+v %v", report, err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE accepted_files SET student_id=?,relative_path=? WHERE version_id=?",
		first.ID, "../evil", upload.ID); err != nil {
		t.Fatal(err)
	}
	report, err = Check(ctx, db, root)
	if err != nil || report.Healthy() {
		t.Fatalf("unsafe projection %+v %v", report, err)
	}
	found := false
	for _, item := range report.Issues {
		if strings.HasPrefix(item.Code, "accepted_unsafe") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unsafe path not detected: %+v", report)
	}
}

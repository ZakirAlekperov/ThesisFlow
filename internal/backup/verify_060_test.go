package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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

func reviewBackupFixture(t *testing.T) (string, string, *review.Service, int64, int64, int64) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	db, err := sqlite.Open(ctx, filepath.Join(root, "source", "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gs := groups.NewRepository(db)
	people := students.NewRepository(db)
	req := requirements.NewRepository(db)
	statuses := materials.NewRepository(db)
	versions := fileversions.NewStore(db, filepath.Join(root, "source", "storage"))
	checks := review.NewService(db, versions)
	for _, init := range []func(context.Context) error{gs.Init, people.Init, req.Init, statuses.Init, versions.Init, checks.Init} {
		if err := init(ctx); err != nil {
			t.Fatal(err)
		}
	}
	g, err := gs.Create(ctx, "ИС-401")
	if err != nil {
		t.Fatal(err)
	}
	student, err := people.Create(ctx, g.ID, "301001", "Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	required, err := req.Create(ctx, g.ID, 0, "Глава 1", "")
	if err != nil {
		t.Fatal(err)
	}
	version, err := versions.Save(ctx, student.ID, required.ID, "Глава 1.docx", []byte("a real saved version"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checks.Accept(ctx, student.ID, required.ID, version.ID); err != nil {
		t.Fatal(err)
	}
	return root, filepath.Join(root, "source", "storage"), checks, student.ID, required.ID, version.ID
}

// TestRestoreAndVerifyRejectSemanticallyCorruptZIP rehashes a broken accepted
// projection inside a perfectly self-consistent ZIP manifest.
// RU: Корректный ZIP с повреждённой связью файлов всё равно отклоняется.
func TestRestoreAndVerifyRejectSemanticallyCorruptZIP(t *testing.T) {
	root, storage, svc, studentID, requirementID, _ := reviewBackupFixture(t)
	ctx := context.Background()
	// An older Windows database may contain '\\' in relative_path.
	// The ZIP's physical paths are canonical '/' names; restored SQL links
	// must still be valid on every supported OS.
	var sourceRelative, acceptedRelative string
	if err := svc.DB.QueryRowContext(ctx, "SELECT relative_path FROM file_versions LIMIT 1").Scan(&sourceRelative); err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.QueryRowContext(ctx, "SELECT relative_path FROM accepted_files LIMIT 1").Scan(&acceptedRelative); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB.ExecContext(ctx, "UPDATE file_versions SET relative_path=?", strings.ReplaceAll(sourceRelative, "/", string(rune(92)))); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DB.ExecContext(ctx, "UPDATE accepted_files SET relative_path=?", strings.ReplaceAll(acceptedRelative, "/", string(rune(92)))); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(root, "good.zip")
	if err := Export(ctx, svc.DB, storage, good); err != nil {
		t.Fatal(err)
	}
	if err := Verify(good); err != nil {
		t.Fatalf("healthy backup rejected: %v", err)
	}
	destination := filepath.Join(root, "restored")
	if err := Restore(good, destination); err != nil {
		t.Fatalf("restore healthy ZIP: %v", err)
	}

	var relative string
	if err := svc.DB.QueryRowContext(ctx,
		"SELECT relative_path FROM accepted_files WHERE student_id=? AND requirement_id=?",
		studentID, requirementID).Scan(&relative); err != nil {
		t.Fatal(err)
	}
	// Export now rejects corrupted source workspaces. Instead mutate an
	// otherwise good ZIP and update its manifest hash; semantic validation
	// must still reject the accepted file that disagrees with SQLite.
	// RU: Байт испорчен в архиве при пересчитанном ZIP-манифесте.
	broken := filepath.Join(root, "damaged.zip")
	repackWithAlteredProjection(t, good, broken, "storage/"+strings.ReplaceAll(relative, "\\", "/"), []byte("wrong bytes"))
	if err := Verify(broken); !errors.Is(err, ErrUnsafeArchive) {
		t.Errorf("verified a semantically broken backup: %v", err)
	}
	unsafeDest := filepath.Join(root, "wrongly-restored")
	if err := Restore(broken, unsafeDest); !errors.Is(err, ErrUnsafeArchive) {
		t.Errorf("restored a broken backup: %v", err)
	}
	if _, err := os.Stat(unsafeDest); !os.IsNotExist(err) {
		t.Errorf("corrupt ZIP published a destination: %v", err)
	}
}

func TestVerifyRejectsFutureSQLiteSchema(t *testing.T) {
	ctx := context.Background()
	root, storage, svc, _, _, _ := reviewBackupFixture(t)
	if _, err := svc.DB.ExecContext(ctx, "PRAGMA user_version = 500"); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "future.zip")
	if err := Export(ctx, svc.DB, storage, archive); err != nil {
		t.Fatal(err)
	}
	if err := Verify(archive); !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("future schema accepted: %v", err)
	}
}

func TestRestoreRejectsCaseCollisionsAndWindowsReservedNames(t *testing.T) {
	for _, names := range [][]string{
		{"storage/Chapter.docx", "storage/chapter.docx"},
		{"storage/CON.txt"},
		{"storage/LPT1.pdf"},
		{"storage/Chapter. "},
	} {
		root := t.TempDir()
		archive := filepath.Join(root, "case.zip")
		file, err := os.Create(archive)
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(file)
		for _, name := range names {
			w, err := z.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("payload")); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		err = Restore(archive, filepath.Join(root, "unsafe-target"))
		if !errors.Is(err, ErrUnsafeArchive) {
			t.Errorf("accepted ambiguous or reserved archive %v: %v", names, err)
		}
	}
}

// repackWithAlteredProjection creates a syntactically correct manifest v1 ZIP
// that contains a corrupt accepted projection. Only tests fabricate this.
// RU: Перепаковываем тестовый ZIP, сверяя новые хеши манифеста.
func repackWithAlteredProjection(t *testing.T, source, destination, filename string, content []byte) {
	t.Helper()
	original, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	data := make(map[string][]byte)
	var names []string
	for _, entry := range original.File {
		handle, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		bytes, err := io.ReadAll(handle)
		closeErr := handle.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read ZIP entry %s: %v %v", entry.Name, err, closeErr)
		}
		data[entry.Name] = bytes
		names = append(names, entry.Name)
	}
	if _, ok := data[filename]; !ok {
		t.Fatalf("projection not in ZIP: %q", filename)
	}
	data[filename] = content
	var metadata manifest
	if err := json.Unmarshal(data["manifest.json"], &metadata); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	metadata.Files[filename] = hex.EncodeToString(sum[:])
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	data["manifest.json"] = encoded
	target, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(target)
	for _, name := range names {
		stream, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stream.Write(data[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
}

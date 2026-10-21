package backup

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestExportRestoreRoundTrip verifies recovery of SQLite and accepted/internal
// files together in an independent workspace.
// Scenario: write a record and a document, export ZIP, restore elsewhere.
// Expected: both the database record and exact file bytes survive.
//
// RU: Проверяет восстановление SQLite и пользовательских файлов вместе.
// Сценарий: создать запись и файл, экспортировать ZIP, восстановить в новый каталог.
// Ожидается: запись базы и байты документа не изменились.
func TestExportRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "source", "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE sample (value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sample(value) VALUES (?)", "Глава 1"); err != nil {
		t.Fatal(err)
	}

	storage := filepath.Join(root, "source", "storage")
	if err := os.MkdirAll(filepath.Join(storage, "accepted"), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("test thesis document\n")
	if err := os.WriteFile(filepath.Join(storage, "accepted", "Глава 1.docx"), original, 0600); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(root, "snapshot.zip")
	if err := Export(ctx, db, storage, archive); err != nil {
		t.Fatalf("Export: %v", err)
	}
	dest := filepath.Join(root, "restored")
	if err := Restore(archive, dest); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	restoredDB, err := sqlite.Open(ctx, filepath.Join(dest, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	var value string
	if err := restoredDB.QueryRow("SELECT value FROM sample").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "Глава 1" {
		t.Errorf("restored row = %q", value)
	}
	got, err := os.ReadFile(filepath.Join(dest, "storage", "accepted", "Глава 1.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("restored content = %q", got)
	}
}

// TestExportRejectsExistingArchive checks that backups never silently overwrite.
// Scenario: output already contains user data.
// Expected: ErrDestinationExists and the existing bytes remain unchanged.
//
// RU: Проверяет запрет перезаписи существующего архива.
// Сценарий: путь назначения уже занят пользовательским файлом.
// Ожидается: ErrDestinationExists и сохранность исходных байтов.
func TestExportRejectsExistingArchive(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "snapshot.zip")
	if err := os.WriteFile(path, []byte("valuable"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(context.Background(), filepath.Join(root, "thesisflow.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Export(context.Background(), db, filepath.Join(root, "storage"), path); !errors.Is(err, ErrDestinationExists) {
		t.Errorf("Export error = %v, want ErrDestinationExists", err)
	}
	value, _ := os.ReadFile(path)
	if string(value) != "valuable" {
		t.Errorf("existing archive overwritten: %q", value)
	}
}

// TestRestoreRejectsExistingWorkspace protects live data on restoration.
// Scenario: destination already exists and contains a sentinel file.
// Expected: ErrDestinationExists and no changes to existing data.
//
// RU: Проверяет отказ восстановления поверх рабочего каталога.
// Сценарий: назначение уже содержит файлы пользователя.
// Ожидается: ErrDestinationExists и сохранность файлов.
func TestRestoreRejectsExistingWorkspace(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "existing")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "sentinel"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(filepath.Join(root, "missing.zip"), dest); !errors.Is(err, ErrDestinationExists) {
		t.Errorf("Restore error = %v, want ErrDestinationExists", err)
	}
	data, _ := os.ReadFile(filepath.Join(dest, "sentinel"))
	if string(data) != "keep" {
		t.Errorf("existing workspace changed: %q", data)
	}
}

// TestRestoreRejectsTraversalPaths prevents a crafted archive from writing
// outside the requested destination.
// Scenario: ZIP contains storage/../../escaped.txt.
// Expected: ErrUnsafeArchive and no escaped file is created.
//
// RU: Проверяет защиту от выхода за пределы каталога при распаковке.
// Сценарий: ZIP содержит путь с переходом через ../.
// Ожидается: ErrUnsafeArchive и отсутствие постороннего файла.
func TestRestoreRejectsTraversalPaths(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "attack.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("storage/../../escaped.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "recovery")
	if err := Restore(archive, dest); !errors.Is(err, ErrUnsafeArchive) {
		t.Errorf("Restore error = %v, want ErrUnsafeArchive", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escaped.txt")); !os.IsNotExist(err) {
		t.Errorf("traversal wrote outside destination: %v", err)
	}
}

// TestRestoreRejectsCorruptedManifest checks that damaged backup contents
// never become a partly restored workspace.
// Scenario: ZIP has a database entry but lacks the required manifest.
// Expected: failed restore and no destination directory.
//
// RU: Проверяет целостность резервной копии.
// Сценарий: архив содержит базу данных, но не имеет манифеста.
// Ожидается: ошибка и отсутствие частично восстановленной рабочей папки.
func TestRestoreRejectsCorruptedManifest(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	x, err := w.Create("thesisflow.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x.Write([]byte("not valid SQLite")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "recover")
	if err := Restore(archive, target); err == nil {
		t.Fatal("Restore accepted archive without manifest")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("partly restored target: %v", err)
	}
}

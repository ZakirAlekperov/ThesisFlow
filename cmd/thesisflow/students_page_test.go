package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestStartServesPersistedStudentsByGroup verifies the complete SQLite-to-HTTP
// path after reopening an existing database.
// Scenario: save students in two groups, restart the app and GET /groups/{id}.
// Expected: only the selected group's persisted students are shown.
//
// RU: Проверяет чтение студентов из SQLite через настоящий HTTP-сервер.
// Сценарий: сохранить студентов двух групп и запустить приложение повторно.
// Ожидается: страница группы показывает только своих студентов.
func TestStartServesPersistedStudentsByGroup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	groupRepo := groups.NewRepository(db)
	if err := groupRepo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("init groups: %v", err)
	}
	studentRepo := students.NewRepository(db)
	if err := studentRepo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("init students: %v", err)
	}
	first, err := groupRepo.Create(ctx, "ИСП-401")
	if err != nil {
		db.Close()
		t.Fatalf("create group: %v", err)
	}
	other, err := groupRepo.Create(ctx, "ПР-301")
	if err != nil {
		db.Close()
		t.Fatalf("create other group: %v", err)
	}
	saved, err := studentRepo.Create(ctx, first.ID, "ЗК-001", "Иванов Иван")
	if err != nil {
		db.Close()
		t.Fatalf("create student: %v", err)
	}
	if _, err := studentRepo.Create(ctx, other.ID, "ЗК-002", "Петров Пётр"); err != nil {
		db.Close()
		t.Fatalf("create other student: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close SQLite: %v", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var body string
	var requestErr error
	err = start(runCtx, io.Discard, "0", path, func(address string) error {
		defer cancel()
		client := &http.Client{Timeout: 3 * time.Second}
		var response *http.Response
		response, requestErr = client.Get(fmt.Sprintf("%s/groups/%d", address, first.ID))
		if requestErr != nil {
			return nil
		}
		defer response.Body.Close()
		var contents []byte
		contents, requestErr = io.ReadAll(response.Body)
		if requestErr != nil {
			return nil
		}
		if response.StatusCode != http.StatusOK {
			requestErr = fmt.Errorf("GET /groups/%d: HTTP %d", first.ID, response.StatusCode)
		}
		body = string(contents)
		return nil
	})
	if err != nil {
		t.Fatalf("start HTTP: %v", err)
	}
	if requestErr != nil {
		t.Fatalf("request student page: %v", requestErr)
	}
	for _, want := range []string{first.Name, saved.FullName, saved.EducationalID, fmt.Sprintf(`data-student-id="%d"`, saved.ID)} {
		if !strings.Contains(body, want) {
			t.Errorf("missing persisted student field %q in HTML: %q", want, body)
		}
	}
	if strings.Contains(body, "Петров Пётр") || strings.Contains(body, "ЗК-002") {
		t.Errorf("another group's student leaked into page: %q", body)
	}
}

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
)

// TestStartServesGroupsFromSQLite verifies the full path from persistent storage
// through HTTP and ensures the page uses the already initialized database.
// Scenario: save a group, restart the app and request the page over localhost.
// Expected: GET / contains the saved group and its permanent database ID.
//
// RU: Проверяет весь путь данных из постоянной SQLite в HTTP-страницу.
// Сценарий: сохранить группу, заново запустить сервер и выполнить GET /.
// Ожидается: страница содержит название группы и её постоянный ID.
func TestStartServesGroupsFromSQLite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	repo := groups.NewRepository(db)
	if err := repo.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("initialize groups: %v", err)
	}
	group, err := repo.Create(ctx, "ИСП-401")
	if err != nil {
		db.Close()
		t.Fatalf("save group: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database before restart: %v", err)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	var page string
	var requestErr error

	err = start(runCtx, io.Discard, "0", path, func(address string) error {
		defer cancel()
		var response *http.Response
		response, requestErr = client.Get(address + "/")
		if requestErr != nil {
			return nil
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			requestErr = fmt.Errorf("GET / returned HTTP %d", response.StatusCode)
			return nil
		}
		var body []byte
		body, requestErr = io.ReadAll(response.Body)
		page = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	if requestErr != nil {
		t.Fatalf("request homepage: %v", requestErr)
	}
	if !strings.Contains(page, group.Name) || !strings.Contains(page, fmt.Sprintf("data-group-id=\"%d\"", group.ID)) {
		t.Errorf("persisted group not rendered on homepage: %q", page)
	}
}

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
)

// TestStartCreatesGroupThroughBrowser verifies real HTTP-to-SQLite persistence.
// Scenario: start a fresh workspace, load the form, submit a new group and exit.
// Expected: POST redirects, the refreshed page contains the group, and the
// exact record remains in SQLite after server shutdown.
//
// RU: Проверяет настоящее создание группы через HTTP и сохранение в SQLite.
// Сценарий: новый каталог, загрузка формы, отправка группы и остановка.
// Ожидается: перенаправление после POST, группа на странице и в базе после
// завершения сервера.
func TestStartCreatesGroupThroughBrowser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	client := &http.Client{Timeout: 3 * time.Second}
	var requestErr error
	var afterCreate string

	err := start(ctx, io.Discard, "0", path, func(address string) error {
		defer cancel()
		var response *http.Response
		response, requestErr = client.Get(address + "/")
		if requestErr != nil {
			return nil
		}
		page, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			requestErr = readErr
			return nil
		}
		match := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(string(page))
		if len(match) != 2 {
			requestErr = fmt.Errorf("csrf token missing from homepage")
			return nil
		}

		form := url.Values{"name": {"ИСП-401"}, "csrf_token": {match[1]}}
		response, requestErr = client.PostForm(address+"/groups", form)
		if requestErr != nil {
			return nil
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			requestErr = fmt.Errorf("POST and redirect status = %d, want 200", response.StatusCode)
			return nil
		}
		contents, err := io.ReadAll(response.Body)
		if err != nil {
			requestErr = err
			return nil
		}
		afterCreate = string(contents)
		return nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if requestErr != nil {
		t.Fatalf("HTTP form flow: %v", requestErr)
	}
	if !strings.Contains(afterCreate, "ИСП-401") {
		t.Errorf("created group missing from refreshed page: %q", afterCreate)
	}

	db, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen SQLite: %v", err)
	}
	defer db.Close()
	saved, err := groups.NewRepository(db).List(context.Background())
	if err != nil {
		t.Fatalf("list persisted groups: %v", err)
	}
	if len(saved) != 1 || saved[0].ID <= 0 || saved[0].Name != "ИСП-401" {
		t.Errorf("persisted groups = %#v, want one group with stable ID", saved)
	}
}

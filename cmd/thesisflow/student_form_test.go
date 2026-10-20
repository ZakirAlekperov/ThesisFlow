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
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
)

// TestStartCreatesStudentFromGroupPage exercises the complete SQLite-backed
// browser flow, including a redirect and persistent storage after shutdown.
// Scenario: save a group, start ThesisFlow, POST the student form and reload.
// Expected: student appears in the correct group and survives restarting.
//
// RU: Проверяет создание студента через настоящую HTTP-форму и SQLite.
// Сценарий: сохранить группу, запустить приложение, отправить форму и прочитать
// страницу снова. Ожидается: студент виден в группе и остаётся после остановки.
func TestStartCreatesStudentFromGroupPage(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "thesisflow.sqlite")
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	groupRepository := groups.NewRepository(db)
	if err := groupRepository.Init(ctx); err != nil {
		db.Close()
		t.Fatalf("init groups: %v", err)
	}
	group, err := groupRepository.Create(ctx, "ИСП-401")
	if err != nil {
		db.Close()
		t.Fatalf("save group: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close original database: %v", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var page string
	var requestErr error

	err = start(runCtx, io.Discard, "0", path, func(address string) error {
		defer cancel()
		client := &http.Client{Timeout: 3 * time.Second}
		groupURL := fmt.Sprintf("%s/groups/%d", address, group.ID)
		response, err := client.Get(groupURL)
		if err != nil {
			requestErr = err
			return nil
		}
		data, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			requestErr = err
			return nil
		}
		token := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`).FindStringSubmatch(string(data))
		if len(token) != 2 {
			requestErr = fmt.Errorf("student form CSRF token is missing")
			return nil
		}
		response, err = client.PostForm(groupURL+"/students", url.Values{
			"full_name":      {"Иванов Иван"},
			"educational_id": {"ЗК-0017"},
			"csrf_token":     {token[1]},
		})
		if err != nil {
			requestErr = err
			return nil
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			requestErr = fmt.Errorf("POST redirect finished with HTTP %d", response.StatusCode)
			return nil
		}
		data, requestErr = io.ReadAll(response.Body)
		page = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if requestErr != nil {
		t.Fatalf("HTTP student form: %v", requestErr)
	}
	if !strings.Contains(page, "Иванов Иван") || !strings.Contains(page, "ЗК-0017") {
		t.Errorf("student absent from refreshed group page: %q", page)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen SQLite: %v", err)
	}
	defer reopened.Close()
	items, err := students.NewRepository(reopened).ListByGroup(ctx, group.ID)
	if err != nil {
		t.Fatalf("read persisted student: %v", err)
	}
	if len(items) != 1 || items[0].FullName != "Иванов Иван" || items[0].EducationalID != "ЗК-0017" || items[0].GroupID != group.ID || items[0].ID <= 0 {
		t.Errorf("persisted students = %#v, want one correctly assigned student", items)
	}
}

package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionReportsReleaseCandidate verifies that installed builds can
// identify their declared version without opening a database or browser.
// Scenario: invoke the version-reporting helper.
// Expected: it prints the release identifier and nothing else.
//
// RU: Проверяет отображение версии без запуска сервера и открытия SQLite.
// Сценарий: вызвать команду версии.
// Ожидается: строка с номером версии без побочных действий.
func TestVersionReportsReleaseCandidate(t *testing.T) {
	var output bytes.Buffer
	if err := writeVersion(&output); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if got := strings.TrimSpace(output.String()); got != "ThesisFlow 0.1.10" {
		t.Errorf("version = %q, want ThesisFlow 0.1.10", got)
	}
}

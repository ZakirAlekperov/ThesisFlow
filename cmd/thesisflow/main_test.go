package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// signalWriter communicates the announced URL to the test without sharing
// a mutable buffer across goroutines.
// RU: signalWriter передаёт тесту объявленный адрес без одновременного
// доступа нескольких горутин к изменяемому буферу.
type signalWriter struct {
	messages chan string
}

func (w signalWriter) Write(p []byte) (int, error) {
	w.messages <- string(p)
	return len(p), nil
}

// TestRunStartsLoopbackHTTPAndOpensBrowser verifies local access and opening.
// Scenario: a free port and an injected browser opener that records the URL.
// Expected: one opener call with the loopback URL; GET / works; shutdown succeeds.
//
// RU: Проверяет доступность сервера и передачу адреса в браузер.
// Сценарий: свободный порт и тестовый обработчик открытия браузера.
// Ожидается: один вызов с локальным URL, успешный GET / и остановка.
func TestRunStartsLoopbackHTTPAndOpensBrowser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := signalWriter{messages: make(chan string, 1)}
	opened := make(chan string, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- run(ctx, writer, "0", func(address string) error {
			opened <- address
			return nil
		})
	}()

	var announced string
	select {
	case announced = <-writer.messages:
	case err := <-finished:
		t.Fatalf("server finished before listening: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("server did not announce its listening URL")
	}

	const prefix = "ThesisFlow: "
	if !strings.HasPrefix(announced, prefix) {
		t.Fatalf("announcement = %q, want prefix %q", announced, prefix)
	}
	address := strings.TrimSpace(strings.TrimPrefix(announced, prefix))
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("parse address %q: %v", address, err)
	}
	if parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		t.Fatalf("listening URL = %q, want HTTP bound to 127.0.0.1", address)
	}

	select {
	case got := <-opened:
		if got != address {
			t.Errorf("browser URL = %q, want %q", got, address)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("browser opener was not called")
	}
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(address + "/")
	if err != nil {
		t.Fatalf("GET homepage: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("GET homepage status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("run after cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down after cancellation")
	}
}

// TestRunFailsOnOccupiedPort verifies that a second instance does not silently
// bind to a different interface or hide a bind failure.
// Scenario: keep a loopback port busy and attempt to start ThesisFlow on it.
// Expected: run returns an error describing a failed server start.
//
// RU: Проверяет обработку занятого порта без перехода на другой интерфейс.
// Сценарий: запущен слушатель на loopback-порту, ThesisFlow использует тот же порт.
// Ожидается: run возвращает ошибку запуска сервера.
func TestRunFailsOnOccupiedPort(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer occupied.Close()
	port := fmt.Sprint(occupied.Addr().(*net.TCPAddr).Port)
	if err := run(context.Background(), io.Discard, port, func(string) error {
		t.Error("browser called on occupied port")
		return nil
	}); err == nil {
		t.Fatal("run on occupied port: want a bind error")
	}
}

// errWriteFailed marks the I/O error produced by the test double.
// RU: errWriteFailed — ошибка записи тестового имитатора.
var errWriteFailed = errors.New("write failed")

// failingWriter simulates an output stream that rejects writes.
// RU: failingWriter имитирует поток вывода, который отклоняет запись.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

// TestRunReturnsOutputError verifies that a failed startup announcement is
// reported instead of being ignored.
// Scenario: run gets a writer that returns errWriteFailed.
// Expected: errors.Is recognizes the original write failure.
//
// RU: Проверяет, что ошибка вывода адреса сервера не игнорируется.
// Сценарий: run получает поток, возвращающий errWriteFailed.
// Ожидается: errors.Is обнаруживает исходную ошибку записи.
func TestRunReturnsOutputError(t *testing.T) {
	if err := run(context.Background(), failingWriter{}, "0", func(string) error {
		t.Error("browser called after failed output")
		return nil
	}); !errors.Is(err, errWriteFailed) {
		t.Fatalf("run() error = %v, want %v", err, errWriteFailed)
	}
}

// TestRunKeepsServerAliveWhenBrowserFails verifies a nonfatal browser error.
// Scenario: the opener reports that the desktop environment is unavailable.
// Expected: a warning and a normal shutdown rather than an application error.
//
// RU: Проверяет безопасную обработку ошибки запуска браузера.
// Сценарий: обработчик возвращает ошибку отсутствия графической среды.
// Ожидается: предупреждение и штатная остановка сервера.
func TestRunKeepsServerAliveWhenBrowserFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var output bytes.Buffer
	browserError := errors.New("desktop unavailable")
	err := run(ctx, &output, "0", func(string) error {
		cancel()
		return browserError
	})
	if err != nil {
		t.Fatalf("run() = %v, want nil", err)
	}
	for _, want := range []string{"ThesisFlow: http://127.0.0.1:", "Browser was not opened", browserError.Error()} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("output %q missing %q", output.String(), want)
		}
	}
}

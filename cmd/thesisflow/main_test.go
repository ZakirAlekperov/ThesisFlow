package main

import (
	"bytes"
	"errors"
	"testing"
)

// TestRunWritesApplicationName verifies the successful startup-output contract.
// Scenario: run receives a writable in-memory buffer.
// Expected: no error and exactly "ThesisFlow\n" in the output.
//
// RU: Проверяет контракт успешного вывода сообщения при запуске.
// Сценарий: run получает доступный для записи буфер в памяти.
// Ожидается: ошибка отсутствует, вывод строго равен "ThesisFlow\n".
func TestRunWritesApplicationName(t *testing.T) {
	var output bytes.Buffer

	if err := run(&output); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	const want = "ThesisFlow\n"
	if got := output.String(); got != want {
		t.Errorf("run() output = %q, want %q", got, want)
	}
}

// errWriteFailed is the expected I/O failure used by the test double.
// RU: Ошибка записи, которую должен вернуть тестовый имитатор.
var errWriteFailed = errors.New("write failed")

// failingWriter simulates an output stream that cannot accept data.
// RU: Имитирует поток вывода, в который невозможно записать данные.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

// TestRunReturnsOutputError verifies that run does not hide an I/O failure.
// Scenario: the output writer rejects a write with errWriteFailed.
// Expected: run returns that same error (checked via errors.Is).
//
// RU: Проверяет, что run не скрывает ошибку записи.
// Сценарий: поток отклоняет запись с ошибкой errWriteFailed.
// Ожидается: run возвращает эту же ошибку (проверка через errors.Is).
func TestRunReturnsOutputError(t *testing.T) {
	if err := run(failingWriter{}); !errors.Is(err, errWriteFailed) {
		t.Fatalf("run() error = %v, want %v", err, errWriteFailed)
	}
}

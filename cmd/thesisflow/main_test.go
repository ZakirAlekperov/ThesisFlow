package main

import (
	"bytes"
	"errors"
	"testing"
)

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

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunReturnsOutputError(t *testing.T) {
	if err := run(failingWriter{}); err == nil {
		t.Fatal("run() error = nil, want write error")
	}
}

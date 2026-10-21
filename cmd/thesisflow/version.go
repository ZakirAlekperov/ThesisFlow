package main

import (
	"fmt"
	"io"
)

// Version identifies this prerelease code line and portable binary.
// RU: Version обозначает текущую предварительную версию приложения.
const Version = "0.1.10"

// writeVersion prints the build's human-readable version without reading
// user files, opening SQLite, or starting the HTTP server.
//
// RU: writeVersion печатает версию без доступа к пользовательским файлам,
// запуска SQLite или открытия HTTP-сервера.
func writeVersion(output io.Writer) error {
	_, err := fmt.Fprintf(output, "ThesisFlow %s\n", Version)
	return err
}

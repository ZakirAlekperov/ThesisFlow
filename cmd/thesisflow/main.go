// ThesisFlow is a local application for managing graduation thesis materials.
// ThesisFlow — локальное приложение для учёта материалов выпускных работ.
package main

import (
	"fmt"
	"io"
	"os"
)

// run writes the startup message to the given output.
// run записывает сообщение о запуске в указанный поток вывода.
func run(output io.Writer) error {
	_, err := fmt.Fprintln(output, "ThesisFlow")
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

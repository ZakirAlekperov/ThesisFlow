// Package browser starts the system default browser for the local web interface.
// Пакет browser открывает локальный веб-интерфейс в браузере по умолчанию.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// commandForOS selects a browser command without invoking a command shell.
// The URL is passed as a separate argument to avoid shell interpretation.
//
// RU: commandForOS выбирает команду открытия браузера без командной оболочки.
// URL передаётся отдельным аргументом, без интерпретации оболочкой.
func commandForOS(goos, address string) (string, []string, error) {
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", address}, nil
	case "darwin":
		return "open", []string{address}, nil
	case "linux":
		return "xdg-open", []string{address}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operating system: %s", goos)
	}
}

// Open attempts to launch the system default browser without waiting for it to
// close. A launch error is returned to the caller; it must not stop the server.
//
// RU: Open пытается запустить браузер по умолчанию, не дожидаясь его закрытия.
// Ошибка запуска возвращается вызывающей стороне и не должна останавливать сервер.
func Open(address string) error {
	program, args, err := commandForOS(runtime.GOOS, address)
	if err != nil {
		return err
	}
	cmd := exec.Command(program, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start default browser: %w", err)
	}
	// Reap the child process even if the platform launcher stays active.
	// RU: Дожидаемся завершения вспомогательного процесса без блокировки сервера.
	go func() { _ = cmd.Wait() }()
	return nil
}

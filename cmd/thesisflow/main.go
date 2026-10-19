// ThesisFlow is a local application for managing graduation thesis materials.
// ThesisFlow — локальное приложение для учёта материалов выпускных работ.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/web"
)

const localPort = "8080"

// run starts the HTTP interface on the loopback address and waits until the
// context is canceled. Port "0" selects a free ephemeral port for tests.
// The application reports the listening URL and propagates startup I/O errors.
//
// RU: run запускает HTTP-интерфейс только на loopback-адресе и работает до
// отмены контекста. Порт "0" позволяет тестам выбрать свободный порт.
// Функция выводит адрес сервера и возвращает ошибки запуска или вывода.
func run(ctx context.Context, output io.Writer, port string) error {
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return fmt.Errorf("listen on localhost: %w", err)
	}
	defer listener.Close()

	address := "http://" + listener.Addr().String()
	if _, err := fmt.Fprintln(output, "ThesisFlow: "+address); err != nil {
		return fmt.Errorf("announce local server: %w", err)
	}

	server := &http.Server{
		Handler:           web.NewHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("serve local HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		// Closing the listener also handles cancellation before Serve registers it.
		// RU: Закрытие слушателя защищает от отмены до запуска Serve.
		_ = listener.Close()
		serveErr := <-serveResult
		if shutdownErr != nil {
			return fmt.Errorf("shutdown local HTTP: %w", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, net.ErrClosed) {
			return fmt.Errorf("serve local HTTP: %w", serveErr)
		}
		return nil
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Stdout, localPort); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

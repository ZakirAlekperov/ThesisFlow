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
	"path/filepath"
	"syscall"
	"time"

	"github.com/ZakirAlekperov/ThesisFlow/internal/browser"
	"github.com/ZakirAlekperov/ThesisFlow/internal/groups"
	"github.com/ZakirAlekperov/ThesisFlow/internal/storage/sqlite"
	"github.com/ZakirAlekperov/ThesisFlow/internal/students"
	"github.com/ZakirAlekperov/ThesisFlow/internal/web"
)

// localPort asks the OS to choose a free local port.
// RU: ОС автоматически выбирает свободный локальный порт.
const localPort = "0"

// run starts the HTTP interface on the loopback address and waits until the
// context is canceled. Port "0" selects a free ephemeral port for tests.
// The injected handler serves local requests. The application reports the URL, attempts to open a browser,
// and propagates startup errors. Browser launch failures are nonfatal.
//
// RU: run запускает HTTP-интерфейс только на loopback-адресе и работает до
// отмены контекста. Порт "0" позволяет тестам выбрать свободный порт.
// Встроенный HTTP-обработчик обслуживает запросы; функция выводит адрес,
// пытается открыть браузер и возвращает ошибки
// запуска или вывода. Ошибка открытия браузера не прерывает работу сервера.
func run(ctx context.Context, output io.Writer, port string, handler http.Handler, openBrowser func(string) error) error {
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
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()

	// Browser launch is best-effort; headless systems can use the printed URL.
	// RU: Ошибка открытия браузера не останавливает локальный сервер.
	if err := openBrowser(address); err != nil {
		_, _ = fmt.Fprintf(output, "Browser was not opened automatically: %v. Open %s manually.\n", err, address)
	}

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

// start opens the persistent database before exposing the HTTP interface.
// Closing the server also closes the database connection.
//
// RU: start открывает постоянную базу до запуска HTTP-интерфейса.
// При завершении работы соединение с базой закрывается.
func start(ctx context.Context, output io.Writer, port, databasePath string, openBrowser func(string) error) error {
	db, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		return fmt.Errorf("initialize local database: %w", err)
	}
	defer db.Close()

	// Create the group schema before allowing the UI to accept requests.
	// RU: Таблица групп создаётся до запуска пользовательского интерфейса.
	repo := groups.NewRepository(db)
	if err := repo.Init(ctx); err != nil {
		return fmt.Errorf("initialize academic groups: %w", err)
	}
	// Students depend on group IDs, so initialize their table second.
	// RU: Таблицу студентов создаём после таблицы учебных групп.
	if err := students.NewRepository(db).Init(ctx); err != nil {
		return fmt.Errorf("initialize students: %w", err)
	}
	// The same SQLite repository serves group listing and creation requests.
	// RU: Чтение и создание групп используют общий SQLite-репозиторий.
	// Student pages use the same persistent SQLite connection.
	// RU: Страницы студентов используют то же постоянное соединение SQLite.
	handler, err := web.NewHandler(repo, students.NewRepository(db))
	if err != nil {
		return fmt.Errorf("initialize HTTP interface: %w", err)
	}
	return run(ctx, output, port, handler, openBrowser)
}

// launch chooses a per-user directory so an installation does not require
// elevated privileges or additional database services.
//
// RU: launch использует пользовательский каталог; права администратора ОС
// и отдельный сервер базы данных для установки не требуются.
func launch(ctx context.Context, output io.Writer, openBrowser func(string) error) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("find user configuration directory: %w", err)
	}
	databasePath := filepath.Join(configDir, "ThesisFlow", "thesisflow.sqlite")
	return start(ctx, output, localPort, databasePath, openBrowser)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := launch(ctx, os.Stdout, browser.Open); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

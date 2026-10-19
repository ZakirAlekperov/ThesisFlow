# ThesisFlow

A lightweight, local graduation-thesis document tracker for educators.

Лёгкий локальный помощник преподавателя для учёта материалов выпускных квалификационных работ.

> **Development status / Состояние разработки:** early prototype (0.1.01). The first HTTP page is available; document storage, Excel import, SQLite and browser auto-opening are not implemented yet.
>
> Ранний прототип (0.1.01). Доступна первая HTTP-страница; хранение документов, импорт Excel, SQLite и автоматическое открытие браузера ещё не реализованы.

## English

### Run locally

Go 1.23 or newer is required to run from source.

```bash
go run ./cmd/thesisflow
```

Open **http://127.0.0.1:8080/** manually in your browser. Stop the local server with **Ctrl+C**. The server listens only on the loopback interface; other computers cannot connect.

If port 8080 is already in use, the program reports an error and exits instead of silently changing its listening address.

### Test

```bash
go test ./...
go vet ./...
```

See [docs/testing.md](docs/testing.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Русский

### Локальный запуск

Для запуска из исходников требуется Go 1.23 или новее.

```bash
go run ./cmd/thesisflow
```

Откройте в браузере адрес **http://127.0.0.1:8080/** вручную. Для остановки сервера нажмите **Ctrl+C**. Сервер слушает только loopback-интерфейс; подключиться с другого компьютера нельзя.

Если порт 8080 уже занят, программа сообщает об ошибке и завершает работу, не переключаясь незаметно на другой адрес.

### Тестирование

```bash
go test ./...
go vet ./...
```

Правила разработки описаны в [docs/testing.md](docs/testing.md) и [CONTRIBUTING.md](CONTRIBUTING.md).

## License / Лицензия

MIT. See [LICENSE](LICENSE).

# ThesisFlow

A lightweight, local graduation-thesis document tracker for educators.

Лёгкий локальный помощник преподавателя для учёта материалов выпускных квалификационных работ.

> **Development status / Состояние разработки:** early prototype (0.1.01). The first HTTP page opens in the default browser. Document storage, Excel import and SQLite are not implemented yet.
>
> Ранний прототип (0.1.01). Первая HTTP-страница автоматически открывается в браузере по умолчанию. Хранение документов, импорт Excel и SQLite ещё не реализованы.

## English

### Run locally

Go 1.23 or newer is required to run from source.

```bash
go run ./cmd/thesisflow
```

The program prints a local URL such as **http://127.0.0.1:37893/** (the port changes at each startup) and attempts to open it automatically in your default browser. If your graphical desktop or URL opener is unavailable, open the printed URL manually. Stop the local server with **Ctrl+C**. The server listens only on the loopback interface; other computers cannot connect.

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

Программа выводит локальный адрес, например **http://127.0.0.1:37893/** (порт меняется при каждом запуске), и пытается автоматически открыть его в браузере по умолчанию. Если графическая среда или средство открытия ссылок недоступно, откройте адрес из консоли вручную. Для остановки сервера нажмите **Ctrl+C**. Сервер слушает только loopback-интерфейс; подключиться с другого компьютера нельзя.

### Тестирование

```bash
go test ./...
go vet ./...
```

Правила разработки описаны в [docs/testing.md](docs/testing.md) и [CONTRIBUTING.md](CONTRIBUTING.md).

## License / Лицензия

MIT. See [LICENSE](LICENSE).

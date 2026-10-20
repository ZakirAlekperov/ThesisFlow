# ThesisFlow

A lightweight, local graduation-thesis document tracker for educators.

Лёгкий локальный помощник преподавателя для учёта материалов выпускных квалификационных работ.

> **Development status / Состояние разработки:** early prototype (0.1.01). The first HTTP page opens in the default browser. SQLite initialization and group persistence are available internally; group UI, student records, document storage and Excel import are not implemented yet.
>
> Ранний прототип (0.1.01). Первая HTTP-страница автоматически открывается в браузере по умолчанию. SQLite инициализируется автоматически, а группы уже можно сохранять через внутренний Go API; интерфейс групп, учёт студентов, хранение документов и импорт Excel ещё не реализованы.

## English

### Run locally

Go 1.23 or newer is required to run from source.

```bash
go run ./cmd/thesisflow
```

The program prints a local URL such as **http://127.0.0.1:37893/** (the port changes at each startup) and attempts to open it automatically in your default browser. If your graphical desktop or URL opener is unavailable, open the printed URL manually. Stop the local server with **Ctrl+C**. The server listens only on the loopback interface; other computers cannot connect. On startup, SQLite is created or reopened at `<user config dir>/ThesisFlow/thesisflow.sqlite`. No Docker or external database server is required.

### Test

```bash
go test ./...
go vet ./...
```

See [docs/testing.md](docs/testing.md), [docs/groups.md](docs/groups.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Русский

### Локальный запуск

Для запуска из исходников требуется Go 1.23 или новее.

```bash
go run ./cmd/thesisflow
```

Программа выводит локальный адрес, например **http://127.0.0.1:37893/** (порт меняется при каждом запуске), и пытается автоматически открыть его в браузере по умолчанию. Если графическая среда или средство открытия ссылок недоступно, откройте адрес из консоли вручную. Для остановки сервера нажмите **Ctrl+C**. Сервер слушает только loopback-интерфейс; подключиться с другого компьютера нельзя. При запуске SQLite создаётся или открывается по пути `<каталог конфигурации пользователя>/ThesisFlow/thesisflow.sqlite`. Docker и отдельный сервер БД не требуются.

### Тестирование

```bash
go test ./...
go vet ./...
```

Правила разработки описаны в [docs/testing.md](docs/testing.md) и [CONTRIBUTING.md](CONTRIBUTING.md). Контракты групп — в [docs/groups.md](docs/groups.md).

## License / Лицензия

MIT. See [LICENSE](LICENSE).

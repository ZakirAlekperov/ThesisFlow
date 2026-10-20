# ThesisFlow

A lightweight, local graduation-thesis document tracker for educators.

Лёгкий локальный помощник преподавателя для учёта материалов выпускных квалификационных работ.

> **Development status / Состояние разработки:** early prototype (0.1.02). The first HTTP page opens in the default browser. SQLite initialization and group creation is available through Go, and the homepage displays saved groups from SQLite. The homepage can create groups; student records can be created and viewed directly from each group's browser page. Excel import and group-wide requirement setup are available. File versioning and document acceptance are not implemented yet.
>
> Ранний прототип (0.1.02). Первая HTTP-страница автоматически открывается в браузере по умолчанию. SQLite инициализируется автоматически, а группы можно создавать через Go API, а главная страница показывает группы из SQLite. Форма создания групп работает; студентов можно добавлять и просматривать на странице соответствующей группы в браузере. Импорт Excel и создание общих требований к документам доступны. Версии файлов и приёмка материалов ещё не реализованы.

## English

### Run locally

Go 1.23 or newer is required to run from source.

```bash
go run ./cmd/thesisflow
```

The program prints a local URL such as **http://127.0.0.1:37893/** (the port changes at each startup) and attempts to open it automatically in your default browser. If your graphical desktop or URL opener is unavailable, open the printed URL manually. Stop the local server with **Ctrl+C**. The server listens only on the loopback interface; other computers cannot connect. On startup, SQLite is created or reopened at `<user config dir>/ThesisFlow/thesisflow.sqlite`. No Docker or external database server is required. The homepage displays existing academic groups, or an empty-state message if none have been added. You can create groups using the form on the homepage. New groups appear immediately after the form redirects to the list. Click a group name to view its students at `/groups/{id}`. The group page also lets you add students by full name and unique educational ID.

### Test

```bash
go test ./...
go vet ./...
```

See [docs/testing.md](docs/testing.md), [docs/groups.md](docs/groups.md), [docs/students.md](docs/students.md) [docs/requirements.md](docs/requirements.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Русский

### Локальный запуск

Для запуска из исходников требуется Go 1.23 или новее.

```bash
go run ./cmd/thesisflow
```

Программа выводит локальный адрес, например **http://127.0.0.1:37893/** (порт меняется при каждом запуске), и пытается автоматически открыть его в браузере по умолчанию. Если графическая среда или средство открытия ссылок недоступно, откройте адрес из консоли вручную. Для остановки сервера нажмите **Ctrl+C**. Сервер слушает только loopback-интерфейс; подключиться с другого компьютера нельзя. При запуске SQLite создаётся или открывается по пути `<каталог конфигурации пользователя>/ThesisFlow/thesisflow.sqlite`. Docker и отдельный сервер БД не требуются. На главной странице отображаются сохранённые группы или сообщение об их отсутствии. Группы можно создавать прямо на главной странице: после отправки формы список обновится. Нажмите название группы для просмотра её студентов. На странице группы есть форма добавления студента с ФИО и уникальным учебным ID.

### Тестирование

```bash
go test ./...
go vet ./...
```

Правила разработки описаны в [docs/testing.md](docs/testing.md) и [CONTRIBUTING.md](CONTRIBUTING.md). Контракты групп — в [docs/groups.md](docs/groups.md), студентов — в [docs/students.md](docs/students.md), требований — в [docs/requirements.md](docs/requirements.md).

## License / Лицензия

MIT. See [LICENSE](LICENSE).

## Group print report / Печатная ведомость группы

EN: Once a group has requirements and documents, open its page and select
**Печатная ведомость группы**. The read-only report shows students, required,
accepted, overdue, and completion percent. Use browser Print / Save as PDF.

RU: На странице учебной группы нажмите **«Печатная ведомость группы»**.
Откроется отчёт с числом студентов, требуемых, принятых, просроченных
документов и процентом готовности. Для печати или PDF используйте печать
браузера. Отчёт не изменяет данные SQLite.

## Portable builds / Переносимые сборки

EN: GitHub Actions **Portable builds** creates downloadable binaries for
macOS ARM64/AMD64, Windows AMD64 and Linux AMD64. No Go installation is
needed to run a downloaded binary. It contains the local browser app and
backup commands. Build artifacts are attached to the workflow run.

RU: GitHub Actions **Portable builds** создаёт архивы приложения для
macOS ARM64/AMD64, Windows AMD64 и Linux AMD64. Для запуска готового файла
устанавливать Go не требуется. В архивах есть приложение и команды
резервного копирования. Сборки доступны как артефакты запуска Actions.

Backup instructions / Инструкция по резервному копированию:
[docs/backup.md](docs/backup.md).

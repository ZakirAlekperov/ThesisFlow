# ThesisFlow

Локальный помощник преподавателя для управления материалами выпускных
квалификационных работ. / A local graduation-thesis document manager for educators.

**Status / Статус: 0.6.0-dev — версия в разработке, не стабильный релиз.**

## Возможности (RU)

ThesisFlow запускается как одно локальное приложение на Go. Интерфейс
открывается в браузере; Docker, React, npm, интернет и внешний SQL-сервер
не нужны. Данные сохраняются в SQLite на компьютере преподавателя.

- Учебные группы и студенты с уникальным учебным ID, редактирование и архив.
- Импорт реального Excel (.xlsx) с выбором листа, предпросмотром и явным
  подтверждением до внесения записей.
- Общие требования группы к документам и их сроки сдачи. В 0.3.0 можно
  редактировать название и срок; если по требованию уже есть принятый файл,
  изменение названия блокируется ради целостности файлов.
- Для одной ВКР: тема с реквизитами приказа, все требуемые документы,
  загрузка одиночных версий или до десяти файлов за одну отправку,
  просмотр истории, принятие и возврат на доработку с обязательной причиной.
- Комментарии преподавателя к каждой конкретной версии документа и неизменяемая
  история рецензирования с указанием даты и номера версии.
- Очередь файлов на проверку (актуальные версии, фильтр группы, переход к
  конкретной версии), ведомость комплектности общих и индивидуальных
  требований, PDF/TXT предпросмотр с ограничениями безопасности,
  печатный журнал замечаний. PDF отображается штатными средствами браузера;
  DOCX/ODT/XLSX можно скачать, но нельзя предварительно просмотреть в UI.
- Раздельное хранение неизменяемых версий и человекочитаемых папок с
  принятыми файлами: группа / студент / нормализованное имя документа.
- Общая ведомость по группе, фильтры статусов и просрочек, печатный отчёт,
  панель готовности и поиск студентов.
- Раздел «Аналитика и отчёты» (`/reports`): согласованные метрики активных
  групп и студентов, персональные требования, фильтры и сортировки,
  отдельный признак ранее принятой копии, просрочки в днях, переходы
  к нужной ВКР, недельная динамика реальных событий. Выгрузки CSV с
  защитой от формульной инъекции и настоящего форматированного XLSX с
  листами «Сводка», «Студенты», «Документы», печать в PDF через браузер.
- Раздел скачиваемых шаблонов и тестовых DOCX/XLSX.
- Резервная копия ZIP через настройки и проверенное восстановление в
  **отдельную** папку, не заменяющее активные данные.
- Диагностика SQLite и документов (`/settings/health`,
  `thesisflow doctor`), сверка SHA-256 версий и принятых копий, проверка
  связей файлов и требований. Отдельная команда `thesisflow backup verify
  <archive.zip>` и проверка внутренней целостности восстановленного ZIP
  **до** публикации нового каталога. Аварийный журнал операций и
  автоматическая активация новой папки ещё в разработке.
- Собственные светлая и тёмная темы, вариант C: компактная горизонтальная
  навигация, контрастные формы и таблицы.

### Установка и запуск

Для запуска из исходников нужен Go 1.23+.

```bash
go run ./cmd/thesisflow
```

Приложение печатает адрес вида `http://127.0.0.1:<port>/` и пытается
открыть его автоматически. Сервер слушает только 127.0.0.1; с другого
компьютера он недоступен. Завершение — Ctrl+C.

SQLite: `<user config dir>/ThesisFlow/thesisflow.sqlite`.
Файлы: `<user config dir>/ThesisFlow/storage/`.
Папка изолированных восстановлений: `<user config dir>/ThesisFlow/restores/`.

Для просмотра версии без запуска БД и браузера:

```bash
go run ./cmd/thesisflow version
```

### Проверка

```bash
gofmt -l .
go test ./...
go vet ./...
go test -race ./...
```

Сборки Windows AMD64, macOS ARM64/AMD64 и Linux AMD64, а также пакеты
для отдельных ОС формируются через GitHub Actions. Не все сборки подписаны
или нотариально заверены. Перед работой с реальными ВКР проверьте
экспорт резервных копий, восстановление и ручной сценарий с тестовыми файлами.

### Документация

- [Возможности и приёмка 0.2.0](docs/release-0.2.0.md)
- [План версии 0.4.0 и приёмка рецензирования](docs/release-0.4.0.md)
- [Сценарии ручной приёмки 0.4.0](docs/acceptance-0.4.0.md)
- [Реализация и приёмка аналитики 0.5.0](docs/release-0.5.0.md)
- [Контроль целостности и проверка резервной копии 0.6.0](docs/release-0.6.0.md)
- [Оформление варианта C](docs/ui-concept-c.md)
- [Редактирование требований](docs/requirement-edit-0.3.md)
- [Контроль обновления статусов](docs/status-freshness-0.3.md)
- [Аудит кода и ограничения](docs/code-review-0.3.md)
- [Миграции SQLite](docs/migrations.md)
- [Целостность файлов](docs/integrity.md)
- [Резервное копирование](docs/backup-browser.md)
- [Восстановление](docs/backup-restore-browser.md)
- [Сборки для ОС](docs/platform-packaging.md)

## English

ThesisFlow is a single local Go application that opens a server-rendered
browser UI and keeps student/thesis metadata in SQLite. No Docker, cloud,
React, Node.js or external database is required.

Core capabilities: current analytics with 3-sheet XLSX/CSV reporting, group and student readiness and factual weekly activity; groups, student records and XLSX import with worksheet
preview/confirmation; configurable shared document requirements and due
dates; immutable uploaded revisions and an approved readable-folder
projection; thesis topic/order metadata; approval/revision history; grouped
status matrix, overdue filters, progress dashboard, printable reports,
search, bundled test templates, backups and isolated verified restores.

The 0.4.0 development line adds version-specific review comments and
mandatory transactional reasons for returning documents for revision.
The 0.3.0 line added concept C navigation and theme consistency,
requirement title/date editing, and direct status-freshness regression tests.
A title cannot be changed after a file for that requirement has been
approved until safe accepted-file relocation is implemented.

Run with `go run ./cmd/thesisflow` (Go 1.23+). The local URL is printed
on startup and opens in your browser if possible. The HTTP server binds
only to loopback. SQLite and files live below the user's configuration
directory in the ThesisFlow folder. Stop with Ctrl+C.

Verify with `go test ./...`, `go vet ./...`, `gofmt -l .`, and
`go test -race ./...` on a supported host. Use real XLSX/DOCX files
for manual acceptance. Check the above documents for current limitations.

**Not yet delivered:** a student-facing portal, multi-user permissions,
cloud synchronization, full DOCX/ГОСТ compliance analysis, guaranteed
atomic batches across many filesystem files, or automatic replacement
of the current workspace after restoring a backup. ZIP backups are not
encrypted. This is development software, not a finalized release.

## License

MIT — [LICENSE](LICENSE).

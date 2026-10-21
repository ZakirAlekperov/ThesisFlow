# Backup archives / Резервное копирование — 0.1.10

## English

The `internal/backup` package creates and restores a ZIP snapshot of the
local ThesisFlow workspace. `backup.Export(ctx, db, storageDir, destination)`
uses SQLite `VACUUM INTO` to create a consistent database snapshot and adds
files from `storage/` with SHA-256 hashes. The archive contains
`thesisflow.sqlite`, optional `storage/...` files and `manifest.json`.

`backup.Restore(archive, newWorkspacePath)` verifies all checksums before
publishing the restored directory. It rejects existing target folders, path
traversal, symbolic links, unexpected entries and missing database files.
A failed recovery does not create a partially restored workspace.

**Safety:** Archive data is **not encrypted**; it may contain students' personal
information and thesis documents. Store it securely. Backups should be made
while the app is stopped so that document files cannot change during the ZIP
snapshot. Database and file snapshots are not yet an atomic multi-resource
transaction. Individual files above 64 MiB or archives above 512 MiB are not
supported by this initial implementation.

The Go service and user-facing backup commands are available; automatic scheduled backups are not included.

## Русский

Пакет `internal/backup` сохраняет и восстанавливает локальные данные
ThesisFlow. `backup.Export(ctx, db, storageDir, destination)` создаёт
согласованный снимок SQLite через `VACUUM INTO`, добавляет файлы из
`storage/` и записывает контрольные суммы SHA-256 в `manifest.json`.

`backup.Restore(archive, newWorkspacePath)` до восстановления проверяет все
суммы и отклоняет существующий каталог назначения, пути выхода за пределы
папки, символические ссылки, неизвестные файлы и архив без базы данных.
При ошибке частично восстановленная папка не создаётся.

**Безопасность:** ZIP **не зашифрован** и может содержать персональные данные
студентов и документы ВКР. Храните резервную копию защищённо. Экспорт следует
делать при остановленном приложении, чтобы файлы документов не изменились во
время архивации. Снимок базы и файлов пока не является единой атомарной
транзакцией. Ограничения начальной версии: файл не более 64 МиБ, ZIP не более
512 МиБ.

Go-сервис и команды экспорта/восстановления доступны; автоматическое создание копий пока не предусмотрено.

## Command line / Командная строка

English — **stop the running ThesisFlow app before creating a backup**. Build
or run the same executable in terminal:

```bash
go run ./cmd/thesisflow backup export /safe/place/thesisflow-snapshot.zip
go run ./cmd/thesisflow backup restore /safe/place/thesisflow-snapshot.zip /safe/place/ThesisFlow-recovered
```

The export command uses the standard OS-specific user configuration directory
(`ThesisFlow/thesisflow.sqlite` and its adjacent `storage/` directory).
Recovery goes to a **new directory only**, never directly over live user data.
After checking recovered data, move it to the active location while the app is
stopped; always keep the existing active workspace until verified.

Русский — **перед созданием резервной копии остановите ThesisFlow**.

```bash
go run ./cmd/thesisflow backup export /папка/thesisflow-snapshot.zip
go run ./cmd/thesisflow backup restore /папка/thesisflow-snapshot.zip /папка/ThesisFlow-recovered
```

Экспорт использует системный каталог настроек пользователя, где находятся
`ThesisFlow/thesisflow.sqlite` и `storage/`. Восстановление выполняется
**только в новую папку**, а не поверх активной базы. Проверьте данные перед
заменой рабочего каталога, сохраняя прежнюю копию до окончания проверки.

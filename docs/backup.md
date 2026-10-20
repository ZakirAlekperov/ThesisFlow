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

The Go service is available now. The user-facing backup command is the next
separate PR; automatic scheduled backups are not included.

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

Go-сервис уже реализован. Команда экспорта/восстановления для пользователя
будет добавлена отдельным PR; автосоздание копий пока не предусмотрено.

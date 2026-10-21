# Safe archiving / Безопасное архивирование — 0.2.0

## English

Groups and students can be archived and later restored by their original
database IDs. Active lists exclude rows with non-NULL `archived_at`; no
user or document row is removed. Migration v1 detects archive columns in both
legacy and freshly created schemas, so upgrades are idempotent.

**Accepted-file guard:** a group rename, student rename, educational ID edit,
or group transfer is rejected with `ErrAcceptedFiles` if any accepted file
currently refers to the old human-readable directory structure. Silent
renaming would orphan existing accepted file projections. A future
transactional filesystem relocation workflow must be tested before permitting
these identity changes on accepted records.

## Русский

Архивирование группы или студента не удаляет записи из SQLite и не
уничтожает историю документов. Активные списки скрывают записи с
`archived_at`; при восстановлении остаются прежние внутренние ID.
Миграция версии 1 умеет работать и со старой, и со свежей схемой.

**Защита принятых файлов:** переименование группы/студента, изменение
учебного ID или перевод между группами запрещаются с ошибкой
`ErrAcceptedFiles`, если у затронутых студентов есть принятые файлы.
Так мы не допускаем рассинхронизации базы и читаемой структуры каталогов.
Для разрешения таких изменений потребуется отдельная атомарная операция
переноса файлов с проверенными сценариями восстановления после сбоя.

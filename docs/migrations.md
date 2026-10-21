# SQLite schema migrations / Миграции SQLite — ThesisFlow 0.2.0

## English

`internal/storage/migrate.Apply(ctx, db)` runs after the old academic group
and student tables are available and **before** the HTTP server starts. It
reads SQLite `PRAGMA user_version` and applies schema version 1 atomically:
optional archival timestamps for groups/students and a student-bound thesis
record with topic and order reference. Existing IDs, names and relationships
remain untouched.

Migration is repeatable; databases produced by a newer application fail with
`ErrFutureVersion` rather than silently downgrading. Schema operations and
`user_version` changes share one transaction. A failed update rolls back.

Note: migration does not replace reliable backup procedures. Back up your
workspace before major upgrades. Do not modify columns manually.

## Русский

`internal/storage/migrate.Apply(ctx, db)` запускается после появления
таблиц групп и студентов и **до запуска HTTP-сервера**. Он читает
`PRAGMA user_version`, затем в одной транзакции переводит схему на
версию 1: добавляет необязательные поля архивирования групп и студентов,
а также таблицу ВКР с темой и реквизитами приказа, привязанную к студенту.

Повторный запуск безопасен. База, созданная более новой версией, не
открывается с риском отката схемы; возвращается `ErrFutureVersion`.
При ошибке обновления изменения транзакции отменяются.

Перед значимыми обновлениями обязательно создавайте резервную копию:
система миграций не заменяет резервное копирование.

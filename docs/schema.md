# SQLite schema compatibility / Совместимость схемы SQLite

## English

ThesisFlow records a separate database schema version in the `app_schema`
singleton table. The initial version is `1`. When an older executable finds
an unknown version, startup stops before it initializes other application
tables. Existing databases from 0.1.10 receive the marker without discarding
their groups, students, documents or revisions.

This first increment **is not** a complete numbered migration engine. Before
changing a domain schema, introduce explicit ordered, transactional migration
scripts and add migration/rollback safety tests. Never modify the marker
manually to bypass a mismatch.

## Русский

ThesisFlow хранит версию структуры SQLite в отдельной таблице `app_schema`.
Начальный номер — `1`. Если исполняемый файл обнаруживает незнакомую
версию, запуск прекращается до инициализации остальных таблиц. Старые базы
0.1.10 получают маркер без удаления групп, студентов, документов и версий.

Это **ещё не** полноценный механизм последовательных миграций. Перед
изменениями схемы необходимо добавить упорядоченные транзакционные миграции
и тесты безопасности. Нельзя вручную изменять номер, чтобы обойти ошибку.

# File integrity audit / Контроль целостности документов

## English

`integrity.Check(ctx, db, storageRoot)` checks every version in
`file_versions` against the actual file's length and recorded SHA-256.
The result distinguishes intact, missing, corrupt and unsafe paths, each with
the permanent version ID and a reason. This inspection is read-only and does
not delete, replace or silently fix files.

It rejects path traversal and symbolic links, checks path components and
limits individual files to the current 20 MiB upload cap. Unsafe files are
never followed outside the managed local directory.

This first check audits **internal immutable versions**. The accepted-file
projection can be regenerated from approved history, but an automated
projection reconciliation and crash-recovery state machine require a separate
verified implementation. Do not equate this audit with an automatic repair
tool.

## Русский

`integrity.Check(ctx, db, storageRoot)` сравнивает все внутренние версии
файлов с размерами и SHA-256, сохранёнными в `file_versions`.
Отдельно учитываются исправные, отсутствующие, повреждённые и опасные
пути. В отчёте указан постоянный ID каждой проблемной версии.
**Проверка ничего не удаляет и не исправляет автоматически.**

Пути за пределы каталога, символические ссылки и файлы больше 20 МиБ
не читаются. Проверяются все составляющие пути, а не только его конец.

Эта версия контролирует **внутреннюю историю файлов**, но пока не
перестраивает принятую человекочитаемую папку. Проверенное восстановление
проекций и состояния после прерванной файловой операции требует отдельного
этапа — отчёт об ошибках не является автоматическим исправлением.

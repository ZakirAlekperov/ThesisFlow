# ThesisFlow 0.1.10 — release candidate / Кандидат в выпуск

Status: **review candidate, not a tagged stable release**.
Состояние: **кандидат на проверку, не опубликованный стабильный релиз**.

## English — implemented in the PR stack

1. 0.1.03: offline XLSX reading, column mapping and student batch import.
2. 0.1.04: group-level and individual document requirements with deadlines.
3. 0.1.05: material status tracking and immutable change history.
4. 0.1.06: retained document versions in internal local storage.
5. 0.1.07: accept/return documents and preserve human-readable approved files.
6. 0.1.08: group readiness percentage and overdue document count.
7. 0.1.09: locally generated, print-ready group summary (save as PDF via browser).
8. 0.1.10: SQLite/document ZIP backup and isolated restore commands,
   macOS (ARM64/x86-64), Windows (x86-64), and Linux (x86-64) portable artifacts,
   browser-side file upload, review, and download.

The application stays local, listens on 127.0.0.1 and requires no Docker or
database server. Run `thesisflow version` or `thesisflow --version` to
identify the build.

**Before release:** review PRs in order; after integration run `go test ./...`,
`go test -race ./...`, `go vet ./...`, validate all target-platform builds,
and perform manual end-to-end checks of XLSX imports, documents, recovery, and
filesystem permissions using throwaway student records.

**Known restrictions:** no separate student portal, no automatic collection
of email attachments, no automatic scheduled backups, no archival editing
workflow for groups/students, no locale-aware Russian student sorting, no
authentication for users on the same local machine, and no automatic deployment
of accepted backup files over the current workspace. Backups are **not encrypted**.
The GUI implements core document workflows but remains a basic MVP UI, not a
finished visual redesign. There is not yet an OS installer or signed/notarized
binary. The active workspace should be stopped before manual backup export.

## Русский — реализованные возможности

1. 0.1.03 — чтение XLSX, сопоставление колонок, пакетный импорт студентов.
2. 0.1.04 — требования к документам группы и студента, сроки сдачи.
3. 0.1.05 — текущие статусы и неизменяемая история их изменений.
4. 0.1.06 — сохранение всех версий документов во внутреннем хранилище.
5. 0.1.07 — принятие/возврат на доработку и понятная структура принятых файлов.
6. 0.1.08 — процент готовности группы и число просроченных документов.
7. 0.1.09 — ведомость группы для печати через браузер и сохранения в PDF.
8. 0.1.10 — резервное копирование SQLite с файлами и восстановление в новую
   папку, переносимые сборки для macOS, Windows и Linux, формы загрузки,
   скачивания и принятия файлов в браузере.

Приложение локальное: `127.0.0.1`, без Docker и отдельного сервера БД.
Проверка версии: `thesisflow version` или `thesisflow --version`.

**Перед выпуском:** просмотреть и последовательно принять зависимые PR,
затем выполнить `go test ./...`, `go test -race ./...`, `go vet ./...`,
проверить сборки и вручную пройти импорт Excel, сдачу/проверку документов,
восстановление архива и права доступа к файлам на тестовых данных.

**Известные ограничения:** нет личного кабинета студента, автоматического
получения файлов из почты, автоматических резервных копий и завершённого
редактирования/архивирования групп и студентов. Сортировка ФИО не учитывает
русские языковые правила; пользователи с доступом к одному локальному
компьютеру не разделены ролями. ZIP-копии **не зашифрованы**. Интерфейс
содержит основные рабочие действия, но ещё не является законченным
визуальным дизайном. Установщик ОС и подпись приложений пока отсутствуют.
Перед ручным экспортом резервной копии следует закрывать приложение.

## PR review order / Порядок проверки

PR #14 → #15 → #16 → #17 → #18 → #19 → #20 → #22 → #23
→ #24 → #25 → #26 → this candidate.

These PRs are stacked: each PR targets the preceding branch rather than main.
After **Squash and merge**, the next PR may require refreshing/rebasing and
retargeting to main, because squash does not preserve the source commit graph.
Do not merge the entire stack blindly or close its prerequisite branches until
integration has been checked. No automatic merges or tags are created.

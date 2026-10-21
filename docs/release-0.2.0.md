# ThesisFlow 0.2.0-rc1 — acceptance checklist / Приёмочные испытания

This branch is a **release candidate**, not a stable tagged release. It is
stacked on PR #39 (and transitively PRs #30–#38). There is no automatic merge.

Данная ветка — **кандидат в выпуск**, а не подтверждённая стабильная версия.
Она зависит от PR #39 и предыдущей цепочки #30–#38.

## Stack / Цепочка PR

#30 — transactional SQLite migrations
#31 — edit/archive/recover groups and students
#32 — one thesis record per student and topic-by-order changes
#33 — safe archived-record visibility and projection paths
#34 — browser group/student management without React
#35 — browser thesis topic form
#36 — Excel sheet selection
#37 — Excel preview / confirm
#38 — Go templates dashboard, search, embedded CSS and theme toggle
#39 — file integrity audit
This release candidate — version display and manual acceptance instructions.

The application remains Go + html/template + custom CSS/JavaScript and SQLite;
no React, Vue, Angular, Node.js, or required frontend build system.

## Manual check using **real files** / Проверка реальными файлами

Download the separate test bundle named
`ThesisFlow_0.2.0_Manual_Test_Kit.zip` from the conversation. All records
are fictional; **the files do not automatically create database records**.

1. Start with a disposable empty workspace and confirm `thesisflow version`
   prints `ThesisFlow 0.2.0-rc1`.
2. Create group `ИСП-401`. Import `01_Список_студентов_ИСП-401.xlsx`
   using headers `ФИО` and `Учебный ID`. Expect 10 students.
3. Check duplicate imports fail without partial changes. Use the separate
   empty Excel roster for manually-entered test students.
4. Create a thesis topic for Ivanov Artem and supply order details according to
   the app's current form rules.
5. Add exactly ten group requirements:
   - Титульный лист
   - Введение
   - Список принятых сокращений
   - Глава 1
   - Глава 2
   - Глава 3
   - Заключение
   - Список источников
   - Приложение 1
   - Приложение 2
6. Open the same student's document page and upload the corresponding
   `.docx` for each of the ten requirements. Review and accept them through
   the browser; ensure all ten are visible in the group readiness report.
7. Upload the extra `Глава 1` draft version, return it for correction, then
   upload and accept the corrected version. Earlier versions must remain
   downloadable and only the accepted version must be projected.
8. Close ThesisFlow, export a backup and restore it to a new directory. Check
   the database records, ten requirements and uploaded files.
9. Edit and archive a disposable student and restore them. Rename a disposable
   group. Confirm saved file paths, history and IDs remain valid.
10. Repeat on macOS ARM64, Windows AMD64 and Linux AMD64 using the portable
    binaries. Run `go test ./...`, `go test -race ./...`, `go vet ./...`.

## Known limitations / Ограничения

The test documents are fictional **educational examples**, not officially
approved academic formatting templates. Local server behavior does not provide
multi-user authentication, student self-service, or encrypted backups. UI for
backup export/restore may remain CLI-only until separately implemented and
security-reviewed. Platform installers and code signing are separate release
requirements. A full manual acceptance report is required before marking
this candidate stable.

Шаблоны учебные и не являются официальными формами образовательной
организации. Нет отдельного студенческого портала и шифрования резервных
копий. Если графическое резервирование ещё не проверено, используйте CLI
при закрытом приложении. Статус стабильного релиза не присваивается без
ручной проверки на целевых ОС.

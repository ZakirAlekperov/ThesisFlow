# ThesisFlow 0.2.0-rc2 — acceptance and review / Приёмка версии

**Status: release candidate. Not a stable or tagged release.**
**Статус: кандидат в выпуск. Стабильный релиз ещё не опубликован.**

## English

This candidate collects work from the following stacked Pull Requests:

- #30: transactional SQLite migrations
- #31–#35: group/student changes, archive/recovery, thesis record and topic UI
- #36–#37: Excel sheet selection, preview and confirmed import
- #38: server-rendered dashboard, search and embedded light/dark CSS+JS
- #39: file integrity audit against stored SHA-256
- #42: backup ZIP export through Settings (with protected writer gate)
- #43: group-wide student/document status matrix
- #44: CSRF-protected ZIP upload and verified restore to **a new folder**

The UI uses Go's standard `html/template` and own CSS/JavaScript, embedded
in the Go binary. **No React, Vue, Angular or Node.js runtime** is required.

### Manual real-file verification

The separate archive `ThesisFlow_0.2.0_Manual_Test_Kit.zip` contains two XLSX
student rosters, **ten actual DOCX thesis documents**, two versions of
Chapter 1, a manual guide and a JSON manifest. All data are fictional.

1. Run in a disposable workspace. Create group `ИСП-401`.
2. Import the provided spreadsheet using the `ФИО` and `Учебный ID`
   headers, reviewing the Excel preview first. Expect **10** unique students.
3. Import the same file again. Expect an error, with no partial new records.
4. Open the group and create exactly these ten required documents:
   **Титульный лист; Введение; Список принятых сокращений; Глава 1;
   Глава 2; Глава 3; Заключение; Список источников; Приложение 1;
   Приложение 2.**
5. Open Ivanov Artem's documents. Upload the ten matching DOCX files through
   the browser. Confirm status and downloadable version for every section.
6. Upload the Chapter 1 draft and return it for revision; then upload and
   accept the corrected version. Verify both remain in history and the
   accepted projection references the corrected one.
7. Open the group-wide status matrix. Confirm student/requirement cells,
   accepted counts, and printable report; verify all ten documents can be
   marked Accepted for the chosen student.
8. Set and update a thesis topic using order requirements. Test renaming,
   archiving and recovery with **disposable** students/groups.
9. Open Settings. Download a ZIP backup, then restore it through the browser
   into a new folder. Check all ten files and database entries there.
10. Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and verify
    portable binaries on Windows, macOS and Linux before a stable tag.

### Limitations

This candidate is not an authenticated multi-user service. Backups contain
unencrypted student data. Restoration **never overwrites the live database**.
The academic DOCX files are fictional testing material, not institutionally
approved official forms. Signed installers and code notarization remain
future delivery steps. End-to-end manual acceptance must be performed by a
person; CI success alone does not establish release readiness.

## Русский

Кандидат содержит изменения PR #30–#39, #42–#44 и сохраняет стек:
Go, `html/template`, собственный CSS и минимальный JavaScript, SQLite,
локальный сервер и единый бинарный файл. React и другие SPA-фреймворки не нужны.

Отдельный ZIP-набор для ручной проверки включает **10 документов ВКР**:
титульный лист, введение, список принятых сокращений, главы 1, 2, 3,
заключение, список источников и два приложения. Также есть Excel-список
10 студентов, пустой шаблон, черновик и исправленная глава 1.

Проверка выполняется **через интерфейс**, без добавления тестовых записей
из Go-кода: создать группу, импортировать Excel с предпросмотром, добавить
10 требований, загрузить DOCX, отправить черновик на доработку, принять
исправление, открыть общую ведомость, выполнить резервное копирование
и восстановить данные в отдельный каталог. Затем повторить основные
проверки на macOS, Windows и Linux.

Ограничения: нет многопользовательской авторизации, резервные копии не
зашифрованы, восстановление не подменяет работающую базу, примеры ВКР
не являются официальными формами колледжа. Полноценные установщики,
подпись и нотариализация приложений пока не подтверждены. Для признания
версии стабильной необходимы ручные приёмочные испытания.

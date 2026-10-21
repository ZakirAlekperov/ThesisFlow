# Browser document workflow / Работа с документами в браузере

## English

The academic group page now links each student to a document checklist at
`GET /groups/{group}/students/{student}`. Requirements are loaded from the
existing group/student requirement repository and show their due dates and
current status. Teachers may upload a document revision (`.docx`, `.pdf`,
`.odt`, `.rtf`, `.txt`, `.xlsx`, up to 20 MiB), download any saved
revision, accept a revision or return it for correction.

On upload, the immutable file is stored in the internal history and marked
**Under review**. Accepting it produces a human-readable accepted-file
projection while retaining historical revisions. On rejection, the document is
marked **Needs revision**. Every write uses the same per-process CSRF token
as other ThesisFlow forms and redirects after success.

The page validates group/student/requirement membership, prevents downloads of
other students' versions, escapes text via `html/template`, and does not expose
internal paths on storage failures. It requires no client JavaScript.

**Limits:** this MVP uses teacher-mediated receipt of files, not a separate
student portal or automatic email pickup. The 20 MiB cap applies per upload.
Review and file storage are integrated with the app but still require manual
platform acceptance testing before a supported release.

## Русский

На странице учебной группы ФИО студента ведёт к его ведомости документов:
`GET /groups/{group}/students/{student}`. Здесь видны требования группы
и студента, сроки сдачи и текущий статус каждого документа.

Преподаватель может загрузить очередную версию файла (`.docx`, `.pdf`,
`.odt`, `.rtf`, `.txt`, `.xlsx`; до 20 МиБ), скачать любую версию,
принять документ либо вернуть его на доработку.

Каждая загрузка сохраняется в отдельном внутреннем хранилище версий и получает
статус **«На проверке»**. Принятие создаёт понятную пользовательскую копию
файла, не удаляя историю. Отклонение даёт статус **«На доработке»**.
Записывающие запросы защищены CSRF-токеном и завершаются перенаправлением.

Проверяется принадлежность студента группе и применимость требований; чужие
версии скачать нельзя. Значения экранируются через `html/template`, а
внутренние пути при ошибках не показываются. JavaScript не требуется.

**Ограничения:** документы поступают к преподавателю и загружаются им вручную.
Самостоятельного студенческого портала и получения вложений из почты пока нет.
Размер отдельного файла — не более 20 МиБ. Перед выпуском стабильной версии
потребуется проверить весь сценарий вручную на целевых ОС.

# Group and student management screen / Управление и архив

## English

The local `/manage` page lists active groups/students and recoverable
archived records. Each action uses server-rendered HTML forms with the same
CSRF protection as other ThesisFlow POST routes. The forms can rename groups,
correct student names/educational IDs, transfer students between active groups,
archive records and restore them without changing database IDs.

**Important:** accepted-document projections are named after the group and
student. Identity-changing edits are rejected with HTTP 409 while accepted
files exist; no file path is silently broken. This limitation will be lifted
only with a tested filesystem relocation transaction.

No React or external JavaScript framework is used.

## Русский

Раздел `/manage` показывает активные группы/студентов и архивные записи,
которые можно восстановить. Все изменения выполняются через стандартные
HTML-формы с CSRF-защитой. Доступны переименование группы, исправление ФИО
и учебного номера, перевод между действующими группами, архивирование и
восстановление записей без изменения внутренних ID.

**Важно:** когда у студента есть принятые файлы, переименование или перевод
возвращает HTTP 409. Это предотвращает потерю соответствия между SQLite и
человекочитаемой файловой структурой. Для снятия ограничения понадобится
отдельный надёжный алгоритм перемещения файлов с тестами восстановления.

Приложение использует шаблоны Go, а не React или другие SPA-фреймворки.

# Excel preview and confirmation / Предпросмотр Excel

## English

On a group page, choose an XLSX and click **Предпросмотр Excel перед импортом**.
ThesisFlow reads sheet names from the workbook relationship structure and
displays up to eight rows of the selected sheet. The teacher can choose a
different tab, enter the full-name and educational ID column headings, and
confirm the import explicitly. **No database records are written during
preview.** The existing transactional importer saves all rows or rolls back.

The uploaded XLSX remains in bounded process memory (up to three pending
previews, 10 MiB compressed per file) with a 20-minute expiry and a random
one-time preview token. A successful commit invalidates it. Forms remain
CSRF-protected, values escaped, and no JavaScript is required.

Direct import is kept for backward compatibility; use preview when testing
unfamiliar worksheets. Spreadsheet formulas are read from cached values only.

## Русский

На странице группы загрузите XLSX и нажмите **«Предпросмотр Excel перед
импортом»**. Приложение определяет названия реальных листов, показывает
первые восемь строк, позволяет выбрать вкладку и указать заголовки ФИО
и учебного номера. **До нажатия «Подтвердить импорт» SQLite не изменяется.**
При ошибке весь пакет откатывается.

Неподтверждённый файл находится только в памяти приложения, ограничен
10 МиБ и сроком 20 минут. Допускается не более трёх ожидающих предпросмотров.
После успешного импорта случайный токен нельзя использовать повторно.
CSRF-защита и экранирование значений действуют во всех формах;
JavaScript-фреймворки не требуются.

Старый прямой импорт оставлен для совместимости. В незнакомых файлах
рекомендуется пользоваться предпросмотром.

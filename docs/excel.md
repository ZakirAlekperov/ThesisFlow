# XLSX reading / Чтение Excel — ThesisFlow 0.1.03

## English

The first Excel increment provides a **bounded, pure-Go reader** for the first
worksheet of an `.xlsx` ZIP workbook. The internal API is
`importxlsx.Read(data []byte) ([][]string, error)`.

The reader understands shared strings, rich text, inline strings, numeric and
cached formula results, and sparse Excel cells. Input ZIP size is capped at
10 MiB; supported XML entries at 16 MiB, rows at 10,000 and columns at 256.
The result is a rectangular table with blank fields where cells were missing.

**Limitations:** at this stage the first worksheet is expected at
`xl/worksheets/sheet1.xml`. Workbook relationship indirection, date styles,
other sheets, formula evaluation, column mapping and group import are not yet
implemented. This is a deliberate intermediate stage; some valid workbooks
may need a more complete OOXML reader before general production use.

Do not import any records into SQLite from this parser yet.

## Русский

Первый этап Excel добавляет **ограниченный по ресурсам XLSX-парсер на Go**
без сторонних библиотек. Внутренний API:
`importxlsx.Read(data []byte) ([][]string, error)`.

Поддерживаются общие и встроенные строки, форматированные фрагменты,
числовые значения и сохранённые результаты формул, разреженные ячейки.
Максимальный размер исходного ZIP — 10 МиБ, XML-компонента — 16 МиБ,
число строк — 10 000, столбцов — 256. Пустые ячейки сохраняют позиции
своих столбцов.

**Ограничения:** пока читается лист по адресу
`xl/worksheets/sheet1.xml`. Перенаправления OOXML, форматы дат,
другие листы, вычисление формул, выбор столбцов и импорт в группу
будут добавляться отдельно. Часть допустимых файлов Excel пока
не будет поддерживаться. Этот парсер ещё не записывает студентов в SQLite.

## Batch group import / Пакетный импорт в группу

### English

`importxlsx.ImportStudents(ctx, db, groupID, rows, nameColumn, idColumn, skipHeader)`
uses **zero-based** column indexes and an existing academic group. It accepts
rows obtained from `Read` or another validated table source.

- The whole batch runs within one SQLite transaction. Invalid/duplicate
  student IDs roll back **all** students in the uploaded batch.
- Existing students and groups remain unchanged on failure.
- Repeated educational IDs are rejected globally, even across groups.
- The first row may be skipped as a header; fully blank rows are ignored.
- Malformed selected columns or missing required fields stop the import.
- A successful call returns the exact inserted student count.

A browser-facing import form with column selection is still required.

### Русский

`importxlsx.ImportStudents(ctx, db, groupID, rows, nameColumn, idColumn, skipHeader)`
принимает **нумерацию колонок с нуля** и ID существующей группы.

- Весь импорт выполняется в **одной транзакции SQLite**. При пустых полях,
  повторных учебных номерах или повреждённых строках **все** изменения
  текущей загрузки откатываются.
- Уже имеющиеся группы и студенты не меняются при ошибке.
- Учебные ID уникальны среди всех групп.
- Первую строку можно пропустить как заголовок; полностью пустые строки
  игнорируются.
- Ошибочный выбор колонок и отсутствующие обязательные поля запрещены.
- После успешного импорта возвращается число добавленных студентов.

Форма загрузки Excel и выбора колонок через браузер — отдельный этап.

## Browser workflow / Загрузка через браузер

### English

Open a group page and use **Import from Excel** to upload an `.xlsx`
workbook. Enter the exact first-row header labels for the full name and
educational ID columns (defaults: `ФИО`, `Учебный ID`). Matching ignores
case and leading/trailing whitespace. The server parses the file in memory,
validates the chosen group and uses the atomic importer.

On success, HTTP 303 redirects back to the group list of students. If any
required value or student number is invalid or duplicated, all changes are
rolled back. The upload is protected by the same random CSRF token as the
student creation form, rejects files over 10 MiB and requires `.xlsx`.

**Current limitation:** the mapping uses editable header-name fields rather
than a two-step preview dropdown. Worksheet relationship indirection and
advanced Excel styles remain future improvements. The file is not retained.

### Русский

Откройте страницу группы и в разделе **«Импорт из Excel»** выберите XLSX.
Введите названия столбцов первой строки с ФИО и учебным номером (по
умолчанию `ФИО` и `Учебный ID`). Регистр и пробелы по краям
игнорируются. Сервер обрабатывает файл, проверяет группу и выполняет
**атомарную транзакцию SQLite**.

После успешного импорта выполняется перенаправление HTTP 303 в список.
При любом повторном учебном ID или неполной строке весь импорт отменяется.
Форма защищена CSRF, размер файла ограничен 10 МиБ, принимается `.xlsx`.
Загруженный файл не сохраняется.

**Текущее ограничение:** заголовки колонок вводятся вручную; предварительный
просмотр с выпадающими списками ещё не реализован. Работа со сложными
листами XLSX также требует дальнейшего развития парсера.

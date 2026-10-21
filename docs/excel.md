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

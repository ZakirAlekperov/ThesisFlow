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

# XLSX ingestion / Чтение XLSX

## English

`internal/importxlsx` reads a bounded first worksheet into rows of cell
strings. It supports Excel shared strings, inline strings and numeric cell
values, retaining blank column positions for a future column mapper.

The current micro-step assumes that the worksheet is stored at
`xl/worksheets/sheet1.xml`. It does **not yet** resolve workbook relations,
formula results beyond cached `<v>`, dates, merged cells, or multiple sheets.
Do not treat it as a complete XLSX reader. Files are limited to 8 MiB
compressed and worksheet/shared-strings entries to 16 MiB uncompressed.
This is a read-only input parser; there are no SQLite writes in this step.

## Русский

`internal/importxlsx` читает первый лист Excel в виде строк значений ячеек.
Поддерживаются общие строки, встроенные строки и числовые значения, а пустые
столбцы сохраняют свои позиции для будущего сопоставления полей.

На этом микрошаге подразумевается файл листа
`xl/worksheets/sheet1.xml`. Разрешение связей книги, полноценная обработка
формул, дат, объединённых ячеек и нескольких листов **ещё не реализованы**.
Это пока не универсальный парсер XLSX. Размер файла ограничен 8 МиБ,
распакованных XML-файлов — 16 МиБ. Этот этап не изменяет SQLite.

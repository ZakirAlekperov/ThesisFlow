# Selecting Excel worksheets / Выбор листа Excel — 0.2.0

## English

`importxlsx.Sheets(data)` lists worksheet tab names by parsing the workbook
metadata and relationship file. `ReadNamed(data, sheetName)` uses the
resolved Office Open XML relationship path; it does not assume that a visible
tab corresponds to `sheet1.xml`. External relationships and traversal paths
are rejected. Compressed workbooks and individual XML parts retain their
existing size limits.

`importxlsx.Service.ImportSelected` maps column headings on the selected
sheet and runs the same atomic SQLite import as the existing single-sheet
method. Nothing is saved until all input rows have passed validation.

This increment supplies the backend for a teacher-facing preview/confirmation
screen, which must be implemented separately.

## Русский

`importxlsx.Sheets(data)` получает названия вкладок XLSX из метаданных книги,
а `ReadNamed(data, sheetName)` определяет фактический XML-файл выбранного
листа по связям OOXML. Порядок вкладок не привязан к `sheet1.xml`.
Опасные внешние ссылки и выход за пределы архива отклоняются; размер
файла и XML-частей ограничен.

`importxlsx.Service.ImportSelected` сопоставляет заголовки выбранной
вкладки и выполняет атомарный импорт через существующую SQLite-операцию.
До завершения всех проверок записи в базу не создаются.

Следующий шаг — экран предпросмотра и подтверждения импорта преподавателем.

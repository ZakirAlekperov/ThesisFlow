# Template library and manual fixtures / Шаблоны и файлы для ручной проверки

## English

The local web UI has a separate **Templates / Шаблоны** tab at `GET /templates`.
The static binary file `internal/web/assets/testkit.zip` is bundled into the
ThesisFlow executable with Go `embed`. Files are downloadable individually;
`GET /templates/download/all` provides the entire ZIP.

The library contains:

| File | Purpose |
|---|---|
| `students_sample.xlsx` | Twenty **fictional** students for group ИСП-401 |
| `students_blank.xlsx` | Blank editable roster with the required column headings |
| `chapter1_draft.docx` | First chapter submission to put Under review |
| `chapter1_revision.docx` | Revised chapter to create version 2 and accept |
| `chapter2_sample.docx` | Separate chapter to return for revision |
| `manual.txt` | Step-by-step manual test scenario |

The Excel file starts **at row 1**, with the exact column headings **ФИО**
and **Учебный ID**, as expected by the actual XLSX importer. Extra columns
are ignored when those two headings are selected in the group's import form.
Every educational ID is unique. None of these files contains a real student's
personal data.

**No hidden database seeding:** downloading files or opening the tab does not
create groups, students, requirements, statuses, documents or any SQLite rows.
Data reaches the application only when someone uses the normal browser forms.

### Manual acceptance scenario

1. Start ThesisFlow with an **isolated empty test workspace**, not real data.
2. Create the academic group `ИСП-401`.
3. Open **Templates**, download `students_sample.xlsx`, return to the group
   and upload it using **Импорт из Excel** with headings `ФИО` and
   `Учебный ID`. Expect exactly 20 students.
4. Import the file again. Expect a duplicate-ID error and **zero partial
   inserts**.
5. Create two group requirements, `Глава 1` and `Глава 2`.
6. Open student `Иванов Артём Сергеевич`, upload Chapter 1 draft and then
   the corrected version against the same requirement. Verify retained
   versions and accept the corrected version.
7. Upload the Chapter 2 example for its requirement and send it back for
   revision. Check statuses and printable group report.
8. Restart the application to ensure all real uploaded files and saved records
   persist. Optionally exercise backup and isolated recovery.

## Русский

В локальном интерфейсе доступна отдельная вкладка **«Шаблоны»**
(`GET /templates`). Настоящие файлы XLSX/DOCX/TXT хранятся во встроенном
ZIP `internal/web/assets/testkit.zip` и входят в готовый исполняемый файл.
Можно скачать файлы по одному или весь комплект.

Главный тестовый список содержит **20 вымышленных студентов** группы
`ИСП-401`. Заголовки Excel в первой строке: **«ФИО»** и **«Учебный ID»**.
Это именно те значения, которые нужно указывать в форме импорта.
Дополнительные столбцы — направление, курс и примечание — не препятствуют
импорту, если выбраны правильные заголовки.

Документы Word проверяют два сценария: последовательную загрузку черновика
и исправленной первой главы, а также возврат второй главы на доработку.
Отдельный пустой Excel предназначен для самостоятельного заполнения.

**Скачивание ничего не добавляет в базу.** Группа, студенты, требования,
версии файлов и статусы появляются только после действий пользователя
через обычные формы ThesisFlow. Это не искусственная тестовая инициализация.

**Не проверяйте комплект на рабочей базе с настоящими студентами.**
Воспользуйтесь отдельным тестовым каталогом или резервной копией и следуйте
инструкции `manual.txt`. После проверки можно удалить сам тестовый каталог
как обычные пользовательские данные.

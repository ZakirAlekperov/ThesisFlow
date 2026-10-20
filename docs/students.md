# Student records / Учёт студентов — ThesisFlow 0.1.02

## English

ThesisFlow stores student records in SQLite and now displays them on a read-only
browser page for the selected academic group. The student creation form is separate.

The `students` table holds:

| Field | Meaning |
| --- | --- |
| `id` | Internal permanent database primary key |
| `group_id` | One required academic group, foreign key to `student_groups.id` |
| `educational_id` | User-visible student / record-book identifier |
| `educational_id_key` | Internal normalized key for global uniqueness |
| `full_name` | Student's full display name |

The internal Go API supports `students.NewRepository(db).Create(ctx, groupID, educationalID, fullName)` and `students.NewRepository(db).ListByGroup(ctx, groupID)`.

### Invariants

- A student belongs to exactly one existing academic group, enforced both in
  application validation and by a SQLite foreign key.
- A student has two different identities: a permanent internal numeric `id`
  and a required educational `educational_id` (e.g., a student record-book ID).
- Educational IDs are unique **across all groups**. Leading/trailing whitespace
  is trimmed and case differences are ignored for duplicate detection.
- Identical full names are allowed; names are not reliable unique identifiers.
- A missing full name, missing educational ID, or nonexistent group is rejected
  with a typed error. Existing records are never silently overwritten.
- Records survive a restart and calling `Init` repeatedly. Schema creation
  never drops or recreates existing data.
- The schema uses `ON DELETE RESTRICT` so accidentally deleting a group
  cannot orphan student records. Group archiving will be added separately.
- A simple index on `group_id` prepares for listing students in a group.

The students table is initialized **after** the group table, before starting
the browser interface. A dedicated schema-migration system remains future work;
this initial table is additive and compatible with existing empty/student-free
workspaces.

**Not implemented in this increment:** student creation forms, moving
students, archiving, Excel import, thesis assignments, document statuses or files.

## Русский

ThesisFlow сохраняет студентов в SQLite и теперь показывает их на странице
выбранной группы в браузере. Форма добавления студентов пока не реализована.

В таблице `students` хранятся:

| Поле | Назначение |
| --- | --- |
| `id` | Постоянный внутренний числовой ID записи |
| `group_id` | Обязательный внешний ключ на одну учебную группу |
| `educational_id` | Уникальный учебный идентификатор студента (например, номер зачётной книжки) |
| `educational_id_key` | Внутренний нормализованный ключ для проверки уникальности |
| `full_name` | Полное ФИО студента |

Внутренний Go API: `students.NewRepository(db).Create(ctx, groupID, educationalID, fullName)` и `students.NewRepository(db).ListByGroup(ctx, groupID)`.

### Правила

- Студент принадлежит **ровно одной существующей группе**. Это проверяет Go
  и дополнительно обеспечивает внешний ключ SQLite.
- Внутренний `id` и учебный `educational_id` — разные идентификаторы.
- Учебные ID уникальны **среди всех групп**; пробелы по краям удаляются,
  регистр букв при проверке повторов не учитывается.
- Одинаковые ФИО у разных студентов допустимы.
- Пустые ФИО и учебный ID, а также неизвестная группа отклоняются
  типизированными ошибками. Существующие записи не перезаписываются.
- Повторный запуск программы и `Init` сохраняют данные и постоянные ID.
- `ON DELETE RESTRICT` не допускает появления студентов без группы при
  случайном удалении группы. Архивирование будет отдельным шагом.
- Индекс `group_id` подготовлен для будущего списка студентов по группе.

Таблица студентов инициализируется **после** групп и до открытия интерфейса.
Полноценная система миграций схемы пока не реализована: на этом этапе
создаётся новая таблица без изменения существующих данных.

**Пока не реализовано:** добавление студентов через браузер,
перевод между группами, архивирование, импорт Excel, ВКР, статусы и документы.

## Group student list / Список студентов группы

### English

`ListByGroup(ctx, groupID)` reads the students of exactly one existing group
from SQLite. Its result contains the **permanent student ID**, group ID,
educational ID and full display name; no extra schema or tables are added.

- An existing group with no students returns a non-nil empty slice.
- A missing, zero or negative group ID returns `ErrGroupNotFound`; this is
  intentionally distinct from an existing empty group.
- Results are ordered by `full_name COLLATE BINARY, id` in SQLite.
  This is deterministic and preserves student identity even for identical
  names; it is **not** locale-sensitive Russian dictionary sorting.
- Database errors and cancelled contexts propagate to callers. No partial
  results are returned.
- The list works after closing/reopening the application database.
- Only the selected group is returned: students of other groups are excluded.

**Browser view:** click a group name on the home page to open `GET /groups/{id}`.
The page displays only that group's students, their permanent record IDs and
educational IDs. An existing empty group shows a friendly empty state;
missing/invalid groups return HTTP 404, and database failures return a generic
HTTP 500. All names and IDs are escaped with `html/template`.

**Next micro-step:** add a student through a browser form.

### Русский

`ListByGroup(ctx, groupID)` читает из SQLite студентов **одной существующей
группы**. Каждая запись содержит постоянный ID студента, ID группы,
учебный идентификатор и ФИО. Структура базы не меняется.

- Если группа есть, но студентов нет, возвращается пустой (не nil) срез.
- Для отсутствующей, нулевой или отрицательной группы возвращается
  `ErrGroupNotFound`, чтобы не путать её с пустой существующей группой.
- Порядок в SQLite: `full_name COLLATE BINARY, id`. Он воспроизводим и
  различает однофамильцев по ID, но **не является русской словарной
  сортировкой** с учётом языковых правил.
- Ошибки SQLite и отмена контекста передаются вызывающей стороне без
  частичного результата.
- После повторного открытия базы список и постоянные ID сохраняются.
- Студенты других групп не попадают в список выбранной группы.

**Страница в браузере:** нажмите название группы на главной странице,
чтобы открыть `GET /groups/{id}`. Отображаются только студенты выбранной
группы, их внутренние ID и номера зачёток. Пустая группа показывает понятное
сообщение, неизвестная группа возвращает HTTP 404, ошибки базы — общий
HTTP 500. ФИО и идентификаторы экранируются через `html/template`.

**Следующий микрошаг:** форма добавления студента через браузер.

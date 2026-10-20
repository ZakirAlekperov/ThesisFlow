# Student records / Учёт студентов — ThesisFlow 0.1.02

## English

This increment introduces the first **persistent student record**. It is a
database-level operation; the student browser form and list are separate steps.

The `students` table holds:

| Field | Meaning |
| --- | --- |
| `id` | Internal permanent database primary key |
| `group_id` | One required academic group, foreign key to `student_groups.id` |
| `educational_id` | User-visible student / record-book identifier |
| `educational_id_key` | Internal normalized key for global uniqueness |
| `full_name` | Student's full display name |

The internal Go API is `students.NewRepository(db).Create(ctx, groupID, educationalID, fullName)`.

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

**Not implemented in this increment:** browser forms and listings, moving
students, archiving, Excel import, thesis assignments, document statuses or files.

## Русский

Этот микрошаг добавляет первую **постоянную запись о студенте** в SQLite.
Создание студентов через браузер и список студентов будут отдельными этапами.

В таблице `students` хранятся:

| Поле | Назначение |
| --- | --- |
| `id` | Постоянный внутренний числовой ID записи |
| `group_id` | Обязательный внешний ключ на одну учебную группу |
| `educational_id` | Уникальный учебный идентификатор студента (например, номер зачётной книжки) |
| `educational_id_key` | Внутренний нормализованный ключ для проверки уникальности |
| `full_name` | Полное ФИО студента |

Внутренний Go API: `students.NewRepository(db).Create(ctx, groupID, educationalID, fullName)`.

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

**Пока не реализовано:** добавление и просмотр студентов через браузер,
перевод между группами, архивирование, импорт Excel, ВКР, статусы и документы.

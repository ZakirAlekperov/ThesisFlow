# Academic groups / Учебные группы (0.1.02)

## English

This initial increment provides **group persistence only**. It does not yet add
an HTTP endpoint or a user-facing form.

- At startup ThesisFlow creates the `student_groups` table if it does not exist.
- `groups.NewRepository(db).Create(ctx, name)` returns a stable numeric group
  ID and its trimmed display name.
- Blank names are rejected with `ErrInvalidName`.
- Names differing only by Unicode case or surrounding whitespace are treated
  as duplicates (`ErrDuplicateName`). The original record is not overwritten.
- Existing records survive restarts and repeated schema initialization.
- The normalized `name_key` column is internal; the displayed name preserves
  the user's capitalization.

For the single-user 0.1 version, group names are unique within the local
workspace. Supporting multiple academic years or organizations can later extend
this uniqueness scope without exposing `name_key` in the UI.

**Not implemented yet:** listing, renaming, archiving, student assignment,
Excel import, group folders, HTTP endpoints and database migration history.
These belong to subsequent small TDD steps.

## Русский

Этот микрошаг реализует **только сохранение учебных групп в SQLite**.
HTTP-метода и формы создания группы в интерфейсе пока нет.

- При запуске ThesisFlow создаёт таблицу `student_groups`, если её ещё нет.
- `groups.NewRepository(db).Create(ctx, name)` возвращает постоянный числовой
  идентификатор группы и название без пробелов по краям.
- Пустое название отклоняется с ошибкой `ErrInvalidName`.
- Названия, отличающиеся лишь регистром символов Unicode или пробелами по краям,
  считаются повторными (`ErrDuplicateName`). Исходная запись не меняется.
- Группы сохраняются после перезапуска и повторной инициализации таблицы.
- Внутреннее поле `name_key` нужно только для проверки уникальности; на экране
  сохраняется написание пользователя.

В индивидуальной версии 0.1 названия уникальны в пределах одного локального
рабочего пространства. В будущем при поддержке нескольких учебных годов
или организаций область уникальности можно расширить.

**Пока не реализовано:** получение списка, переименование и архивирование
групп, привязка студентов, импорт Excel, папки групп, HTTP API и история
миграций схемы. Это отдельные небольшие этапы TDD.

# Academic groups / Учебные группы (0.1.02)

## English

These increments provide group creation and listing through Go and a **read-only browser view** of saved groups. A group-creation form is not yet available.

- At startup ThesisFlow creates the `student_groups` table if it does not exist.
- `groups.NewRepository(db).Create(ctx, name)` returns a stable numeric group
  ID and its trimmed display name.
- Blank names are rejected with `ErrInvalidName`.
- Names differing only by Unicode case or surrounding whitespace are treated
  as duplicates (`ErrDuplicateName`). The original record is not overwritten.
- Existing records survive restarts and repeated schema initialization.
- `groups.NewRepository(db).List(ctx)` returns all groups with stable IDs, sorted by their internal normalized name key and then ID.
- An empty database yields a non-nil empty list. If a query fails or is cancelled, it returns an error and no partial results.
- The normalized `name_key` column is internal; the displayed name preserves
  the user's capitalization.

For the single-user 0.1 version, group names are unique within the local
workspace. Supporting multiple academic years or organizations can later extend
this uniqueness scope without exposing `name_key` in the UI.

The home page at `GET /` renders groups in repository order. For an empty workspace it displays an empty state; a database error results in HTTP 500. Group names are escaped by `html/template` so they cannot inject HTML.

**Not implemented yet:** a browser form for creating groups, renaming, archiving,
student assignment, Excel import, group folders and database migration history.
These belong to subsequent small TDD steps.

## Русский

Эти микрошаги реализуют создание и получение списка групп через Go и **отображение сохранённых групп в браузере**. Формы создания группы пока нет.

- При запуске ThesisFlow создаёт таблицу `student_groups`, если её ещё нет.
- `groups.NewRepository(db).Create(ctx, name)` возвращает постоянный числовой
  идентификатор группы и название без пробелов по краям.
- Пустое название отклоняется с ошибкой `ErrInvalidName`.
- Названия, отличающиеся лишь регистром символов Unicode или пробелами по краям,
  считаются повторными (`ErrDuplicateName`). Исходная запись не меняется.
- Группы сохраняются после перезапуска и повторной инициализации таблицы.
- `groups.NewRepository(db).List(ctx)` возвращает все группы с постоянными ID в порядке внутреннего нормализованного названия, затем ID.
- При отсутствии групп возвращается пустой срез (не `nil`). При ошибке или отмене запроса частичный список не возвращается.
- Внутреннее поле `name_key` нужно только для проверки уникальности; на экране
  сохраняется написание пользователя.

В индивидуальной версии 0.1 названия уникальны в пределах одного локального
рабочего пространства. В будущем при поддержке нескольких учебных годов
или организаций область уникальности можно расширить.

Главная страница `GET /` отображает группы в порядке хранилища. Для новой базы
выводится сообщение об отсутствии групп; ошибка чтения возвращает HTTP 500.
Названия экранируются через `html/template`, поэтому не могут внедрять HTML.

**Пока не реализовано:** форма создания групп в браузере, переименование,
архивирование, привязка студентов, импорт Excel, папки групп и миграции схемы. Это отдельные небольшие этапы TDD.

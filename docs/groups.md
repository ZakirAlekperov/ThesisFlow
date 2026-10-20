# Academic groups / Учебные группы (0.1.02)

## English

These increments provide group creation and listing through Go and a **read-only browser view** of saved groups. A browser group-creation form is available using POST /groups.

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

**Not implemented yet:** renaming, archiving,
student assignment, Excel import, group folders and database migration history.
These belong to subsequent small TDD steps.

## Русский

Эти микрошаги реализуют создание и получение списка групп через Go и **отображение сохранённых групп в браузере**. Форма создания группы доступна через POST /groups.

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

**Пока не реализовано:** переименование,
архивирование, привязка студентов, импорт Excel, папки групп и миграции схемы. Это отдельные небольшие этапы TDD.

## Browser form / Форма в браузере

### English

The home page displays a group-name field and a **Create group** button.
Submitting the form sends `POST /groups` with a per-process random CSRF
token. The form does not require JavaScript.

- Success: HTTP 303 redirect to `/` (Post/Redirect/Get); the saved group then
  appears in the list. Refreshing does not repeat the insertion.
- Blank name: HTTP 400 with an actionable Russian error.
- Existing name (case-insensitive): HTTP 409 with an actionable Russian error;
  the user input is preserved.
- Unrecognized/missing token: HTTP 403, without a database write.
- Oversized form: HTTP 413; unsupported media type: HTTP 415.
- Internal storage errors: generic HTTP 500, without filesystem/SQL details.

The token is generated from the operating system's cryptographic randomness
when the handler is created. Responses are not cached, and the page does not
load third-party JavaScript. The server remains bound to `127.0.0.1`.

### Русский

На главной странице доступны поле названия и кнопка **«Создать группу»**.
Форма отправляет `POST /groups` со случайным CSRF-токеном, действительным
только во время текущего запуска приложения. JavaScript не требуется.

- Успех: перенаправление HTTP 303 на `/` по схеме Post/Redirect/Get;
  новая группа появляется в списке, а обновление страницы не повторяет POST.
- Пустое название: HTTP 400 и понятное сообщение на русском языке.
- Повтор названия без учёта регистра: HTTP 409 с сохранением ввода.
- Отсутствующий или поддельный токен: HTTP 403 без записи в SQLite.
- Слишком большая форма: HTTP 413; неверный формат: HTTP 415.
- Ошибка SQLite: общее сообщение HTTP 500, без внутренних подробностей.

Токен создаётся при инициализации HTTP-обработчика через криптографический
генератор ОС. Кеширование страницы отключено, внешние скрипты не используются.
Сервер по-прежнему принимает подключения только на `127.0.0.1`.

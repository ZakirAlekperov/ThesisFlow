# Testing / Тестирование

## English

ThesisFlow is developed in small TDD increments (red → green → refactor). Each change begins with a test that describes observable behavior; production code follows only after the expected failure has been confirmed where practical.

Each nontrivial test must document in **English and Russian**:

- **Purpose:** which contract or rule is being protected.
- **Scenario:** the inputs or relevant preconditions.
- **Expected result:** the observable outcome, including error behavior.

Use Go's `testing` package and standard library test doubles where sufficient. Tests must be independent, deterministic, and must not depend on user files or a running server unless explicitly marked as integration tests. Avoid comments that merely repeat what the code visibly does.

To run the initial checks from the repository root:

```bash
go test ./...
go vet ./...
```

Current coverage:

| Test | Contract |
| --- | --- |
| `TestNewHandlerServesHomePage` | GET `/` returns HTML page named ThesisFlow. |
| `TestNewHandlerRejectsUnsupportedRoutes` | Unknown paths return 404 and unsupported methods return 405. |
| `TestRunStartsLoopbackHTTPAndOpensBrowser` | Opens the local URL exactly once, serves the homepage and shuts down cleanly. |
| `TestRunFailsOnOccupiedPort` | Reports an error when the requested loopback port is occupied. |
| `TestRunReturnsOutputError` | Reports the writer error while announcing the HTTP address. |
| `TestRunKeepsServerAliveWhenBrowserFails` | Browser-launch errors are nonfatal and a manual URL remains available. |
| `TestCommandForOS` | Selects shell-free launch commands for Windows, macOS and Linux; rejects unsupported platforms. |
| `TestOpenCreatesDatabase` | Creates SQLite file in a missing directory and enables foreign keys. |
| `TestOpenPreservesData` | Preserves a saved record across close and reopen. |
| `TestOpenRejectsInvalidPath` | Reports an invalid directory rather than silently switching paths. |
| `TestOpenRespectsCancellation` | Does not create the DB if the startup context is already cancelled. |
| `TestStartInitializesLocalDatabase` | Initializes SQLite before opening the HTTP interface. |
| `TestCreateGroupStoresNormalizedNameAndID` | Creates a trimmed group with a persistent SQLite ID. |
| `TestCreateGroupRejectsEmptyName` | Rejects blank group names and leaves the table unchanged. |
| `TestCreateGroupRejectsDuplicateName` | Rejects case-insensitive duplicate names without overwriting. |
| `TestCreateGroupKeepsIDAfterReopen` | Keeps group identity after reopening and reinitializing SQLite. |
| `TestStartInitializesGroupsSchema` | Creates the groups table before exposing the local HTTP interface. |
| `TestListReturnsEmptySlice` | Returns a non-nil empty group list for a fresh workspace. |
| `TestListReturnsGroupsInStableNameOrder` | Returns all group names and original IDs in repeatable normalized-name order. |
| `TestListPersistsAcrossReopen` | Lists the original group and ID after closing and reopening SQLite. |
| `TestListRespectsCanceledContext` | Preserves context cancellation and avoids partial results. |

The web tests use `httptest` and verify the handler without opening a listening socket. Browser-launch integration is tested with an injected opener (no real browser is launched during tests). The new SQLite tests use a temporary directory and do not modify the user's database. Only the minimal group table is initialized; student tables, schema migrations, document storage and the HTTP group UI have **not** been implemented yet. Server integration tests use an ephemeral loopback port to avoid conflicts.

## Русский

ThesisFlow разрабатывается небольшими шагами TDD («красный» → «зелёный» → рефакторинг). Каждое изменение начинается с теста, описывающего наблюдаемое поведение; рабочий код добавляется после подтверждения ожидаемого падения теста, когда это практически возможно.

Каждый нетривиальный тест документируется **на русском и английском языках**:

- **Назначение:** какой контракт или правило защищает тест.
- **Сценарий:** входные данные и важные предусловия.
- **Ожидаемый результат:** наблюдаемое поведение, включая обработку ошибок.

Используем стандартный пакет `testing` и простые тестовые имитаторы Go, если этого достаточно. Тесты должны быть независимыми, детерминированными и не зависеть от пользовательских файлов или запущенного сервера, кроме специально обозначенных интеграционных тестов. Не добавляем комментарии, которые лишь пересказывают очевидные строки кода.

Запуск проверок из корня репозитория:

```bash
go test ./...
go vet ./...
```

Текущие проверки:

| Тест | Контракт |
| --- | --- |
| `TestNewHandlerServesHomePage` | GET `/` возвращает HTML-страницу ThesisFlow. |
| `TestNewHandlerRejectsUnsupportedRoutes` | Неизвестные пути возвращают 404, неподдерживаемые методы — 405. |
| `TestRunStartsLoopbackHTTPAndOpensBrowser` | Передаёт локальный URL в браузер один раз, обслуживает главную страницу и корректно останавливается. |
| `TestRunFailsOnOccupiedPort` | Возвращает ошибку, если указанный локальный порт занят. |
| `TestRunReturnsOutputError` | Передаёт ошибку записи при выводе адреса сервера. |
| `TestRunKeepsServerAliveWhenBrowserFails` | Ошибка открытия браузера не останавливает сервер, адрес остаётся доступен вручную. |
| `TestCommandForOS` | Выбирает команды без оболочки для Windows, macOS и Linux; неизвестные платформы отклоняет. |
| `TestOpenCreatesDatabase` | Создаёт SQLite в новом каталоге и включает внешние ключи. |
| `TestOpenPreservesData` | Сохраняет запись после закрытия и повторного открытия. |
| `TestOpenRejectsInvalidPath` | Сообщает о недопустимом каталоге, не меняя путь молча. |
| `TestOpenRespectsCancellation` | Не создаёт БД при заранее отменённом контексте. |
| `TestStartInitializesLocalDatabase` | Создаёт SQLite перед открытием HTTP-интерфейса. |
| `TestCreateGroupStoresNormalizedNameAndID` | Создаёт группу с очищенным названием и постоянным ID. |
| `TestCreateGroupRejectsEmptyName` | Запрещает пустые названия без изменения таблицы. |
| `TestCreateGroupRejectsDuplicateName` | Запрещает повторные названия независимо от регистра. |
| `TestCreateGroupKeepsIDAfterReopen` | Сохраняет ID группы после повторного открытия и инициализации SQLite. |
| `TestStartInitializesGroupsSchema` | Создаёт таблицу групп до запуска HTTP-интерфейса. |
| `TestListReturnsEmptySlice` | Возвращает непустое значение среза без элементов для новой базы. |
| `TestListReturnsGroupsInStableNameOrder` | Возвращает названия и исходные ID в стабильном порядке. |
| `TestListPersistsAcrossReopen` | Возвращает сохранённую группу с прежним ID после перезапуска. |
| `TestListRespectsCanceledContext` | Учитывает отмену контекста без частичного результата. |

HTTP-тесты используют `httptest` и проверяют обработчик без открытия сетевого порта. Вызов открытия браузера проверяется через тестовый имитатор (настоящий браузер в тестах не запускается). SQLite-тесты используют временные каталоги и не изменяют базу пользователя. Создаётся только минимальная таблица групп; таблицы студентов, миграции схемы, файловое хранилище и интерфейс управления группами пока **не** реализованы. Интеграционные тесты используют свободный локальный порт, чтобы исключить конфликты.

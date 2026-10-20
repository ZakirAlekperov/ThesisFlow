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
| `TestHomePageShowsEmptyGroupsState` | Shows a clear empty state when the group list is empty. |
| `TestHomePageListsSavedGroups` | Renders saved group names and IDs in repository order. |
| `TestHomePageEscapesGroupNames` | Escapes HTML-like group names so they cannot inject markup. |
| `TestHomePageHandlesGroupQueryError` | Responds with generic HTTP 500 on data-access errors without leaking details. |
| `TestStartServesGroupsFromSQLite` | Loads an existing SQLite group through the real HTTP startup path. |
| `TestGroupFormIsAvailable` | Exposes accessible POST form and a random CSRF token. |
| `TestGroupFormCreatesAndRedirects` | Calls Create once and redirects with 303 on success. |
| `TestGroupFormShowsValidationErrors` | Shows errors for blank or duplicate names and preserves input. |
| `TestGroupFormRejectsMissingAndIncorrectCSRFToken` | Blocks unauthorized POSTs before writing data. |
| `TestGroupFormRejectsOversizeBody` | Rejects oversized input without database writes. |
| `TestGroupFormHidesStorageErrors` | Hides internal database failure details from browser. |
| `TestStartCreatesGroupThroughBrowser` | Submits a real browser form and persists the group after shutdown. |
| `TestCreateStudentStoresNormalizedFields` | Saves the student’s name, educational ID and exactly one group with a permanent ID. |
| `TestCreateStudentRejectsMissingFields` | Rejects blank names/IDs and nonpositive group IDs without writing records. |
| `TestCreateStudentRequiresExistingGroup` | Rejects references to nonexistent academic groups. |
| `TestCreateStudentRejectsDuplicateIDAcrossGroups` | Enforces globally unique educational IDs independent of group and case. |
| `TestCreateStudentSurvivesReopen` | Preserves all student fields and IDs across database restart. |
| `TestStudentsTableEnforcesForeignKey` | Rejects orphan students even when inserted by direct SQL. |
| `TestCreateStudentRespectsCancellation` | Propagates context cancellation and avoids writes. |
| `TestListByGroupReturnsEmptySlice` | Returns an empty non-nil list for an existing group with no students. |
| `TestListByGroupReturnsOnlyRequestedGroupInNameOrder` | Filters other groups and orders names and same-name records deterministically. |
| `TestListByGroupRejectsUnknownGroup` | Distinguishes missing groups from existing empty groups. |
| `TestListByGroupSurvivesReopen` | Preserves group-scoped records and student IDs across SQLite restart. |
| `TestListByGroupRespectsCancellation` | Returns context cancellation without partial data. |
| `TestListByGroupPropagatesDatabaseErrors` | Reports SQLite errors instead of a misleading empty list. |
| `TestHomePageLinksToGroupStudents` | Links each group to a read-only student page using its permanent ID. |
| `TestGroupPageShowsStudentsAndStableIDs` | Displays only the selected group's names, educational IDs and persistent record IDs. |
| `TestGroupPageShowsEmptyState` | Shows an empty state for an existing group without students. |
| `TestGroupPageRejectsUnknownAndInvalidIDs` | Returns 404 for invalid or missing groups without reading student records. |
| `TestGroupPageEscapesNamesAndIDs` | Escapes untrusted group/student fields to prevent HTML injection. |
| `TestGroupPageHidesDatabaseErrors` | Uses generic HTTP 500 messages rather than revealing storage failures. |
| `TestGroupPageRejectsWriteMethods` | Keeps the new student page read-only with HTTP 405. |
| `TestStartServesPersistedStudentsByGroup` | Serves existing group students through the real SQLite-backed HTTP server. |
| `TestStartInitializesStudentsSchema` | Initializes the students table before HTTP startup. |

The web tests use `httptest` and verify the handler without opening a listening socket. Browser-launch integration is tested with an injected opener (no real browser is launched during tests). The new SQLite tests use a temporary directory and do not modify the user's database. Group and student tables are initialized. Full schema migrations, student pages, editing, archiving, Excel import and document storage have **not** been implemented yet. Student tests use temporary SQLite files and leave the user's database unchanged. Browser group creation is covered by CSRF and HTTP integration tests. The home page now reads groups from SQLite using an injected repository. Server integration tests use an ephemeral loopback port to avoid conflicts.

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
| `TestHomePageShowsEmptyGroupsState` | Показывает понятное сообщение при отсутствии групп. |
| `TestHomePageListsSavedGroups` | Выводит названия и постоянные ID в порядке базы. |
| `TestHomePageEscapesGroupNames` | Экранирует HTML-подобные названия от внедрения разметки. |
| `TestHomePageHandlesGroupQueryError` | Возвращает HTTP 500 без раскрытия деталей ошибки БД. |
| `TestStartServesGroupsFromSQLite` | Показывает группу из существующей SQLite через настоящий HTTP-сервер. |
| `TestGroupFormIsAvailable` | Показывает POST-форму и случайный CSRF-токен. |
| `TestGroupFormCreatesAndRedirects` | Создаёт группу один раз и возвращает 303. |
| `TestGroupFormShowsValidationErrors` | Показывает ошибки пустого или повторного названия с сохранением ввода. |
| `TestGroupFormRejectsMissingAndIncorrectCSRFToken` | Блокирует посторонние POST до записи данных. |
| `TestGroupFormRejectsOversizeBody` | Отклоняет слишком большие формы без изменения SQLite. |
| `TestGroupFormHidesStorageErrors` | Скрывает внутренние подробности ошибки БД. |
| `TestStartCreatesGroupThroughBrowser` | Отправляет настоящую форму и проверяет сохранность группы. |
| `TestCreateStudentStoresNormalizedFields` | Сохраняет ФИО, учебный ID и одну группу с постоянным ID. |
| `TestCreateStudentRejectsMissingFields` | Запрещает пустые поля и неположительные ID групп без записи данных. |
| `TestCreateStudentRequiresExistingGroup` | Запрещает ссылку на несуществующую учебную группу. |
| `TestCreateStudentRejectsDuplicateIDAcrossGroups` | Обеспечивает глобальную уникальность учебного ID между группами. |
| `TestCreateStudentSurvivesReopen` | Сохраняет все поля и постоянные ID после перезапуска SQLite. |
| `TestStudentsTableEnforcesForeignKey` | Запрещает студентов без группы даже при прямом SQL-запросе. |
| `TestCreateStudentRespectsCancellation` | Учитывает отмену контекста без записи данных. |
| `TestListByGroupReturnsEmptySlice` | Для существующей группы без студентов возвращает пустой (не nil) срез. |
| `TestListByGroupReturnsOnlyRequestedGroupInNameOrder` | Отделяет студентов нужной группы и сохраняет стабильный порядок. |
| `TestListByGroupRejectsUnknownGroup` | Отличает несуществующую группу от существующей пустой. |
| `TestListByGroupSurvivesReopen` | Сохраняет студенческие ID и список группы после перезапуска SQLite. |
| `TestListByGroupRespectsCancellation` | Возвращает отмену контекста без частичных результатов. |
| `TestListByGroupPropagatesDatabaseErrors` | Возвращает ошибки SQLite вместо вводящего в заблуждение пустого списка. |
| `TestHomePageLinksToGroupStudents` | Открывает страницу студентов по постоянному ID группы. |
| `TestGroupPageShowsStudentsAndStableIDs` | Показывает студентов выбранной группы с ФИО и обоими идентификаторами. |
| `TestGroupPageShowsEmptyState` | Показывает сообщение для существующей группы без студентов. |
| `TestGroupPageRejectsUnknownAndInvalidIDs` | Возвращает HTTP 404 для неизвестных ID без запросов студентов. |
| `TestGroupPageEscapesNamesAndIDs` | Экранирует поля групп и студентов, защищая от HTML-инъекций. |
| `TestGroupPageHidesDatabaseErrors` | Возвращает общий HTTP 500, не раскрывая подробности SQLite. |
| `TestGroupPageRejectsWriteMethods` | Не разрешает запись на странице студентов (HTTP 405). |
| `TestStartServesPersistedStudentsByGroup` | Показывает сохранённых студентов через настоящий HTTP-сервер и SQLite. |
| `TestStartInitializesStudentsSchema` | Создаёт таблицу студентов до запуска HTTP-сервера. |

HTTP-тесты используют `httptest` и проверяют обработчик без открытия сетевого порта. Вызов открытия браузера проверяется через тестовый имитатор (настоящий браузер в тестах не запускается). SQLite-тесты используют временные каталоги и не изменяют базу пользователя. Инициализируются таблицы групп и студентов. Полноценные миграции схемы, страницы студентов, редактирование, архивирование, импорт Excel и файловое хранилище пока **не** реализованы. Тесты студентов используют временную SQLite и не затрагивают базу пользователя. Создание групп в браузере проверяется с учётом CSRF и HTTP-интеграции. Главная страница теперь читает группы из SQLite через внедрённый репозиторий. Интеграционные тесты используют свободный локальный порт, чтобы исключить конфликты.

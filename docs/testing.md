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
| `TestRunWritesApplicationName` | Writes exactly `ThesisFlow\n` on successful startup output. |
| `TestRunReturnsOutputError` | Returns the writer's error instead of silently ignoring it. |

These tests do **not** verify HTTP serving, browser startup, SQLite initialization, or file storage; those features have not been implemented yet.

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
| `TestRunWritesApplicationName` | При успешной записи выводит строго `ThesisFlow\n`. |
| `TestRunReturnsOutputError` | Возвращает ошибку потока, а не игнорирует её. |

Проверки пока **не** охватывают HTTP-сервер, открытие браузера, инициализацию SQLite и файловое хранилище: эти функции ещё не реализованы.

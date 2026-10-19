# Contributing to ThesisFlow / Участие в разработке ThesisFlow

## English

ThesisFlow is a small Go application developed test-first. Keep changes focused and avoid adding infrastructure or features outside the current milestone.

### Branches and pull requests

- `main` contains reviewed, stable changes; do not commit directly to it.
- Create a short-lived branch from `main`: `feature/*`, `fix/*`, `docs/*`, `refactor/*`, or `chore/*`.
- Write tests before implementation whenever feasible (red → green → refactor).
- Every nontrivial test must document **purpose, scenario, and expected result in both English and Russian**.
- Document new code and public contracts in both languages.
- Open a pull request targeting `main`. Explain the change and testing performed.
- Merge only after review and successful checks.

Run locally from the repository root:

```bash
go test ./...
go vet ./...
gofmt -l .
```

`gofmt -l .` must produce no output. See [docs/testing.md](docs/testing.md) for the testing policy.

## Русский

ThesisFlow — небольшое Go-приложение, разрабатываемое по TDD. Изменения должны быть компактными; не добавляем инфраструктуру и функциональность за пределами текущего этапа.

### Ветки и Pull Request

- `main` содержит проверенные стабильные изменения; напрямую в неё не коммитим.
- Создаём короткоживущие ветки от `main`: `feature/*`, `fix/*`, `docs/*`, `refactor/*` или `chore/*`.
- По возможности сначала пишем тесты, затем реализацию («красный» → «зелёный» → рефакторинг).
- Каждый нетривиальный тест документируется **на русском и английском**: назначение, сценарий, ожидаемый результат.
- Новый код и публичные контракты также документируем на обоих языках.
- Изменения отправляем через Pull Request в `main` с описанием и результатами проверок.
- Объединяем только после ревью и успешных проверок.

Команды проверки из корня репозитория:

```bash
go test ./...
go vet ./...
gofmt -l .
```

Команда `gofmt -l .` не должна выводить ничего. Подробные требования к тестам: [docs/testing.md](docs/testing.md).

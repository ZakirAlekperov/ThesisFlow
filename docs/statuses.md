# Material statuses / Состояния сдачи документов — 0.1.05

## English

For each pair of **student + applicable requirement**, ThesisFlow tracks
a current status and an append-only history of changes. The four statuses are:

- `missing` — not submitted / «Не сдан»
- `under_review` — waiting for teacher review / «На проверке»
- `needs_revision` — sent back for corrections / «На доработке»
- `accepted` — accepted / «Принят»

The internal API is `materials.NewRepository(db)`, `Init(ctx)`,
`Get(ctx,studentID,requirementID)`, `Set(ctx,studentID,requirementID,status)`
and `History(ctx,studentID,requirementID)`.

Unsubmitted documents have the virtual `missing` status and no fake history.
A real status change updates the current value and appends its event in
**one SQLite transaction**. Repeating the same value is idempotent; it does
not create a duplicate event. The timestamp is registered from the
application system clock, not supplied by the user. Requirements from
different groups or assigned to a different student cannot be modified.

This is the core status engine, not yet a complete teacher UI for review.
Versions of actual files, approval and projections are separate steps.

## Русский

Для каждой пары **студент + относящееся к нему требование** ThesisFlow
хранит текущий статус и неизменяемую историю изменений.

- `missing` — «Не сдан».
- `under_review` — «На проверке».
- `needs_revision` — «На доработке».
- `accepted` — «Принят».

Go API: `materials.NewRepository(db)`, `Init(ctx)`,
`Get(ctx,studentID,requirementID)`, `Set(ctx,studentID,requirementID,status)`,
`History(ctx,studentID,requirementID)`.

До первой сдачи документ имеет виртуальный статус «Не сдан» без
искусственно созданных событий истории. При изменении статуса запись
текущего состояния и новое историческое событие сохраняются **одной
транзакцией SQLite**. Повтор того же состояния не дублирует события.
Дата и время фиксируются по системным часам, пользователь их не меняет.
Статус чужого документа изменить нельзя.

Это пока механизм хранения статусов. Экран работы преподавателя, загрузка
версий файлов и подтверждение принятых документов будут отдельными шагами.

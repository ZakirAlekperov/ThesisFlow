# Readiness and deadlines / Готовность и сроки — 0.1.08

## English

The `readiness.Service.Group(ctx, groupID, day)` method calculates progress
directly from students, applicable group/student requirements and current material
statuses. The numerator counts **Accepted only**; the denominator is the number
of applicable requirements across students. Zero requirements means 0% (no false
completion). A requirement is overdue only if its due date precedes the supplied
calendar day and its status is not Accepted. No mutable cached percentage or
custom submission date is introduced.

This increment exposes a Go service. Browser metrics and deadline editing remain
separate integration work.

## Русский

Метод `readiness.Service.Group(ctx, groupID, day)` считает готовность группы
непосредственно по студентам, применимым требованиям и текущим статусам
документов. В числитель входят **только принятые документы**; знаменатель —
общее число требований для студентов группы. При отсутствии требований
готовность составляет 0%, а не ложные 100%. Просроченными считаются
непринятые документы, срок которых строго раньше заданной календарной даты.
Отдельных изменяемых процентов и ручной даты сдачи нет.

Этот микрошаг добавляет Go-сервис. Вывод показателей в интерфейсе и
редактирование сроков пока не подключены.

Dependencies / Зависимости: PR #20 (review and accepted-file projection).

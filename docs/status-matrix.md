# Status matrix / Общая ведомость документов

## English

The group page links to `GET /groups/{id}/matrix`. The teacher sees a
wide server-rendered table of active students versus the **group-wide**
document requirements. Columns follow the requirements repository's stable
ordering, rows follow the student repository's ordering.

Every cell reflects the persisted state: Missing, Under review, Needs
revision or Accepted. The page uses Go `html/template`, inlined custom CSS,
an accessible HTML table and a print-friendly layout. There are no SPA
frameworks or external JavaScript dependencies.

Individual student-only requirements remain on each student's personal
document page rather than becoming sparse columns across the entire group.
The table is calculated on request and does not duplicate statuses in SQLite.

## Русский

На странице группы появилась ссылка «Таблица сдачи документов группы»,
которая открывает `GET /groups/{id}/matrix`. Таблица содержит активных
студентов по строкам и **общие обязательные документы** по столбцам.

В ячейках отображаются текущие статусы: «Не сдан», «На проверке»,
«На доработке», «Принят». Данные читаются из SQLite при каждом открытии,
отдельная таблица со скопированными статусами не создаётся.

Интерфейс построен через Go `html/template`, собственные CSS и обычный
HTML `table` с прокруткой по горизонтали и стилями для печати.
React, Vue и другие SPA-фреймворки не используются.

Индивидуальные документы конкретного студента остаются в его карточке:
они не добавляются редкими отдельными столбцами для всей группы.

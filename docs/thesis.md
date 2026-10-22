# Thesis topic and order / Тема ВКР и приказ — 0.1.14

## English

Each student has **at most one** thesis record in SQLite
(`thesis_records.student_id` is its primary key). The topic and order
reference are stored independently of submitted file versions.

`thesis.Repository.Set(ctx, studentID, topic, orderRef)` requires a real
student, a non-empty topic and non-empty order reference. Changing an approved
topic cannot reuse the previous order reference. An invalid edit leaves
existing data unchanged. `Get` returns an empty record until the topic is
approved, rather than inventing one.

This increment provides domain persistence; the teacher form and history of
administrative approval orders are separate follow-ups.

## Русский

У каждого студента **не более одной ВКР**. Её тема и реквизиты приказа
хранятся в `thesis_records` отдельно от загружаемых документов.

`thesis.Repository.Set(ctx, studentID, topic, orderRef)` проверяет
существование студента, непустую тему и реквизиты приказа. Изменить ранее
утверждённую тему с тем же приказом нельзя: нужны новые реквизиты.
Некорректное изменение не затрагивает ранее сохранённые данные.
До утверждения темы `Get` возвращает пустую запись.

Пока реализован слой хранения; форма преподавателя и полноценный журнал
приказов будут добавлены отдельно.

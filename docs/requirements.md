# Document requirements / Требования к документам (0.1.04)

## English

ThesisFlow stores expected graduation-thesis materials in the SQLite
`requirements` table. Each requirement has a stable ID, title, optional
deadline (ISO `YYYY-MM-DD`), group ID, and optional student ID.

A requirement with `student_id IS NULL` is a **group-wide requirement**
visible to every student assigned to that group. A requirement with
`student_id` set is **student-specific** and visible only to the selected
student. The group-wide definition is stored once: it is not copied into
individual student rows, so changes to group profiles can propagate naturally.

Internal API:

- `requirements.NewRepository(db).Init(ctx)`
- `Create(ctx, groupID, studentID, title, deadline)` (`studentID == 0`
  means the full group)
- `ListForGroup(ctx, groupID)`
- `ListForStudent(ctx, groupID, studentID)`

Titles are trimmed and must be unique within their exact group or personal
scope (case-insensitive). A student cannot receive a personal requirement
under an unrelated group. Deadlines are optional but must be valid ISO calendar
dates when present. SQL foreign keys restrict accidental parent deletion.

On a group page, the teacher can create and view **group-wide** requirements.
The POST form uses the existing CSRF token. More advanced requirement profile
editing, individual student forms, document status and file management are
future steps.

## Русский

ThesisFlow хранит обязательные материалы ВКР в таблице SQLite
`requirements`. У каждого требования есть постоянный ID, название,
необязательный срок сдачи (`ГГГГ-ММ-ДД`), ID группы и, при необходимости,
ID конкретного студента.

Когда `student_id IS NULL`, документ требуется **всем студентам группы**.
Если указан `student_id`, требование индивидуальное. Общее требование
хранится **один раз**: список не размножается по всем студентам, поэтому
изменения профиля группы автоматически видны в их карточках.

Внутренний API:

- `requirements.NewRepository(db).Init(ctx)`
- `Create(ctx, groupID, studentID, title, deadline)` (для всей группы
  укажите `studentID == 0`)
- `ListForGroup(ctx, groupID)`
- `ListForStudent(ctx, groupID, studentID)`

Название очищается от пробелов и должно быть уникально в своей группе
или индивидуальной области независимо от регистра. Нельзя назначить
требование студенту чужой группы. Срок может отсутствовать, но заданная
дата обязана быть настоящей календарной датой ISO. Внешние ключи SQLite
защищают целостность связей.

На странице учебной группы преподаватель может **создавать и просматривать**
общие требования. Форма защищена тем же CSRF-токеном. Редактирование
профилей, индивидуальные формы, статусы и загрузка документов будут
разработаны отдельно.

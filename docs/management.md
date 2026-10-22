# Record management / Управление записями — 0.1.12

## English

The `management.Repository` updates groups/students **in place**:
`RenameGroup`, `UpdateStudent`, `MoveStudent`, `SetGroupArchived` and
`SetStudentArchived`. Permanent internal IDs never change. Global
case-insensitive educational ID uniqueness is retained; a target group must
exist and be active. Archival is reversible and never deletes the corresponding
document history. Missing records, invalid fields and duplicate identifiers
return typed errors.

This increment implements the persistence layer. An archive view and
user-facing editing forms require their own HTTP tests and follow-up PRs.
Updating the group name does not yet relocate previously accepted files;
do not treat this as a completed filesystem-rename operation.

## Русский

`management.Repository` меняет существующие записи **без создания новых ID**:
переименование группы, исправление ФИО/учебного номера, перевод между
группами, архивирование и восстановление. Номер студента уникален между
группами; перенос допускается только в существующую активную группу.
Исторические версии файлов при архивировании не удаляются.

Этот этап добавляет слой хранения. Формы браузера и просмотр архива будут
следующими отдельными PR. Переименование группы пока не перемещает ранее
принятые файлы — файловые операции необходимо согласовать дополнительно.

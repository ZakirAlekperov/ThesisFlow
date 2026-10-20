# Document versions / Версии документов — ThesisFlow 0.1.06

## English

ThesisFlow stores every submitted document revision separately under its
application data directory. A dedicated `file_versions` SQLite table records
the stable file ID, student and requirement IDs, original name, internal
relative path, size, SHA-256 checksum and system registration timestamp.

Internal path format:

```text
ThesisFlow/
  thesisflow.sqlite
  storage/
    .internal/
      versions/
        <student-ID>/
          <requirement-ID>/
            <random-id>.docx
```

`fileversions.NewStore(db, root)` provides `Init`,
`Save(ctx,studentID,requirementID,filename,bytes)`, `List` and `Read`.
Every uploaded version is immutable. Filenames from the student's upload are
never used as storage paths. A valid submission updates the material status to
`under_review` and appends an event **in the same SQLite transaction** as the
new version metadata.

File bytes are written to a temporary path, synchronized and renamed before
the database commit; if the database write fails, the new file is removed.
A sudden power failure in the small filesystem/SQLite boundary may leave an
unreferenced file, so a recovery scan will eventually be needed.

Only common document extensions are allowed: DOCX, PDF, ODT, RTF, TXT, XLSX.
Maximum file size for the initial release is 20 MiB.

The human-readable **accepted** directory is deliberately separate and will
be created by the approval step. Uploading another version does **not**
overwrite any old accepted document. Physical local backups are planned
separately.

## Русский

Каждая полученная версия документа хранится отдельно в служебной части
локального хранилища. SQLite-таблица `file_versions` фиксирует постоянный
ID версии, студента, требование, исходное имя, относительный путь, размер,
SHA-256 и системное время регистрации.

Внутренняя структура:

```text
ThesisFlow/
  thesisflow.sqlite
  storage/
    .internal/
      versions/
        <ID-студента>/
          <ID-требования>/
            <случайный-ID>.docx
```

API: `fileversions.NewStore(db, root)`; методы `Init`, `Save`,
`List`, `Read`. Каждая версия неизменяема; исходное имя файла не
используется как путь. Новая сдача переводит документ в «На проверке» и
пишет событие истории **в рамках одной транзакции SQLite** с записью версии.

Файл сначала записывается во временный путь, синхронизируется и
переименовывается. При неудачной записи в базу новый файл удаляется.
При внезапном выключении питания между файловой операцией и фиксацией
SQLite теоретически возможен лишний служебный файл — позднее потребуется
проверка и восстановление согласованности.

Допустимые расширения: DOCX, PDF, ODT, RTF, TXT, XLSX.
Максимальный размер — 20 МиБ.

**Человекочитаемая папка принятых документов** будет создана на этапе
подтверждения преподавателем. Загрузка новой версии не заменяет старый
принятый документ. Резервное копирование — отдельный этап.

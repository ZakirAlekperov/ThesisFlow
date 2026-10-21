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

## Accepted-file projection / Папка принятых документов (0.1.07)

### English

The approval service `review.NewService(db, versions)` provides
`Init(ctx)`, `Accept(ctx,studentID,requirementID,versionID)` and
`Reject(ctx,studentID,requirementID,versionID)`.

When a teacher accepts a version, ThesisFlow creates a readable copy:

```text
storage/
  ИСП_401 [1]/
    Иванов Иван [ЗК_001-17]/
      Глава 1.docx
  .internal/
    versions/
      ... every historical version, unchanged ...
```

The original upload name is **never** used as the accepted filename. The
requirement title supplies the filename, and the extension comes from the
reviewed version. Folder names retain readable group/student information;
internal numeric IDs eliminate collisions. Characters invalid on macOS,
Linux or Windows are normalized.

A replacement upload changes status back to `under_review` but **does not**
replace the previously accepted file. Only a new `Accept` replaces the
accepted projection. The previous accepted copy is backed up temporarily
during the replacement and restored if the database update fails. Rejection
marks the current document `needs_revision` and leaves the accepted copy.

Approval updates `accepted_files`, the accepted version flag, the current
material status and its event history in one SQLite transaction. The
filesystem/SQL boundary still requires power-loss recovery tooling in a
future reliability pass. A full backup should include both the database and
the entire `storage` directory.

### Русский

Служба `review.NewService(db, versions)` предоставляет методы
`Init`, `Accept` («Принять») и `Reject` («На доработку»).

После приёмки формируется понятная файловая структура:

```text
storage/
  ИСП_401 [1]/
    Иванов Иван [ЗК_001-17]/
      Глава 1.docx
  .internal/
    versions/
      ... все исторические версии документов ...
```

Имя исходного файла не влияет на имя принятого документа: используется
название требования, например «Глава 1.docx». В именах каталогов сохраняются
ФИО/название группы и уникальные ID; запрещённые символы ОС нормализуются.

Повторная загрузка переводит документ «На проверке», но **не удаляет
ранее принятую копию**. Замена происходит исключительно при новой приёмке.
Старая копия временно защищена резервным переименованием и восстанавливается
при ошибке записи SQLite. При отправке на доработку принятая копия остаётся.

Решение преподавателя, флаг принятой версии, актуальный статус и история
фиксируются одной транзакцией SQLite. Но SQLite и файловая система не имеют
общей транзакции: для полной защиты от внезапного обесточивания потребуется
дополнительный механизм восстановления. Резервная копия обязана включать
и SQLite, и **весь** каталог `storage`.

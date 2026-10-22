# Browser backup recovery / Восстановление через браузер

## English

The Settings tab now supports downloading a backup ZIP and uploading one for
validation and isolated restoration. The incoming multipart form is protected
by CSRF and an archive-size bound. The existing `backup.Restore` service
validates checksums and rejects malicious paths, links, corrupt manifests
and attempts to overwrite existing destinations.

The recovered workspace is placed under the user's private ThesisFlow
`restores/restore-<random>` directory. The application reports its location
but **does not automatically replace the currently open SQLite database**.
This is intentional: teachers must inspect a recovered copy first and
choose when to switch while the app is stopped.

The ZIP is unencrypted and may contain personal student records. No data
are sent to an external network service; all work is performed locally.
Do not upload archives received from untrusted parties.

## Русский

В разделе «Настройки» можно скачать ZIP-копию и загрузить архив для
проверки и восстановления в **отдельный каталог**. Загрузка защищена
CSRF-токеном и ограничением размера. Существующий сервис `backup.Restore`
проверяет SHA-256, не допускает опасных путей, символических ссылок,
повреждённых манифестов и перезаписи существующих данных.

Восстановленная папка создаётся внутри `ThesisFlow/restores/restore-<случайное>`
в пользовательском каталоге. Приложение показывает её путь, но
**не подменяет открытую рабочую базу**. Для использования восстановленных
данных необходимо проверить их и выполнить переключение после закрытия
приложения.

Архив не зашифрован и содержит персональные данные. Обработка выполняется
локально, без облачных сервисов. Не загружайте архивы из недоверенных
источников.

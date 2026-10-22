# Backup in the browser / Резервная копия через браузер

## English

The teacher dashboard links to `GET /settings`. The settings page offers
**Download backup**. A CSRF-protected `POST /settings/backup` exports SQLite
using the existing VACUUM INTO snapshot mechanism and bundles the files under
`storage/`. The archive is streamed as an attachment and deleted from the
temporary directory afterward.

The web handler uses a writer gate: during export, other browser POST
operations wait so that files and corresponding database records are not
modified by concurrent browser uploads. **Caveat:** external processes
modifying the storage directory are not controlled by this mutex. For the
strongest guarantee, close the application and use the CLI export command.

The backup includes personal data and is **not encrypted**. Store it in a
protected location. Restoration is intentionally still restricted to a
separate destination through `thesisflow backup restore ZIP DIRECTORY`.
Overwriting active data directly from the browser would be unsafe.

## Русский

На панели преподавателя появилась ссылка `/settings` — «Настройки».
Кнопка «Скачать резервную копию» вызывает защищённый CSRF-токеном
`POST /settings/backup`. Сервер создаёт снимок SQLite через VACUUM INTO,
добавляет файлы из `storage/`, отправляет ZIP и удаляет временный файл.

На время экспорта прочие POST-запросы из интерфейса ждут завершения
копирования. **Ограничение:** процессы, непосредственно изменяющие файлы
вне ThesisFlow, не блокируются. Для максимальной надёжности закройте
приложение и используйте команду резервного копирования из терминала.

В ZIP содержатся персональные данные **без шифрования**. Храните архив
безопасно. Восстановление разрешено только в новую папку командой
`thesisflow backup restore ZIP DIRECTORY`; перезапись действующей базы
из браузера не предусмотрена из соображений сохранности данных.

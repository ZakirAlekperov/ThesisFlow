# Platform packages / Пакеты для установки ThesisFlow 0.2.0

## English

GitHub Actions now publishes **developer-preview packages** alongside the
existing portable Go executables, without Node.js, Docker, or CGO.

- **Windows AMD64:** ZIP with `thesisflow.exe`, `install.ps1`,
  `uninstall.ps1`. Running `install.ps1` in PowerShell copies the program
  to the current user's `LOCALAPPDATA/Programs/ThesisFlow` and creates a
  Start Menu shortcut. No administrator privileges are required.
  `uninstall.ps1` removes only the binary and shortcut — **never** the
  database, thesis documents or backups.
- **macOS ARM64 / AMD64:** a zipped `ThesisFlow.app` bundle containing
  the native Go binary and an `Info.plist`. A drag to Applications installs
  the preview; no external JavaScript runtimes.
- **Linux AMD64:** a `.deb` package placing the binary in
  `/usr/local/bin/thesisflow`. The existing `.tar.gz` portable artifact
  remains available.

**Not signed or notarized.** Windows SmartScreen or macOS Gatekeeper may warn.
Do not bypass organization security policies to install unsigned preview
software. Official distribution requires signing/notarization certificates,
platform-specific manual QA and release approval. The package scripts do not
create/delete student records.

## Русский

GitHub Actions создаёт предварительные пакеты для установки приложения:

- **Windows AMD64:** ZIP с `thesisflow.exe`, `install.ps1`,
  `uninstall.ps1`. Установка выполняется для текущего пользователя,
  добавляется ярлык меню «Пуск», права администратора не нужны. Удаление
  программы **не трогает** базу SQLite, документы и резервные копии.
- **macOS ARM64 / AMD64:** архив с `ThesisFlow.app`, который можно
  скопировать в «Программы».
- **Linux AMD64:** пакет `.deb` для установки бинарника в
  `/usr/local/bin/thesisflow`; переносимый `.tar.gz` также остаётся.

Пакеты **не подписаны цифровой подписью и не нотариализованы Apple**.
Системы защиты могут предупреждать об этом. Не обходите требования
безопасности организации ради установки предварительной сборки.
Для официального выпуска нужны сертификаты подписи и ручные испытания
на целевых операционных системах.

В интерфейсе используются Go `html/template`, собственные CSS/JS и SQLite;
React и отдельная сборка фронтенда не требуются.

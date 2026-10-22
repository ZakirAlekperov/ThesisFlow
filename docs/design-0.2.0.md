# ThesisFlow 0.2.0 server-rendered UI / Интерфейс на шаблонах Go

## English

ThesisFlow's teacher dashboard at `/dashboard` calculates current group-level
student count, accepted documents, overdue requirements and weighted readiness
percent from existing repositories. The dedicated `/search?q=...` endpoint
finds up to 100 active student records by full name, educational ID or group
without changing SQLite. User text is escaped using standard `html/template`.

The visual style uses first-party `static/theme.css`, embedded by Go
`embed.FS` and served locally at `/static/theme.css`. A tiny standalone
`static/theme.js` allows optional light/dark theme switching in localStorage.
The app does **not** require React, Vue, Angular, Node, npm, Bootstrap,
Tailwind, an external asset host or a second HTTP server.

New pages can reuse the local styles and conventional HTML forms. Search and
dashboard work with JavaScript disabled; only the theme toggle needs JS.
Client-side scripts must not bypass server-side validation or CSRF protection.

## Русский

По адресу `/dashboard` выводится панель преподавателя: количество студентов,
принятых и просроченных документов, общий процент готовности, состояние
каждой группы. Все показатели вычисляются из текущих данных, а не
подменяются демонстрационными. Поиск `/search?q=...` находит до 100
активных студентов по ФИО, учебному номеру и группе без изменения SQLite.
Ввод пользователя безопасно экранируется через `html/template`.

Собственные стили `static/theme.css` встроены в один Go-бинарник через
`embed.FS`. Небольшой `static/theme.js` позволяет переключать светлую
и тёмную тему; выбор сохраняется в локальном хранилище браузера.
**React, Vue, Angular, Node, npm, Bootstrap, Tailwind и CDN не используются.**

Серверная навигация, поисковые запросы и формы работают без JavaScript.
Для смены темы достаточно минимального собственного JS. Защита CSRF и
проверки данных всегда остаются на сервере.

package web

import "html/template"

// documentTemplate is the server-rendered workspace for a single thesis.
// All student names, file names and dates are escaped by html/template.
// RU: Карточка одной ВКР; все пользовательские строки экранируются сервером.
var documentTemplate = template.Must(template.New("thesis-workspace").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ВКР: {{.FullName}} — ThesisFlow</title>
  <link rel="stylesheet" href="/static/theme.css">
  <script src="/static/theme.js" defer></script>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; font: 16px/1.5 system-ui, -apple-system, sans-serif; }
    .workspace { max-width: 1100px; padding: 34px 20px 70px; margin: 0 auto; }
    .breadcrumbs { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 24px; }
    .breadcrumbs a { text-decoration: none; }
    h1 { font-size: clamp(1.55rem, 3vw, 2.15rem); line-height: 1.2; margin: 0 0 8px; }
    h2 { font-size: 1.22rem; margin: 0 0 14px; }
    h3 { font-size: 1rem; margin: 0 0 12px; }
    .muted { color: var(--tf-muted, #5e6c82); }
    .intro { margin: 0 0 24px; }
    .panel { background: var(--tf-panel, white); border: 1px solid var(--tf-border, #dbe3ef); border-radius: 14px; padding: 22px; margin-bottom: 18px; }
    .metrics { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(165px, 1fr)); margin: 18px 0; }
    .metric { padding: 17px; border: 1px solid var(--tf-border, #dbe3ef); background: var(--tf-panel, white); border-radius: 12px; }
    .metric strong { display: block; font-size: 1.75rem; line-height: 1.15; }
    progress { width: 100%; height: 14px; accent-color: #247c67; margin-top: 10px; }
    .toc { display: flex; flex-wrap: wrap; gap: 8px; padding: 0; list-style: none; }
    .toc a { display: inline-block; padding: 8px 12px; border: 1px solid var(--tf-border, #dbe3ef); border-radius: 9px; text-decoration: none; }
    .document-heading { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; flex-wrap: wrap; }
    .status { display: inline-block; padding: 3px 10px; border-radius: 100px; background: var(--tf-bg, #f4f7fb); border: 1px solid var(--tf-border, #dbe3ef); font-size: .87rem; font-weight: 600; }
    .warning { font-weight: 600; color: #aa541c; }
    .doc-form { display: flex; flex-wrap: wrap; gap: 12px; align-items: end; margin: 14px 0; }
    .doc-form label { flex: 1 1 270px; }
    input[type="file"], input[name="topic"], input[name="order_reference"] { display: block; width: 100%; padding: 8px; border-radius: 8px; border: 1px solid var(--tf-border, #dbe3ef); font: inherit; margin-top: 4px; }
    .topic-form { display: grid; gap: 12px; max-width: 700px; }
    button { background: #2456a9; color: #fff; font: inherit; border: 0; padding: 9px 15px; border-radius: 8px; }
    .versions { list-style: none; padding: 0; margin: 12px 0 0; }
    .versions li { border-top: 1px solid var(--tf-border, #dbe3ef); padding: 12px 0; }
    .versions a { overflow-wrap: anywhere; }
    .version-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; }
    .version-actions form { margin: 0; }
    .version-actions button { background: transparent; color: var(--tf-accent, #2456a9); border: 1px solid var(--tf-border, #dbe3ef); }
    .batch-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(235px, 1fr)); gap: 10px; margin-bottom: 16px; }
    .batch-grid label { border: 1px solid var(--tf-border, #dbe3ef); border-radius: 9px; padding: 10px; }
    .batch-grid input[type="file"] { margin-top: 8px; }
    .history { margin: 12px 0 0; }
    .history ul { margin: 10px 0; padding-left: 20px; }
    a:focus-visible, button:focus-visible, input:focus-visible { outline: 3px solid #367cc0; outline-offset: 3px; }
    @media (max-width: 650px) { .workspace { padding: 22px 14px; } .panel { padding: 16px; } }
    @media print { .doc-form, .version-actions, .breadcrumbs, #thesisflow-theme-toggle { display: none !important; } }
  </style>
</head>
<body>
<main class="workspace">
  <nav class="breadcrumbs" aria-label="Навигация">
    <a href="/dashboard">Панель преподавателя</a>
    <a href="/groups/{{.GroupID}}">Группа {{.GroupName}}</a>
    <a href="/groups/{{.GroupID}}/matrix">Ведомость группы</a>
  </nav>
  <h1>ВКР: {{.FullName}}</h1>
  <p class="intro muted">Группа: {{.GroupName}} · Учебный ID: {{.EducationalID}}</p>
  {{if .Notice}}<p class="panel" role="status">{{.Notice}}</p>{{end}}

  <section class="panel" aria-labelledby="progress-title">
    <h2 id="progress-title">Готовность работы</h2>
    <p><strong>{{.Accepted}} из {{.Total}} принято</strong></p>
    <progress value="{{.Progress}}" max="100" aria-label="Доля принятых документов">{{.Progress}}%</progress>
    <div class="metrics" aria-label="Состояния документов">
      <div class="metric">Не сдано: <strong>{{.Missing}}</strong></div>
      <div class="metric">На проверке: <strong>{{.UnderReview}}</strong></div>
      <div class="metric">На доработке: <strong>{{.NeedsRevision}}</strong></div>
      <div class="metric">Принято: <strong>{{.Accepted}}</strong></div>
    </div>
    {{if .Overdue}}<p class="warning" role="status">Просрочено документов: {{.Overdue}}</p>{{end}}
    {{if .Documents}}
    <h3>Перейти к документу</h3>
    <ul class="toc">{{range .Documents}}<li><a href="#requirement-{{.Requirement.ID}}">{{.Requirement.Title}}</a></li>{{end}}</ul>
    {{else}}<p>Для студента ещё нет требований к документам.</p>{{end}}
  </section>

  {{if .CanEditTopic}}
  <section class="panel" aria-labelledby="topic-title">
    <h2 id="topic-title">Тема ВКР и приказ</h2>
    <p><strong>Тема:</strong> {{if .ThesisTopic}}{{.ThesisTopic}}{{else}}ещё не утверждена{{end}}</p>
    <p><strong>Приказ:</strong> {{if .OrderRef}}{{.OrderRef}}{{else}}не указан{{end}}</p>
    <details>
      <summary>Назначить или изменить тему</summary>
      <form class="topic-form" method="post" action="/groups/{{.GroupID}}/students/{{.StudentID}}/topic">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <label>Утверждённая тема <input name="topic" value="{{.ThesisTopic}}" required maxlength="500"></label>
        <label>Реквизиты приказа <input name="order_reference" value="{{.OrderRef}}" required maxlength="500"></label>
        <button type="submit">Сохранить тему ВКР</button>
      </form>
    </details>
  </section>
  {{end}}

  {{if .Documents}}
  <section class="panel" id="batch-upload" aria-labelledby="batch-title">
    <h2 id="batch-title">Пакетная загрузка документов</h2>
    <p class="muted">Выберите файл напротив каждого нужного документа. Пустые поля будут пропущены. Проверяйте соответствие файла требованию перед отправкой.</p>
    <form method="post" enctype="multipart/form-data" action="/groups/{{.GroupID}}/students/{{.StudentID}}/documents/batch">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <div class="batch-grid">
        {{range .Documents}}
        <label>{{.Requirement.Title}}
          <input type="file" name="document_{{.Requirement.ID}}" accept=".docx,.pdf,.odt,.rtf,.txt,.xlsx">
        </label>
        {{end}}
      </div>
      <p class="muted">За один раз: до 10 файлов, каждый до 20 МиБ, общий размер до 40 МиБ. Новые версии поступят на проверку, принятые ранее файлы сохранятся до следующего утверждения.</p>
      <button type="submit">Загрузить выбранные документы</button>
    </form>
  </section>
  {{end}}

  {{range .Documents}}
  <section class="panel" id="requirement-{{.Requirement.ID}}" aria-labelledby="title-{{.Requirement.ID}}">
    <div class="document-heading">
      <h2 id="title-{{.Requirement.ID}}">{{.Requirement.Title}}</h2>
      <span class="status">{{.Status}}</span>
    </div>
    <p class="muted">Срок сдачи: {{if .Requirement.DueDate}}{{.Requirement.DueDate}}{{else}}не назначен{{end}}
    {{if .IsOverdue}}<span class="warning"> · Срок истёк</span>{{end}}</p>
    <form class="doc-form" method="post" enctype="multipart/form-data" action="/groups/{{$.GroupID}}/students/{{$.StudentID}}/documents/{{.Requirement.ID}}/upload">
      <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
      <label>Загрузить новую версию <input type="file" name="document" required></label>
      <button type="submit">Загрузить</button>
    </form>
    <h3>Версии документа ({{len .Versions}})</h3>
    {{if .Versions}}
    <ol class="versions">
      {{range .Versions}}
      <li>
        <a href="/groups/{{$.GroupID}}/students/{{$.StudentID}}/documents/{{.RequirementID}}/versions/{{.ID}}/download">{{.OriginalName}}</a>
        <span class="muted"> · {{.UploadedAt}} · {{.Size}} байт · Версия #{{.ID}}</span>
        <div class="version-actions">
          <form method="post" action="/groups/{{$.GroupID}}/students/{{$.StudentID}}/documents/{{.RequirementID}}/accept/{{.ID}}">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <button type="submit">Принять</button>
          </form>
          <form method="post" action="/groups/{{$.GroupID}}/students/{{$.StudentID}}/documents/{{.RequirementID}}/reject/{{.ID}}">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <button type="submit">На доработку</button>
          </form>
        </div>
      </li>
      {{end}}
    </ol>
    {{else}}<p class="muted">Файлы пока не загружались.</p>{{end}}
    {{if .History}}
    <details class="history">
      <summary>История статусов ({{len .History}})</summary>
      <ul>{{range .History}}<li>{{.At}} — {{.Status}}</li>{{end}}</ul>
    </details>
    {{end}}
  </section>
  {{end}}
</main>
</body>
</html>`))

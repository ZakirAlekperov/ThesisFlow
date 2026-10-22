package web

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
)

// templateKit is a real ZIP archive with XLSX, DOCX and TXT fixtures, stored
// as a binary repository asset. It is never written into the user's database.
//
// RU: templateKit — настоящий ZIP с файлами XLSX, DOCX и TXT внутри проекта.
// Скачивание архива или его файлов не изменяет базу данных пользователя.
//
//go:embed assets/testkit.zip
var templateKit []byte

// downloadableTemplate describes an allowlisted asset available to the user.
// RU: downloadableTemplate описывает разрешённый для скачивания файл.
type downloadableTemplate struct {
	ID          string
	Title       string
	Description string
	Category    string
	Filename    string
	Member      string
	ContentType string
}

const excelType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
const wordType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// templateCatalog is a fixed allowlist: no request path becomes a filesystem path.
// RU: templateCatalog — строгий перечень; URL не превращается в путь к файлу.
var templateCatalog = []downloadableTemplate{
	{
		ID: "students-sample", Title: "Тестовый список — 20 студентов",
		Description: "Вымышленная группа ИСП-401. Готов к импорту: ФИО и уникальные учебные ID уже заполнены.",
		Category:    "Excel", Filename: "Студенты_ИСП-401_20_человек.xlsx",
		Member: "students_sample.xlsx", ContentType: excelType,
	},
	{
		ID: "students-blank", Title: "Пустой шаблон списка студентов",
		Description: "Те же заголовки столбцов; заполняйте собственные ФИО и учебные ID перед импортом.",
		Category:    "Excel", Filename: "Шаблон_списка_студентов.xlsx",
		Member: "students_blank.xlsx", ContentType: excelType,
	},
	{
		ID: "chapter1-draft", Title: "Глава 1 — черновик",
		Description: "Первая версия учебного документа для проверки загрузки и статуса «На проверке».",
		Category:    "Документы ВКР", Filename: "Глава_1_черновик.docx",
		Member: "chapter1_draft.docx", ContentType: wordType,
	},
	{
		ID: "chapter1-revision", Title: "Глава 1 — исправленная версия",
		Description: "Загрузите после черновика как вторую версию того же требования, затем примите документ.",
		Category:    "Документы ВКР", Filename: "Глава_1_исправленная.docx",
		Member: "chapter1_revision.docx", ContentType: wordType,
	},
	{
		ID: "chapter2-sample", Title: "Глава 2 — проектирование",
		Description: "Отдельный документ для проверки статуса «На доработке».",
		Category:    "Документы ВКР", Filename: "Глава_2_пример.docx",
		Member: "chapter2_sample.docx", ContentType: wordType,
	},
	{
		ID: "manual", Title: "Инструкция по ручному тестированию",
		Description: "Сценарий проверки импорта студентов, версии документа и изменения статусов.",
		Category:    "Инструкции", Filename: "Инструкция_ThesisFlow.txt",
		Member: "manual.txt", ContentType: "text/plain; charset=utf-8",
	},
	{
		ID: "all", Title: "Весь тестовый комплект",
		Description: "Excel, DOCX и инструкция одним ZIP-архивом.",
		Category:    "Комплект", Filename: "ThesisFlow_тестовые_документы.zip",
		ContentType: "application/zip",
	},
}

// templatesPage is a standalone read-only section in the local web interface.
// RU: templatesPage — самостоятельный раздел без изменения данных.
var templatesPage = template.Must(template.New("templates").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Шаблоны — ThesisFlow</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, sans-serif; background:#f6f7fb; color:#202939; }
    * { box-sizing:border-box; }
    body { margin:0; }
    main { width:min(850px, 100% - 32px); margin:48px auto; }
    .tabs { display:flex; flex-wrap:wrap; gap:8px; margin-bottom:30px; border-bottom:1px solid #dce2eb; padding-bottom:12px; }
    .tabs a { display:inline-block; padding:10px 16px; border-radius:8px; text-decoration:none; color:#214eaf; }
    .tabs a[aria-current=page] { background:#214eaf; color:white; font-weight:600; }
    h1 { font-size:2rem; margin:0 0 12px; }
    .subtitle { color:#596578; line-height:1.65; margin:0 0 28px; }
    .item { display:flex; justify-content:space-between; flex-wrap:wrap; align-items:center; gap:16px; padding:20px 0; border-bottom:1px solid #e4e8f0; }
    .item:last-child { border-bottom:0; }
    .panel { background:white; border:1px solid #e4e8f0; border-radius:16px; padding:24px; }
    .item h2 { font-size:1.05rem; margin:4px 0 6px; }
    .item p { margin:0; max-width:570px; line-height:1.55; color:#596578; }
    .type { color:#687488; font-size:.8rem; }
    .download { white-space:nowrap; border:1px solid #214eaf; color:#214eaf; border-radius:8px; padding:10px 15px; text-decoration:none; font-weight:600; }
    .download:hover, .download:focus-visible { background:#edf4ff; }
    .note { line-height:1.6; font-size:.9rem; color:#596578; margin-top:22px; }
  </style><link rel="stylesheet" href="/static/theme.css"><script src="/static/theme.js" defer></script>
</head>
<body>
  <main>
    <nav class="tabs" aria-label="Разделы приложения">
      <a href="/">Учебные группы</a>
      <a href="/templates" aria-current="page">Шаблоны</a>
      <a href="/dashboard">Панель преподавателя</a>
      <a href="/search">Поиск студентов</a>
      <a href="/manage">Управление и архив</a>
    </nav>
    <h1>Шаблоны и тестовые документы</h1>
    <p class="subtitle">Скачайте реальный файл, затем загрузите его в ThesisFlow обычным способом. Все ФИО и учебные идентификаторы вымышлены.</p>
    <section class="panel" aria-label="Каталог шаблонов">
      {{range .}}
      <div class="item">
        <div>
          <span class="type">{{.Category}}</span>
          <h2>{{.Title}}</h2>
          <p>{{.Description}}</p>
        </div>
        <a class="download" href="/templates/download/{{.ID}}" download>Скачать</a>
      </div>
      {{end}}
    </section>
    <p class="note">Раздел не создаёт группы и студентов и не загружает документы в хранилище автоматически. Для импорта списка сначала создайте учебную группу, затем используйте на странице группы форму «Импорт из Excel». Заголовки: «ФИО» и «Учебный ID».</p>
  </main>
</body>
</html>
`))

// renderTemplates shows the fixed catalog without accessing SQLite.
// RU: renderTemplates показывает перечень файлов без обращения к SQLite.
func renderTemplates(w http.ResponseWriter, r *http.Request) {
	var output bytes.Buffer
	if err := templatesPage.Execute(&output, templateCatalog); err != nil {
		http.Error(w, "Не удалось отобразить шаблоны.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(output.Bytes())
}

// templateBytes selects only a known file within the embedded ZIP archive.
// RU: templateBytes извлекает из встроенного ZIP только разрешённый файл.
func templateBytes(item downloadableTemplate) ([]byte, error) {
	if item.Member == "" {
		return templateKit, nil
	}
	reader, err := zip.NewReader(bytes.NewReader(templateKit), int64(len(templateKit)))
	if err != nil {
		return nil, fmt.Errorf("open embedded test kit: %w", err)
	}
	for _, file := range reader.File {
		if file.Name != item.Member {
			continue
		}
		if file.UncompressedSize64 > 10<<20 {
			return nil, fmt.Errorf("embedded test file exceeds size limit")
		}
		stream, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open embedded test document: %w", err)
		}
		data, err := io.ReadAll(io.LimitReader(stream, (10<<20)+1))
		closeErr := stream.Close()
		if err != nil {
			return nil, fmt.Errorf("read embedded test document: %w", err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close embedded test document: %w", closeErr)
		}
		if len(data) > 10<<20 {
			return nil, fmt.Errorf("embedded test file exceeds size limit")
		}
		return data, nil
	}
	return nil, fmt.Errorf("embedded test file not found")
}

// downloadTemplate serves XLSX/DOCX/TXT as actual downloadable files.
// Unknown IDs are 404; no request can read local application data.
//
// RU: downloadTemplate отдаёт настоящие XLSX/DOCX/TXT для скачивания.
// Неизвестные ID дают 404; доступ к файлам приложения отсутствует.
func downloadTemplate(w http.ResponseWriter, r *http.Request) {
	var selected *downloadableTemplate
	for i := range templateCatalog {
		if templateCatalog[i].ID == r.PathValue("id") {
			selected = &templateCatalog[i]
			break
		}
	}
	if selected == nil {
		http.NotFound(w, r)
		return
	}
	data, err := templateBytes(*selected)
	if err != nil {
		http.Error(w, "Не удалось получить шаблон.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", selected.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": selected.Filename}))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	_, _ = w.Write(data)
}

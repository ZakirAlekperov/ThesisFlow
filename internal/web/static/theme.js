// Tiny first-party helper: the server renders all actual application data.
// Небольшой локальный переключатель темы, данные формирует Go.
(() => {
  const key = "thesisflow-theme";
  let selected = "light";
  try {
    if (localStorage.getItem(key) === "dark") selected = "dark";
  } catch (_) {}
  document.documentElement.dataset.theme = selected;
  document.addEventListener("DOMContentLoaded", () => {
    const button = document.createElement("button");
    button.id = "thesisflow-theme-toggle";
    button.type = "button";
    button.textContent = selected === "dark" ? "Светлая тема" : "Тёмная тема";
    button.setAttribute("aria-label", "Переключить светлую и тёмную тему");
    button.addEventListener("click", () => {
      selected = selected === "dark" ? "light" : "dark";
      document.documentElement.dataset.theme = selected;
      button.textContent = selected === "dark" ? "Светлая тема" : "Тёмная тема";
      try { localStorage.setItem(key, selected); } catch (_) {}
    });
    document.body.prepend(button);
  });
})();

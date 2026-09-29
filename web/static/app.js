"use strict";
document.querySelectorAll("[data-password]").forEach(button => {
  button.addEventListener("click", () => {
    const input = document.getElementById(button.dataset.password);
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    button.textContent = show ? "Скрыть" : "Показать";
    button.setAttribute("aria-label", show ? "Скрыть пароль" : "Показать пароль");
    button.setAttribute("aria-pressed", String(show));
  });
});

const systemTheme = matchMedia("(prefers-color-scheme: dark)");
function syncThemeButtons() {
  const theme = document.documentElement.dataset.theme;
  const dark = theme === "dark" || (theme === "system" && systemTheme.matches);
  document.querySelectorAll("[data-theme-toggle]").forEach(button => {
    button.value = dark ? "light" : "dark";
    button.querySelector("[data-theme-label]").textContent = dark ? "Светлая тема" : "Тёмная тема";
    const label = dark ? "Включить светлую тему" : "Включить тёмную тему";
    button.setAttribute("aria-label", label);
    button.title = label;
  });
}
syncThemeButtons();
systemTheme.addEventListener("change", syncThemeButtons);
document.querySelectorAll(".theme-control").forEach(form => {
  form.addEventListener("submit", async event => {
    event.preventDefault();
    const button = form.querySelector("[data-theme-toggle]");
    if (button.disabled) return;
    const status = form.querySelector("[data-theme-status]");
    const values = new URLSearchParams(new FormData(form));
    values.set("theme", button.value);
    button.disabled = true;
    status.hidden = true;
    try {
      const response = await fetch(form.action, {
        method: "POST", body: values, credentials: "same-origin",
        headers: {Accept: "application/json"}
      });
      if (!response.ok) throw new Error("Theme save failed");
      const result = await response.json();
      if (result.theme !== "light" && result.theme !== "dark") throw new Error("Invalid theme");
      document.documentElement.dataset.theme = result.theme;
      syncThemeButtons();
      document.dispatchEvent(new Event("themechange"));
    } catch {
      status.textContent = "Не удалось сохранить тему. Попробуйте ещё раз.";
      status.hidden = false;
    } finally { button.disabled = false; }
  });
});

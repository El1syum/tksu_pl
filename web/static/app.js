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


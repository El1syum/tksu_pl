"use strict";
const rubles = cents => new Intl.NumberFormat("ru-RU", {style: "currency", currency: "RUB"}).format(cents / 100);
const months = ["Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"];
const charts = {category: null, month: null};
const requests = {category: 0, month: 0};
async function getStats(url) {
  const response = await fetch(url, {headers: {Accept: "application/json"}});
  const data = await response.json();
  if (!response.ok) throw new Error(response.status === 401 ? "Сессия истекла. Войдите в аккаунт заново." : data.error || "Не удалось загрузить данные");
  return data;
}
async function loadChart(kind) {
  const request = ++requests[kind];
  const form = document.getElementById(`${kind}-stats-form`);
  const status = document.getElementById(`${kind}-status`);
  const area = document.getElementById(`${kind}-chart-area`);
  const button = form.querySelector("button");
  button.disabled = true;
  status.hidden = false;
  status.textContent = "Загружаем расходы…";
  status.classList.remove("error-text");
  area.hidden = true;
  if (kind === "category") document.getElementById("category-legend").replaceChildren();
  else { document.getElementById("year-summary").hidden = true; document.getElementById("month-table").replaceChildren(); }
  try {
    const query = new URLSearchParams(new FormData(form));
    const data = await getStats(`/api/stats/by-${kind}?${query}`);
    if (request !== requests[kind]) return;
    if (charts[kind]) { charts[kind].destroy(); charts[kind] = null; }
    const total = data.reduce((sum, item) => sum + item.amount, 0);
    if (kind === "category") {
      document.getElementById("category-total").textContent = rubles(total);
      const legend = document.getElementById("category-legend");
      for (const item of data) {
        const row = document.createElement("div"); row.className = "legend-row";
        const name = document.createElement("span"); name.textContent = item.name;
        const amount = document.createElement("strong"); amount.textContent = rubles(item.amount);
        const percent = document.createElement("span"); percent.className = "legend-percent"; percent.textContent = `${Math.round(item.amount / total * 100)}%`;
        row.append(name, amount, percent); legend.append(row);
      }
    } else {
      document.getElementById("year-total").textContent = rubles(total);
      document.getElementById("year-summary").hidden = false;
      const tbody = document.getElementById("month-table");
      for (const item of data) {
        const row = document.createElement("tr"), name = document.createElement("td"), amount = document.createElement("td");
        name.textContent = months[item.month - 1]; amount.textContent = rubles(item.amount); row.append(name, amount); tbody.append(row);
      }
    }
    if (!total) { status.textContent = "За этот период пока нет расходов. Добавьте трату или выберите другой период."; return; }
    if (!(await chartReady)) throw new Error("Не удалось загрузить графики. Данные доступны ниже; попробуйте обновить страницу.");
    if (request !== requests[kind]) return;
    area.hidden = false; status.hidden = true;
    const options = {
      responsive: true, maintainAspectRatio: false,
      animation: matchMedia("(prefers-reduced-motion: reduce)").matches ? false : {duration: 450},
      plugins: {legend: {display: false}, tooltip: {backgroundColor: "#213a33", padding: 12, callbacks: {label: context => ` ${rubles((kind === "category" ? context.raw : context.parsed.y) * 100)}`}}}
    };
    if (kind === "category") options.cutout = "78%";
    else {
      const style = getComputedStyle(document.documentElement);
      const tickColor = style.getPropertyValue("--muted").trim();
      const gridColor = style.getPropertyValue("--border").trim();
      options.scales = {x: {grid: {display: false}, border: {display: false}, ticks: {color: tickColor}}, y: {beginAtZero: true, border: {display: false}, grid: {color: gridColor}, ticks: {color: tickColor, callback: value => new Intl.NumberFormat("ru-RU", {notation: "compact"}).format(value) + " ₽"}}};
    }
    charts[kind] = new Chart(document.getElementById(`${kind}-chart`), {
      type: kind === "category" ? "doughnut" : "bar",
      data: {labels: kind === "category" ? data.map(d => d.name) : months,
        datasets: [{data: data.map(d => d.amount / 100), backgroundColor: kind === "category" ? data.map(d => d.color) : "#198572", borderWidth: 0, borderRadius: kind === "category" ? 3 : 5, spacing: kind === "category" ? 4 : 0, maxBarThickness: 28}]}, options
    });
  } catch (error) {
    if (request !== requests[kind]) return;
    status.hidden = false; status.classList.add("error-text"); status.textContent = error.message || "Не удалось загрузить статистику. Попробуйте ещё раз.";
  } finally { if (request === requests[kind]) button.disabled = false; }
}
function updateChartTheme() {
  if (!charts.month) return;
  const style = getComputedStyle(document.documentElement);
  const scales = charts.month.options.scales;
  scales.x.ticks.color = scales.y.ticks.color = style.getPropertyValue("--muted").trim();
  scales.y.grid.color = style.getPropertyValue("--border").trim();
  charts.month.update("none");
}
document.addEventListener("themechange", updateChartTheme);
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (document.documentElement.dataset.theme === "system") updateChartTheme();
});
for (const kind of ["category", "month"]) {
  document.getElementById(`${kind}-stats-form`).addEventListener("submit", event => { event.preventDefault(); loadChart(kind); });
  loadChart(kind);
}

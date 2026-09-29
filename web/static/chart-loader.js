"use strict";
// Pinned CDN asset with SRI; use the identical bundled copy when offline.
const chartReady = (async () => {
  function loadChartScript(src, integrity) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      const timer = setTimeout(() => { script.remove(); reject(new Error("Chart.js load timeout")); }, 4000);
      script.src = src;
      if (integrity) { script.integrity = integrity; script.crossOrigin = "anonymous"; }
      script.onload = () => { clearTimeout(timer); resolve(); };
      script.onerror = () => { clearTimeout(timer); script.remove(); reject(new Error("Chart.js load failed")); };
      document.head.append(script);
    });
  }
  try {
    await loadChartScript("https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.min.js", "sha384-jb8JQMbMoBUzgWatfe6COACi2ljcDdZQ2OxczGA3bGNeWe+6DChMTBJemed7ZnvJ");
  } catch {
    try { await loadChartScript("/static/vendor/chart.umd.min.js"); } catch { return false; }
  }
  return typeof Chart !== "undefined";
})();

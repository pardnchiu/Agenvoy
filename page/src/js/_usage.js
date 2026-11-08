const USAGE_SERIES = [
  { key: "input", label: "input", color: "--chart-input" },
  { key: "output", label: "output", color: "--chart-output" },
  { key: "hit", label: "cache read", color: "--chart-cache" },
];

const USAGE_BAR_HEIGHT = 22;

const ECHARTS_SRC = "https://cdn.jsdelivr.net/npm/echarts@6.0.0/dist/echarts.min.js";

let echartsLoading = null;

function ensureECharts() {
  if (typeof echarts !== "undefined") {
    return Promise.resolve(true);
  }
  if (echartsLoading) {
    return echartsLoading;
  }

  echartsLoading = new Promise((resolve) => {
    const tag = document.createElement("script");
    tag.src = ECHARTS_SRC;
    tag.onload = () => resolve(true);
    tag.onerror = () => {
      console.error("ensureECharts", ECHARTS_SRC);
      resolve(false);
    };
    document.head.appendChild(tag);
  });
  return echartsLoading;
}

let usageScope = "";
let usageCache = null;
let usageChart = null;

function usageDom() {
  return {
    header: document.querySelector("section.monitor nav.period"),
    all: $("#usage-all"),
    list: $("#usage-list"),
    summary: $("#usage-summary"),
    chart: $("#usage-chart"),
    table: $("#usage-table"),
  };
}

function currentUsagePeriod() {
  const header = usageDom().header;
  return (header && header.dataset.selected) || "24h";
}

async function fetchUsagePeriods(sessionId) {
  const url = sessionId ? `${API}/v1/session/${encodeURIComponent(sessionId)}?usage=1` : `${API}/v1/usage`;
  try {
    const response = await fetch(url);
    if (!response.ok) {
      return {};
    }
    const body = (await response.json()) || {};
    return (sessionId ? body.usage : body.periods) || {};
  } catch (err) {
    console.error("fetchUsagePeriods", err);
    return {};
  }
}

async function fetchUsageSessions() {
  try {
    const response = await fetch(`${API}/v1/sessions`);
    if (response.ok) {
      return (await response.json()).sessions || [];
    }
  } catch (err) {
    console.error("fetchUsageSessions", err);
  }
  return [];
}

function usageLink(period, id) {
  return "?page=monitor&tab=Usage&period=" + period + (id ? "&chat=" + encodeURIComponent(id) : "");
}

function markUsagePeriods() {
  const header = usageDom().header;
  if (!header) {
    return;
  }
  for (const link of header.querySelectorAll("a")) {
    link.href = usageLink(link.getAttribute("name"), usageScope);
  }
}

function usageCard(id, title, subtitle) {
  const card = _("a.card", { href: usageLink(currentUsagePeriod(), id) }, [
    _("strong", title),
    _("p", subtitle),
  ]);
  card.dataset.name = id;
  card.dataset.selected = id === usageScope ? "1" : "0";
  return card;
}

function usageRows(summary) {
  const rows = [];
  for (const model of Object.keys(summary || {})) {
    const one = summary[model] || {};
    const row = {
      model: model,
      input: Number(one.input) || 0,
      output: Number(one.output) || 0,
      write: Number(one.write) || 0,
      hit: Number(one.hit) || 0,
    };
    row.total = row.input + row.output + row.hit;
    if (row.total > 0) {
      rows.push(row);
    }
  }
  rows.sort((a, b) => b.total - a.total || a.model.localeCompare(b.model));
  return rows;
}

function usageTotals(rows) {
  const sum = { input: 0, output: 0, write: 0, hit: 0, total: 0 };
  for (const row of rows) {
    sum.input += row.input;
    sum.output += row.output;
    sum.write += row.write;
    sum.hit += row.hit;
    sum.total += row.total;
  }
  return sum;
}

function usageNumber(value) {
  return Number(value || 0).toLocaleString();
}

function usageHitRate(input, hit) {
  const read = (input || 0) + (hit || 0);
  if (read <= 0) {
    return "0.00";
  }
  return ((hit / read) * 100).toFixed(2);
}

function renderUsageSummary(rows, totals, period) {
  const dom = usageDom();
  if (!dom.summary) {
    return;
  }

  const cells = [
    { label: "total tokens · " + period, value: usageNumber(totals.total) },
    { label: "input", value: usageNumber(totals.input) },
    { label: "output", value: usageNumber(totals.output) },
    { label: "cache read", value: usageNumber(totals.hit) + " (" + usageHitRate(totals.input, totals.hit) + "%)" },
    { label: "models", value: String(rows.length) },
  ];

  dom.summary.innerHTML = "";
  for (const cell of cells) {
    dom.summary.appendChild(_("div.cell", [_("p", cell.label), _("strong", cell.value)]));
  }
}

function renderUsageTable(rows, period) {
  const dom = usageDom();
  if (!dom.table) {
    return;
  }

  dom.table.innerHTML = "";
  if (rows.length === 0) {
    dom.table.appendChild(_("p.empty", "no usage in " + period));
    return;
  }

  const head = _("tr", [
    _("th", "model"),
    _("th.num", "input"),
    _("th.num", "output"),
    _("th.num", "cache read"),
    _("th.num", "total"),
  ]);

  const body = [];
  for (const row of rows) {
    body.push(
      _("tr", [
        _("td", row.model),
        _("td.num", usageNumber(row.input)),
        _("td.num", usageNumber(row.output)),
        _("td.num", usageNumber(row.hit) + " (" + usageHitRate(row.input, row.hit) + "%)"),
        _("td.num.total", usageNumber(row.total)),
      ]),
    );
  }

  dom.table.appendChild(_("table", [_("thead", [head]), _("tbody", body)]));
}

function renderUsageChart(rows) {
  const dom = usageDom();
  if (!dom.chart || typeof echarts === "undefined") {
    return;
  }

  if (usageChart) {
    usageChart.dispose();
    usageChart = null;
  }
  if (rows.length === 0) {
    dom.chart.innerHTML = "";
    dom.chart.style.height = "";
    return;
  }

  const style = getComputedStyle(document.documentElement);
  const color = (name) => style.getPropertyValue(name).trim();
  const models = rows.map((row) => row.model).reverse();
  dom.chart.style.height = Math.max(224, rows.length * USAGE_BAR_HEIGHT + 72) + "px";
  usageChart = echarts.init(dom.chart, null, { renderer: "canvas" });
  usageChart.setOption({
    animation: false,
    grid: { left: 8, right: 24, top: 32, bottom: 8, containLabel: true },
    legend: {
      top: 0,
      left: 0,
      icon: "roundRect",
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { fontSize: 11, color: color("--chart-text") },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow", shadowStyle: { color: color("--chart-pointer") } },
      backgroundColor: color("--chart-tip-bg"),
      borderColor: color("--chart-tip-border"),
      textStyle: { color: color("--chart-tip-text") },
      valueFormatter: (value) => usageNumber(value),
    },
    xAxis: {
      type: "value",
      axisLabel: { fontSize: 11, color: color("--chart-muted"), formatter: (value) => compactToken(value) },
      splitLine: { lineStyle: { color: color("--chart-grid") } },
    },
    yAxis: {
      type: "category",
      data: models,
      axisLabel: {
        interval: 0,
        fontSize: 11,
        color: color("--chart-text"),
        width: 220,
        overflow: "truncate",
      },
      axisTick: { show: false },
      axisLine: { lineStyle: { color: color("--chart-axis") } },
    },
    series: USAGE_SERIES.map((one) => ({
      name: one.label,
      type: "bar",
      stack: "token",
      itemStyle: { color: color(one.color) },
      barMaxWidth: 14,
      data: rows.map((row) => row[one.key]).reverse(),
    })),
  });
}

window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
  if (usageChart) {
    paintUsage();
  }
});

function resizeUsageChart() {
  if (usageChart) {
    usageChart.resize();
  }
}

function paintUsage() {
  const summary = (usageCache || {})[currentUsagePeriod()] || {};
  const rows = usageRows(summary);
  renderUsageSummary(rows, usageTotals(rows), currentUsagePeriod());
  renderUsageChart(rows);
  renderUsageTable(rows, currentUsagePeriod());
}

async function selectUsage(sessionId) {
  usageScope = sessionId || "";
  usageCache = await fetchUsagePeriods(usageScope);
  paintUsage();
}

async function renderUsagePage(sessionId) {
  const dom = usageDom();
  if (!dom.list) {
    return;
  }

  usageScope = sessionId || "";
  markUsagePeriods();
  const sessions = await fetchUsageSessions();

  if (dom.all) {
    dom.all.href = usageLink(currentUsagePeriod(), "");
    dom.all.dataset.selected = usageScope === "" ? "1" : "0";
  }

  dom.list.innerHTML = "";
  for (const one of sessions) {
    if (!one || !one.id) {
      continue;
    }
    const name = one.name || one.id;
    dom.list.appendChild(usageCard(one.id, one.self_id ? `${name} (${one.self_id})` : name, one.model || one.id));
  }

  window.addEventListener("resize", resizeUsageChart);
  await ensureECharts();
  await selectUsage(usageScope);
}

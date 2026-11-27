const DAEMON_PAGE_LINES = 100;
const DAEMON_LEVELS = ["info", "debug", "warn", "error"];
const DAEMON_SLOG = /\blevel=([A-Z]+)/;
const DAEMON_STDLOG = /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2} ([A-Z]+)\b/;
const DAEMON_SLOG_LINE = /^time=(\S+) level=[A-Z]+ msg=("(?:[^"\\]|\\.)*"|\S*) ?(.*)$/;
const DAEMON_STDLOG_LEVEL = /^(\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}) [A-Z]+\b ?/;
const DAEMON_SCOPE = ["keyword", "from", "to"];
const DAEMON_UNIT = { m: 60000, h: 3600000, d: 86400000, w: 604800000 };
const DAEMON_SPAN = /^(\d+)([mhdw])$/;

function daemonDom() {
  return $("#daemon-log");
}

function daemonValue(id) {
  const dom = $(id);
  return dom ? dom.value.trim() : "";
}

function daemonDuration(text) {
  const matched = DAEMON_SPAN.exec(text.toLowerCase());
  if (!matched) {
    return 0;
  }
  return Number(matched[1]) * DAEMON_UNIT[matched[2]];
}

function daemonStamp(at) {
  const pad = (n) => String(n).padStart(2, "0");
  return [
    at.getFullYear(),
    pad(at.getMonth() + 1),
    pad(at.getDate()),
    pad(at.getHours()),
    pad(at.getMinutes()),
  ].join("-");
}

function daemonScope() {
  const params = praseURL();
  const query = new URLSearchParams();
  for (const key of DAEMON_SCOPE) {
    if (params[key]) {
      query.set(key, params[key]);
    }
  }
  const text = query.toString();
  return text ? `?${text}` : "";
}

let daemonStart = 0;

function daemonQuery(before) {
  const params = praseURL();
  const query = new URLSearchParams(daemonScope());
  if (DAEMON_LEVELS.includes(params.target)) {
    query.set("level", params.target);
  }
  query.set("limit", DAEMON_PAGE_LINES);
  if (before !== undefined) {
    query.set("before", before);
  }
  return `?${query.toString()}`;
}

async function fetchDaemonPage(before) {
  try {
    const response = await fetch(`${API}/v1/daemon${daemonQuery(before)}`);
    if (response.ok) {
      return await response.json();
    }
  } catch (err) {
    console.error("fetchDaemonPage", err);
  }
  return null;
}

function daemonLines(content) {
  const frame = _("temp");
  content = (content || "").replace(/\n+$/, "");
  if (content) {
    for (const line of content.split("\n")) {
      frame.appendChild(daemonLine(line));
    }
  }
  return frame;
}

function renderDaemonMore(dom, more) {
  let button = dom.querySelector(":scope > button.more");
  if (!more) {
    if (button) {
      button.remove();
    }
    return;
  }
  if (!button) {
    button = _("button.more", { type: "button" }, i18n("button_load_more"));
    button.addEventListener("click", loadMoreDaemon);
    dom.prepend(button);
  }
}

async function loadMoreDaemon() {
  const dom = daemonDom();
  if (!dom || daemonStart <= 0) {
    return;
  }
  const button = dom.querySelector(":scope > button.more");
  if (button) {
    button.disabled = true;
  }

  const page = await fetchDaemonPage(daemonStart);
  if (button) {
    button.disabled = false;
  }
  if (!page) {
    return;
  }

  const height = dom.scrollHeight;
  const top = dom.scrollTop;
  const frame = daemonLines(page.content);
  if (button) {
    button.after(frame);
  } else {
    dom.prepend(frame);
  }
  daemonStart = page.start || 0;
  renderDaemonMore(dom, page.more);
  dom.scrollTop = top + dom.scrollHeight - height;
}

function daemonSubmit() {
  const params = praseURL();
  const query = new URLSearchParams({ page: "monitor", tab: "Daemon" });
  if (params.target) {
    query.set("target", params.target);
  }

  const keyword = daemonValue("#daemon-keyword");
  if (keyword) {
    query.set("keyword", keyword);
  }

  const agoText = daemonValue("#daemon-ago");
  if (agoText) {
    const ago = daemonDuration(agoText);
    if (!ago) {
      pushToast("WARN", i18n("error_not_duration", agoText), nowClock());
      return;
    }

    let span = ago;
    const spanText = daemonValue("#daemon-span");
    if (spanText) {
      span = daemonDuration(spanText);
      if (!span) {
        pushToast("WARN", i18n("error_not_duration", spanText), nowClock());
        return;
      }
      if (span > ago) {
        pushToast("WARN", i18n("error_span_too_wide", spanText, agoText), nowClock());
        return;
      }
    }

    const from = new Date(Date.now() - ago);
    query.set("from", daemonStamp(from));
    query.set("to", daemonStamp(new Date(from.getTime() + span)));
  }

  window.location.href = `?${query.toString()}`;
}

function daemonLevel(text) {
  const matched = DAEMON_SLOG.exec(text) || DAEMON_STDLOG.exec(text);
  if (!matched) {
    return "";
  }
  const level = matched[1].toLowerCase();
  return DAEMON_LEVELS.includes(level) ? level : "";
}

function daemonText(text) {
  const slog = DAEMON_SLOG_LINE.exec(text);
  if (slog) {
    let msg = slog[2];
    if (msg.startsWith('"')) {
      try {
        msg = JSON.parse(msg);
      } catch {
        msg = msg.slice(1, -1);
      }
    }
    return [slog[1].slice(0, 19), msg, slog[3]].filter(Boolean).join(" ");
  }
  return text.replace(DAEMON_STDLOG_LEVEL, "$1 ");
}

function daemonLine(text) {
  const line = _("span");
  line.textContent = daemonText(text) + "\n";
  line.dataset.level = daemonLevel(text);
  return line;
}

async function renderDaemonPage() {
  const dom = daemonDom();
  if (!dom) {
    return;
  }

  const params = praseURL();
  const panel = $("#daemon-panel");
  const target = DAEMON_LEVELS.includes(params.target) ? params.target : "all";
  if (panel) {
    panel.dataset.target = target;
    for (const card of panel.querySelectorAll("div.side > a.new, div.list > a.card")) {
      card.dataset.selected = card.getAttribute("name") === target ? "1" : "0";
    }
  }

  const keyword = $("#daemon-keyword");
  if (keyword && params.keyword) {
    keyword.value = params.keyword;
  }

  const page = await fetchDaemonPage();

  dom.innerHTML = "";
  daemonStart = page ? page.start || 0 : 0;
  if (page) {
    dom.appendChild(daemonLines(page.content));
  }
  renderDaemonMore(dom, page ? page.more : false);
  dom.scrollTop = dom.scrollHeight;
}

function appendDaemonLog(level, text, time) {
  const dom = daemonDom();
  if (!dom || !text) {
    return false;
  }
  if (daemonScope()) {
    return true;
  }

  const stick = dom.scrollHeight - dom.scrollTop - dom.clientHeight < AUTO_SCROLL_SLACK;
  const stamp = time || new Date().toISOString();
  dom.appendChild(daemonLine(`time=${stamp} level=${(level || "INFO").toUpperCase()} msg="${text}"`));
  if (stick) {
    dom.scrollTop = dom.scrollHeight;
  }
  return true;
}

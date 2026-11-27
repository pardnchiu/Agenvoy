const HISTORY_PAGE_SIZE = 20;

function historyDom() {
  return {
    all: $("#history-all"),
    list: $("#history-list"),
    entries: $("#history-entries"),
    pager: $("#history-pager"),
    detail: $("#details-list"),
    back: $("#details-back"),
    body: $("#details-body"),
  };
}

function historyLink(sessionId, offset) {
  const url = praseURL();
  const params = { page: "monitor", tab: "History" };
  if (sessionId) {
    params.target = sessionId;
  }
  if (offset > 0) {
    params.offset = offset;
  }
  for (const key of ["keyword", "from", "to"]) {
    if (url[key]) {
      params[key] = url[key];
    }
  }
  return getLink(params);
}

function rangeSubmit(tab, prefix) {
  const url = praseURL();
  const params = { page: "monitor", tab: tab };
  if (url.target) {
    params.target = url.target;
  }

  const keyword = ($(`#${prefix}-keyword`) ? $(`#${prefix}-keyword`).value : "").trim();
  if (keyword) {
    params.keyword = keyword;
  }

  const agoText = ($(`#${prefix}-ago`) ? $(`#${prefix}-ago`).value : "").trim();
  if (agoText) {
    const ago = daemonDuration(agoText);
    if (!ago) {
      pushToast("WARN", i18n("error_not_duration", agoText), nowClock());
      return;
    }

    let span = ago;
    const spanText = ($(`#${prefix}-span`) ? $(`#${prefix}-span`).value : "").trim();
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
    params.from = daemonStamp(from);
    params.to = daemonStamp(new Date(from.getTime() + span));
  }

  window.location.href = getLink(params);
}

function historyMatch(task, from, to) {
  if (!from && !to) {
    return true;
  }
  const at = new Date(task.end_at);
  if (Number.isNaN(at.getTime())) {
    return false;
  }
  const stamp = daemonStamp(at);
  return (!from || stamp >= from) && (!to || stamp < to);
}

function detailsLink(sessionId, hash, item) {
  return getLink({ page: "monitor", tab: "Details", target: sessionId, hash: hash, item: item });
}

function historyText(text) {
  const raw = text || "";
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch (err) {
    return raw;
  }
}

function textNode(tag, text) {
  const dom = _(tag);
  dom.textContent = text == null ? "" : String(text);
  return dom;
}

function copyButton(read) {
  const button = _("button.copy", { type: "button", name: i18n("button_copy_content") }, [
    _("span.material-symbols-outlined", "content_copy"),
  ]);
  button.addEventListener("click", function () {
    if (!navigator.clipboard) {
      return;
    }

    const icon = button.querySelector("span");
    navigator.clipboard
      .writeText(read())
      .then(() => {
        icon.textContent = "check_circle";
        setTimeout(() => (icon.textContent = "content_copy"), 1000);
      })
      .catch((err) => console.error("copyButton", err));
  });
  return button;
}

function codeBlock(text) {
  const body = _("pre");
  body.textContent = text;
  return _("div.code", [copyButton(() => body.textContent), body]);
}

function labeledBlock(label, text) {
  return _("div.block", [textNode("strong", label), codeBlock(text)]);
}

function historyClock(text) {
  const at = new Date(text);
  if (Number.isNaN(at.getTime())) {
    return text || "";
  }
  const pad = (n) => String(n).padStart(2, "0");
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}`;
}

async function historyTasks(sessionId, keyword) {
  const query = keyword ? `?keyword=${encodeURIComponent(keyword)}` : "";
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}/task/history${query}`);
    if (response.ok) {
      const tasks = (await response.json()).tasks || [];
      for (const task of tasks) {
        task.session = sessionId;
      }
      return tasks;
    }
  } catch (err) {
    console.error("historyTasks", err);
  }
  return [];
}

async function historyAllTasks(sessions, keyword) {
  const groups = await Promise.all(sessions.filter((one) => one && one.id).map((one) => historyTasks(one.id, keyword)));
  return groups.flat().sort((a, b) => new Date(b.end_at) - new Date(a.end_at));
}

async function historyDetail(sessionId, hash) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}/task/${encodeURIComponent(hash)}/history`);
    if (!response.ok) {
      return null;
    }
    return JSON.parse((await response.json()).content || "{}");
  } catch (err) {
    console.error("historyDetail", err);
    return null;
  }
}

function historySessionList(dom, sessions, sessionId) {
  if (dom.all) {
    dom.all.href = historyLink("", 0);
    dom.all.dataset.selected = sessionId === "" ? "1" : "0";
  }

  dom.list.innerHTML = "";
  const frame = _("temp");
  for (const one of sessions) {
    if (!one || !one.id) {
      continue;
    }
    const name = one.name || one.id;
    const card = _("a.card", { href: historyLink(one.id, 0) }, [
      textNode("strong", one.self_id ? `${name} (${one.self_id})` : name),
      textNode("p", one.model || one.id),
    ]);
    card.dataset.name = one.id;
    card.dataset.selected = one.id === sessionId ? "1" : "0";
    frame.appendChild(card);
  }
  dom.list.appendChild(frame);
}

function renderHistoryPager(dom, sessionId, offset, total) {
  dom.pager.innerHTML = "";
  if (total <= HISTORY_PAGE_SIZE) {
    return;
  }

  const last = Math.floor((total - 1) / HISTORY_PAGE_SIZE) * HISTORY_PAGE_SIZE;
  const prev = _("a", { href: historyLink(sessionId, Math.max(offset - HISTORY_PAGE_SIZE, 0)) }, i18n("text_prev"));
  const next = _("a", { href: historyLink(sessionId, Math.min(offset + HISTORY_PAGE_SIZE, last)) }, i18n("text_next"));
  if (offset <= 0) {
    prev.dataset.disabled = "1";
  }
  if (offset >= last) {
    next.dataset.disabled = "1";
  }

  dom.pager.appendChild(prev);
  dom.pager.appendChild(textNode("p", `${offset + 1}-${Math.min(offset + HISTORY_PAGE_SIZE, total)} / ${total}`));
  dom.pager.appendChild(next);
}

async function renderHistoryPage(sessionId, offset) {
  const dom = historyDom();
  if (!dom.list) {
    return;
  }

  const sessions = await fetchUsageSessions();
  historySessionList(dom, sessions, sessionId);

  const label = {};
  for (const one of sessions) {
    if (one && one.id) {
      label[one.id] = one.name || one.id;
    }
  }

  dom.entries.innerHTML = "";
  dom.pager.innerHTML = "";

  const url = praseURL();
  const keyword = url.keyword || "";
  const all = sessionId ? await historyTasks(sessionId, keyword) : await historyAllTasks(sessions, keyword);
  const tasks = all.filter((task) => historyMatch(task, url.from || "", url.to || ""));

  const search = $("#history-keyword");
  if (search && url.keyword) {
    search.value = url.keyword;
  }
  if (tasks.length === 0) {
    dom.entries.appendChild(textNode("p.empty", i18n("label_empty_history")));
    return;
  }

  const start = Math.min(Math.max(offset, 0), Math.floor((tasks.length - 1) / HISTORY_PAGE_SIZE) * HISTORY_PAGE_SIZE);
  const entries = _("temp");
  for (const task of tasks.slice(start, start + HISTORY_PAGE_SIZE)) {
    const when = historyClock(task.end_at);
    entries.appendChild(
      _("a.row", { href: detailsLink(task.session, task.task_hash) }, [
        textNode("strong", task.objective || task.task_hash),
        textNode("p", sessionId ? when : `${when} · ${label[task.session] || task.session}`),
      ]),
    );
  }
  dom.entries.appendChild(entries);
  renderHistoryPager(dom, sessionId, start, tasks.length);
}

function detailsCard(label, hint, href, selected) {
  const card = _("a.card", { href: href }, [textNode("strong", label), textNode("p", hint)]);
  card.dataset.selected = selected ? "1" : "0";
  return card;
}

async function renderDetailsPage(sessionId, hash, item) {
  const dom = historyDom();
  if (!dom.body || !dom.detail) {
    return;
  }

  dom.detail.innerHTML = "";
  dom.body.innerHTML = "";
  if (dom.back) {
    dom.back.href = historyLink(sessionId, 0);
  }

  if (!sessionId || !hash) {
    dom.body.appendChild(textNode("p.empty", i18n("label_no_selection")));
    return;
  }

  const detail = await historyDetail(sessionId, hash);
  if (!detail) {
    dom.body.appendChild(textNode("p.empty", i18n("error_failed_to_load")));
    return;
  }

  const tools = detail.tool_results || [];
  const at = Number(item);
  const picked = item !== "" && Number.isInteger(at) && tools[at] ? at : -1;

  dom.detail.appendChild(
    detailsCard(
      i18n("text_result"),
      `${detail.model || "auto"} · ${detail.reasoning || "medium"}`,
      detailsLink(sessionId, hash, ""),
      picked < 0,
    ),
  );
  tools.forEach((one, index) => {
    dom.detail.appendChild(
      detailsCard(one.name || i18n("text_tool"), i18n("label_tool_call", index + 1), detailsLink(sessionId, hash, String(index)), picked === index),
    );
  });

  if (picked >= 0) {
    dom.body.appendChild(_("div.head", [textNode("strong", tools[picked].name || i18n("text_tool")), textNode("p", tools[picked].id || "")]));
    if (tools[picked].args) {
      dom.body.appendChild(labeledBlock(i18n("text_args"), historyText(tools[picked].args)));
    }
    dom.body.appendChild(labeledBlock(i18n("text_result"), historyText(tools[picked].result)));
    return;
  }

  dom.body.appendChild(
    _("div.head", [
      textNode("strong", detail.objective || hash),
      textNode("p", `${detail.model || "auto"} · ${detail.reasoning || "medium"}`),
    ]),
  );

  for (const one of detail.todos || []) {
    dom.body.appendChild(_("div.todo", [textNode("span", one.status || ""), textNode("p", one.content || "")]));
  }

  if (detail.reply) {
    dom.body.appendChild(codeBlock(historyText(detail.reply)));
  }
}

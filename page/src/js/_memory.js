async function memoryPost(action, body, sessionId) {
  const sid = sessionId || currentSessionId;
  if (!sid) {
    console.error("memoryPost", action, "no session id", { sessionId: sessionId, currentSessionId: currentSessionId });
    alert(i18n("error_no_session", action));
    return null;
  }

  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sid)}/memory`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: action, ...(body || {}) }),
    });
    const detail = await response.json().catch(() => ({}));
    if (!response.ok) {
      console.error("memoryPost", action, response.status, detail);
      alert(i18n("error_failed_memory", action, detail.error || response.status));
      return null;
    }
    return detail;
  } catch (err) {
    console.error("memoryPost", action, err);
    alert(i18n("error_failed_memory", action, err.message || err));
    return null;
  }
}

const MEMORY_ACTIONS = [
  { key: "summary", labelKey: "label_summary", hintKey: "hint_summary_memory" },
  { key: "compact", labelKey: "label_compact", hintKey: "hint_compact" },
  { key: "reset", labelKey: "label_reset", hintKey: "hint_reset" },
];

function openMemoryPicker(sessionId) {
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, i18n("text_cancel"));
  const root = _("div.popup", [_("div.panel", [_("strong", i18n("text_memory")), list, _("footer", [cancel])])]);
  root.id = "memory-popup";

  const close = () => root.remove();
  const run = function (key) {
    if (key === "summary") {
      memorySummary(sessionId);
      return;
    }
    if (key === "compact") {
      memoryCompact(sessionId);
      return;
    }
    memoryReset(sessionId);
  };

  for (const one of MEMORY_ACTIONS) {
    const box = _("input", { type: "radio", name: "memory-pick", value: one.key });
    box.addEventListener("change", () => {
      run(one.key);
      box.checked = false;
    });
    list.appendChild(_("label", [box, _("div", [_("strong", i18n(one.labelKey)), _("p", i18n(one.hintKey))])]));
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

async function memorySummary(sessionId) {
  if (!confirm(i18n("confirm_summary"))) {
    return;
  }
  const result = await memoryPost("summary", null, sessionId);
  if (!result) {
    return;
  }
  alert(i18n(result.count === 1 ? "error_summary_done_one" : "error_summary_done_many", result.count || 0));
}

async function memoryCompact(sessionId) {
  if (!confirm(i18n("confirm_compact"))) {
    return;
  }
  const result = await memoryPost("compact", null, sessionId);
  if (!result) {
    return;
  }
  if (!result.removed) {
    alert(i18n("error_compact_nothing"));
    return;
  }
  alert(i18n("error_compacted", result.removed));
  window.location.reload();
}

async function memoryReset(sessionId) {
  if (!confirm(i18n("confirm_reset"))) {
    return;
  }
  const wipe = confirm(i18n("confirm_wipe"));
  const result = await memoryPost("reset", { mode: wipe ? "all" : "summary" }, sessionId);
  if (!result) {
    return;
  }
  alert(i18n(wipe ? "error_reset_done" : "error_reset_done_kept", result.removed || 0));
  window.location.reload();
}

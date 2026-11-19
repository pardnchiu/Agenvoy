const CONFIRM_OPTIONS = [
  { key: "yes", label: "Yes", body: { approve: true }, style: "approve" },
  { key: "remember", label: "Yes, don't ask again", body: { approve: true, remember: true }, style: "approve" },
  { key: "allow_turn", label: "Yes, allow this turn", body: { approve: true, allow_turn: true }, style: "approve" },
  { key: "skip", label: "No, skip this", body: { approve: false }, style: "reject", reason: true },
  { key: "abort", label: "Abort, keep for resume", body: { approve: false, abort: true }, style: "abort" },
  { key: "cancel", label: "Cancel, delete task", body: { approve: false, cancel: true }, style: "abort" },
];

const CONFIRM_TEXT = {
  password: "System password",
  password_empty: "type your system password first",
  password_rejected: "password rejected",
  reason: "Why not? Tell it what to do instead (enter to skip)",
  send: "send",
};

async function resolveToolConfirm(sessionId, requestId, body, dom, text) {
  const note = dom.querySelector("p.note");
  try {
    const response = await fetch(
      `${API}/v1/session/${encodeURIComponent(sessionId || currentSessionId)}/confirm/${encodeURIComponent(requestId)}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
    if (response.status === 401 || response.status === 403) {
      const detail = await response.json().catch(() => ({}));
      if (note) {
        note.textContent = detail.error || text.password_rejected;
      }
      return;
    }
  } catch (err) {
    console.error("resolveToolConfirm", err);
  }
  dom.remove();
}

function askRejectReason(card, text, done) {
  const action = card.querySelector("div.action");
  if (!action) {
    done("");
    return;
  }

  const field = _("input", {
    type: "text",
    class: "reason",
    autocomplete: "off",
    placeholder: text.reason,
  });
  const send = _("button", { type: "button", class: "reject" }, [_("p", text.send)]);
  const row = _("div.action", [field, send]);

  send.addEventListener("click", () => done(field.value.trim()));
  field.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      done(field.value.trim());
    }
  });

  action.replaceWith(row);
  field.focus();
}

function renderToolConfirm(event, sessionId) {
  const requestId = event.confirm_hash || "";
  const dom = chatPart("confirm", sessionId);
  if (!requestId || !dom) {
    return;
  }
  clearPendingHint(sessionId);
  if (dom.querySelector(`div.tool-confirm[data-id="${requestId}"]`)) {
    return;
  }

  const body = [_("p.name", event.confirm_title || "")];
  if (event.tool_display) {
    body.push(_("pre.args", event.tool_display));
  }
  if (event.explain) {
    body.push(_("p.explain", event.explain));
  }

  const restricted = event.restricted || [];
  const cached = Boolean(event.password_cached);
  let password = null;
  let note = null;
  for (const line of event.restricted_lines || []) {
    body.push(_("p.restricted", line));
  }
  if (event.restricted_note) {
    body.push(_("p.restricted", event.restricted_note));
  }
  if (restricted.length > 0 && cached) {
    note = _("p.note", "");
    body.push(note);
  }
  if (restricted.length > 0 && !cached) {
    password = _("input", {
      type: "password",
      autocomplete: "off",
      "data-1p-ignore": "true",
      "data-lpignore": "true",
      placeholder: text.password,
    });
    note = _("p.note", "");
    password.addEventListener("input", () => {
      note.textContent = "";
    });
    body.push(password);
    body.push(note);
  }

  const action = _("div.action");
  let primary = null;
  const card = _("div.tool-confirm", { "data-id": requestId }, [...body, action]);
  const labels = event.confirm_options || {};
  const text = Object.assign({}, CONFIRM_TEXT, event.confirm_text || {});
  for (const option of CONFIRM_OPTIONS) {
    if (restricted.length > 0 && (option.body.remember || option.body.allow_turn)) {
      continue;
    }
    const button = _("button", { type: "button", class: option.style }, [_("p", labels[option.key] || option.label)]);
    button.addEventListener("click", () => {
      const payload = Object.assign({}, option.body);
      if (password && option.body.approve) {
        const field = card.querySelector('input[type="password"]') || password;
        if (field.value === "") {
          note.textContent = text.password_empty;
          field.focus();
          return;
        }
        payload.password = field.value;
        note.textContent = "";
      }
      if (option.reason) {
        askRejectReason(card, text, (reason) => {
          if (reason !== "") {
            payload.reason = reason;
          }
          resolveToolConfirm(sessionId, requestId, payload, card, text);
        });
        return;
      }
      resolveToolConfirm(sessionId, requestId, payload, card, text);
    });
    if (password && option.body.approve && !primary) {
      primary = button;
    }
    action.appendChild(button);
  }

  if (password && primary) {
    password.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        primary.click();
      }
    });
  }

  dom.appendChild(card);
  scrollToBottom(true, sessionId);
  if (password) {
    password.focus();
  }
}

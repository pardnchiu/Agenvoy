const CONFIRM_OPTIONS = [
  { label: "yes", body: { approve: true }, style: "approve" },
  { label: "yes, don't ask again", body: { approve: true, remember: true }, style: "approve" },
  { label: "yes, allow this turn", body: { approve: true, allow_turn: true }, style: "approve" },
  { label: "no, skip this", body: { approve: false }, style: "reject", reason: true },
  { label: "abort task", body: { approve: false, abort: true }, style: "abort" },
  { label: "cancel task", body: { approve: false, cancel: true }, style: "abort" },
];

async function resolveToolConfirm(sessionId, requestId, body, dom) {
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
        note.textContent = detail.error || "password rejected";
      }
      return;
    }
  } catch (err) {
    console.error("resolveToolConfirm", err);
  }
  dom.remove();
}

function askRejectReason(card, done) {
  const action = card.querySelector("div.action");
  if (!action) {
    done("");
    return;
  }

  const field = _("input", {
    type: "text",
    class: "reason",
    autocomplete: "off",
    placeholder: "Why not? tell it what to do instead (enter to skip)",
  });
  const send = _("button", { type: "button", class: "reject" }, [_("p", "send")]);
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

  const body = [_("p.name", event.tool_name || "tool")];
  const args = (event.tool_display || event.tool_args || "").trim();
  if (args && args !== "{}") {
    body.push(_("pre.args", args));
  }
  if (event.explain) {
    body.push(_("p.explain", event.explain));
  }

  const restricted = event.restricted || [];
  const cached = Boolean(event.password_cached);
  let password = null;
  let note = null;
  if (restricted.length > 0 && cached) {
    body.push(_("p.restricted", `outside the allow list · ${restricted.join(", ")} · sudo still valid, no password needed`));
    note = _("p.note", "");
    body.push(note);
  }
  if (restricted.length > 0 && !cached) {
    body.push(_("p.restricted", `needs your system password · ${restricted.join(", ")}`));
    password = _("input", {
      type: "password",
      autocomplete: "off",
      "data-1p-ignore": "true",
      "data-lpignore": "true",
      placeholder: "System password",
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
  for (const option of CONFIRM_OPTIONS) {
    if (restricted.length > 0 && (option.body.remember || option.body.allow_turn)) {
      continue;
    }
    const button = _("button", { type: "button", class: option.style }, [_("p", option.label)]);
    button.addEventListener("click", () => {
      const payload = Object.assign({}, option.body);
      if (password && option.body.approve) {
        const field = card.querySelector('input[type="password"]') || password;
        if (field.value === "") {
          note.textContent = "type your system password first";
          field.focus();
          return;
        }
        payload.password = field.value;
        note.textContent = "";
      }
      if (option.reason) {
        askRejectReason(card, (text) => {
          if (text !== "") {
            payload.reason = text;
          }
          resolveToolConfirm(sessionId, requestId, payload, card);
        });
        return;
      }
      resolveToolConfirm(sessionId, requestId, payload, card);
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

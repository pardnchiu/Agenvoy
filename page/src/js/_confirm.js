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

const ASK_TEXT = {
  placeholder: "Type your answer...",
  send: "Send",
  skip: "Skip",
};

function renderAskPrompt(event, sessionId) {
  const requestId = event.confirm_hash || "";
  const dom = chatPart("confirm", sessionId);
  const questions = event.questions || [];
  if (!requestId || !dom || questions.length === 0) {
    return;
  }
  clearPendingHint(sessionId);
  if (dom.querySelector(`div.tool-confirm[data-id="${requestId}"]`)) {
    return;
  }

  const text = Object.assign({}, ASK_TEXT, event.ask_text || {});
  const answers = questions.map((q) => (q.multi_select ? [] : ""));
  const card = _("div.tool-confirm", { "data-id": requestId });
  const submit = () => resolveToolConfirm(sessionId, requestId, { answers: answers }, card, CONFIRM_TEXT);
  const direct = questions.length === 1 && (questions[0].options || []).length > 0 && !questions[0].multi_select;

  questions.forEach((question, index) => {
    card.appendChild(_("p.name", question.question || ""));
    if (question.detail) {
      card.appendChild(_("p.detail", question.detail));
    }

    const options = question.options || [];
    if (direct) {
      const action = _("div.action");
      options.forEach((option, i) => {
        const button = _("button", { type: "button", class: i === 0 ? "approve" : "reject" }, [_("p", option)]);
        button.addEventListener("click", () => {
          answers[index] = option;
          submit();
        });
        action.appendChild(button);
      });
      card.appendChild(action);
      return;
    }

    if (options.length > 0) {
      for (const option of options) {
        const box = _("input", { type: question.multi_select ? "checkbox" : "radio", name: `${requestId}-${index}`, value: option });
        box.addEventListener("change", () => {
          if (!question.multi_select) {
            answers[index] = option;
            return;
          }
          const picked = answers[index].filter((item) => item !== option);
          if (box.checked) {
            picked.push(option);
          }
          answers[index] = picked;
        });
        card.appendChild(_("label.option", [box, _("span", option)]));
      }
      return;
    }

    const field = _("input", {
      type: question.secret ? "password" : "text",
      class: "reason",
      autocomplete: "off",
      "data-1p-ignore": "true",
      "data-lpignore": "true",
      placeholder: text.placeholder,
    });
    field.addEventListener("input", () => (answers[index] = field.value));
    field.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.isComposing) {
        e.preventDefault();
        submit();
      }
    });
    card.appendChild(_("div.action", [field]));
  });

  if (!direct) {
    const skip = _("button", { type: "button", class: "reject" }, [_("p", text.skip)]);
    skip.addEventListener("click", () => {
      answers.fill("");
      submit();
    });
    const send = _("button", { type: "button", class: "approve" }, [_("p", text.send)]);
    send.addEventListener("click", submit);
    card.appendChild(_("div.action", [skip, send]));
  }

  dom.appendChild(card);
  scrollToBottom(true, sessionId);
}

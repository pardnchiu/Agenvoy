async function openPersonaPopup(sessionId) {
  const sid = sessionId || currentSessionId;
  if (!sid) {
    return;
  }

  let current = {};
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sid)}`);
    const detail = await response.json().catch(() => ({}));
    if (!response.ok) {
      alert(i18n("error_failed_persona", detail.error || response.status));
      return;
    }
    current = detail;
  } catch (err) {
    console.error("openPersonaPopup", err);
    alert(i18n("error_failed_persona", err.message || err));
    return;
  }

  const self = personaField(current.self_id || "", i18n("hint_self_id"), true);
  const name = personaField(current.name || "", i18n("hint_name"), true);
  const role = personaField(current.role || "", i18n("hint_role"), false);

  const cancel = _("button", { type: "button" }, i18n("text_cancel"));
  const save = _("button", { type: "button", class: "submit" }, i18n("text_save"));

  const root = _("div.popup", [
    _("div.panel", [
      _("strong", i18n("text_bot")),
      _("p", i18n("text_name")),
      name.field,
      _("p", i18n("label_self_id")),
      self.field,
      _("p", i18n("text_role")),
      role.field,
      _("footer", [cancel, save]),
    ]),
  ]);
  root.id = "persona-popup";

  const close = function () {
    document.removeEventListener("keydown", escape);
    root.remove();
  };
  const escape = function (e) {
    if (e.key === "Escape") {
      close();
    }
  };
  const submit = async function () {
    save.disabled = true;
    const done = await savePersona(sid, self.box.value.trim(), name.box.value.trim(), role.box.value);
    save.disabled = false;
    if (done) {
      close();
    }
  };

  for (const one of [name, self]) {
    one.box.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" || e.shiftKey || e.isComposing) {
        return;
      }
      e.preventDefault();
      submit();
    });
  }
  cancel.addEventListener("click", close);
  save.addEventListener("click", submit);
  root.addEventListener("click", function (e) {
    if (e.target === root) {
      close();
    }
  });
  document.addEventListener("keydown", escape);

  document.body.appendChild(root);
  name.box.focus();
  name.box.select();
}

function personaField(value, placeholder, single) {
  const box = _("textarea", { placeholder: placeholder });
  box.value = value;

  const mirror = _("pre");
  mirror.textContent = box.value + "\n";

  box.addEventListener("input", function () {
    if (single) {
      box.value = box.value.replace(/\n/g, "");
    }
    mirror.textContent = box.value + "\n";
  });

  return { field: _("label.input", [box, mirror]), box: box };
}

async function savePersona(sessionId, selfId, name, role) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId || currentSessionId)}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ self_id: selfId, name: name, role: role }),
    });
    const detail = await response.json().catch(() => ({}));
    if (!response.ok) {
      alert(i18n("error_failed_persona", detail.error || response.status));
      return false;
    }
  } catch (err) {
    console.error("savePersona", err);
    alert(i18n("error_failed_persona", err.message || err));
    return false;
  }
  return true;
}

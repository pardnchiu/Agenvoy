const RULE_TEMPLATE = `# Role

<one line: who this agent is and who it works for>

## Always

-

## Never

-

## Output

-
`;

let roleEditing = "";

function roleDom() {
  return {
    form: $("#role-form"),
    list: $("#role-list"),
    title: $("#role-title"),
    content: $("#role-content"),
    submit: document.querySelector("#role-form button.submit"),
  };
}

function roleError(text) {
  alert(text);
}

function markRoleEditing(dom, editing) {
  if (dom.form) {
    if (editing) {
      dom.form.dataset.editing = "1";
    } else {
      delete dom.form.dataset.editing;
    }
  }
  if (dom.submit) {
    dom.submit.textContent = editing ? i18n("text_save") : i18n("text_add");
  }
}

async function renderRole() {
  const dom = roleDom();
  if (!dom.list) {
    return;
  }

  let items = [];
  try {
    const response = await fetch(`${API}/v1/roles`);
    if (response.ok) {
      items = (await response.json()).roles || [];
    }
  } catch (err) {
    console.error("renderRole", err);
  }

  dom.list.innerHTML = "";
  const picked = praseURL().target || "";
  let found = false;

  for (const item of items) {
    if (!item.name) continue;
    if (item.name === picked) {
      found = true;
    }

    const remove = _("button", { type: "button" }, [_("span.material-symbols-outlined", "delete")]);
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteRole(item.name);
    });

    const card = _("div.card", [
      _("strong", item.name),
      _("p", item.updated_at ? roleDate(item.updated_at) : ""),
      remove,
    ]);
    card.dataset.name = item.name;
    card.dataset.selected = item.name === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "features", tab: "Roles", target: item.name });
    });
    dom.list.appendChild(card);
  }

  if (found) {
    openRole(picked);
  }
}

function roleDate(seconds) {
  const date = new Date(seconds * 1000);
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

async function openRole(name) {
  const dom = roleDom();
  if (!dom.title) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/role/${encodeURIComponent(name)}`);
    if (!response.ok) {
      roleError(`HTTP ${response.status}`);
      return;
    }
    const body = await response.json();
    dom.title.value = body.name || "";
    dom.content.value = body.content || "";
    roleEditing = body.name || "";
    markRoleEditing(dom, true);
  } catch (err) {
    console.error("openRole", err);
    roleError(err.message || i18n("text_failed"));
  }
}

function resetRole() {
  const dom = roleDom();
  if (dom.title) dom.title.value = "";
  if (dom.content) dom.content.value = RULE_TEMPLATE;
  markRoleEditing(dom, false);
  roleEditing = "";
  markSelectedCard(dom.list, "");
}

async function saveRole() {
  const dom = roleDom();
  if (!dom.title) {
    return;
  }

  const name = dom.title.value.trim();
  if (!name) {
    roleError(i18n("error_name_required"));
    return;
  }

  const body = { name: name, content: dom.content ? dom.content.value : "" };

  let method = "POST";
  if (roleEditing) {
    method = "PATCH";
    if (roleEditing !== name) {
      body.rename = name;
      body.name = roleEditing;
    }
  }

  try {
    const response = await fetch(`${API}/v1/role`, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      roleError(detail.error || `HTTP ${response.status}`);
      return;
    }
    const saved = ((await response.json()) || {}).name || name;
    window.location.href = getLink({ page: "features", tab: "Roles", target: saved });
  } catch (err) {
    console.error("saveRole", err);
    roleError(err.message || i18n("text_failed"));
  }
}

async function deleteRole(name) {
  if (!confirm(i18n("confirm_delete", name))) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/role?name=${encodeURIComponent(name)}`, { method: "DELETE" });
    if (!response.ok) {
      roleError(`HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("deleteRole", err);
    roleError(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "features", tab: "Roles" });
}

function deleteEditingRole() {
  if (roleEditing) {
    deleteRole(roleEditing);
  }
}

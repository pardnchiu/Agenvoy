let role = "";
let roleName = "";
let roleList = [];

async function getRoleList() {
  roleList = [];
  try {
    const response = await fetch(`${API}/v1/roles`);
    if (response.ok) {
      roleList = (await response.json()).roles || [];
    }
  } catch (err) {
    console.error("getRoleList", err);
  }

  await selectRole(readChatConfig(currentSessionId).role);
}

function markRole(name) {
  const dom = $("section.chat button.role");
  if (!dom) {
    return;
  }
  if (name) {
    dom.dataset.selected = "1";
    dom.title = name;
    dom.name = name.slice(0, 16);
    return;
  }
  delete dom.dataset.selected;
  dom.removeAttribute("title");
  dom.name = i18n("tooltip_use");
}

async function selectRole(name) {
  if (!name) {
    role = "";
    roleName = "";
    writeChatConfig(currentSessionId, { role: "" });
    markRole("");
    return;
  }

  let content = "";
  try {
    const response = await fetch(`${API}/v1/role/${encodeURIComponent(name)}`);
    if (response.ok) {
      content = (await response.json()).content || "";
    }
  } catch (err) {
    console.error("selectRole", err);
  }

  if (!content) {
    role = "";
    roleName = "";
    writeChatConfig(currentSessionId, { role: "" });
    markRole("");
    return;
  }

  role = content;
  roleName = name;
  writeChatConfig(currentSessionId, { role: name });
  markRole(name);
}

function openRolePicker() {
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, i18n("text_cancel"));
  const root = _("div.popup", [_("div.panel", [_("strong", i18n("text_role")), list, _("footer", [cancel])])]);
  root.id = "role-popup";

  const close = () => root.remove();
  const add = function (value, body) {
    const box = _("input", { type: "radio", name: "role-pick", value: value });
    box.checked = roleName === value;
    box.addEventListener("change", () => {
      selectRole(value);
      close();
    });
    list.appendChild(_("label", [box, _("div", body)]));
  };

  add("", [_("strong", i18n("text_default"))]);
  for (const item of roleList) {
    if (!item.name) continue;
    add(item.name, [_("strong", item.name)]);
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

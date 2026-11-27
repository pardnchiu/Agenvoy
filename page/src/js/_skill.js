let skill = "";

function clearSkill() {
  skill = "";
  markSkill("");
}

async function openSkillPicker() {
  let items = [];
  try {
    const response = await fetch(`${API}/v1/skills`);
    if (response.ok) {
      items = (await response.json()).skills || [];
    }
  } catch (err) {
    console.error("openSkillPicker", err);
  }

  const list = _("div.list");

  const cancel = _("button", { type: "button" }, i18n("text_cancel"));
  const root = _("div.popup", [_("div.panel", [_("strong", i18n("text_skill")), list, _("footer", [cancel])])]);
  root.id = "skill-popup";

  const close = () => root.remove();
  const add = function (value, body) {
    const box = _("input", { type: "radio", name: "skill-pick", value: value });
    box.checked = skill === value;
    box.addEventListener("change", () => {
      skill = value;
      markSkill(skill);
      prefixChatInput(skill);
      close();
    });
    list.appendChild(_("label", [box, _("div", body)]));
  };

  add("", [_("strong", i18n("text_none"))]);
  for (const item of items) {
    if (!item.name) continue;
    add(item.name, item.description ? [_("strong", item.name), _("p", item.description)] : [_("strong", item.name)]);
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

function prefixChatInput(name) {
  const dom = $("#chat-input");
  if (!dom) {
    return;
  }

  const rest = dom.value.replace(/^\/\S*\s*/, "");
  dom.value = name ? `/${name} ${rest}` : rest;
  if (dom.nextElementSibling) {
    dom.nextElementSibling.textContent = dom.value + "\n";
  }
  dom.focus();
  dom.setSelectionRange(dom.value.length, dom.value.length);
}

function markSkill(name) {
  const dom = $("section.chat button.skill");
  if (!dom) {
    return;
  }
  if (name) {
    dom.dataset.selected = "1";
    dom.title = name;
    return;
  }
  delete dom.dataset.selected;
  dom.removeAttribute("title");
}

let skillTabName = "";
let skillTabPath = "";
let skillSourceBoxes = {};

function skillTabDom() {
  return {
    form: $("#skill-form"),
    list: $("#skill-list"),
    name: $("#skill-name"),
    content: $("#skill-content"),
    files: $("#skill-files"),
    allowList: $("#skill-allow-list"),
    sourceList: $("#skill-source-list"),
    remove: document.querySelector("#skill-form button.remove"),
  };
}

async function skillTabState() {
  const out = { skills: [], allowed: {}, source: {} };
  try {
    const response = await fetch(`${API}/v1/allowlist?scope=global`);
    if (response.ok) {
      const body = ((await response.json()) || {}).skill || {};
      out.skills = body.skills || [];
      out.allowed = body.allowed || {};
      out.source = body.source || {};
    }
  } catch (err) {
    console.error("skillTabState", err);
  }
  return out;
}

async function renderSkillTab() {
  const dom = skillTabDom();
  if (!dom.list) {
    return;
  }

  const { skills, allowed, source } = await skillTabState();

  dom.list.innerHTML = "";
  const picked = praseURL().target || "";

  for (const name of skills) {
    const mark = allowed[name] ? i18n("text_trust") : i18n("label_ask_each_time");
    const card = _("div.card", [_("strong", name), _("p", `${source[name] || i18n("text_unknown")} · ${mark}`)]);
    card.dataset.name = name;
    card.dataset.selected = name === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "features", tab: "Skills", target: name });
    });
    dom.list.appendChild(card);
  }

  if (picked && skills.includes(picked)) {
    openSkillTab(picked);
  }
}

async function openSkillTab(name) {
  const dom = skillTabDom();
  if (!dom.form) {
    return;
  }

  let body = null;
  try {
    const response = await fetch(`${API}/v1/skill/${encodeURIComponent(name)}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
    body = await response.json();
  } catch (err) {
    console.error("openSkillTab", err);
    alert(err.message || i18n("text_failed"));
    return;
  }

  skillTabName = body.name || name;
  skillTabPath = body.path || "";
  dom.name.value = body.source ? `${skillTabName} · ${body.source}` : skillTabName;
  const files = [{ path: "SKILL.md", content: body.content || "" }].concat(body.files || []);
  dom.content.value = files[0].content;
  renderSkillFiles(dom, files);
  dom.form.dataset.editing = "1";
  delete dom.form.dataset.view;
  delete dom.allowList.dataset.open;
  delete dom.sourceList.dataset.open;
  dom.remove.style.display = body.deletable ? "" : "none";
}

function renderSkillFiles(dom, files) {
  if (!dom.files) {
    return;
  }

  dom.files.innerHTML = "";
  if (files.length === 0) {
    return;
  }

  const buttons = [];
  for (const one of files) {
    const button = _("button", { type: "button" }, one.path);
    button.dataset.selected = "0";
    button.addEventListener("click", function () {
      for (const other of buttons) {
        other.dataset.selected = other === button ? "1" : "0";
      }
      dom.content.value = one.content || "";
    });
    buttons.push(button);
    dom.files.appendChild(button);
  }
  buttons[0].dataset.selected = "1";
}

async function openSkillConfig() {
  const dom = skillTabDom();
  if (!dom.form) {
    return;
  }

  skillTabName = "";
  skillTabPath = "";
  renderSkillFiles(dom, []);
  delete dom.form.dataset.editing;
  dom.form.dataset.view = "config";
  markSelectedCard(dom.list, "");
  delete dom.sourceList.dataset.open;
  renderSkillAllowList();
}

async function openSkillSource() {
  const dom = skillTabDom();
  if (!dom.form) {
    return;
  }

  skillTabName = "";
  skillTabPath = "";
  renderSkillFiles(dom, []);
  delete dom.form.dataset.editing;
  dom.form.dataset.view = "source";
  markSelectedCard(dom.list, "");
  delete dom.allowList.dataset.open;

  let body = null;
  try {
    const response = await fetch(`${API}/v1/config/skill_source`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
    body = await response.json();
  } catch (err) {
    console.error("openSkillSource", err);
    alert(err.message || i18n("text_failed"));
    return;
  }

  const disabled = body.disabled || [];
  skillSourceBoxes = {};
  dom.sourceList.innerHTML = "";
  dom.sourceList.dataset.open = "1";
  dom.sourceList.appendChild(_("strong", i18n("title_source")));
  for (const name of body.sources || []) {
    const box = _("input", { type: "checkbox" });
    box.checked = !disabled.includes(name);
    skillSourceBoxes[name] = box;
    dom.sourceList.appendChild(_("label.tool", [box, _("p", name)]));
  }
}

async function saveSkillSource() {
  const boxes = skillSourceBoxes;
  const disabled = Object.keys(boxes).filter((name) => !boxes[name].checked);
  try {
    const response = await fetch(`${API}/v1/config/skill_source`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ disabled: disabled }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("saveSkillSource", err);
    alert(err.message || i18n("text_failed"));
    return;
  }
  renderSkillTab();
}

async function renderSkillAllowList() {
  const dom = skillTabDom();
  if (!dom.allowList) {
    return;
  }

  const { skills, allowed } = await skillTabState();

  dom.allowList.innerHTML = "";
  dom.allowList.dataset.open = "1";
  dom.allowList.appendChild(_("strong", i18n("title_trust")));

  for (const name of skills) {
    const box = _("input", { type: "checkbox" });
    box.checked = Boolean(allowed[name]);
    box.addEventListener("change", () => toggleSkillAllow(name));
    dom.allowList.appendChild(_("label.tool", [box, _("p", name)]));
  }
}

async function toggleSkillAllow(name) {
  if (!name) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/allowlist`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ skill: { scope: "global", name: name } }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("toggleSkillAllow", err);
    alert(err.message || i18n("text_failed"));
    return;
  }
  renderSkillTab();
}

async function deleteSkillTab() {
  if (!skillTabName || !confirm(i18n("confirm_delete", skillTabName))) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/skill?name=${encodeURIComponent(skillTabName)}`, { method: "DELETE" });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("deleteSkillTab", err);
    alert(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "features", tab: "Skills" });
}

async function openSkillFolder() {
  const at = skillTabPath.lastIndexOf("/");
  if (at < 1) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/file/open?path=${encodeURIComponent(skillTabPath.slice(0, at))}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("openSkillFolder", err);
    alert(err.message || i18n("text_failed"));
  }
}

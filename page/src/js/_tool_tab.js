function toolTabDom() {
  return {
    form: $("#tool-form"),
    list: $("#tool-list"),
    name: $("#tool-name"),
    content: $("#tool-content"),
    files: $("#tool-files"),
    allowList: $("#tool-allow-list"),
  };
}

function toolAllowLabel(tool) {
  if (tool.always_allow) {
    return `${i18n("text_allow")} · tool.json`;
  }
  return tool.allowed ? i18n("text_allow") : i18n("label_ask_each_time");
}

async function toolTabState() {
  try {
    const response = await fetch(`${API}/v1/tools`);
    if (response.ok) {
      return ((await response.json()) || {}).tools || [];
    }
  } catch (err) {
    console.error("toolTabState", err);
  }
  return [];
}

async function renderToolTab() {
  const dom = toolTabDom();
  if (!dom.list) {
    return;
  }

  const tools = await toolTabState();

  dom.list.innerHTML = "";
  const picked = praseURL().target || "";

  for (const tool of tools) {
    const card = _("div.card", [_("strong", tool.name), _("p", toolAllowLabel(tool))]);
    card.dataset.name = tool.name;
    card.dataset.selected = tool.name === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "features", tab: "Tools", target: tool.name });
    });
    dom.list.appendChild(card);
  }

  if (picked && tools.some((tool) => tool.name === picked)) {
    openToolTab(picked);
  }
}

async function openToolTab(name) {
  const dom = toolTabDom();
  if (!dom.form) {
    return;
  }

  let body = null;
  try {
    const response = await fetch(`${API}/v1/tool/${encodeURIComponent(name)}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
    body = await response.json();
  } catch (err) {
    console.error("openToolTab", err);
    alert(err.message || i18n("text_failed"));
    return;
  }

  const files = body.files || [];
  dom.name.value = body.name || name;
  dom.content.value = files.length > 0 ? files[0].content || "" : "";
  dom.form.dataset.path = body.path || "";
  renderToolFiles(dom, files);
  dom.form.dataset.editing = "1";

  delete dom.form.dataset.view;
  delete dom.allowList.dataset.open;
}

function renderToolFiles(dom, files) {
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

async function openToolConfig() {
  const dom = toolTabDom();
  if (!dom.form) {
    return;
  }

  renderToolFiles(dom, []);
  delete dom.form.dataset.editing;
  dom.form.dataset.view = "config";
  markSelectedCard(dom.list, "");
  renderToolAllowList();
}

async function renderToolAllowList() {
  const dom = toolTabDom();
  if (!dom.allowList) {
    return;
  }

  const tools = await toolTabState();

  dom.allowList.innerHTML = "";
  dom.allowList.dataset.open = "1";
  dom.allowList.appendChild(_("strong", i18n("title_allow_tools")));

  if (tools.length === 0) {
    dom.allowList.appendChild(_("p", i18n("label_no_script_tools")));
    return;
  }

  for (const tool of tools) {
    const box = _("input", { type: "checkbox" });
    box.checked = Boolean(tool.allowed);
    if (tool.always_allow) {
      box.disabled = true;
      dom.allowList.appendChild(_("label.tool", [box, _("p", `${tool.name} · ${i18n("hint_tool_always_allow")}`)]));
      continue;
    }
    box.addEventListener("change", () => toggleToolAllow(tool.name));
    dom.allowList.appendChild(_("label.tool", [box, _("p", tool.name)]));
  }
}

async function toggleToolAllow(name) {
  if (!name) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/tool/allow`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      renderToolAllowList();
      return;
    }
  } catch (err) {
    console.error("toggleToolAllow", err);
    alert(err.message || i18n("text_failed"));
    renderToolAllowList();
    return;
  }
  renderToolTab();
}


async function openToolFolder() {
  const path = toolTabDom().form?.dataset.path;
  if (!path) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/file/open?path=${encodeURIComponent(path)}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("openToolFolder", err);
    alert(err.message || i18n("text_failed"));
  }
}

let hookTabName = "";
let hookTabFiles = [];
let hookTabExt = "";
let hookTabDrafts = {};

function hookTabDom() {
  return {
    form: $("#hook-form"),
    list: $("#hook-list"),
    name: $("#hook-name"),
    files: $("#hook-files"),
    content: $("#hook-content"),
    remove: document.querySelector("#hook-form > footer button.remove"),
    open: document.querySelector("#hook-form > footer button.test"),
  };
}

function hookFileLabel(name, files, one) {
  let label = `${name}.${one.ext}`;
  if (one.primary && files.filter((file) => file.exists).length > 1) {
    label += " (primary)";
  }
  if (!one.exists) {
    label += " (new)";
  }
  return label;
}

async function renderHookTab() {
  const dom = hookTabDom();
  if (!dom.list) {
    return;
  }

  let hooks = [];
  try {
    const response = await fetch(`${API}/v1/hooks`);
    if (response.ok) {
      hooks = ((await response.json()) || {}).hooks || [];
    }
  } catch (err) {
    console.error("renderHookTab", err);
  }

  dom.list.innerHTML = "";
  const names = hooks.map((hook) => hook.name);
  const query = praseURL().target || "";
  const picked = names.includes(query) ? query : names[0] || "";

  for (const hook of hooks) {
    const files = hook.files || [];
    const installed = files.filter((one) => one.exists);
    const summary =
      installed.length === 0
        ? "not added"
        : installed.map((one) => (one.primary && installed.length > 1 ? `${one.ext} (primary)` : one.ext)).join(" · ");
    const card = _("div.card", [_("strong", hook.label || hook.name), _("p", summary)]);
    card.dataset.name = hook.name;
    card.dataset.selected = hook.name === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "features", tab: "Hooks", target: hook.name });
    });
    dom.list.appendChild(card);
  }

  if (picked) {
    loadHookTab(picked);
  }
}

async function loadHookTab(name) {
  const dom = hookTabDom();
  if (!dom.form || !name) {
    return;
  }

  let body = null;
  try {
    const response = await fetch(`${API}/v1/hook/${encodeURIComponent(name)}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
    body = await response.json();
  } catch (err) {
    console.error("loadHookTab", err);
    alert(err.message || "failed");
    return;
  }

  hookTabName = body.name || name;
  hookTabFiles = body.files || [];
  hookTabDrafts = {};
  for (const one of hookTabFiles) {
    hookTabDrafts[one.ext] = one.content || "";
  }

  dom.name.value = body.label || hookTabName;

  const primary = hookTabFiles.find((one) => one.primary) || hookTabFiles[0];
  renderHookFiles(primary ? primary.ext : "");
}

function renderHookFiles(ext) {
  const dom = hookTabDom();
  if (!dom.files) {
    return;
  }

  dom.files.innerHTML = "";
  for (const one of hookTabFiles) {
    const button = _("button", { type: "button" }, hookFileLabel(hookTabName, hookTabFiles, one));
    button.dataset.selected = one.ext === ext ? "1" : "0";
    button.addEventListener("click", () => {
      hookTabDrafts[hookTabExt] = dom.content.value;
      renderHookFiles(one.ext);
    });
    dom.files.appendChild(button);
  }

  hookTabExt = ext;
  dom.content.value = hookTabDrafts[ext] || "";
  const current = hookTabFiles.find((one) => one.ext === ext);
  const exists = Boolean(current && current.exists);
  if (dom.remove) dom.remove.disabled = !exists;
  if (dom.open) dom.open.disabled = !exists;
}

function currentHookFile() {
  return hookTabFiles.find((one) => one.ext === hookTabExt) || null;
}

async function saveHookTab() {
  const dom = hookTabDom();
  if (!hookTabName || !hookTabExt || !dom.content) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/hook`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: hookTabName, ext: hookTabExt, content: dom.content.value }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("saveHookTab", err);
    alert(err.message || "failed");
    return;
  }

  window.location.href = getLink({ page: "features", tab: "Hooks", target: hookTabName });
}

async function deleteHookTab() {
  const one = currentHookFile();
  if (!one || !one.exists || !confirm(`Delete "${hookTabName}.${one.ext}"?`)) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/hook`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: hookTabName, ext: one.ext }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("deleteHookTab", err);
    alert(err.message || "failed");
    return;
  }

  const left = hookTabFiles.some((other) => other.ext !== one.ext && other.exists);
  window.location.href = getLink({ page: "features", tab: "Hooks", target: left ? hookTabName : "" });
}

async function openHookFile() {
  const one = currentHookFile();
  if (!one || !one.exists) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/file/open?path=${encodeURIComponent(one.path)}`);
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("openHookFile", err);
    alert(err.message || "failed");
  }
}

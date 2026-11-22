async function systemConfig() {
  try {
    const response = await fetch(`${API}/v1/config/system`);
    if (response.ok) {
      return await response.json();
    }
  } catch (err) {
    console.error("systemConfig", err);
  }
  return { reply_lang: "auto", languages: [] };
}

async function startupConfig() {
  try {
    const response = await fetch(`${API}/v1/config/startup`);
    if (response.ok) {
      return ((await response.json()) || {}).enabled === true;
    }
  } catch (err) {
    console.error("startupConfig", err);
  }
  return false;
}

async function officialGuideConfig() {
  try {
    const response = await fetch(`${API}/v1/config/official_guide`);
    if (response.ok) {
      return ((await response.json()) || {}).enabled !== false;
    }
  } catch (err) {
    console.error("officialGuideConfig", err);
  }
  return true;
}

async function saveSystemOfficialGuide() {
  const select = $("#system-official-guide");
  if (!select) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/config/official_guide`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enable: select.value === "enabled" }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveSystemOfficialGuide", err);
    alert(err.message || "failed");
  }
  renderSystem();
}

async function saveSystemStartup() {
  const select = $("#system-startup");
  if (!select) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/config/startup`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enable: select.value === "enabled" }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveSystemStartup", err);
    alert(err.message || "failed");
  }
  renderSystem();
}

async function renderSystem() {
  const select = $("#system-lang");
  if (!select) {
    return;
  }

  const config = await systemConfig();
  const current = config.reply_lang || "auto";
  const languages = config.languages || [];

  select.innerHTML = "";
  for (const one of languages) {
    select.appendChild(_("option", { value: one.code }, one.label || one.code));
  }
  if (!languages.some((one) => one.code === current)) {
    select.appendChild(_("option", { value: current }, current));
  }
  select.value = current;

  const startup = $("#system-startup");
  if (startup) {
    startup.value = (await startupConfig()) ? "enabled" : "disabled";
  }

  const official = $("#system-official-guide");
  if (official) {
    official.value = (await officialGuideConfig()) ? "enabled" : "disabled";
  }

  const output = $("#system-output");
  if (output) {
    const dir = await outputDirConfig();
    output.value = dir.output_dir || "";
    output.placeholder = dir.resolved || "~/Downloads";
  }

  renderSystemVersion();
}

async function systemUpdateInfo() {
  try {
    const response = await fetch(`${API}/v1/system/update`);
    const detail = (await response.json().catch(() => ({}))) || {};
    return { ok: response.ok, ...detail };
  } catch (err) {
    console.error("systemUpdateInfo", err);
    return { ok: false };
  }
}

function renderUpdateEntry(available, latest) {
  const entry = $("#left-tab-update");
  if (!entry) {
    return;
  }

  entry.hidden = !available;
  entry.href = latest ? RELEASE_TAG_URL + encodeURIComponent(latest) : RELEASE_URL;
  const label = latest ? `See update (${latest})` : "See update";
  const text = entry.querySelector("p");
  if (text) {
    text.textContent = label;
  }
  entry.setAttribute("name", label);
}

function hasNewVersion(detail) {
  return Boolean(detail.ok && detail.version && detail.latest && detail.version !== detail.latest);
}

async function checkUpdate() {
  const detail = await systemUpdateInfo();
  const available = hasNewVersion(detail);
  renderUpdateEntry(available, detail.latest);
  const button = $("#system-update");
  if (button) {
    button.hidden = !available;
  }
}

function watchUpdate() {
  checkUpdate();
  setInterval(checkUpdate, UPDATE_INTERVAL);
}

const RELEASE_URL = "https://github.com/agenvoy/Agenvoy/releases";
const RELEASE_TAG_URL = `${RELEASE_URL}/tag/`;

async function renderSystemVersion() {
  const label = $("#system-version");
  const button = $("#system-update");
  const release = $("#system-release");
  if (!label || !button) {
    return;
  }

  const detail = await systemUpdateInfo();
  if (!detail.ok) {
    label.textContent = `${detail.version || "unknown"} (latest unavailable)`;
    button.hidden = true;
    if (release) {
      release.hidden = true;
    }
    return;
  }
  label.textContent = detail.version;
  const title = $("#system-update-title");
  if (title && detail.latest) {
    title.textContent = `Update (${detail.latest})`;
  }
  button.hidden = !hasNewVersion(detail);
  if (release && detail.latest) {
    release.href = RELEASE_TAG_URL + encodeURIComponent(detail.latest);
    release.hidden = false;
  }
  renderUpdateEntry(hasNewVersion(detail), detail.latest);
}

async function outputDirConfig() {
  try {
    const response = await fetch(`${API}/v1/config/output_dir`);
    if (response.ok) {
      return (await response.json()) || {};
    }
  } catch (err) {
    console.error("outputDirConfig", err);
  }
  return {};
}

async function openOutputDir() {
  const dir = await outputDirConfig();
  const path = (dir.resolved || dir.output_dir || "").trim();
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
    console.error("openOutputDir", err);
    alert(err.message || "failed");
  }
}

async function saveSystemOutput() {
  const input = $("#system-output");
  if (!input) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/config/output_dir`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ output_dir: input.value.trim() }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveSystemOutput", err);
    alert(err.message || "failed");
  }
  renderSystem();
}

async function runSystemUpdate() {
  const detail = await systemUpdateInfo();
  if (detail.ok && !hasNewVersion(detail)) {
    const button = $("#system-update");
    if (button) {
      button.hidden = true;
    }
    renderUpdateEntry(false, detail.latest);
    return;
  }
  const target = detail.ok ? `${detail.version} → ${detail.latest}` : "the latest release";
  if (!confirm(`Install ${target}?\nThis overwrites the installed binary.`)) {
    return;
  }

  const button = $("#system-update");
  if (button) {
    button.disabled = true;
  }

  try {
    const response = await fetch(`${API}/v1/system/update`, { method: "POST" });
    if (response.ok) {
      if (button) {
        button.textContent = "opened in Terminal";
      }
      return;
    }
    const detail = await response.json().catch(() => ({}));
    alert(detail.error || `HTTP ${response.status}`);
  } catch (err) {
    console.error("runSystemUpdate", err);
    alert(err.message || "failed");
  }

  if (button) {
    button.disabled = false;
  }
}

async function saveSystemLang() {
  const select = $("#system-lang");
  if (!select) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/config/system`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reply_lang: select.value }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveSystemLang", err);
    alert(err.message || "failed");
  }
  renderSystem();
}

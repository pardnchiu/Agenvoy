const CONFIG_KEY = "webui_config";
const LEFT_TAB_WIDE = 1280;

function isWide() {
  return document.documentElement.clientWidth >= LEFT_TAB_WIDE;
}

const AUTO_SCROLL_SLACK = 64;
const PIN_CHAT_MAX = 3;
const PIN_CHAT_SEED = [
  "cli-0ed57a60-d5b7-4fe4-bb14-939f91a8e185",
  "tg-83a827ab47b6551ec9b3ad721da92e7dc9676372f1406f2ae997677539c772d1",
  "chat-c09b3e4c-e66f-43fc-bc68-d5eae319ae5c",
];

function praseURL() {
  const url = new URL(window.location.href);
  const params = {};
  for (const [key, value] of url.searchParams) {
    params[key] = value;
  }
  return params;
}

function writeConfig(config) {
  try {
    localStorage.setItem(CONFIG_KEY, JSON.stringify(config));
  } catch (err) {
    console.error(err);
  }
}

function readConfig() {
  let config = null;

  try {
    config = JSON.parse(localStorage.getItem(CONFIG_KEY));
  } catch (err) {
    console.error(err);
    config = null;
  }

  if (typeof config !== "object" || config === null || Array.isArray(config)) {
    config = {};
  }

  if (config.left_tab_collapsed !== "1" && config.left_tab_collapsed !== "0") {
    config.left_tab_collapsed = "0";
    writeConfig(config);
  }

  if (typeof config.harness_enable !== "boolean") {
    config.harness_enable = false;
    writeConfig(config);
  }

  if (!Array.isArray(config.pin_chat)) {
    config.pin_chat = PIN_CHAT_SEED.slice();
    writeConfig(config);
  } else {
    const pinned = config.pin_chat.filter((id) => typeof id === "string" && id !== "").slice(0, PIN_CHAT_MAX);
    if (pinned.length !== config.pin_chat.length) {
      config.pin_chat = pinned;
      writeConfig(config);
    }
  }
  return config;
}

const CHAT_DRAFT = "new";

function readChatConfig(chatId) {
  const entry = (readConfig().chat || {})[chatId || CHAT_DRAFT] || {};
  return {
    // ! rule will deprecate in the future
    role: typeof entry.role === "string" ? entry.role : typeof entry.rule === "string" ? entry.rule : "",
    work_dir: typeof entry.work_dir === "string" ? entry.work_dir : "",
  };
}

function writeChatConfig(chatId, patch) {
  const key = chatId || CHAT_DRAFT;
  const config = readConfig();
  config.chat = config.chat || {};
  config.chat[key] = Object.assign(readChatConfig(key), patch);
  writeConfig(config);
}

function clearChatDraft() {
  const config = readConfig();
  if (!config.chat || !config.chat[CHAT_DRAFT]) {
    return;
  }
  delete config.chat[CHAT_DRAFT];
  writeConfig(config);
}

function adoptChatConfig(chatId) {
  const config = readConfig();
  const draft = (config.chat || {})[CHAT_DRAFT];
  if (!chatId || !draft) {
    return;
  }
  config.chat[chatId] = Object.assign(readChatConfig(chatId), draft);
  delete config.chat[CHAT_DRAFT];
  writeConfig(config);
}

function chatPanel(sessionId) {
  const chat = $("section.chat");
  if (!chat) {
    return null;
  }

  const id = sessionId || currentSessionId;
  return id
    ? chat.querySelector(`:scope > [data-id="${id}"], :scope > div.subagent-panel > [data-id="${id}"]`)
    : chat.querySelector(":scope > main");
}

let activePins = [];

function setPinChats(list) {
  activePins = Array.isArray(list) ? list : [];
}

function pinChats() {
  return activePins;
}

async function sessionExists(sessionId) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}`);
    return response.ok;
  } catch (err) {
    console.error("sessionExists", err);
    return true;
  }
}

async function prunePinChat(config) {
  if (!config.pin_chat.length) {
    return;
  }

  let list = [];
  try {
    const response = await fetch(`${API}/v1/sessions`);
    if (!response.ok) {
      return;
    }
    list = (await response.json()).sessions || [];
  } catch (err) {
    console.error("prunePinChat", err);
    return;
  }

  const alive = new Set(list.map((one) => one.id));
  const pinned = config.pin_chat.filter((id) => alive.has(id));
  if (pinned.length === config.pin_chat.length) {
    return;
  }

  config.pin_chat = pinned;
  writeConfig(config);
}

function addPinChat(sessionId) {
  if (!sessionId) {
    return;
  }

  const config = readConfig();
  if (config.pin_chat.includes(sessionId)) {
    return;
  }
  if (config.pin_chat.length >= PIN_CHAT_MAX) {
    alert(
      `Pinned panels are limited to ${PIN_CHAT_MAX}.\n\nUnpin one from its panel header, then pin this chat again.`,
    );
    return;
  }

  config.pin_chat.push(sessionId);
  writeConfig(config);
  window.location.reload();
}

function unpinChat(sessionId) {
  if (!sessionId) {
    return false;
  }

  const config = readConfig();
  const pinned = config.pin_chat.filter((id) => id !== sessionId);
  if (pinned.length === config.pin_chat.length) {
    return false;
  }

  config.pin_chat = pinned;
  writeConfig(config);
  return true;
}

function removePinChat(sessionId) {
  if (unpinChat(sessionId)) {
    window.location.href = getLink({ page: "chat", chat: sessionId });
  }
}

function panelSession(dom) {
  const panel = dom && dom.closest ? dom.closest("section.chat > *") : null;
  return panel ? panel.dataset.id || "" : "";
}

function chatMessages(sessionId) {
  const panel = chatPanel(sessionId);
  return panel ? panel.querySelector(":scope > section.messages") : null;
}

function chatPart(name, sessionId) {
  const dom = chatMessages(sessionId);
  return dom ? dom.querySelector(`:scope > section.${name}`) : null;
}

function sourceBox(text) {
  return _("pre.source", { textContent: text || "" });
}

function openAgenvoyFile(path) {
  if (!path) {
    return;
  }
  fetch(`${API}/v1/file/open?path=${encodeURIComponent(path)}`).catch((err) => console.error("openAgenvoyFile", err));
}

const DOC_EXTENSION = ["md", "markdown", "txt", "text"];
const PAGE_EXTENSION = ["html", "htm"];

function pathExtension(path) {
  const name = String(path || "")
    .split("/")
    .pop();
  const at = name.lastIndexOf(".");
  return at > 0 ? name.slice(at + 1).toLowerCase() : "";
}

function isDocumentPath(path) {
  const ext = pathExtension(path);
  return DOC_EXTENSION.includes(ext) || PAGE_EXTENSION.includes(ext);
}

function closeSubview() {
  const box = document.querySelector("section.chat > div.subview");
  if (box) {
    box.innerHTML = "";
  }
}

async function openFileInSubview(path) {
  const box = document.querySelector("section.chat > div.subview");
  if (!box) {
    openAgenvoyFile(path);
    return;
  }

  const frame = _("iframe.body", { title: path });
  const close = _("button", { name: "Close" }, [_("span.material-symbols-outlined", "right_panel_open")]);
  close.addEventListener("click", closeSubview);
  const open = _("button", { name: "Open with system app" }, [_("span.material-symbols-outlined", "open_in_new")]);
  open.addEventListener("click", () => openAgenvoyFile(path));
  box.innerHTML = "";
  box.appendChild(_("article", [_("header", [close, _("p", `\u200E${path}`), open]), frame]));

  const ext = pathExtension(path);
  if (PAGE_EXTENSION.includes(ext)) {
    frame.src = `${API}/v1/file?path=${encodeURIComponent(path)}`;
    return;
  }

  try {
    const response = await fetch(`${API}/v1/file?path=${encodeURIComponent(path)}`);
    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      throw new Error(data.error || response.status);
    }
    const section = document.createElement("section");
    section.className = "md-render nanomd";
    section.dataset.mode = "light";
    section.innerHTML = MDParser.Parse(await response.text()).html();
    frame.srcdoc = subviewDocument(section.outerHTML, true);
  } catch (err) {
    console.error("openFileInSubview", err);
    frame.srcdoc = subviewDocument(
      `<p>Failed to open "${escapeSubviewText(path)}": ${escapeSubviewText(err.message)}</p>`,
    );
  }
}

function escapeSubviewText(text) {
  return String(text == null ? "" : text)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function subviewDocument(body, markdown = false) {
  const script = markdown
    ? `<script src="/vendor/nanomd/parser.min.js"></script><script>MDParser.Init({ fileEndpoint: ${JSON.stringify(`${API}/v1/file/open?path={filepath}`)}, parseHeading: false, enable: { mermaid: true, latex: true, chartjs: true, highlight: true } }).then(() => { if (window.Chart) { Chart.defaults.animation = false; } return MDParser.Refresh(document.body); }).catch((err) => console.error(err.message))</script>`
    : "";
  return `<!doctype html><html><head><meta charset="utf-8"><base href="${location.origin}${location.pathname}"><link rel="stylesheet" href="public/index.css"></head><body class="subview">${body}${script}</body></html>`;
}

const parserReady = MDParser.Init({
  fileEndpoint: `${API}/v1/file/open?path={filepath}`,
  parseHeading: false,
  enable: { mermaid: true, latex: true, chartjs: true, highlight: true },
})
  .then(() => {
    if (window.Chart) {
      Chart.defaults.animation = false;
    }
  })
  .catch((err) => console.error(err.message));

async function refreshMarkdown(root) {
  await parserReady;
  return MDParser.Refresh(root).catch((err) => console.error(`[Internal] refreshMarkdown: ${err.message}`));
}

function openPath(path) {
  if (isDocumentPath(path)) {
    openFileInSubview(path);
    return;
  }
  openAgenvoyFile(path);
}

function bindCodeCopy() {
  MDParser.OnCopyClick((ok, pre) => {
    pre.classList.remove("success", "error");
    pre.classList.add(ok ? "success" : "error");
    setTimeout(() => pre.classList.remove("success", "error"), 1000);
  });
}

function bindFileLink() {
  document.addEventListener(
    "click",
    function (e) {
      const link = e.target.closest("a.md-file-link");
      if (!link || !isDocumentPath(link.dataset.path)) {
        return;
      }
      e.preventDefault();
      e.stopPropagation();
      openFileInSubview(link.dataset.path);
    },
    true,
  );
}

function fileBox(files) {
  const list = _("ul");
  const box = _("details.files", [
    _("summary", ["Files", _("span.material-symbols-outlined", "keyboard_arrow_down")]),
    list,
  ]);
  box.open = true;
  box.hidden = true;
  renderFileBox(box, files);
  return box;
}

function renderFileBox(box, files) {
  if (!box) {
    return;
  }

  const list = box.querySelector("ul");
  list.innerHTML = "";
  box.hidden = !Array.isArray(files) || files.length === 0;
  if (box.hidden) {
    return;
  }

  for (const path of files) {
    const link = _("a", { href: path }, path);
    link.addEventListener("click", (event) => {
      event.preventDefault();
      openPath(path);
    });
    list.appendChild(_("li", [link]));
  }
}

function copyBtn() {
  const dom = _("button", { name: "Copy content" }, [_("span.material-symbols-outlined", "content_copy")]);
  dom.addEventListener("click", function () {
    const bubble = dom.closest("div.assistant, div.user");
    const source = bubble && bubble.querySelector("pre.source");
    if (!source || !navigator.clipboard) {
      return;
    }

    const icon = dom.querySelector("span");
    navigator.clipboard
      .writeText(source.textContent)
      .then(() => {
        icon.textContent = "check_circle";
        setTimeout(() => (icon.textContent = "content_copy"), 1000);
      })
      .catch((err) => console.error("copy", err));
  });
  return dom;
}

function historyBtn(sessionId, task) {
  const dom = _("button", { name: "Task history" }, [_("span.material-symbols-outlined", "history")]);
  dom.addEventListener("click", function () {
    window.location.href = detailsLink(sessionId, task, "");
  });
  return dom;
}

function bindSelectPicker() {
  document.addEventListener("click", function (e) {
    const label = e.target.closest("label:has(select)");
    if (!label || e.target.tagName === "SELECT") {
      return;
    }

    const dom = label.querySelector("select");
    if (dom && typeof dom.showPicker === "function") {
      dom.showPicker();
    }
  });
}

function droppedPaths(transfer) {
  const paths = [];
  const push = function (value) {
    const path = filePathOf(value);
    if (path && !paths.includes(path)) {
      paths.push(path);
    }
  };

  for (const line of (transfer.getData("text/uri-list") || "").split("\n")) {
    push(line);
  }
  if (paths.length === 0) {
    push(transfer.getData("text/plain"));
  }
  return paths;
}

function filePathOf(value) {
  value = (value || "").trim();
  if (value === "" || value.startsWith("#")) {
    return "";
  }
  if (value.startsWith("file://")) {
    try {
      return decodeURIComponent(new URL(value).pathname);
    } catch (err) {
      console.error("droppedPaths", err);
      return "";
    }
  }
  return value.startsWith("/") || value.startsWith("~/") ? value : "";
}

function dropTarget() {
  const popup = $("#workdir-popup");
  return popup ? popup.querySelector("textarea") : $("#chat-input");
}

function bindInputDrop() {
  window.addEventListener("dragover", function (e) {
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = "copy";
    }
    const dom = dropTarget();
    if (dom) {
      dom.dataset.dragging = "1";
    }
  });

  for (const name of ["dragleave", "dragend", "drop"]) {
    window.addEventListener(name, function (e) {
      if (name === "dragleave" && e.relatedTarget !== null) {
        return;
      }
      const dom = dropTarget();
      if (dom) {
        delete dom.dataset.dragging;
      }
    });
  }

  window.addEventListener("drop", async function (e) {
    const dom = dropTarget();
    if (!dom) {
      return;
    }

    const paths = droppedPaths(e.dataTransfer);
    const entries = paths.length === 0 ? droppedEntries(e.dataTransfer) : [];
    if (paths.length === 0 && entries.length === 0) {
      return;
    }
    e.preventDefault();

    for (const entry of entries) {
      try {
        const found = await locateDropped(entry);
        if (found.length === 0) {
          alert(`can not find where [${entry.name}] lives on this machine`);
          continue;
        }
        const path = found.length === 1 ? found[0] : pickPath(entry.name, found);
        if (path) {
          paths.push(path);
        }
      } catch (err) {
        console.error("locateDropped", err);
        alert(`failed to locate [${entry.name}]：${err.message}`);
      }
    }
    if (paths.length === 0) {
      return;
    }
    if (dom.closest("#workdir-popup")) {
      dom.value = paths[0];
      dom.nextElementSibling.textContent = dom.value + "\n";
      dom.setSelectionRange(dom.value.length, dom.value.length);
      dom.focus();
      return;
    }
    insertAtCursor(dom, paths.map((p) => `"${p}"`).join(" "));
  });
}

function droppedEntries(transfer) {
  const out = [];
  for (const item of Array.from((transfer && transfer.items) || [])) {
    if (item.kind !== "file") {
      continue;
    }
    const entry = typeof item.webkitGetAsEntry === "function" ? item.webkitGetAsEntry() : null;
    if (entry && entry.isDirectory) {
      out.push({ name: entry.name, dir: true, entry: entry });
      continue;
    }
    const file = item.getAsFile();
    if (file) {
      out.push({ name: file.name, dir: false, file: file });
    }
  }
  if (out.length > 0) {
    return out;
  }
  return Array.from((transfer && transfer.files) || []).map((file) => ({ name: file.name, dir: false, file: file }));
}

const DIR_FINGERPRINT_MAX = 24;

function readEntryBatch(reader) {
  return new Promise(function (resolve) {
    reader.readEntries(resolve, function (err) {
      console.error("readEntries", err);
      resolve([]);
    });
  });
}

async function directoryChildren(entry) {
  if (!entry || typeof entry.createReader !== "function") {
    return [];
  }
  const reader = entry.createReader();
  const names = [];
  for (;;) {
    const batch = await readEntryBatch(reader);
    if (batch.length === 0) {
      return names;
    }
    for (const child of batch) {
      names.push(child.name);
      if (names.length >= DIR_FINGERPRINT_MAX) {
        return names;
      }
    }
  }
}

async function locateDropped(entry) {
  let query;
  if (entry.dir) {
    query = new URLSearchParams({ name: entry.name, dir: "1" });
    for (const child of await directoryChildren(entry.entry)) {
      query.append("child", child);
    }
  } else {
    query = new URLSearchParams({
      name: entry.file.name,
      size: entry.file.size,
      mtime: entry.file.lastModified,
    });
  }

  const res = await fetch(`${API}/v1/file/locate?${query}`);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || res.status);
  }
  return data.paths || [];
}

function pickPath(name, list) {
  const menu = list.map((p, i) => `${i + 1}. ${p}`).join("\n");
  const answer = prompt(`${list.length} matches for [${name}], enter a number:\n${menu}`, "1");
  return list[Number(answer) - 1] || "";
}

function insertAtCursor(dom, text) {
  const start = dom.selectionStart;
  const end = dom.selectionEnd;
  const before = dom.value.slice(0, start);
  const after = dom.value.slice(end);
  const lead = before === "" || before.endsWith(" ") || before.endsWith("\n") ? "" : " ";

  dom.value = before + lead + text + after;
  const caret = (before + lead + text).length;
  dom.setSelectionRange(caret, caret);
  dom.nextElementSibling.textContent = dom.value + "\n";
  dom.focus();
}

const listSelected = new WeakMap();

function scrollListSelection() {
  for (const list of document.querySelectorAll("div.list")) {
    const card = list.querySelector('[data-selected="1"]');
    if (!card || card.getClientRects().length === 0) {
      continue;
    }
    const key = card.dataset.name || card.getAttribute("name") || card.getAttribute("href") || card.textContent;
    if (listSelected.get(list) === key) {
      continue;
    }
    listSelected.set(list, key);
    card.scrollIntoView({ block: "center" });
  }
}

function watchListSelection() {
  let pending = false;
  new MutationObserver(() => {
    if (pending) {
      return;
    }
    pending = true;
    requestAnimationFrame(() => {
      pending = false;
      scrollListSelection();
    });
  }).observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ["data-selected"] });
}

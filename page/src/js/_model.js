const CHAT_ID = /^(chat-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|(tg|dc)-[0-9a-f]{64})$/i;
const SESSION_ID = /^((chat|cli|temp)-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|(tg|dc)-[0-9a-f]{64})$/i;

const MODEL_AUTO = "auto";

const MODEL_COOKIE = "agenvoy_model";
const REASONING_COOKIE = "agenvoy_reasoning";

let modelList = [];
let currentModel = MODEL_AUTO;

function readCookie(key) {
  const hit = document.cookie.split("; ").find((one) => one.startsWith(key + "="));
  return hit ? decodeURIComponent(hit.slice(key.length + 1)) : "";
}

function writeCookie(key, value) {
  document.cookie = `${key}=${encodeURIComponent(value)}; path=/; max-age=31536000; samesite=lax`;
}

function modelLabel(name) {
  return name.slice(name.indexOf("@") + 1) || name;
}

function ensureModel() {
  return currentModel || MODEL_AUTO;
}

function markModel(name) {
  const dom = $("#chat-model");
  if (!dom) {
    return;
  }
  const label = modelLabel(name);
  dom.name = label;
  const text = dom.querySelector("p");
  if (text) {
    text.textContent = label;
  }
}

async function fetchModels() {
  try {
    const response = await fetch(`${API}/v1/models`);
    if (!response.ok) {
      return null;
    }
    return (await response.json()).models || [];
  } catch (err) {
    console.error("loadModelList", err);
    return null;
  }
}

function hasRealModel(list) {
  return Array.isArray(list) && list.some((name) => name !== MODEL_AUTO);
}

async function getModelList(sessionId, preloaded) {
  modelList = [];
  const list = preloaded === undefined ? await fetchModels() : preloaded;
  if (list === null) {
    return;
  }
  modelList = list;

  if (!hasRealModel(modelList)) {
    markModelMissing();
    return;
  }

  const chat = $("section.chat");
  if (chat) {
    chat.dataset.model = "1";
  }

  const fallback = (await getSessionModel(sessionId)) || modelList[0];
  const saved = readCookie(MODEL_COOKIE);
  currentModel = modelList.includes(saved) ? saved : fallback;
  markModel(currentModel);
}

function markModelMissing() {
  const chat = $("section.chat");
  if (!chat) {
    return;
  }
  chat.dataset.model = "0";
  for (const dom of chat.querySelectorAll("header, div.input, section.messages")) {
    dom.remove();
  }
}

function selectModel(name) {
  if (!name) {
    return;
  }
  currentModel = name;
  markModel(name);
  writeCookie(MODEL_COOKIE, name);
}

function openModelPicker() {
  const names = modelList.length > 0 ? modelList : [ensureModel()];
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, "cancel");
  const root = _("div.popup", [_("div.panel", [_("strong", "Model"), list, _("footer", [cancel])])]);
  root.id = "model-popup";

  const close = () => root.remove();

  for (const name of names) {
    const box = _("input", { type: "radio", name: "model-pick", value: name });
    box.checked = currentModel === name;
    box.addEventListener("change", () => {
      selectModel(name);
      close();
    });
    list.appendChild(_("label", [box, _("div", [_("strong", name)])]));
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

async function getSessionModel(sessionId) {
  if (!SESSION_ID.test(sessionId || "")) {
    return "";
  }

  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}`);
    if (!response.ok) {
      return "";
    }

    return (await response.json()).model || "";
  } catch (err) {
    console.error("sessionModel", err);
    return "";
  }
}

const REASONING_ICON = {
  auto: "auto_awesome",
  none: "signal_cellular_0_bar",
  low: "signal_cellular_1_bar",
  medium: "signal_cellular_2_bar",
  high: "signal_cellular_3_bar",
  xhigh: "network_cell",
  max: "signal_cellular_4_bar",
};

let reasoningLevel = "medium";
let reasoningList = [];

function ensureReasoning() {
  return reasoningLevel || "medium";
}

function markReasoning(level) {
  const dom = $("#chat-reasoning");
  if (!dom) {
    return;
  }
  dom.name = level;
  const icon = dom.querySelector("span");
  if (icon) {
    icon.textContent = REASONING_ICON[level] || REASONING_ICON.medium;
  }
}

async function getReasoningList(sessionId) {
  if (!SESSION_ID.test(sessionId || "")) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}`);
    if (!response.ok) {
      return;
    }
    const body = await response.json();
    const levels = body.levels || [];
    if (levels.length === 0) {
      return;
    }

    reasoningList = levels;
    reasoningLevel = body.reasoning || levels[0];
    markReasoning(reasoningLevel);
  } catch (err) {
    console.error("getReasoningList", err);
  }
}

async function loadReasoning(sessionId) {
  await getReasoningList(sessionId);
  const saved = readCookie(REASONING_COOKIE);
  const levels = reasoningList.length > 0 ? reasoningList : Object.keys(REASONING_ICON);
  if (levels.includes(saved)) {
    reasoningLevel = saved;
    markReasoning(saved);
  }
}

function selectReasoning(level) {
  if (!level) {
    return;
  }
  reasoningLevel = level;
  markReasoning(level);
  writeCookie(REASONING_COOKIE, level);
}

function openReasoningPicker() {
  const levels = reasoningList.length > 0 ? reasoningList : Object.keys(REASONING_ICON);
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, "cancel");
  const root = _("div.popup", [_("div.panel", [_("strong", "Reasoning"), list, _("footer", [cancel])])]);
  root.id = "reasoning-popup";

  const close = () => root.remove();

  for (const level of levels) {
    const box = _("input", { type: "radio", name: "reasoning-pick", value: level });
    box.checked = reasoningLevel === level;
    box.addEventListener("change", () => {
      selectReasoning(level);
      close();
    });
    list.appendChild(_("label", [box, _("div", [_("strong", level)])]));
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

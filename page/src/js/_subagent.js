const SUBAGENT_SUBSCRIBE_DELAY = 300;
const SUBAGENT_BATCH_WINDOW = 60000;
const SUBAGENT_STARTED = "subagent started";
const SUBAGENT_SESSION = /\bsession=(temp-\S+)(?: parent=(\S+))?/;

let subagentSessions = [];
let subagentSubscribeTimer = 0;
let subagentPending = [];
let subagentFinished = [];
const subagentParent = new Map();
const subagentPinnedAt = new Map();

function subagentPanel(sessionId) {
  const unpin = _("button", { class: "unpin", type: "button", name: "Unpin" }, [
    _("span.material-symbols-outlined", "keep_off"),
  ]);
  unpin.addEventListener("click", () => unpinSubagent(sessionId));
  return _("section", [
    _("header", [unpin]),
    _("div.logo", [_("section", [_("img", { src: "public/logo-min.svg", alt: "Logo" }), _("p", "Agenvoy")])]),
    _("section.messages", [_("section.pending", { "data-index": "0" }), _("section.confirm")]),
  ]);
}

function pinSubagent(text) {
  const match = SUBAGENT_SESSION.exec(text || "");
  const box = $("section.chat > div.subagent-panel");
  if (!match || !box) {
    return;
  }
  const sessionId = match[1];
  const parent = match[2] || "";
  for (const id of subagentFinished.filter((one) => one !== sessionId)) {
    unpinSubagent(id);
  }
  subagentFinished = [];
  if (subagentSessions.includes(sessionId) || pinChats().includes(sessionId)) {
    return;
  }
  const batchStart = Date.now() - SUBAGENT_BATCH_WINDOW;
  const stale = subagentSessions.find((one) => (subagentPinnedAt.get(one) || 0) < batchStart);
  if (subagentSessions.length >= PIN_CHAT_MAX && stale) {
    unpinSubagent(stale);
  }
  if (subagentSessions.length >= PIN_CHAT_MAX) {
    return;
  }

  const panel = subagentPanel(sessionId);
  panel.dataset.index = String(subagentSessions.length);
  panel.dataset.id = sessionId;
  panel.dataset.hint = sessionId;
  box.appendChild(panel);
  subagentSessions.push(sessionId);
  subagentPending.push(sessionId);
  subagentParent.set(sessionId, parent);
  subagentPinnedAt.set(sessionId, Date.now());

  clearTimeout(subagentSubscribeTimer);
  subagentSubscribeTimer = setTimeout(function () {
    subscribe(subscribedSession);
    for (const id of subagentPending) {
      renderChat(id);
    }
    subagentPending = [];
  }, SUBAGENT_SUBSCRIBE_DELAY);
}

function unpinSubagent(sessionId) {
  if (!subagentSessions.includes(sessionId)) {
    return;
  }
  subagentSessions = subagentSessions.filter((id) => id !== sessionId);
  subagentParent.delete(sessionId);
  subagentPinnedAt.delete(sessionId);
  subagentFinished = subagentFinished.filter((id) => id !== sessionId);
  const panel = chatPanel(sessionId);
  if (panel) {
    panel.remove();
  }
  for (const [i, id] of subagentSessions.entries()) {
    const one = chatPanel(id);
    if (one) {
      one.dataset.index = String(i);
    }
  }
}

function parentDone(sessionId) {
  for (const id of subagentSessions.filter((one) => subagentParent.get(one) === sessionId)) {
    if (subagentFinished.includes(id)) {
      unpinSubagent(id);
    }
  }
}

const API = window.location.origin;
const SKIP_EVENTS = [
  "EventConnected",
  "EventSkillResult",
  "EventTextDone",
  "EventAgentSelect",
  "EventSummaryGenerate",
  "EventToolCallStart",
  "EventToolCallText",
  "EventToolCallEnd",
  "EventToolResult",
  "EventUsageUpdate",
  "EventPending",
];
let currentSessionId = "";

let HISTORY = [];
const TASK = {};
const taskingMarks = new Map();

function resetTasking() {
  for (const hash of Object.keys(TASK)) {
    delete TASK[hash];
  }
  taskingMarks.clear();
}

function taskingOf(hash) {
  if (!TASK[hash]) {
    const asked = HISTORY.findLast((entry) => entry.role === "user");
    TASK[hash] = { input: asked ? asked.content : "", thinking: "", result: "", is_completed: false };
    taskingMarks.set(hash, { streamed: false, resumed: false });
  }
  return TASK[hash];
}

function trackTasking(hash, event) {
  const type = event.type;
  if (type === "EventAgentResult" || type === "EventSuggest" || type === "EventFileChanged" || type === "EventTodoUpdate") {
    return;
  }

  if (type === "EventDone" || type === "EventCanceled" || type === "EventError") {
    const ended = TASK[hash];
    if (ended && !ended.is_completed) {
      ended.is_completed = true;
      if (type === "EventDone") {
        HISTORY.push({ role: "assistant", content: ended.result });
      }
    }
    taskingMarks.delete(hash);
    return;
  }

  const tasking = taskingOf(hash);
  const mark = taskingMarks.get(hash);
  if (!mark) {
    return;
  }

  if (type === "EventTextDelta" || type === "EventText") {
    if (type === "EventText" && mark.streamed) {
      return;
    }
    const text = event.text || "";
    if (!text) {
      return;
    }
    if (type === "EventTextDelta") {
      mark.streamed = true;
    }
    if (tasking.result && (mark.resumed || type === "EventText")) {
      tasking.result += mark.resumed ? "\n\n" : "\n";
    }
    mark.resumed = false;
    tasking.result += text;
    return;
  }

  const line = (type === "EventReasoning" ? event.text || "" : formatEvent(event)).trim();
  if (!line) {
    return;
  }
  tasking.thinking += (tasking.thinking ? "\n\n" : "") + line;
  mark.resumed = Boolean(tasking.result);
}
const streamViews = new Map();
const assistantViews = new WeakMap();
const assistantItems = new WeakMap();
const taskIds = new Map();

const TASK_COOKIE = "agenvoy_task";
const taskAwait = new Set();

function readTaskCookie() {
  const hit = document.cookie.split("; ").find((one) => one.startsWith(TASK_COOKIE + "="));
  if (!hit) {
    return null;
  }
  const [session, task] = decodeURIComponent(hit.slice(TASK_COOKIE.length + 1)).split("|");
  return session && task ? { session: session, task: task } : null;
}

function writeTaskCookie(sessionId, taskHash) {
  if (!sessionId || !taskHash) {
    return;
  }
  const value = encodeURIComponent(sessionId + "|" + taskHash);
  document.cookie = `${TASK_COOKIE}=${value}; path=/; max-age=604800; samesite=lax`;
  renderSendMode(sessionId);
}

function clearTaskCookie(sessionId) {
  document.cookie = `${TASK_COOKIE}=; path=/; max-age=0; samesite=lax`;
  renderSendMode(sessionId);
}

const inputTasks = new Map();

function setInputTask(sessionId, taskHash) {
  const id = sessionId || currentSessionId;
  if (!id) {
    return;
  }
  if (taskHash) {
    inputTasks.set(id, taskHash);
  } else {
    inputTasks.delete(id);
  }
  renderSendMode(id);
}

function setPaused(sessionId, paused) {
  const view = streamOf(sessionId);
  if (!view || view.paused === paused) {
    return;
  }
  view.paused = paused;
}

function renderSendMode(sessionId) {
  const panel = chatPanel(sessionId);
  const input = panel ? panel.querySelector(":scope > div.input") : null;
  if (!input) {
    return;
  }

  const hash = inputTasks.get(sessionId || currentSessionId) || "";
  if (hash) {
    input.dataset.taskid = hash;
  } else {
    delete input.dataset.taskid;
  }

  const dom = input.querySelector("button.send");
  if (!dom) {
    return;
  }
  const box = input.querySelector("textarea");
  const stop = hash !== "" && (!box || box.value.trim() === "");
  dom.dataset.mode = stop ? "stop" : "send";
  dom.setAttribute("name", stop ? "Cancel" : "Send");
  dom.querySelector("span").textContent = stop ? "stop" : "send";
}

function streamOf(sessionId) {
  return streamViews.get(sessionId || currentSessionId) || null;
}

function taskStream(sessionId, task) {
  const dom = chatMessages(sessionId);
  if (!dom || !task) {
    return null;
  }
  const list = dom.querySelectorAll(`:scope > div.assistant[data-task="${CSS.escape(task)}"]`);
  const last = list[list.length - 1];
  if (!last) {
    return null;
  }
  const view = assistantViews.get(last);
  if (view) {
    return view;
  }
  const item = assistantItems.get(last);
  if (!item) {
    return null;
  }
  assistantItems.delete(last);
  const upgraded = newStreamItem({ model: item.meta.model, trace: item.Reasoning, text: item.content, task: task }, sessionId);
  last.replaceWith(upgraded.dom);
  renderTodo(upgraded, item.todos);
  return upgraded;
}

function setStream(sessionId, view) {
  const id = sessionId || currentSessionId;
  if (!id) {
    return;
  }
  if (view) {
    streamViews.set(id, view);
  } else {
    streamViews.delete(id);
  }
}

function taskOf(sessionId) {
  return taskIds.get(sessionId || currentSessionId) || "";
}

function setTask(sessionId, taskId) {
  const id = sessionId || currentSessionId;
  if (!id) {
    return;
  }
  if (taskId) {
    taskIds.set(id, taskId);
  } else {
    taskIds.delete(id);
  }
}

const localEcho = new Map();

function pushEcho(sessionId, text) {
  const id = sessionId || currentSessionId;
  const list = localEcho.get(id) || [];
  list.push(text);
  localEcho.set(id, list);
}

function takeEcho(sessionId, text) {
  const list = localEcho.get(sessionId || currentSessionId);
  if (!list) {
    return false;
  }
  const i = list.indexOf(text);
  if (i === -1) {
    return false;
  }
  list.splice(i, 1);
  return true;
}

async function stopRunning(sessionId) {
  const sid = sessionId || currentSessionId;
  if (!sid) return;
  if (!confirm("Cancel this task?")) return;

  const taskId = taskOf(sid) || "current";
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sid)}/cancel/${encodeURIComponent(taskId)}`, { method: "POST" });
    if (!response.ok) {
      pushToast(TOAST_LEVEL.ERROR, `cancel failed: ${response.status}`, nowClock());
    }
  } catch (err) {
    console.error("stopRunning", err);
    pushToast(TOAST_LEVEL.ERROR, `cancel failed: ${err?.message || err}`, nowClock());
  }
}

async function send(content, target) {
  content = (content || "").trim();
  if (content === "") return;

  target = (target || "").trim();
  const primary = target === "" || target === currentSessionId;

  let sessionId = target;
  if (primary) {
    try {
      sessionId = await ensureSessionId();
    } catch (err) {
      console.error("send", err);
      return;
    }

    const fresh = currentSessionId !== sessionId;
    setSession(sessionId);
    subscribe(sessionId);
    if (fresh) {
      prependChat(sessionId, content);
      saveSessionModel(sessionId, ensureModel());
      saveSessionReasoning(sessionId, ensureReasoning());
      adoptChatConfig(sessionId);
    }
  }

  const model = primary ? ensureModel() : "auto";
  const chat = readChatConfig(sessionId);

  const picked = primary ? skill : "";
  if (primary) {
    clearSkill();
  }

  if (primary) {
    HISTORY.push({ role: "user", content: content });
  }

  const dom = chatMessages(sessionId);
  clearPending(sessionId);
  const running = taskStream(sessionId, inputTasks.get(sessionId) || "");
  if (running && !running.paused) {
    appendUserText(dom, content);
  } else {
    dom.appendChild(newUserItem({ content: content, meta: { send_at: sendAt() } }));
    setStream(sessionId, newStreamItem({}, sessionId));
  }
  taskAwait.add(sessionId);
  pushEcho(sessionId, content);
  scrollToBottom(true, sessionId);

  try {
    const response = await fetch(`${API}/v1/send`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        content: content,
        sse: false,
        session_id: sessionId,
        persist: true,
        model: model === "auto" ? "" : model,
        system_prompt: primary ? role : "",
        work_dir: chat.work_dir,
        skill: picked,
      }),
    });
    if (!response.ok) {
      const view = streamOf(sessionId);
      if (view) {
        renderEvent(view, { type: "EventError", text: `HTTP ${response.status}` });
      }
    }
  } catch (err) {
    const view = streamOf(sessionId);
    if (view) {
      renderEvent(view, { type: "EventError", text: err.message });
    }
  }
}

function appendUserText(dom, content) {
  const bubbles = dom.querySelectorAll("div.user");
  const bubble = bubbles[bubbles.length - 1];
  if (!bubble) {
    dom.appendChild(newUserItem({ content: content, meta: { send_at: sendAt() } }));
    return;
  }

  const mark = steerMark(sendAt(), content, bubble.dataset.steered !== "1");
  bubble.dataset.steered = "1";
  const text = bubble.querySelector("p");
  if (text) {
    text.textContent += mark;
  }
  const source = bubble.querySelector("pre.source");
  if (source) {
    source.textContent += mark;
  }
}

async function ensureSessionId() {
  if (currentSessionId) {
    return currentSessionId;
  }

  const response = await fetch(`${API}/v1/session`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ prefix: "chat-" }),
  });
  const sessionId = (await response.json()).session_id || "";
  if (!sessionId) {
    throw new Error("session_id missing");
  }
  return sessionId;
}

function setSession(sessionId) {
  if (!sessionId || currentSessionId === sessionId) return;
  currentSessionId = sessionId;

  const panel = $("section.chat > main");
  if (panel) {
    panel.dataset.id = sessionId;
  }

  const url = new URL(window.location.href);
  url.searchParams.set("page", "chat");
  url.searchParams.set("chat", sessionId);
  history.replaceState({}, "", url);
  renderSendMode(sessionId);
}

function atBottom(sessionId) {
  const dom = chatMessages(sessionId);
  if (!dom) {
    return false;
  }
  return dom.scrollHeight - dom.scrollTop - dom.clientHeight <= AUTO_SCROLL_SLACK;
}

function scrollToBottom(force, sessionId) {
  const dom = chatMessages(sessionId);
  if (!dom || dom.scrollHeight < dom.clientHeight) {
    return;
  }

  if (!force) {
    const distance = dom.scrollHeight - dom.scrollTop - dom.clientHeight;
    if (distance > AUTO_SCROLL_SLACK) return;
  }

  dom.scrollTop = dom.scrollHeight;
  requestAnimationFrame(() => {
    dom.scrollTop = dom.scrollHeight;
  });
}

function prependChat(sessionID, label) {
  const dom = $("#left-tab-chat-list");
  if (!dom || !sessionID || dom.querySelector(`[data-id="${sessionID}"]`)) {
    return;
  }

  for (const selected of dom.querySelectorAll("[data-selected]")) {
    delete selected.dataset.selected;
  }
  dom.prepend(chatListItem(sessionID, label || sessionID));
}

function sendAt() {
  const date = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function appendInboundUser(text, sessionId) {
  text = (text || "").trim();
  if (!text || text.startsWith("[Resumed Task") || text.startsWith("[Shell command]")) {
    return;
  }
  if (takeEcho(sessionId, text)) {
    return;
  }

  const dom = chatMessages(sessionId);
  if (!dom) {
    return;
  }
  if ((sessionId || currentSessionId) === currentSessionId) {
    HISTORY.push({ role: "user", content: text });
  }
  setStream(sessionId, null);
  dom.appendChild(newUserItem({ content: text, meta: { send_at: sendAt() } }));
  scrollToBottom(true, sessionId);
}

function eventSkip(event) {
  const type = event.type || "";
  if (event.source || (type === "EventToolCall" && event.tool_name === "write_todo")) {
    return true;
  }
  return SKIP_EVENTS.includes(type);
}

function newStreamItem(init, sessionId) {
  init = init || {};
  const sid = sessionId || currentSessionId;

  const reasoning = _("section.md-render.nanomd", { "data-mode": "light" });
  const think = _("details.reasoning", [_("summary", ["Reasoning", _("span.material-symbols-outlined", "keyboard_arrow_down")]), reasoning]);
  think.hidden = !init.trace;
  think.open = true;
  think.dataset.streaming = "1";

  const model = _("p", init.model || "...");
  const answer = _("section.md-render.nanomd", { "data-mode": "light" });
  const source = sourceBox(init.text || "");
  const files = fileBox([]);
  const footer = _("footer");
  const todo = _("section.todo");
  const body = _("section", [model, think, answer, source, files, footer, todo]);
  const dom = _("div.assistant", [_("img", "public/logo-min.svg"), body]);
  dom.dataset.streaming = "1";
  if (init.task) {
    dom.dataset.task = init.task;
  }

  chatMessages(sid).appendChild(dom);

  const view = {
    session: sid,
    task: init.task || "",
    dom: dom,
    body: body,
    model: model,
    think: think,
    reasoning: reasoning,
    answer: answer,
    source: source,
    files: files,
    footer: footer,
    todo: todo,
    text: init.text || "",
    answered: false,
    reasoned: false,
    streamed: false,
    textStarted: Boolean(init.text),
    trace: init.trace || "",
  };
  assistantViews.set(dom, view);
  if (view.trace) {
    render(view.reasoning, view.trace, sid);
  }
  if (view.text) {
    render(view.answer, view.text, sid);
  }
  return view;
}

function render(dom, markdown, sessionId) {
  const stick = atBottom(sessionId);
  dom.innerHTML = MDParser.Parse(channelText(markdown, sessionId)).html();
  refreshMarkdown(dom);
  if (stick) {
    scrollToBottom(true, sessionId);
  }
}

const hookBoxes = new WeakMap();

function renderHookEvent(event, sessionId) {
  if (event.source) {
    return;
  }
  const text = (event.text || "").trim();
  const dom = chatMessages(sessionId);
  if (!text || !dom) {
    return;
  }

  let target = null;
  if (event.task_hash) {
    const list = dom.querySelectorAll(`:scope > div.assistant[data-task="${CSS.escape(event.task_hash)}"]`);
    target = list[list.length - 1] || null;
  }
  if (!target) {
    const view = streamOf(sessionId);
    target = view ? view.dom : null;
  }
  appendHook(target, text, sessionId);
}

function appendHook(target, text, sessionId) {
  const body = target ? target.querySelector(":scope > section") : null;
  if (!body || !text) {
    return;
  }

  let box = hookBoxes.get(target);
  if (!box) {
    box = newHookBox();
    hookBoxes.set(target, box);
    body.appendChild(box.dom);
  }
  box.list.push(text);
  box.index = box.list.length - 1;
  renderHookBox(box);
  if (sessionId) {
    scrollToBottom(false, sessionId);
  }
}

function renderHooks(target, list) {
  for (const text of list || []) {
    appendHook(target, text, "");
  }
}

function newHookBox() {
  const state = _("strong", { textContent: "" });
  const content = _("p", { textContent: "" });
  const count = _("p", { textContent: "" });
  const prev = _("button", { type: "button", title: i18n("text_prev") }, [_("span.material-symbols-outlined", "chevron_left")]);
  const next = _("button", { type: "button", title: i18n("text_next") }, [_("span.material-symbols-outlined", "chevron_right")]);
  const dom = _("section.hook", [state, content, _("footer", [count, prev, next])]);

  const box = { dom, state, content, count, prev, next, list: [], index: 0 };
  prev.addEventListener("click", () => {
    box.index = Math.max(box.index - 1, 0);
    renderHookBox(box);
  });
  next.addEventListener("click", () => {
    box.index = Math.min(box.index + 1, box.list.length - 1);
    renderHookBox(box);
  });
  return box;
}

function renderHookBox(box) {
  const text = box.list[box.index] || "";
  const split = text.indexOf(": ");
  box.state.textContent = split > 0 ? text.slice(0, split) : i18n("text_hook");
  box.content.textContent = split > 0 ? text.slice(split + 2) : text;
  box.count.textContent = `${box.index + 1} / ${box.list.length}`;
  box.prev.disabled = box.index <= 0;
  box.next.disabled = box.index >= box.list.length - 1;
}

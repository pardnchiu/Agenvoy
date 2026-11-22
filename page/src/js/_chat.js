let chatListRetry = 0;

const CHAT_MODE_COOKIE = "agenvoy_chat_mode";

const otherTags = {
  cli: "TUI",
  tg: "TG",
  dc: "DC",
  temp: "TEMP",
};

function retryChatList() {
  clearTimeout(chatListRetry);
  chatListRetry = setTimeout(renderChatList, 3000);
}

async function renderChatList() {
  const dom = $("#left-tab-chat-list");
  const pinDom = $("#left-tab-pin-list");
  if (!dom) {
    return;
  }

  let list = [];
  try {
    const response = await fetch(`${API}/v1/sessions`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}`);
    }
    list = (await response.json()).sessions || [];
  } catch (err) {
    console.error("loadChatList", err);
    retryChatList();
    return;
  }
  clearTimeout(chatListRetry);

  dom.innerHTML = "";
  if (pinDom) {
    pinDom.innerHTML = "";
  }

  const pinned = pinChats();
  const frameChat = _("temp");
  const framePin = _("temp");

  for (const e of list) {
    const panel = document.querySelector(`section.chat > section[data-id="${e.id}"]`);
    if (panel) {
      panel.dataset.hint = e.self_id || e.id;
    }

    if (pinned.includes(e.id)) {
      framePin.appendChild(pinListItem(e.id, e.name || e.id));
      continue;
    }
    if (e.id.startsWith("chat-")) {
      frameChat.appendChild(chatListItem(e.id, e.name || e.id));
      continue;
    }
    const tag = otherTags[e.id.split("-")[0]];
    if (tag) {
      frameChat.appendChild(chatListItem(e.id, e.name || e.id, tag));
    }
  }

  dom.appendChild(frameChat);
  filterChatList();
  if (pinDom) {
    pinDom.appendChild(framePin);
  }

  const active = currentSessionId
    ? document.querySelector(`section.chats [data-id="${currentSessionId}"]`)
    : null;
  if (active) {
    const group = active.closest("details");
    if (group) {
      group.open = true;
    }
  }
}

function setChatListMode(mode) {
  writeCookie(CHAT_MODE_COOKIE, mode);
  filterChatList();
}

function filterChatList() {
  const input = $("#left-tab-chat-search");
  const keyword = input ? input.value.trim().toLowerCase() : "";
  const mode = readCookie(CHAT_MODE_COOKIE) === "all" ? "all" : "chat";
  for (const button of document.querySelectorAll("#left-tab-chat-mode > button")) {
    button.dataset.selected = button.dataset.mode === mode ? 1 : 0;
  }
  for (const row of document.querySelectorAll("#left-tab-chat-list > div")) {
    const hidden = (mode === "chat" && row.dataset.tag) || (keyword && !row.dataset.name.includes(keyword));
    row.style.display = hidden ? "none" : "";
  }
}

function rowActionItem(icon, label, style, action) {
  const props = { type: "button", title: label, "aria-label": label };
  if (style) {
    props.class = style;
  }

  const dom = _("button", props, [_("span.material-symbols-outlined", icon)]);
  dom.addEventListener("click", function (e) {
    e.preventDefault();
    e.stopPropagation();
    action();
  });
  return dom;
}

function pinListItem(sessionId, title) {
  const body = [
    _("a", { href: getLink({ page: "chat", chat: sessionId }) }, title),
    _("div.actions", [
      rowActionItem("keep_off", "Unpin", "", () => removePinChat(sessionId)),
      rowActionItem("delete", "Delete", "remove", function () {
        if (confirm(`Delete "${title}"?`)) {
          deleteChat(sessionId);
        }
      }),
    ]),
  ];

  return _(
    "div",
    {
      "data-id": sessionId,
      "data-selected": sessionId === currentSessionId ? 1 : 0,
    },
    body,
  );
}

function chatListItem(sessionId, title, tag) {
  const actions = _("div.actions", [
    rowActionItem("keep", "Pin", "", () => addPinChat(sessionId)),
    rowActionItem("delete", "Delete", "remove", function () {
      if (confirm(`Delete "${title}"?`)) {
        deleteChat(sessionId);
      }
    }),
  ]);

  return _(
    "div",
    {
      "data-id": sessionId,
      "data-name": title.toLowerCase(),
      "data-tag": tag || "",
      "data-selected": sessionId === currentSessionId ? 1 : 0,
    },
    [
      _(
        "a",
        { href: getLink({ page: "chat", chat: sessionId }) },
        tag ? [_(`span.tag.${tag.toLowerCase()}`, tag), _("span", title)] : title,
      ),
      actions,
    ],
  );
}

async function deleteChat(sessionId) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("deleteChat", err);
    alert(err.message || "failed");
    return;
  }

  const unpinned = unpinChat(sessionId);

  const row = document.querySelector(`section.chats [data-id="${sessionId}"]`);
  if (row) {
    row.remove();
  }

  const open = new URL(window.location.href).searchParams.get("chat") || "";
  if (sessionId === currentSessionId || sessionId === open) {
    window.location.href = getLink({ page: "chat" });
    return;
  }
  if (unpinned) {
    window.location.reload();
  }
}

function renameChat(sessionId, title) {
  if (!sessionId || !title) {
    return;
  }

  const row = document.querySelector(`section.chats [data-id="${sessionId}"]`);
  if (!row) {
    return;
  }
  row.dataset.name = title.toLowerCase();
  const label = row.querySelector(":scope > a > span:not(.tag)") || row.querySelector(":scope > a");
  if (label) {
    label.textContent = title;
  }
  filterChatList();
}

async function renderChat(sessionId) {
  const dom = chatMessages(sessionId);
  if (!dom || !sessionId) {
    return;
  }

  let content = "";
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}?chat=1`);
    if (!response.ok) {
      return;
    }
    content = (await response.json()).chat || "";
  } catch (err) {
    console.error("loadChat", err);
    return;
  }

  for (const bubble of dom.querySelectorAll(":scope > div.user, :scope > div.assistant")) {
    bubble.remove();
  }

  const mine = readTaskCookie();
  if (mine && mine.session === sessionId) {
    const state = actionTaskState(content, mine.task);
    if (state === "finished") {
      clearTaskCookie(sessionId);
    } else if (state === "running") {
      setTask(sessionId, mine.task);
      setInputTask(sessionId, mine.task);
    }
  }

  const items = parseActionLog(content);
  const waitingItem = items.findLast((item) => item.role !== "user" && item.task);
  if (waitingItem && waitingItem.paused) {
    waitingTasks.set(sessionId, waitingItem.task);
    setTask(sessionId, waitingItem.task);
    setInputTask(sessionId, waitingItem.task);
  } else {
    waitingTasks.delete(sessionId);
  }
  const frame = _("temp");

  if (sessionId === currentSessionId) {
    HISTORY = [];
    resetTasking();
    for (const item of items) {
      if (item.role === "assistant" && item.pending) {
        if (item.task) {
          const tasking = taskingOf(item.task);
          tasking.thinking = item.Reasoning || "";
          tasking.result = item.content || "";
        }
        continue;
      }
      HISTORY.push({ role: item.role, content: item.content || "" });
    }
  }

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item.pending && item.role === "assistant" && i === items.length - 1) {
      dom.appendChild(frame);
      const view = newStreamItem(
        { model: item.meta.model, trace: item.Reasoning, text: item.content, task: item.task },
        sessionId,
      );
      if (item.paused) {
        view.paused = true;
      }
      if (item.turns.length) {
        view.turns = item.turns;
        view.prompts = item.prompts;
        view.prompt = item.prompt;
        view.turnsOpen = true;
        renderAnswer(view);
      }
      setStream(sessionId, view);
      renderTodo(view, item.todos);
      continue;
    }

    if (item.role === "user") {
      frame.appendChild(newUserItem(item));
      continue;
    }
    if (item.turns && item.turns.length) {
      item.content = composeTurns(item.turns, item.prompts, item.content, item.paused || !item.finished, !item.pending);
    }
    const node = newAssisatantItem(item, sessionId);
    if (item.pending) {
      assistantItems.set(node, item);
    }
    frame.appendChild(node);
  }
  dom.appendChild(frame);
  scrollToBottom(true, sessionId);
}

function assistantFooter(meta, sessionId, task) {
  const buttons = [copyBtn()];
  if (sessionId && task) {
    buttons.push(historyBtn(sessionId, task));
  }
  const children = [];
  if (meta.canceled) {
    children.push(_("p.canceled", "canceled"));
  }
  if (meta.error) {
    children.push(_("p.canceled", meta.error));
  }
  if (meta.duration) {
    children.push(_("p", meta.duration + (meta.tps ? `(${meta.tps})` : "")));
  }
  if (meta.input) {
    children.push(_("div", [_("span.material-symbols-outlined", "arrow_upward_alt"), _("p", meta.input)]));
  }
  if (meta.output) {
    children.push(_("div", [_("span.material-symbols-outlined", "arrow_downward_alt"), _("p", meta.output)]));
  }
  children.push(_("p", displayAt(meta.send_at)));
  const label = (meta.model || "") + (meta.model && meta.reasoning ? `/${meta.reasoning}` : "");
  const stats = label ? _("div.stats", { name: label }, children) : _("div.stats", children);
  return _("footer", buttons.concat([stats]));
}

function displayAt(at) {
  if (!at) {
    return "";
  }
  const now = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} `;
  if (at.startsWith(today)) {
    return at.slice(today.length);
  }
  const year = `${now.getFullYear()}-`;
  return at.startsWith(year) ? at.slice(year.length) : at;
}

function newUserItem(item) {
  const dom = _("div.user", [
    _("p", item.content),
    sourceBox(item.content),
    _("footer", [_("p", displayAt(item.meta.send_at)), copyBtn()]),
  ]);

  if (item.steered) {
    dom.dataset.steered = "1";
  }
  return dom;
}

const refreshObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) {
        continue;
      }
      refreshObserver.unobserve(entry.target);
      const messages = entry.target.parentElement;
      const stick = messages.scrollHeight - messages.scrollTop - messages.clientHeight <= AUTO_SCROLL_SLACK;
      const sessionId = panelSession(entry.target);
      for (const section of entry.target.querySelectorAll("section.md-render[data-pending]")) {
        section.innerHTML = MDParser.Parse(pendingMarkdown.get(section)).html();
        pendingMarkdown.delete(section);
        delete section.dataset.pending;
      }
      refreshMarkdown(entry.target).then(() => {
        if (stick) {
          scrollToBottom(true, sessionId);
        }
      });
    }
  },
  { rootMargin: "128px 0px" },
);

const pendingMarkdown = new WeakMap();

function pendingMarkdownSection(markdown) {
  const section = _("section.md-render.nanomd", { "data-mode": "light", "data-pending": "" });
  section.textContent = markdown;
  pendingMarkdown.set(section, markdown);
  return section;
}

function newAssisatantItem(item, sessionId) {
  const body = [];

  if (item.Reasoning) {
    body.push(
      _("details.reasoning", [
        _("summary", ["Reasoning", _("span.material-symbols-outlined", "keyboard_arrow_down")]),
        pendingMarkdownSection(channelText(item.Reasoning, sessionId)),
      ]),
    );
  }

  if (item.content) {
    body.push(pendingMarkdownSection(channelText(item.content, sessionId)));
  }

  body.push(sourceBox(item.content));
  body.push(fileBox(item.files || []));
  body.push(assistantFooter(item.meta, sessionId, item.task));

  const dom = _("div.assistant", [_("img", { src: "public/logo-min.svg" }), _("section", body)]);
  refreshObserver.observe(dom);
  if (item.task) {
    dom.dataset.task = item.task;
  }
  return dom;
}

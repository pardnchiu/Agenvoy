let pendingTask = null;
let pendingAnswers = [];

async function loadPending(sessionId, taskHash, ownerId) {
  const dom = chatPart("pending", sessionId);
  if (!dom || !sessionId || !taskHash) {
    return;
  }
  const owner = ownerId || sessionId;

  clearPending(sessionId);

  let questions = [];
  try {
    const url = `${API}/v1/session/${encodeURIComponent(owner)}/task/${encodeURIComponent(taskHash)}/questions`;
    const response = await fetch(url);
    if (!response.ok) {
      return;
    }
    questions = (await response.json()).questions || [];
  } catch (err) {
    console.error("loadPending", err);
    return;
  }
  if (questions.length === 0) {
    return;
  }

  pendingTask = { sessionId: sessionId, owner: owner, taskHash: taskHash, questions: questions };
  pendingAnswers = questions.map((q) => (q.multi_select ? [] : ""));

  for (let i = 0; i < questions.length; i++) {
    dom.appendChild(pendingCard(questions[i], i, questions.length));
  }
  dom.dataset.index = "0";
  dom.dataset.task = taskHash;
  setInputTask(sessionId, taskHash);
  scrollToBottom(true, sessionId);
}

function clearPending(sessionId) {
  const dom = chatPart("pending", sessionId);
  if (!dom) {
    return;
  }
  dom.innerHTML = "";
  delete dom.dataset.index;
  delete dom.dataset.task;
  if (!pendingTask || !sessionId || pendingTask.sessionId === sessionId) {
    pendingTask = null;
    pendingAnswers = [];
  }
}

function closePendingTask(sessionId, taskHash) {
  if (!taskHash) {
    return;
  }

  const dom = chatPart("pending", sessionId);
  if (!dom) {
    return;
  }

  for (const hint of dom.querySelectorAll(`div.hint[data-task="${taskHash}"]`)) {
    hint.remove();
  }

  if (dom.dataset.task !== taskHash) {
    return;
  }
  clearPending(sessionId);
  setInputTask(sessionId, "");
}

function pendingCard(question, index, total) {
  const body = [_("strong", question.question || "")];

  if (question.detail) {
    body.push(_("p", { textContent: question.detail }));
  }

  const options = question.options || [];
  if (options.length === 0) {
    body.push(pendingInput(question, index));
  } else {
    for (const option of options) {
      body.push(pendingOption(question, index, option));
    }
  }

  const last = index === total - 1;
  const submit = _("button", { type: "button", class: "submit" }, last ? "Send" : "Next");
  submit.addEventListener("click", () => answerPending(index, total));

  const skip = _("button", { type: "button", class: "skip" }, "Skip");
  skip.addEventListener("click", () => {
    pendingAnswers[index] = question.multi_select ? [] : "";
    answerPending(index, total);
  });

  body.push(_("footer", [_("p", `${index + 1} / ${total}`), skip, submit]));
  return _("div", body);
}

function pendingInput(question, index) {
  if (question.secret) {
    const dom = _("input", { type: "password", placeholder: "Type your answer..." });
    dom.addEventListener("input", () => (pendingAnswers[index] = dom.value));
    return dom;
  }

  const dom = _("textarea", { rows: "3", placeholder: "Type your answer..." });
  dom.addEventListener("input", () => (pendingAnswers[index] = dom.value));
  dom.addEventListener("keydown", (e) => {
    if (e.key !== "Enter" || e.shiftKey || e.isComposing) {
      return;
    }
    e.preventDefault();
    answerPending(index, pendingTask ? pendingTask.questions.length : index + 1);
  });
  return dom;
}

function pendingOption(question, index, option) {
  const box = _("input", {
    type: question.multi_select ? "checkbox" : "radio",
    name: `pending-${index}`,
    value: option,
  });

  box.addEventListener("change", () => {
    if (!question.multi_select) {
      pendingAnswers[index] = option;
      return;
    }
    const picked = pendingAnswers[index].filter((item) => item !== option);
    if (box.checked) {
      picked.push(option);
    }
    pendingAnswers[index] = picked;
  });

  return _("label.option", [box, _("span", option)]);
}

function answerPending(index, total) {
  const sessionId = pendingTask ? pendingTask.sessionId : "";
  const dom = chatPart("pending", sessionId);
  if (!dom || !pendingTask) {
    return;
  }

  if (index < total - 1) {
    dom.dataset.index = String(index + 1);
    scrollToBottom(true, sessionId);
    return;
  }

  const task = pendingTask;
  const answers = pendingAnswers;
  clearPending(sessionId);
  resumePending(task, answers);
}

async function cancelPending(sessionId) {
  if (!pendingTask || pendingTask.sessionId !== sessionId) {
    return;
  }
  const taskHash = pendingTask.taskHash;
  if (!confirm("Cancel this task?")) {
    return;
  }
  const owner = pendingTask.owner || sessionId;
  clearPending(sessionId);
  setInputTask(sessionId, "");
  clearTaskCookie(sessionId);
  await deletePending(owner, taskHash);
}

async function resumePending(task, answers) {
  const owner = task.owner || task.sessionId;
  const url = `${API}/v1/session/${encodeURIComponent(owner)}/task/${encodeURIComponent(task.taskHash)}/resume`;
  writeTaskCookie(task.sessionId, task.taskHash);
  setInputTask(task.sessionId, task.taskHash);

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers: answers }),
    });
    if (!response.ok) {
      console.error("resumePending", response.status);
    }
  } catch (err) {
    console.error("resumePending", err);
  }
}

async function listResumable(sessionId) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}/task`);
    if (!response.ok) {
      console.error("listResumable", response.status);
      return [];
    }
    return (await response.json()).pending || [];
  } catch (err) {
    console.error("listResumable", err);
    return [];
  }
}

async function deletePending(sessionId, taskHash) {
  const url = `${API}/v1/session/${encodeURIComponent(sessionId)}/task/${encodeURIComponent(taskHash)}`;
  try {
    const response = await fetch(url, { method: "DELETE" });
    if (!response.ok && response.status !== 404) {
      const detail = await response.json().catch(() => ({}));
      alert(detail.error || `HTTP ${response.status}`);
      return false;
    }
  } catch (err) {
    console.error("deletePending", err);
    alert(err.message || "failed");
    return false;
  }
  if (pendingTask && pendingTask.sessionId === sessionId && pendingTask.taskHash === taskHash) {
    clearPending(sessionId);
    pendingTask = null;
  }
  return true;
}

function clearPendingHint(sessionId) {
  const dom = chatPart("pending", sessionId);
  if (!dom) {
    return;
  }
  for (const old of dom.querySelectorAll(":scope > div.hint")) {
    old.remove();
  }
}

function hintBlocked(sessionId) {
  const dom = chatPart("pending", sessionId);
  const confirmDom = chatPart("confirm", sessionId);
  return !dom || dom.childElementCount > 0 || (confirmDom && confirmDom.childElementCount > 0);
}

async function renderPendingHint(sessionId) {
  if (!sessionId || hintBlocked(sessionId)) {
    return;
  }

  const tasks = await listResumable(sessionId);
  const mine = readTaskCookie();
  if (mine && mine.session === sessionId) {
    const own = tasks.find((one) => one.task_hash === mine.task);
    if (own && own.has_questions) {
      loadPending(sessionId, own.task_hash);
      return;
    }
  }
  if (tasks.length === 0 || hintBlocked(sessionId)) {
    return;
  }

  const dom = chatPart("pending", sessionId);
  for (const one of tasks) {
    const title = String(one.objective || "").replace(/\s+/g, " ").trim() || one.task_hash;
    if (one.background > 0) {
      waitingTasks.set(sessionId, one.task_hash);
      setInputTask(sessionId, one.task_hash);
      continue;
    }
    const open = _("button", { type: "button", class: "open" }, [
      _("span.material-symbols-outlined", "live_help"),
      _("p", title),
    ]);
    const remove = _("button", { type: "button", class: "remove", title: "Delete", "aria-label": "Delete" }, [
      _("span.material-symbols-outlined", "delete"),
    ]);
    const hint = _("div", { class: "hint", "data-task": one.task_hash }, [open, remove]);
    remove.addEventListener("click", async () => {
      if (!confirm(`Delete this task?\n\n${title}`)) {
        return;
      }
      if (!(await deletePending(sessionId, one.task_hash))) {
        return;
      }
      hint.remove();
      const cookie = readTaskCookie();
      if (cookie && cookie.session === sessionId && cookie.task === one.task_hash) {
        clearTaskCookie(sessionId);
      }
    });
    open.addEventListener("click", () => {
      writeTaskCookie(sessionId, one.task_hash);
      if (one.has_questions) {
        loadPending(sessionId, one.task_hash);
        return;
      }
      if (!confirm(`Resume in this chat?\n\n${title}`)) {
        return;
      }
      startResume(sessionId, one.task_hash);
    });
    dom.appendChild(hint);
  }
}

function startResume(sessionId, taskHash) {
  if (pendingTask && pendingTask.sessionId === sessionId && pendingTask.taskHash === taskHash) {
    clearPending(sessionId);
  }
  resumePending({ sessionId: sessionId, taskHash: taskHash }, []);
}

async function listLiveTasks(sessionId) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}/task?live=1`);
    if (!response.ok) {
      console.error("listLiveTasks", response.status);
      return [];
    }
    return (await response.json()).pending || [];
  } catch (err) {
    console.error("listLiveTasks", err);
    return [];
  }
}

async function stopLiveTask(sessionId, taskHash) {
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(sessionId)}/cancel/${encodeURIComponent(taskHash)}`, { method: "POST" });
    if (!response.ok) {
      pushToast(TOAST_LEVEL.ERROR, `cancel failed: ${response.status}`, nowClock());
      return false;
    }
  } catch (err) {
    console.error("stopLiveTask", err);
    pushToast(TOAST_LEVEL.ERROR, `cancel failed: ${err?.message || err}`, nowClock());
    return false;
  }
  return true;
}

async function renderTaskButton(sessionId) {
  const panel = chatPanel(sessionId);
  const dom = panel ? panel.querySelector(":scope > header button.task") : null;
  if (!dom || !sessionId) {
    return;
  }
  dom.dataset.live = (await listLiveTasks(sessionId)).length > 0 ? "1" : "0";
}

async function openTaskPicker(sessionId) {
  if (!sessionId) {
    return;
  }

  const tasks = await listLiveTasks(sessionId);
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, "cancel");
  const root = _("div.popup", [_("div.panel", [_("strong", "Live task"), list, _("footer", [cancel])])]);
  root.id = "task-popup";

  const close = () => root.remove();

  if (tasks.length === 0) {
    list.appendChild(_("p.empty", "No running task"));
  }
  for (const one of tasks) {
    const title = String(one.objective || "").replace(/\s+/g, " ").trim() || one.task_hash;
    const box = _("input", { type: "radio", name: "task-pick", value: one.task_hash });
    const row = _("label", [box, _("div", [_("strong", title), _("p", one.task_hash)])]);
    box.addEventListener("change", async () => {
      box.checked = false;
      if (!confirm(`Stop this task?\n\n${title}`)) {
        return;
      }
      if (await stopLiveTask(sessionId, one.task_hash)) {
        row.remove();
        if (!list.querySelector("label")) {
          list.appendChild(_("p.empty", "No running task"));
        }
      }
    });
    list.appendChild(row);
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

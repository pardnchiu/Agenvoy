function renderEvent(view, event) {
  const type = event.type;

  if (type === "EventAgentResult") {
    view.model = event.text || "";
    if (view.text) {
      view.prompts = (view.prompts || []).concat([view.prompt === undefined ? originPrompt(view) : view.prompt]);
      view.prompt = view.nextPrompt || "";
      view.nextPrompt = "";
      view.turns = (view.turns || []).concat([view.text]);
      view.text = "";
      view.textStarted = false;
      view.streamed = false;
      view.resumed = false;
      view.turnsOpen = true;
      renderAnswer(view);
    }
    const name = (event.text || "").trim();
    if (name && name !== view.shownModel) {
      const action = view.shownModel ? "Switch" : "Model";
      const label = event.reasoning ? `${name} ${event.reasoning}` : name;
      view.shownModel = name;
      renderReasoning(view, `⏵ \`${action}(${label})\``);
    }
    return;
  }

  if (type === "EventTextDelta") {
    view.streamed = true;
    dropResumedText(view);
    view.text += event.text || "";
    renderAnswer(view);
    return;
  }

  if (type === "EventText") {
    if (view.streamed) {
      return;
    }
    dropResumedText(view);
    const join = view.textStarted ? "\n" : "";
    view.text += join + (event.text || "");
    view.textStarted = true;
    renderAnswer(view);
    return;
  }

  if (type === "EventReasoning") {
    renderReasoning(view, event.text || "");
    return;
  }

  if (type === "EventCanceled") {
    view.think.open = false;
    const canceled = assistantFooter(
      {
        send_at: sendAt(),
        model: view.model,
        canceled: true,
        duration: compactDuration(event.duration),
      },
      view.session,
      view.task,
    );
    view.footer.replaceWith(canceled);
    view.footer = canceled;
    return;
  }

  if (type === "EventFileChanged") {
    view.changedFiles = event.files || [];
    renderFileBox(view.files, view.changedFiles);
    return;
  }

  if (type === "EventDone") {
    if (view.turns && view.turns.length) {
      view.turnsOpen = false;
      renderAnswer(view);
    }
    const model = event.model || view.model;
    view.model = model;
    loadModelQuota(view, model);
    loadFollowup(view);
    view.think.open = false;
    delete view.think.dataset.streaming;
    const usage = event.usage || {};
    const footer = assistantFooter(
      {
        send_at: sendAt(),
        model: model,
        reasoning: event.reasoning || "",
        duration: compactDuration(event.duration),
        input: event.usage_input || "",
        output: compactToken(usage.output_tokens),
        tps: formatTPS(usage.output_tokens, event.output_elapsed),
      },
      view.session,
      view.task,
    );
    view.footer.replaceWith(footer);
    view.footer = footer;
    return;
  }

  renderReasoning(view, formatEvent(event));
}

function renderAnswer(view) {
  view.source.textContent = view.text;
  render(view.answer, composeTurns(view.turns, view.prompts, view.text, view.turnsOpen), view.session);
  if (view.answered) {
    return;
  }
  view.answered = true;
  scrollToBottom(true, view.session);
}

function originPrompt(view) {
  let bubble = view.dom ? view.dom.previousElementSibling : null;
  while (bubble && !bubble.classList.contains("user")) {
    bubble = bubble.previousElementSibling;
  }
  const source = bubble ? bubble.querySelector("pre.source") || bubble.querySelector("p") : null;
  return source ? source.textContent.split("\n===\n")[0] : "";
}

function dropResumedText(view) {
  if (!view.resumed) {
    return;
  }
  view.resumed = false;
  view.text = "";
  view.textStarted = false;
}

function renderReasoning(view, line) {
  line = (line || "").trim();
  if (!line) {
    return;
  }
  view.trace += (view.trace ? "\n\n" : "") + line;
  view.think.hidden = false;
  view.resumed = Boolean(view.text);
  render(view.reasoning, view.trace, view.session);
  if (view.reasoned) {
    return;
  }
  view.reasoned = true;
  scrollToBottom(true, view.session);
}

function renderSuggest(view) {
  if (!view.suggests || view.suggests.length === 0) {
    return;
  }

  const dom = _("section.suggests");
  for (const text of view.suggests) {
    const btn = _("button", { type: "button" }, text);
    btn.addEventListener("click", () => send(text, view.session));
    dom.appendChild(btn);
  }
  view.body.appendChild(dom);
  scrollToBottom(false, view.session);
}

function formatEvent(event) {
  const type = event.type;

  if (type === "EventToolCall") {
    const label = (event.tool_display || "").replace(/\s+/g, " ").slice(0, 120);
    return label ? `⏵ \`${label}\`` : "";
  }

  if (type === "EventToolSkipped") {
    return `⏵ skipped \`${event.tool_name || ""}\``;
  }

  if (type === "EventCompact") {
    return "⏵ compacted history";
  }

  if (type === "EventExecError") {
    return `⚠ ${event.tool_name || ""} — ${event.text || "unknown error"}`;
  }

  if (type === "EventError") {
    return `⚠ ${event.text || "unknown error"}`;
  }

  if (type === "EventCanceled") {
    return "⚠ canceled";
  }

  return event.text || "";
}

async function loadFollowup(view) {
  if (!view.session) {
    return;
  }
  try {
    const response = await fetch(`${API}/v1/session/${encodeURIComponent(view.session)}/followup`, { method: "POST" });
    if (!response.ok) {
      return;
    }
    const body = await response.json();
    renameChat(view.session, body.title || "");
    view.suggests = body.suggests || [];
    renderSuggest(view);
  } catch (err) {
    console.error("loadFollowup", err);
  }
}

async function loadModelQuota(view, model) {
  if (!model || !model.includes("@")) {
    return;
  }
  try {
    const response = await fetch(`${API}/v1/model/quota?model=${encodeURIComponent(model)}`);
    if (!response.ok) {
      return;
    }
    const text = (await response.json()).quota || "";
    if (!text) {
      return;
    }
    const stats = view.footer.querySelector("div.stats");
    const label = stats?.getAttribute("name");
    if (label) {
      stats.setAttribute("name", `${label} · ${text}`);
    }
  } catch (err) {
    console.error("loadModelQuota", err);
  }
}

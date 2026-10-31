const ACTION_LINE = /^\[([^\]]+)\]\[([^\]]+)\]\[([^\]]+)\](?:\[([^\]]*)\])?\s?([\s\S]*)$/;
const ACTION_NEWLINE = "\u001f";
const DURATION_UNIT = { ns: 1e-6, us: 1e-3, "\u00b5s": 1e-3, ms: 1, s: 1000, m: 60000, h: 3600000 };

function parseActionLog(content) {
  const list = [];
  const taskItems = new Map();
  const writerItem = new Map();

  const itemOf = (task, writer, sendAt) => {
    let item = task ? taskItems.get(task) : null;
    if (item) {
      return item;
    }
    const current = writerItem.get(writer);
    if (current && !current.finished && (!task || !current.task)) {
      item = current;
    } else {
      item = logItem(sendAt);
      list.push(item);
      writerItem.set(writer, item);
    }
    if (task) {
      item.task = task;
      taskItems.set(task, item);
    }
    return item;
  };

  for (const line of content.split("\n")) {
    const match = ACTION_LINE.exec(line);
    if (!match) {
      continue;
    }

    const sendAt = match[1].slice(0, 16);
    const writer = match[2];
    const kind = match[3];
    const task = match[4] || "";
    const body = match[5].split(ACTION_NEWLINE).join("\n").trim();
    const owner = task ? taskItems.get(task) : writerItem.get(writer);

    if (owner && kind !== "pending") {
      owner.paused = false;
    }

    let item = null;
    switch (kind) {
      case "user":
        if (body.startsWith("[Resumed Task") || body.startsWith("[Scheduled Task") || body.startsWith("[Shell command]")) {
          break;
        }

        writerItem.delete(writer);
        if (body) {
          list.push({ role: "user", content: body, meta: { send_at: sendAt } });
        }
        break;

      case "steer": {
        if (!body) {
          break;
        }
        const last = list.findLast((entry) => entry.role === "user" || (entry.finished && (entry.content || entry.Reasoning)));
        if (last && last.role === "user") {
          last.content += steerMark(sendAt, body, !last.steered);
          last.steered = true;
          break;
        }
        list.push({ role: "user", content: body, meta: { send_at: sendAt } });
        break;
      }

      case "agent_result":
        item = itemOf(task, writer, sendAt);
        item.meta.model = body;
        break;

      case "edited_files": {
        item = itemOf(task, writer, sendAt);
        try {
          const files = JSON.parse(body);
          if (Array.isArray(files)) {
            item.files = files;
          }
        } catch (err) {
          console.error("parseActionLog edited_files", err);
        }
        break;
      }

      case "thinking":
        item = itemOf(task, writer, sendAt);
        item.Reasoning += (item.Reasoning ? "\n\n" : "") + body;
        item.resumed = Boolean(item.content);
        break;

      case "tool_call": {
        item = itemOf(task, writer, sendAt);
        const todos = parseTodoArgs(body);
        if (todos) {
          item.todos = todos;
          break;
        }
        item.Reasoning += (item.Reasoning ? "\n\n" : "") + formatTool(body);
        item.resumed = Boolean(item.content);
        break;
      }

      case "todo": {
        item = itemOf(task, writer, sendAt);
        const list = parseTodoLine(body);
        if (list) {
          item.todos = list;
        }
        break;
      }

      case "pending":
        if (owner) {
          owner.paused = true;
        }
        break;

      case "skill_result":
        break;

      case "assistant":
        item = itemOf(task, writer, sendAt);
        if (item.resumed) {
          item.content = "";
        }
        item.content += (item.content ? "\n\n" : "") + body;
        item.resumed = false;
        item.meta.send_at = sendAt;
        break;

      case "error": {
        item = itemOf(task, writer, sendAt);

        item.meta.send_at = sendAt;
        item.meta.error = body;
        item.finished = true;
        break;
      }

      case "canceled": {
        item = itemOf(task, writer, sendAt);

        const meta = formatDone(body);
        item.meta.model = meta.model || item.meta.model;
        item.meta.duration = meta.duration;
        item.meta.send_at = sendAt;
        item.meta.canceled = sendAt;
        item.finished = true;
        break;
      }

      case "done": {
        item = itemOf(task, writer, sendAt);

        const meta = formatDone(body);
        item.meta.model = meta.model || item.meta.model;
        item.meta.reasoning = meta.reasoning;
        item.meta.duration = meta.duration;
        item.meta.input = meta.input;
        item.meta.output = meta.output;
        item.meta.tps = meta.tps;
        item.meta.send_at = sendAt;
        item.finished = true;
        break;
      }
    }
  }

  return list.filter((item, i) => {
    if (item.role === "user") {
      return true;
    }
    item.pending = !item.finished;
    return Boolean(item.content || item.Reasoning || (item.pending && i === list.length - 1));
  });
}

function actionTaskState(content, taskHash) {
  if (!taskHash) {
    return "";
  }
  let state = "";
  for (const line of content.split("\n")) {
    const match = ACTION_LINE.exec(line);
    if (!match || match[4] !== taskHash) {
      continue;
    }
    switch (match[3]) {
      case "done":
      case "canceled":
      case "error":
        return "finished";
      default:
        state = "running";
    }
  }
  return state;
}

function logItem(sendAt) {
  return {
    role: "assistant",
    task: "",
    content: "",
    Reasoning: "",
    files: [],
    resumed: false,
    finished: false,
    meta: { model: "", send_at: sendAt },
  };
}

function steerMark(sendAt, body, first) {
  return (first ? "\n===\n" : "\n") + sendAt + " - " + body;
}

function parseTodoLine(body) {
  try {
    const list = JSON.parse(body);
    return Array.isArray(list) ? list : null;
  } catch (err) {
    console.error("parseTodoLine", err);
    return null;
  }
}

function parseTodoArgs(body) {
  if (!body.startsWith("write_todo ")) {
    return null;
  }
  try {
    const todos = JSON.parse(body.slice("write_todo ".length)).todos;
    return Array.isArray(todos) ? todos : null;
  } catch (err) {
    console.error("parseTodoArgs", err);
    return null;
  }
}

function formatTool(body) {
  const label = body.replace(/\s+/g, " ").slice(0, 120);
  return label ? `⏵ \`${label}\`` : "";
}

function formatDone(body) {
  const model = (body.split(/\s+/)[0] || "").includes("=") ? "" : body.split(/\s+/)[0] || "";
  const duration = /\bdur=(\S+)/.exec(body);
  const input = /\bin=(\d+)(?:\s*\((\d+)%\))?/.exec(body);
  const output = /\bout=(\d+)/.exec(body);
  const outputDuration = /\boutdur=(\S+)/.exec(body);
  const reasoning = /\breasoning=(\S+)/.exec(body);
  return {
    model: model,
    reasoning: reasoning ? reasoning[1] : "",
    duration: duration ? compactDuration(duration[1]) : "",
    input: input ? compactToken(input[1]) + (input[2] ? `(${input[2]}%)` : "") : "",
    output: output ? compactToken(output[1]) : "",
    tps: output && outputDuration ? formatTPS(output[1], outputDuration[1]) : "",
  };
}

function durationMS(value) {
  let ms = 0;
  if (typeof value === "number") {
    ms = value / 1e6;
  } else if (typeof value === "string") {
    for (const match of value.matchAll(/([\d.]+)(ms|us|\u00b5s|ns|s|m|h)/g)) {
      ms += parseFloat(match[1]) * DURATION_UNIT[match[2]];
    }
  }
  return Number.isFinite(ms) ? ms : 0;
}

function formatTPS(output, elapsed) {
  const ms = durationMS(elapsed);
  const tokens = Number(output);
  if (ms <= 0 || !Number.isFinite(tokens) || tokens <= 0) {
    return "";
  }
  return `${(tokens / (ms / 1000)).toFixed(1)}t/s`;
}

function compactDuration(value) {
  const ms = durationMS(value);
  if (ms <= 0) {
    return "";
  }
  if (ms < 1000) {
    return `${Math.round(ms)}ms`;
  }

  const sec = ms / 1000;
  if (sec < 60) {
    return `${sec.toFixed(1)}s`;
  }
  return `${Math.floor(sec / 60)}m ${Math.round(sec % 60)}s`;
}

function compactToken(value) {
  if (value === null || value === undefined || value === "") {
    return "";
  }

  const number = Number(value);
  if (!Number.isFinite(number)) {
    return "";
  }

  if (number < 1_000) {
    return String(number);
  }
  if (number < 1_000_000) {
    return `${(number / 1_000).toFixed(1)}k`;
  }
  return `${(number / 1_000_000).toFixed(1)}m`;
}

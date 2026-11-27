let mcpEditing = "";
let mcpStream = null;

function mcpDom() {
  return {
    form: $("#mcp-form"),
    list: $("#mcp-list"),
    name: $("#mcp-name"),
    transport: $("#mcp-transport"),
    auth: $("#mcp-auth"),
    command: $("#mcp-command"),
    args: $("#mcp-args"),
    env: $("#mcp-env"),
    url: $("#mcp-url"),
    headers: $("#mcp-headers"),
    oauth: $("#mcp-oauth"),
    submit: document.querySelector("#mcp-form button.submit"),
    tools: $("#mcp-tools"),
  };
}

function mcpError(text) {
  alert(text);
}

function markSelectedCard(list, name) {
  if (!list) {
    return;
  }
  for (const card of list.querySelectorAll("div.card")) {
    card.dataset.selected = card.dataset.name === name ? "1" : "0";
  }
}

function mcpLines(text) {
  return (text || "")
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function mcpPairs(text, separator) {
  const dic = {};
  for (const line of mcpLines(text)) {
    const at = line.indexOf(separator);
    if (at < 1) continue;
    const key = line.slice(0, at).trim();
    if (!key) continue;
    dic[key] = line.slice(at + separator.length).trim();
  }
  return dic;
}

function mcpPairText(dic, separator) {
  return Object.entries(dic || {})
    .map(([k, v]) => `${k}${separator}${v}`)
    .join("\n");
}

async function mcpServers() {
  const out = { servers: {}, oauth: {}, status: {} };

  try {
    const [listRes, statusRes] = await Promise.all([fetch(`${API}/v1/mcp`), fetch(`${API}/v1/mcp/status`)]);
    if (listRes.ok) {
      const body = await listRes.json();
      out.servers = body.servers || {};
      out.oauth = body.oauth || {};
    }
    if (statusRes.ok) {
      for (const item of (await statusRes.json()).servers || []) {
        out.status[item.Name] = item;
      }
    }
  } catch (err) {
    console.error("mcpServers", err);
  }
  return out;
}

async function renderMcp() {
  const dom = mcpDom();
  if (!dom.list) {
    return;
  }

  const { servers, oauth, status } = await mcpServers();
  const names = Object.keys(servers).sort();
  const picked = praseURL().target || "";

  dom.list.innerHTML = "";

  for (const name of names) {
    const server = servers[name] || {};
    const state = status[name];
    const transport = server.url ? "http" : "stdio";

    const detail = [transport];
    let flag = "off";
    if (state && state.Connected) {
      flag = "on";
    } else if (state && state.Error) {
      detail.push(state.Error);
      flag = "error";
    } else {
      detail.push(i18n("label_not_connected"));
    }
    if (server.auth === "oauth") {
      detail.push(oauth[name] ? i18n("label_oauth_authorized") : i18n("label_oauth_pending"));
    }

    const remove = _("button", { type: "button" }, [_("span.material-symbols-outlined", "delete")]);
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteMcp(name);
    });

    const card = _("div.card", [_("strong", name), _("p", detail.join(" · ")), remove]);
    card.dataset.name = name;
    card.dataset.state = flag;
    card.dataset.selected = name === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "config", tab: "MCP", target: name });
    });
    dom.list.appendChild(card);
  }

  if (picked && servers[picked]) {
    fillMcpForm(picked, servers[picked], oauth[picked] === true);
  }
}

function fillMcpForm(name, server, authorized) {
  const dom = mcpDom();
  if (!dom.form) {
    return;
  }

  const transport = server.url ? "http" : "stdio";
  dom.name.value = name || "";
  dom.name.readOnly = Boolean(name);
  setMcpTransport(transport);
  dom.auth.value = server.auth === "oauth" ? "oauth" : "";
  dom.command.value = server.command || "";
  dom.args.value = (server.args || []).join("\n");
  dom.env.value = mcpPairText(server.env, "=");
  dom.url.value = server.url || "";
  dom.headers.value = mcpPairText(server.headers, ": ");
  dom.form.dataset.transport = transport;

  mcpEditing = name || "";
  if (name) {
    dom.form.dataset.editing = "1";
  } else {
    delete dom.form.dataset.editing;
  }
  if (dom.submit) {
    dom.submit.textContent = name ? i18n("text_save") : i18n("text_add");
  }
  renderMcpOAuth(authorized);
  renderMcpTools(name);
}

async function openMcp(name) {
  const { servers, oauth } = await mcpServers();
  const server = servers[name];
  if (!server) {
    mcpError(i18n("error_not_found_mcp", name));
    return;
  }
  fillMcpForm(name, server, oauth[name] === true);
}

function resetMcp() {
  closeMcpLogin();
  markSelectedCard(mcpDom().list, "");
  fillMcpForm("", {}, false);
}

function mcpTransport() {
  const dom = mcpDom();
  const active = dom.transport && dom.transport.querySelector('button[data-selected="1"]');
  return active ? active.name : "stdio";
}

function setMcpTransport(value) {
  const dom = mcpDom();
  if (!dom.transport) {
    return;
  }
  for (const one of dom.transport.querySelectorAll("button")) {
    if (one.name === value) {
      one.dataset.selected = "1";
    } else {
      delete one.dataset.selected;
    }
  }
  if (dom.form) {
    dom.form.dataset.transport = value;
  }
}

function mcpTransportChange(e) {
  const button = e && e.target ? e.target.closest("button") : null;
  if (button && button.name) {
    setMcpTransport(button.name);
  }
  renderMcpOAuth(false);
}

function mcpAuthChange() {
  renderMcpOAuth(false);
}

async function saveMcp() {
  const dom = mcpDom();
  if (!dom.form) {
    return;
  }

  const name = dom.name.value.trim();
  if (!name) {
    mcpError(i18n("error_name_required"));
    return;
  }

  const server = {};
  if (mcpTransport() === "http") {
    const url = dom.url.value.trim();
    if (!url) {
      mcpError(i18n("error_url_required"));
      return;
    }
    server.url = url;
    const headers = mcpPairs(dom.headers.value, ":");
    if (Object.keys(headers).length > 0) {
      server.headers = headers;
    }
    if (dom.auth.value === "oauth") {
      server.auth = "oauth";
    }
  } else {
    const command = dom.command.value.trim();
    if (!command) {
      mcpError(i18n("error_command_required"));
      return;
    }
    server.command = command;
    const args = mcpLines(dom.args.value);
    if (args.length > 0) {
      server.args = args;
    }
    const env = mcpPairs(dom.env.value, "=");
    if (Object.keys(env).length > 0) {
      server.env = env;
    }
  }

  try {
    const response = await fetch(`${API}/v1/mcp`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name, server: server }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      mcpError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("saveMcp", err);
    mcpError(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "config", tab: "MCP", target: name });
}

async function removeMcpServer(name) {
  const response = await fetch(`${API}/v1/mcp/remove`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: name }),
  });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new Error(detail.error || `HTTP ${response.status}`);
  }
}

async function deleteMcp(name) {
  if (!name || !confirm(i18n("confirm_remove_mcp", name))) {
    return;
  }

  try {
    await removeMcpServer(name);
  } catch (err) {
    console.error("deleteMcp", err);
    mcpError(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "config", tab: "MCP" });
}

function deleteEditingMcp() {
  if (mcpEditing) {
    deleteMcp(mcpEditing);
  }
}

async function reconnectMcp() {
  try {
    const response = await fetch(`${API}/v1/mcp/reconnect`, { method: "POST" });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      mcpError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("reconnectMcp", err);
    mcpError(err.message || i18n("text_failed"));
    return;
  }
  renderMcp();
}

function renderMcpOAuth(authorized, note) {
  const dom = mcpDom();
  if (!dom.oauth) {
    return;
  }

  dom.oauth.innerHTML = "";
  if (!mcpEditing || mcpTransport() !== "http" || dom.auth.value !== "oauth") {
    delete dom.oauth.dataset.open;
    delete dom.form.dataset.oauth;
    return;
  }
  dom.form.dataset.oauth = mcpStream ? "busy" : authorized ? "done" : "pending";

  if (!note) {
    delete dom.oauth.dataset.open;
    return;
  }

  dom.oauth.dataset.open = "1";
  dom.oauth.appendChild(_("div.row", [_("p", note)]));
}

function mcpToolPrefix(name) {
  return `mcp__${name}__`;
}

async function mcpToolList(prefix) {
  let tools = [];
  try {
    const response = await fetch(`${API}/v1/mcp/tools`);
    if (response.ok) {
      tools = (await response.json()).tools || [];
    }
  } catch (err) {
    console.error("mcpToolList", err);
  }

  return tools
    .filter((tool) => (tool.name || "").startsWith(prefix))
    .map((tool) => ({
      entry: tool.name,
      label: tool.name.slice(prefix.length),
      description: (tool.description || "").split("\n")[0],
    }))
    .sort((a, b) => a.label.localeCompare(b.label));
}

async function mcpGranted(prefix) {
  try {
    const response = await fetch(`${API}/v1/allowlist?prefix=${encodeURIComponent(prefix)}`);
    if (response.ok) {
      return (((await response.json()) || {}).tool || {}).entries || [];
    }
  } catch (err) {
    console.error("mcpGranted", err);
  }
  return [];
}

async function saveMcpPermission(prefix, entries) {
  try {
    const response = await fetch(`${API}/v1/allowlist`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ tool: { prefix: prefix, entries: entries } }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      mcpError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveMcpPermission", err);
    mcpError(err.message || i18n("text_failed"));
  }
}

function mcpToolRow(label, description, checked, onToggle) {
  const box = _("input", { type: "checkbox" });
  box.checked = checked;
  box.addEventListener("change", () => onToggle(box.checked));

  return _("label.tool", [box, _("p", label), _("span", description || "")]);
}

async function renderMcpTools(name) {
  const dom = mcpDom();
  if (!dom.tools) {
    return;
  }

  dom.tools.innerHTML = "";
  if (!name) {
    delete dom.tools.dataset.open;
    return;
  }
  dom.tools.dataset.open = "1";

  const prefix = mcpToolPrefix(name);
  const [tools, granted] = await Promise.all([mcpToolList(prefix), mcpGranted(prefix)]);
  const all = granted.includes(`${prefix}*`);

  dom.tools.appendChild(_("strong", i18n("label_always_allow")));

  dom.tools.appendChild(
    mcpToolRow(i18n("text_all"), i18n("hint_all"), all, (checked) => {
      saveMcpPermission(prefix, checked ? [`${prefix}*`] : []).then(() => renderMcpTools(name));
    }),
  );

  if (tools.length === 0) {
    dom.tools.appendChild(_("p.empty", i18n("label_no_tools")));
    return;
  }

  for (const tool of tools) {
    const checked = all || granted.includes(tool.entry);
    dom.tools.appendChild(
      mcpToolRow(tool.label, tool.description, checked, (on) => {
        let next = tools.filter((item) => (all || granted.includes(item.entry)) && item.entry !== tool.entry);
        if (on) {
          next = next.concat([tool]);
        }
        const entries = next.map((item) => item.entry);
        saveMcpPermission(prefix, entries.length === tools.length ? [`${prefix}*`] : entries).then(() =>
          renderMcpTools(name),
        );
      }),
    );
  }
}

function closeMcpLogin() {
  if (mcpStream) {
    mcpStream.close();
    mcpStream = null;
  }
}

function startMcpLogin(name) {
  closeMcpLogin();
  renderMcpOAuth(false, i18n("label_waiting_provider"));

  mcpStream = new EventSource(`${API}/v1/mcp/oauth?name=${encodeURIComponent(name)}`);
  mcpStream.onmessage = function (e) {
    let event = {};
    try {
      event = JSON.parse(e.data);
    } catch (err) {
      console.error("startMcpLogin", err);
      return;
    }

    if (event.url) {
      window.open(event.url, "_blank", "noreferrer");
      renderMcpLoginPaste(name, event.url);
      return;
    }
    if (!event.done) {
      return;
    }

    closeMcpLogin();
    if (!event.ok) {
      mcpError(event.error || i18n("error_oauth_failed"));
    }
    openMcp(name);
    renderMcp();
  };
  mcpStream.onerror = function () {
    closeMcpLogin();
    renderMcpOAuth(false, i18n("label_stream_closed"));
  };
}

function renderMcpLoginPaste(name, url) {
  const dom = mcpDom();
  if (!dom.oauth) {
    return;
  }

  const open = _("a", { href: url, target: "_blank", rel: "noreferrer" }, i18n("label_open_authorization"));
  const input = _("input", { type: "text", placeholder: i18n("placeholder_redirect") });
  const submit = _("button", { type: "button" }, i18n("text_submit"));
  submit.addEventListener("click", () => submitMcpCallback(name, input.value.trim()));

  dom.oauth.innerHTML = "";
  dom.oauth.dataset.open = "1";
  if (dom.form) {
    dom.form.dataset.oauth = "busy";
  }
  dom.oauth.appendChild(_("div.row", [open]));
  dom.oauth.appendChild(_("div.row", [input, submit]));
}

async function submitMcpCallback(name, url) {
  if (!url) {
    mcpError(i18n("error_redirect_required"));
    return;
  }

  try {
    const response = await fetch(`${API}/v1/mcp/oauth/callback`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name, url: url }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      mcpError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("submitMcpCallback", err);
    mcpError(err.message || i18n("text_failed"));
  }
}

async function clearMcpOAuth(name) {
  if (!confirm(i18n("confirm_clear_oauth", name))) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/mcp/oauth`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: name }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      mcpError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("clearMcpOAuth", err);
    mcpError(err.message || i18n("text_failed"));
    return;
  }
  openMcp(name);
  renderMcp();
}

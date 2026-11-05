const providerProbe = {};
let providerQuota = null;
let providerQuotaLoading = null;
let modelView = "add";
let modelProvider = "";
let modelAbort = null;
let modelStream = null;
const modelOAuth = { id: "", code: "", url: "" };
const DECISION_DISPATCHER = "decision";

function modelDom() {
  return {
    form: $("#model-form"),
    list: $("#model-list"),
    catalog: $("#model-catalog"),
    routing: $("#model-routing"),
    priority: $("#model-priority"),
    models: $("#model-models"),
  };
}

function modelError(text) {
  alert(text);
}


function providerConsoleLink(id, page) {
  return `${API}/v1/provider/${encodeURIComponent(id)}/console?page=${page}`;
}

function providerConsolePages(catalog, id) {
  return (catalog.find((item) => item.id === id) || {}).console || [];
}

function providerMethod(catalog, id) {
  const provider = catalog.find((item) => item.id === id);
  return Object.keys((provider || {}).methods || {})[0] || "api_key";
}

function providerFailure(id, method, error, pages) {
  const billing = pages.includes("billing") ? providerConsoleLink(id, "billing") : undefined;
  const status = (String(error).match(/HTTP (\d{3})/) || [])[1] || "";
  const text = String(error);

  if (/credit|spending limit|quota|insufficient|billing|payment/i.test(text)) {
    return {
      text: "This account is out of credits or has hit its spending limit. Top it up, then reload.",
      link: billing,
      linkLabel: "open billing",
    };
  }
  if (method === "oauth") {
    return { text: "The login expired or was revoked. Sign in again to restore access.", login: true };
  }
  if (status === "401" || status === "403") {
    return {
      text: "The stored key was rejected. Delete it and paste a new one.",
      remove: true,
      link: billing,
      linkLabel: "open console",
    };
  }
  return {
    text: `The provider answered ${status ? `HTTP ${status}` : "an error"} instead of a model list.`,
    remove: true,
    link: billing,
    linkLabel: "open console",
  };
}

let catalogIds = [];
let decisionIds = [];

function isCompatPrefix(id) {
  return id.startsWith("compat[") || (catalogIds.length > 0 && !catalogIds.includes(id));
}

function providerKeyName(id) {
  const compat = id.match(/^compat\[(.+)\]$/);
  if (compat) {
    return `COMPAT_${compat[1].toUpperCase()}_API_KEY`;
  }
  if (isCompatPrefix(id)) {
    return `COMPAT_${id.toUpperCase()}_API_KEY`;
  }
  return `${id.toUpperCase()}_API_KEY`;
}

function providerKeyNames(id) {
  if (id === "cloudflare") {
    return ["CLOUDFLARE_API_KEY", "CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_GATEWAY_ID"];
  }
  return [providerKeyName(id)];
}

async function deleteKey(key) {
  try {
    const response = await fetch(`${API}/v1/key?key=${encodeURIComponent(key)}`, { method: "DELETE" });
    if (!response.ok && response.status !== 404) {
      const detail = await response.json().catch(() => ({}));
      console.error("deleteKey", key, detail.error || response.status);
    }
  } catch (err) {
    console.error("deleteKey", key, err);
  }
}

async function deleteProvider(prefix) {
  const catalog = await providerCatalog();
  const method = providerMethod(catalog, prefix);
  const target = method === "oauth" ? "its stored login" : providerKeyNames(prefix).join(", ");
  if (!confirm(`Remove ${prefix}? Its registered models and ${target} are deleted.`)) {
    return;
  }

  await saveProviderModels(prefix, []).catch((err) => console.error("deleteProvider models", err));

  if (method === "oauth") {
    try {
      const response = await fetch(`${API}/v1/provider/${encodeURIComponent(prefix)}/oauth`, { method: "DELETE" });
      if (!response.ok) {
        const detail = await response.json().catch(() => ({}));
        modelError(detail.error || `HTTP ${response.status}`);
      }
    } catch (err) {
      console.error("deleteProvider oauth", err);
    }
  } else {
    for (const key of providerKeyNames(prefix)) {
      await deleteKey(key);
    }
  }

  delete providerProbe[prefix];
  providerQuota = null;
  selectProviderAdd();
}

async function removeProviderKey(id) {
  const keys = providerKeyNames(id);
  if (!confirm(`Delete ${keys.join(", ")}?`)) {
    return;
  }

  for (const key of keys) {
    await deleteKey(key);
  }

  delete providerProbe[id];
  providerQuota = null;
  selectProviderAdd();
}

async function providerCatalog() {
  try {
    const response = await fetch(`${API}/v1/providers`);
    if (response.ok) {
      const list = (await response.json()).providers || [];
      catalogIds = list.map((provider) => provider.id);
      decisionIds = list.filter((provider) => provider.decision).map((provider) => provider.id);
      return list;
    }
  } catch (err) {
    console.error("providerCatalog", err);
  }
  return [];
}

async function registeredModels() {
  try {
    const response = await fetch(`${API}/v1/models`);
    if (response.ok) {
      return ((await response.json()).data || []).map((item) => item.id).filter((id) => id && id !== "auto");
    }
  } catch (err) {
    console.error("registeredModels", err);
  }
  return [];
}

function fetchProviderQuota() {
  if (providerQuota) {
    return Promise.resolve(providerQuota);
  }
  if (providerQuotaLoading) {
    return providerQuotaLoading;
  }

  providerQuotaLoading = fetch(`${API}/v1/providers/quota`)
    .then((response) => (response.ok ? response.json() : { quota: {} }))
    .then((body) => {
      providerQuota = body.quota || {};
      return providerQuota;
    })
    .catch((err) => {
      console.error("fetchProviderQuota", err);
      return {};
    })
    .finally(() => {
      providerQuotaLoading = null;
    });
  return providerQuotaLoading;
}

function applyQuotaBadges(quotas) {
  const dom = modelDom();
  if (!dom.list) {
    return;
  }

  for (const card of dom.list.querySelectorAll("div.card")) {
    const existing = card.querySelector("span.quota");
    if (existing) {
      existing.remove();
    }
    const quota = quotaLabel((quotas || {})[card.dataset.name]);
    if (!quota) {
      continue;
    }
    const badge = _("span.quota", quota.text);
    badge.dataset.state = quota.state;
    card.insertBefore(badge, card.querySelector("button"));
  }
}

function quotaLabel(entry) {
  const remaining = entry?.remaining;
  if (!remaining) {
    return null;
  }
  if (remaining.balance !== undefined) {
    return { text: `$${remaining.balance.toFixed(2)}`, state: quotaLevel("balance", remaining.balance) };
  }
  const values = [remaining.five_hour, remaining.week, remaining.total].filter((value) => value !== undefined).map(Math.round);
  if (values.length === 0) {
    return null;
  }
  return { text: values.map((value) => `${value}%`).join("/"), state: quotaLevel("percent", Math.min(...values)) };
}

function quotaLevel(kind, value) {
  if (kind === "balance" || value >= 50) {
    return "okay";
  }
  return value >= 30 ? "caution" : "error";
}

async function storedKeys() {
  try {
    const response = await fetch(`${API}/v1/keys`);
    if (response.ok) {
      return (await response.json()).keys || [];
    }
  } catch (err) {
    console.error("storedKeys", err);
  }
  return [];
}

function modelPrefix(id) {
  const at = id.indexOf("@");
  return at < 0 ? "" : id.slice(0, at);
}

async function probeProvider(prefix) {
  if (providerProbe[prefix]) {
    return providerProbe[prefix];
  }
  const result = await providerModelList(prefix);
  if (result.aborted) {
    return { aborted: true };
  }
  providerProbe[prefix] = result.error
    ? { ok: false, error: result.error }
    : { ok: true, models: result.models, windows: result.windows };
  return providerProbe[prefix];
}

async function renderModel() {
  const dom = modelDom();
  if (!dom.list) {
    return;
  }

  const [catalog, registered, keys] = await Promise.all([providerCatalog(), registeredModels(), storedKeys()]);
  const quotas = providerQuota;

  const active = [];
  for (const provider of catalog) {
    if (provider.logged_in || keys.includes(providerKeyName(provider.id))) {
      active.push(provider.id);
    }
  }
  for (const id of registered) {
    const prefix = modelPrefix(id);
    if (prefix && !active.includes(prefix)) {
      active.push(prefix);
    }
  }

  const order = catalog.map((provider) => provider.id);
  const prefixes = order.filter((id) => active.includes(id)).concat(active.filter((id) => !order.includes(id)));

  const label = {};
  for (const provider of catalog) {
    label[provider.id] = provider.label;
  }
  const count = (prefix) => registered.filter((id) => modelPrefix(id) === prefix).length;

  const sideIds = prefixes.filter((id) => !decisionIds.includes(id));
  const picked = praseURL().target || "";
  if (picked && sideIds.includes(picked)) {
    modelView = "provider";
    modelProvider = picked;
  } else {
    modelView = "add";
    modelProvider = "";
  }

  dom.list.innerHTML = "";
  for (const [name, view] of [["add-provider", "add"]]) {
    const button = $(`section.config div.side button[name="${name}"]`);
    if (button) {
      button.dataset.selected = modelView === view ? "1" : "0";
    }
  }

  const providerCard = (prefix) => {
    const total = count(prefix);
    const children = [_("strong", label[prefix] || prefix), _("p", `${total} model${total === 1 ? "" : "s"}`)];

    const quota = quotaLabel((quotas || {})[prefix]);
    if (quota) {
      const badge = _("span.quota", quota.text);
      badge.dataset.state = quota.state;
      children.push(badge);
    }

    const remove = _("button", { type: "button" }, [_("span.material-symbols-outlined", "delete")]);
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteProvider(prefix);
    });
    children.push(remove);

    const card = _("div.card", children);
    card.dataset.name = prefix;
    card.dataset.selected = modelView === "provider" && prefix === modelProvider ? "1" : "0";
    card.addEventListener("click", () => selectProvider(prefix));
    return card;
  };

  for (const prefix of sideIds.filter((id) => count(id) > 0)) {
    dom.list.appendChild(providerCard(prefix));
  }

  const empty = sideIds.filter((id) => count(id) === 0);
  if (empty.length > 0) {
    dom.list.appendChild(_("span.group", "No models"));
    for (const prefix of empty) {
      dom.list.appendChild(providerCard(prefix));
    }
  }

  if (!providerQuota) {
    fetchProviderQuota().then(applyQuotaBadges);
  }

  if (dom.form) {
    dom.form.dataset.view = modelView;
  }
  if (modelView === "provider") {
    renderProviderModels(modelProvider, registered);
    return;
  }
  renderProviderCatalog(catalog, prefixes);
  renderModelRouting(registered);
  renderModelPriority();
}

async function modelPriority() {
  try {
    const response = await fetch(`${API}/v1/model/priority`);
    if (response.ok) {
      const body = (await response.json()) || {};
      return { names: body.models || [], tiers: body.tiers || {}, tierOptions: body.tier_options || [] };
    }
  } catch (err) {
    console.error("modelPriority", err);
  }
  return { names: [], tiers: {}, tierOptions: [] };
}

async function saveModelTier(model, tier) {
  try {
    const response = await fetch(`${API}/v1/model/tier`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model, tier }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      modelError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveModelTier", err);
    modelError(err.message || "failed");
  }
  renderModelPriority();
}

function tierLabel(tier) {
  if (!tier) {
    return "none";
  }
  return tier === "pass" ? "pass" : `${tier}-tier`;
}

function openTierPicker(name, current, options) {
  const list = _("div.list");

  const cancel = _("button", { type: "button" }, "cancel");
  const root = _("div.popup", [_("div.panel", [_("strong", `Tier · ${name}`), list, _("footer", [cancel])])]);
  root.id = "tier-popup";

  const close = () => root.remove();

  for (const option of [...options.filter((one) => !one.tier), ...options.filter((one) => one.tier)]) {
    const box = _("input", { type: "radio", name: "tier-pick", value: option.tier });
    box.checked = current === option.tier;
    box.addEventListener("change", () => {
      close();
      saveModelTier(name, option.tier);
    });
    list.appendChild(_("label", [box, _("div", [_("strong", tierLabel(option.tier)), _("p", option.detail.replaceAll("  ", " · "))])]));
  }

  cancel.addEventListener("click", close);
  root.addEventListener("click", (e) => {
    if (e.target === root) close();
  });

  document.body.appendChild(root);
}

function tierButton(name, current, options) {
  const button = _("button.tier", { type: "button" }, tierLabel(current));
  button.draggable = false;
  button.addEventListener("click", () => openTierPicker(name, current, options));
  return button;
}

async function saveModelPriority(names) {
  try {
    const response = await fetch(`${API}/v1/model/priority`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ models: names }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      modelError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveModelPriority", err);
    modelError(err.message || "failed");
  }
  renderModelPriority();
}

let priorityDrag = -1;

function priorityRow(name, rank, total, move, tier) {
  const rankText = `#${rank + 1}`;
  const label = _("div.label", [
    _("p", rank === total - 1 && total > 1 ? `${rankText} · final` : rankText),
    _("strong", name),
  ]);
  const row = _("div.routing", [_("span.material-symbols-outlined.grip", "drag_indicator"), label, tier]);

  row.draggable = true;
  row.dataset.rank = String(rank);

  row.addEventListener("dragstart", (e) => {
    priorityDrag = rank;
    row.dataset.dragging = "1";
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", String(rank));
  });
  row.addEventListener("dragend", () => {
    priorityDrag = -1;
    for (const one of document.querySelectorAll("#model-priority div.routing")) {
      delete one.dataset.dragging;
      delete one.dataset.drop;
    }
  });
  row.addEventListener("dragover", (e) => {
    if (priorityDrag === -1 || priorityDrag === rank) {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    row.dataset.drop = priorityDrag < rank ? "below" : "above";
  });
  row.addEventListener("dragleave", () => {
    delete row.dataset.drop;
  });
  row.addEventListener("drop", (e) => {
    e.preventDefault();
    e.stopPropagation();
    const from = priorityDrag === -1 ? Number(e.dataTransfer.getData("text/plain")) : priorityDrag;
    delete row.dataset.drop;
    if (Number.isNaN(from) || from === rank) {
      return;
    }
    move(from, rank);
  });

  return row;
}

async function renderModelPriority() {
  const dom = modelDom();
  if (!dom.priority) {
    return;
  }

  const { names, tiers, tierOptions } = await modelPriority();
  if (modelView !== "add") {
    return;
  }

  const box = dom.priority.querySelector("section");
  if (!box) {
    return;
  }

  const move = (from, to) => {
    const next = names.slice();
    const [one] = next.splice(from, 1);
    next.splice(to, 0, one);
    saveModelPriority(next);
  };

  for (const row of box.querySelectorAll("div.routing")) {
    row.remove();
  }
  names.forEach((name, rank) =>
    box.appendChild(priorityRow(name, rank, names.length, move, tierButton(name, tiers[name] || "", tierOptions))),
  );
}

async function modelRouting() {
  try {
    const response = await fetch(`${API}/v1/model`);
    if (response.ok) {
      const body = (await response.json()) || {};
      return {
        dispatcher: body.dispatcher || "",
        dispatcherDecision: Boolean(body.dispatcher_beta),
        summary: body.summary || "",
        image: body.image || "",
        imageOptions: body.image_options || [],
        decision: body.decision || "",
        decisionOptions: body.decision_options || [],
        stt: body.stt || "",
        tts: body.tts || "",
      };
    }
  } catch (err) {
    console.error("modelRouting", err);
  }
  return { dispatcher: "", dispatcherDecision: false, summary: "", image: "", imageOptions: [], decision: "", decisionOptions: [], stt: "", tts: "" };
}

async function saveRouting(body) {
  try {
    const response = await fetch(`${API}/v1/model`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      modelError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveRouting", err);
    modelError(err.message || "failed");
  }
  renderModel();
}

function routingSelect(kind) {
  const dom = modelDom();
  return dom.routing ? dom.routing.querySelector(`select[data-kind="${kind}"]`) : null;
}

function fillRoutingSelect(kind, current, options, placeholder) {
  const select = routingSelect(kind);
  if (!select) {
    return;
  }
  select.innerHTML = "";
  select.appendChild(_("option", { value: "" }, placeholder));
  for (const name of options) {
    select.appendChild(_("option", { value: name }, name));
  }
  select.value = options.includes(current) ? current : "";
}

function fillDispatcherDecision(model, on) {
  const select = routingSelect("dispatcher");
  if (!select) {
    return;
  }
  const label = `Decision Model · ${model || "off"}`;
  const option = _("option", { value: DECISION_DISPATCHER }, label);
  option.disabled = !model;
  select.appendChild(option);
  if (on && model) {
    select.value = DECISION_DISPATCHER;
  }
}

async function fillAudioOptions(stt, tts) {
  let body = {};
  try {
    const response = await fetch(`${API}/v1/model/audio`);
    if (!response.ok) {
      return;
    }
    body = (await response.json()) || {};
  } catch (err) {
    console.error("fillAudioOptions", err);
    return;
  }

  fillRoutingSelect("stt", stt, body.stt_options || [], "off");
  fillRoutingSelect("tts", tts, body.tts_options || [], "off");
}

async function renderModelRouting(registered) {
  const dom = modelDom();
  if (!dom.routing) {
    return;
  }

  const routing = await modelRouting();
  if (modelView !== "add") {
    return;
  }

  fillRoutingSelect("dispatcher", routing.dispatcher, registered, "auto · first registered model");
  fillDispatcherDecision(routing.decision, routing.dispatcherDecision);
  fillRoutingSelect("summary", routing.summary, registered, "auto · first registered model");
  fillRoutingSelect("image", routing.image, routing.imageOptions, "off");
  fillRoutingSelect("decision", routing.decision, routing.decisionOptions, "off");
  fillAudioOptions(routing.stt, routing.tts);
}

function selectProvider(prefix) {
  window.location.href = getLink({ page: "config", tab: "Model", target: prefix });
}

function selectProviderAdd() {
  window.location.href = getLink({ page: "config", tab: "Model" });
}

function providerDetails(provider, method, added) {
  const head = _("div.head", [_("strong", provider.label)]);
  const pages = provider.console || [];
  if (method !== "oauth" && pages.includes("key")) {
    head.appendChild(
      _("p", [
        `Create one in the ${provider.label} `,
        _("a", { href: providerConsoleLink(provider.id, "key"), target: "_blank", rel: "noreferrer" }, "Console"),
        ".",
      ]),
    );
  }
  if (method === "oauth" && pages.includes("plan")) {
    head.appendChild(
      _("p", [
        `Subscribe on the ${provider.label} `,
        _("a", { href: providerConsoleLink(provider.id, "plan"), target: "_blank", rel: "noreferrer" }, "Plans"),
        " page.",
      ]),
    );
  }
  if (method === "custom") {
    head.appendChild(_("p", "Any OpenAI-compatible endpoint."));
  }
  const card = _("section.provider", [head, providerCredentialForm(provider, method, added)]);
  card.dataset.added = added ? "1" : "0";
  return card;
}

const MODEL_CUSTOM = ["cloudflare", "compat"];

function modelFilter() {
  const nav = $("#model-filter");
  const active = nav && nav.querySelector('button[data-selected="1"]');
  return active ? active.name : MODEL_GROUPS[0];
}

function setModelFilter(value) {
  const nav = $("#model-filter");
  if (!nav) {
    return;
  }
  for (const one of nav.querySelectorAll("button")) {
    if (one.name === value) {
      one.dataset.selected = "1";
    } else {
      delete one.dataset.selected;
    }
  }
}

function modelFilterChange(e) {
  const button = e && e.target ? e.target.closest("button") : null;
  if (!button || !button.name) {
    return;
  }
  setModelFilter(button.name);
  renderModel();
}

function matchModelFilter(provider, filter) {
  if (MODEL_CUSTOM.includes(provider.id)) {
    return filter === "custom";
  }
  return Object.keys(provider.methods || {}).includes(filter);
}

const MODEL_GROUPS = ["oauth", "api_key", "custom"];

function providerGroup(provider, method) {
  return MODEL_CUSTOM.includes(provider.id) ? "custom" : method;
}

function renderProviderCatalog(catalog, added) {
  const dom = modelDom();
  if (!dom.catalog) {
    return;
  }

  dom.catalog.innerHTML = "";
  dom.catalog.dataset.open = "1";

  const filter = modelFilter();
  const visible = catalog.filter((item) => !item.hidden && matchModelFilter(item, filter));

  for (const group of MODEL_GROUPS) {
    const list = visible.filter((item) => providerGroup(item, providerMethod(catalog, item.id)) === group);
    if (list.length === 0) {
      continue;
    }
    for (const provider of list) {
      const method = Object.keys(provider.methods || {})[0] || "";
      dom.catalog.appendChild(providerDetails(provider, method, added.includes(provider.id)));
    }
  }
}

function providerCredentialForm(provider, method, added) {
  const submitLabel = added ? "renew" : method === "oauth" ? "login" : "add";

  if (provider.id === "claude-code") {
    if (added) {
      return _("div.row.end");
    }
    const start = _("button.submit", { type: "button" }, "add");
    start.addEventListener("click", () => selectProvider(provider.id));
    return _("div.row.end", [start]);
  }

  if (method === "oauth") {
    if (modelOAuth.id === provider.id) {
      return _("div.row", [_("p", "waiting for the browser...")]);
    }
    const start = _("button.submit", { type: "button" }, submitLabel);
    start.addEventListener("click", () => startProviderOAuth(provider.id));
    return _("div.row.end", [start]);
  }

  if (method === "custom") {
    const name = _("input", { type: "text", placeholder: "Name, e.g. ollama" });
    const url = _("input", { type: "text", placeholder: "http://127.0.0.1:11434/v1" });
    const key = _("input", { type: "password", placeholder: "API key · optional" });
    const save = _("button.submit", { type: "button" }, submitLabel);
    save.addEventListener("click", () =>
      saveProviderKey(provider.id, { name: name.value.trim(), url: url.value.trim(), api_key: key.value.trim() }),
    );
    return _("div.field", [name, url, _("div.row", [key, save])]);
  }

  const key = _("input", { type: "password", placeholder: "API key" });

  if (provider.id === "cloudflare") {
    const account = _("input", { type: "text", placeholder: "Account ID" });
    const gateway = _("input", { type: "text", placeholder: "AI Gateway ID · optional" });
    const saveCf = _("button.submit", { type: "button" }, submitLabel);
    saveCf.addEventListener("click", () =>
      saveProviderKey(provider.id, {
        api_key: key.value.trim(),
        account_id: account.value.trim(),
        gateway_id: gateway.value.trim(),
      }),
    );
    return _("div.field", [key, account, _("div.row", [gateway, saveCf])]);
  }

  const save = _("button.submit", { type: "button" }, submitLabel);
  save.addEventListener("click", () => saveProviderKey(provider.id, { api_key: key.value.trim() }));
  return _("div.row", [key, save]);
}

async function saveProviderKey(id, body) {
  if (!body.api_key && id !== "compat") {
    modelError("api key is required");
    return;
  }
  if (id === "compat" && (!body.name || !body.url)) {
    modelError("name and url are required");
    return;
  }
  if (id === "cloudflare" && !body.account_id) {
    modelError("account ID is required for Cloudflare");
    return;
  }

  try {
    const response = await fetch(`${API}/v1/provider/${encodeURIComponent(id)}/key`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      modelError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("saveProviderKey", err);
    modelError(err.message || "failed");
    return;
  }

  if (id === "openai") {
    alert(
      "OpenAI key saved.\n\n" +
        "Vector storage needs a daemon restart before it works:\n" +
        "    agen stop && agen\n\n" +
        "Entries written before the restart keep no vector.",
    );
  }

  const prefix = id === "compat" ? body.name.toLowerCase() : id;
  delete providerProbe[prefix];
  providerQuota = null;
  if (decisionIds.includes(prefix)) {
    selectProviderAdd();
    return;
  }
  selectProvider(prefix);
}

function codeButton(code) {
  const button = _("button.code", { type: "button" }, [
    _("span.material-symbols-outlined", "content_copy"),
    _("p", `code: ${code}`),
  ]);
  button.addEventListener("click", () => {
    navigator.clipboard.writeText(code).catch((err) => console.error("codeButton", err));
    const label = button.querySelector("p");
    label.textContent = "copied";
    setTimeout(() => (label.textContent = `code: ${code}`), 1000);
  });
  return button;
}

function closeProviderOAuth() {
  if (modelStream) {
    modelStream.close();
    modelStream = null;
  }
  $("#oauth-popup")?.remove();
}

function openOAuthPopup(id) {
  $("#oauth-popup")?.remove();

  const cancel = _("button", { type: "button" }, "cancel");
  const open = _("button.submit", { type: "button" }, "open website");
  const root = _("div.popup", [
    _("div.panel", [
      _("strong", `Sign in · ${id}`),
      _("p", "enter this code in the browser"),
      codeButton(modelOAuth.code),
      _("footer", [cancel, open]),
    ]),
  ]);
  root.id = "oauth-popup";

  open.addEventListener("click", () => window.open(modelOAuth.url, "_blank", "noreferrer"));
  cancel.addEventListener("click", () => {
    closeProviderOAuth();
    modelOAuth.id = "";
    modelOAuth.code = "";
    renderModel();
  });

  document.body.appendChild(root);
}

function startProviderOAuth(id) {
  closeProviderOAuth();
  modelOAuth.id = id;
  modelOAuth.code = "";
  modelOAuth.url = "";
  modelView = "add";
  renderModel();

  modelStream = new EventSource(`${API}/v1/provider/${encodeURIComponent(id)}/oauth`);
  modelStream.onmessage = function (e) {
    let event = {};
    try {
      event = JSON.parse(e.data);
    } catch (err) {
      console.error("startProviderOAuth", err);
      return;
    }

    if (event.url) {
      modelOAuth.url = event.url;
      modelOAuth.code = event.user_code || "";
      renderModel();
      if (modelOAuth.code) {
        openOAuthPopup(id);
      } else {
        window.open(event.url, "_blank", "noreferrer");
      }
      return;
    }
    if (!event.done) {
      return;
    }

    closeProviderOAuth();
    modelOAuth.id = "";
    modelOAuth.code = "";
    if (!event.ok) {
      modelError(event.error || "oauth failed");
      renderModel();
      return;
    }
      delete providerProbe[id];
    providerQuota = null;
    selectProvider(id);
  };
  modelStream.onerror = function () {
    closeProviderOAuth();
    modelOAuth.id = "";
    modelOAuth.code = "";
    renderModel();
  };
}

function cancelModelFetch() {
  if (modelAbort) {
    modelAbort.abort();
    modelAbort = null;
  }
}

async function providerModelList(prefix) {
  const controller = new AbortController();
  cancelModelFetch();
  modelAbort = controller;

  try {
    const response = await fetch(`${API}/v1/provider/${encodeURIComponent(prefix)}/models`, {
      signal: controller.signal,
    });
    if (response.ok) {
      const body = await response.json();
      return { models: body.models || [], windows: body.windows || {} };
    }
    const detail = await response.json().catch(() => ({}));
    return { error: detail.error || `HTTP ${response.status}` };
  } catch (err) {
    if (err.name === "AbortError") {
      return { aborted: true };
    }
    console.error("providerModelList", err);
    return { error: err.message || "failed" };
  } finally {
    if (modelAbort === controller) {
      modelAbort = null;
    }
  }
}

async function saveProviderModels(prefix, models) {
  try {
    const response = await fetch(`${API}/v1/models`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prefix: prefix, models: models }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      modelError(detail.error || `HTTP ${response.status}`);
    }
  } catch (err) {
    console.error("saveProviderModels", err);
    modelError(err.message || "failed");
  }
  renderModel();
}

function tokenText(value) {
  if (value >= 1000000) {
    return `${Math.round(value / 1000000)}M`;
  }
  if (value >= 1000) {
    return `${Math.round(value / 1000)}K`;
  }
  return String(value);
}

function windowText(window) {
  if (!window) {
    return "";
  }
  const parts = [];
  if (window.in > 0) {
    parts.push(tokenText(window.in));
  }
  if (window.out > 0) {
    parts.push(tokenText(window.out));
  }
  return parts.join("/");
}

function modelRow(label, checked, onToggle, window) {
  const box = _("input", { type: "checkbox" });
  box.checked = checked;
  box.addEventListener("change", () => onToggle(box.checked));
  const body = [box, _("p", label)];
  const text = windowText(window);
  if (text) {
    body.push(_("span.window", text));
  }
  return _("label.tool", body);
}

async function renderProviderModels(prefix, registered) {
  const dom = modelDom();
  if (!dom.models || !prefix) {
    return;
  }

  const remove = _("button.remove", { type: "button" }, "delete");
  remove.addEventListener("click", () => deleteProvider(prefix));

  dom.models.innerHTML = "";
  dom.models.dataset.open = "1";
  dom.models.appendChild(_("div.row", [_("strong", `${prefix} · pick the models this agent can use`), remove]));

  const probe = await probeProvider(prefix);
  if (probe.aborted) {
    return;
  }

  if (!probe.ok) {
    const catalog = await providerCatalog();
    const failure = providerFailure(prefix, providerMethod(catalog, prefix), probe.error, providerConsolePages(catalog, prefix));

    dom.models.appendChild(_("p.empty", failure.text));

    const actions = [];
    if (failure.login) {
      const login = _("button.submit", { type: "button" }, "sign in again");
      login.addEventListener("click", () => startProviderOAuth(prefix));
      actions.push(login);
    }
    if (failure.remove) {
      const remove = _("button.remove", { type: "button" }, "delete key");
      remove.addEventListener("click", () => removeProviderKey(prefix));
      actions.push(remove);
    }
    if (failure.link) {
      actions.push(_("a.submit", { href: failure.link, target: "_blank", rel: "noreferrer" }, failure.linkLabel));
    }
    if (actions.length > 0) {
      dom.models.appendChild(_("div.row", [_("p", "")].concat(actions)));
    }

    return;
  }
  const available = probe.models;

  const active = (registered || []).filter((id) => modelPrefix(id) === prefix).map((id) => id.slice(prefix.length + 1));
  const names = available.slice().sort();
  for (const name of active) {
    if (!names.includes(name)) {
      names.push(name);
    }
  }

  if (names.length === 0) {
    dom.models.appendChild(_("p.empty", "no models returned by this provider"));
    return;
  }

  const filter = _("input", { type: "text", placeholder: `Filter ${names.length} models` });
  filter.addEventListener("input", () => {
    const query = filter.value.trim().toLowerCase();
    for (const row of dom.models.querySelectorAll("label.tool")) {
      row.style.display = row.textContent.toLowerCase().includes(query) ? "" : "none";
    }
  });
  dom.models.appendChild(filter);

  for (const name of names) {
    dom.models.appendChild(
      modelRow(
        name,
        active.includes(name),
        (on) => {
          const next = on ? active.concat([name]) : active.filter((item) => item !== name);
          saveProviderModels(prefix, next);
        },
        (probe.windows || {})[name],
      ),
    );
  }
}

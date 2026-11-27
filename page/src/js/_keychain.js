let keychainEditing = "";

function keychainDom() {
  return {
    form: $("#keychain-form"),
    list: $("#keychain-list"),
    name: $("#keychain-name"),
    value: $("#keychain-value"),
    submit: document.querySelector("#keychain-form button.submit"),
  };
}

function keychainError(text) {
  alert(text);
}

async function keychainKeys() {
  try {
    const response = await fetch(`${API}/v1/keys`);
    if (response.ok) {
      return (await response.json()).keys || [];
    }
  } catch (err) {
    console.error("keychainKeys", err);
  }
  return [];
}

async function renderKeychain() {
  const dom = keychainDom();
  if (!dom.list) {
    return;
  }

  const keys = await keychainKeys();
  const picked = praseURL().target || "";

  dom.list.innerHTML = "";

  for (const key of keys) {
    if (!key) continue;

    const remove = _("button", { type: "button" }, [_("span.material-symbols-outlined", "delete")]);
    remove.addEventListener("click", (e) => {
      e.stopPropagation();
      deleteKeychain(key);
    });

    const card = _("div.card", [_("strong", key), _("p", i18n("text_stored")), remove]);
    card.dataset.name = key;
    card.dataset.selected = key === picked ? "1" : "0";
    card.addEventListener("click", () => {
      window.location.href = getLink({ page: "config", tab: "Keychain", target: key });
    });
    dom.list.appendChild(card);
  }

  if (picked && keys.includes(picked)) {
    openKeychain(picked);
  }
}

function openKeychain(key) {
  const dom = keychainDom();
  if (!dom.form) {
    return;
  }

  dom.name.value = key;
  dom.name.readOnly = true;
  dom.value.value = "";
  dom.value.placeholder = i18n("placeholder_value_replace");
  dom.form.dataset.editing = "1";
  if (dom.submit) {
    dom.submit.textContent = i18n("text_save");
  }
  keychainEditing = key;
}

function resetKeychain() {
  const dom = keychainDom();
  if (!dom.form) {
    return;
  }

  dom.name.value = "";
  dom.name.readOnly = false;
  dom.value.value = "";
  dom.value.placeholder = i18n("placeholder_value");
  delete dom.form.dataset.editing;
  if (dom.submit) {
    dom.submit.textContent = i18n("text_add");
  }
  keychainEditing = "";
  markSelectedCard(dom.list, "");
}

async function saveKeychain() {
  const dom = keychainDom();
  if (!dom.form) {
    return;
  }

  const key = dom.name.value.trim();
  const value = dom.value.value.trim();
  if (!key) {
    keychainError(i18n("error_key_required"));
    return;
  }
  if (!value) {
    keychainError(keychainEditing ? i18n("error_new_value_required") : i18n("error_value_required"));
    return;
  }

  try {
    const response = await fetch(`${API}/v1/keys`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: key, value: value }),
    });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      keychainError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("saveKeychain", err);
    keychainError(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "config", tab: "Keychain", target: key });
}

async function deleteKeychain(key) {
  if (!key || !confirm(i18n("confirm_delete", key))) {
    return;
  }

  try {
    const response = await fetch(`${API}/v1/key?key=${encodeURIComponent(key)}`, { method: "DELETE" });
    if (!response.ok) {
      const detail = await response.json().catch(() => ({}));
      keychainError(detail.error || `HTTP ${response.status}`);
      return;
    }
  } catch (err) {
    console.error("deleteKeychain", err);
    keychainError(err.message || i18n("text_failed"));
    return;
  }

  window.location.href = getLink({ page: "config", tab: "Keychain" });
}

function deleteEditingKeychain() {
  if (keychainEditing) {
    deleteKeychain(keychainEditing);
  }
}

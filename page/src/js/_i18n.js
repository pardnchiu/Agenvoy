let I18N = {};
let I18N_LANG = "en";

async function loadI18n() {
  try {
    const response = await fetch(`${API}/v1/i18n?lang=${encodeURIComponent(navigator.language || "")}`);
    if (response.ok) {
      const body = await response.json();
      I18N = body.dict || {};
      I18N_LANG = body.lang || "en";
    }
  } catch (err) {
    console.error("loadI18n", err);
  }
  document.documentElement.lang = I18N_LANG;
}

function i18n(key, ...args) {
  let text = I18N[key];
  if (typeof text !== "string") {
    return key;
  }
  const parts = text.split("%s");
  return parts.reduce((out, part, i) => out + (i <= args.length && i > 0 ? String(args[i - 1]) : i > 0 ? "%s" : "") + part, "");
}

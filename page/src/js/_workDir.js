async function openWorkDirPrompt() {
  const current = readChatConfig(currentSessionId).work_dir;
  const input = await workDirPopup(current);
  if (input === null) {
    return;
  }

  const path = input.trim();
  if (path === "") {
    writeChatConfig(currentSessionId, { work_dir: "" });
    markWorkDir("");
    return;
  }
  if (!path.startsWith("/") && !path.startsWith("~/")) {
    alert(i18n("error_absolute_only"));
    return;
  }

  let error = "";
  try {
    const response = await fetch(`${API}/v1/workdir?path=${encodeURIComponent(path)}`);
    if (!response.ok) {
      error = ((await response.json().catch(() => ({}))).error || `HTTP ${response.status}`).toString();
    }
  } catch (err) {
    error = err.message || i18n("error_unreachable");
  }
  if (error) {
    alert(error);
    return;
  }

  writeChatConfig(currentSessionId, { work_dir: path });
  markWorkDir(path);
}

function workDirPopup(value) {
  return new Promise(function (resolve) {
    const box = _("textarea", { placeholder: i18n("placeholder_path") });
    box.value = value || "";

    const mirror = _("pre");
    mirror.textContent = box.value + "\n";

    const cancel = _("button", { type: "button" }, i18n("text_cancel"));
    const save = _("button", { type: "button", class: "submit" }, i18n("text_save"));

    const root = _("div.popup", [
      _("div.panel", [
        _("strong", i18n("tooltip_working_directory")),
        _("p", i18n("error_absolute_only")),
        _("label.input", [box, mirror]),
        _("footer", [cancel, save]),
      ]),
    ]);
    root.id = "workdir-popup";

    let settled = false;
    const close = function (result) {
      if (settled) {
        return;
      }
      settled = true;
      document.removeEventListener("keydown", escape);
      root.remove();
      resolve(result);
    };
    const escape = function (e) {
      if (e.key === "Escape") {
        close(null);
      }
    };

    box.addEventListener("input", function () {
      mirror.textContent = box.value + "\n";
    });
    box.addEventListener("keydown", function (e) {
      if (e.key !== "Enter" || e.shiftKey || e.isComposing) {
        return;
      }
      e.preventDefault();
      close(box.value);
    });
    cancel.addEventListener("click", function () {
      close(null);
    });
    save.addEventListener("click", function () {
      close(box.value);
    });
    root.addEventListener("click", function (e) {
      if (e.target === root) {
        close(null);
      }
    });
    document.addEventListener("keydown", escape);

    document.body.appendChild(root);
    box.focus();
    box.select();
  });
}

function markWorkDir(path) {
  const dom = $("section.chat button.work-dir");
  if (!dom) {
    return;
  }
  if (path) {
    dom.dataset.selected = "1";
    dom.title = path;
    return;
  }
  delete dom.dataset.selected;
  dom.removeAttribute("title");
}

function renderWorkDirMark() {
  markWorkDir(readChatConfig(currentSessionId).work_dir);
}

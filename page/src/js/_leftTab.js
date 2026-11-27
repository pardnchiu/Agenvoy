const feature = {
  Roles: { icon: "contract" },
  Schedule: { icon: "schedule" },
  Skills: { icon: "lightbulb_2" },
  Hooks: { icon: "webhook" },
};

const configTab = {
  Model: { icon: "deployed_code" },
  MCP: { icon: "electrical_services" },
  Keychain: { icon: "key" },
  System: { icon: "tune" },
};

const monitorTab = {
  Usage: { icon: "bar_chart" },
  History: { icon: "history" },
  Lessons: { icon: "cognition_2" },
  Daemon: { icon: "terminal" },
};

const usageTab = {
  "24h": "schedule",
  "7d": "schedule",
  "28d": "schedule",
};

const leftTab = {
  "New Chat": {
    icon: "add_comment",
    href: getLink({ page: "chat" }),
  },
};

for (const name of ["Roles", "Schedule"]) {
  leftTab[name] = {
    icon: feature[name].icon,
    href: getLink({ page: "features", tab: name }),
  };
}

function localizeTabs() {
  const labels = {
    "New Chat": i18n("label_new_chat"),
    Roles: i18n("text_roles"),
    Schedule: i18n("text_schedule"),
    Skills: i18n("text_skills"),
    Hooks: i18n("text_hooks"),
    Model: i18n("text_model"),
    Keychain: i18n("text_keychain"),
    System: i18n("text_system"),
    Usage: i18n("text_usage"),
    History: i18n("text_history"),
    Lessons: i18n("text_lessons"),
    Daemon: i18n("text_daemon"),
  };
  for (const tabs of [leftTab, feature, configTab, monitorTab]) {
    for (const name of Object.keys(tabs)) {
      tabs[name].label = labels[name] || name;
    }
  }
}

function markLeftTab(params) {
  const name =
    params.page === "features" ? params.tab : params.page === "monitor" ? "Monitor" : params.page === "config" ? "Config" : "";
  document
    .querySelectorAll("section.left-tab > section:not(.chats) > a[name], section.left-tab > footer > a[name]")
    .forEach((link) => {
      link.dataset.selected = name && link.dataset.tab === name ? "1" : "0";
    });
}

function getLink(params) {
  let path = "?";
  if (params.page) {
    path += `page=${params.page}`;
  }
  if (params.tab) {
    path += `&tab=${params.tab}`;
  }
  if (params.chat) {
    path += `&chat=${params.chat}`;
  }
  if (params.target) {
    path += `&target=${encodeURIComponent(params.target)}`;
  }
  if (params.offset) {
    path += `&offset=${params.offset}`;
  }
  if (params.hash) {
    path += `&hash=${encodeURIComponent(params.hash)}`;
  }
  if (params.item) {
    path += `&item=${encodeURIComponent(params.item)}`;
  }
  if (params.keyword) {
    path += `&keyword=${encodeURIComponent(params.keyword)}`;
  }
  if (params.from) {
    path += `&from=${params.from}`;
  }
  if (params.to) {
    path += `&to=${params.to}`;
  }
  return path;
}

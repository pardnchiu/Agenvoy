const feature = {
  Roles: "contract",
  Schedule: "schedule",
  Skills: "lightbulb_2",
  Hooks: "webhook",
};

const configTab = {
  Model: "deployed_code",
  MCP: "electrical_services",
  Keychain: "key",
  System: "tune",
};

const monitorTab = {
  Usage: "bar_chart",
  History: "history",
  Lessons: "cognition_2",
  Daemon: "terminal",
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

for (const name of Object.keys(feature)) {
  leftTab[name] = {
    icon: feature[name],
    href: getLink({ page: "features", tab: name }),
  };
}

function markLeftTab(params) {
  const name =
    params.page === "features" ? params.tab : params.page === "monitor" ? "Monitor" : params.page === "config" ? "Config" : "";
  document
    .querySelectorAll("section.left-tab > section:not(.chats) > a[name], section.left-tab > footer > a[name]")
    .forEach((link) => {
      link.dataset.selected = name && link.getAttribute("name") === name ? "1" : "0";
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

document.addEventListener("DOMContentLoaded", async function () {
  revealIconsWhenReady();
  watchServer();
  watchUpdate();

  const config = readConfig();
  let params = praseURL();

  if (params.page == null) {
    params.page = "chat";
  }

  if (params.page == "config") {
    const tabs = Object.keys(configTab);
    const matched = tabs.find((name) => name.toLowerCase() === String(params.tab || "").toLowerCase());
    params.tab = matched || tabs[0];
  }

  if (params.page == "monitor") {
    const tabs = Object.keys(monitorTab).concat(["Details"]);
    const matched = tabs.find((name) => name.toLowerCase() === String(params.tab || "").toLowerCase());
    params.tab = matched || tabs[0];

    const periods = Object.keys(usageTab);
    const period = periods.find((name) => name.toLowerCase() === String(params.period || "").toLowerCase());
    params.period = period || periods[0];
  }

  if (params.chat) {
    if (!SESSION_ID.test(params.chat) || !(await sessionExists(params.chat))) {
      window.location.href = getLink({ page: params.page, tab: params.tab });
      return;
    }
  }
  params.chat = params.chat || "";

  const models = await fetchModels();
  const hasModel = hasRealModel(models);
  if (hasModel) {
    await prunePinChat(config);
  }
  setPinChats(hasModel || models === null ? config.pin_chat.slice() : []);

  const pinnedChats = pinChats();
  if (pinnedChats.includes(params.chat)) {
    window.location.href = getLink({ page: params.page });
    return;
  }
  const voiceDisabled = pinnedChats.length > 0;

  console.log("config", config);
  console.log("params", params);

  function submit(from) {
    const panel = from && from.closest ? from.closest("section.chat > *") : null;
    const dom = panel ? panel.querySelector("div.input textarea") : $("#chat-input");
    if (!dom) {
      return;
    }
    const content = dom.value;
    dom.value = "";
    dom.nextElementSibling.textContent = "\n";
    send(content, panelSession(dom));
  }

  function wheelDelta(e, target) {
    if (e.deltaMode === 1) return e.deltaY * 16;
    if (e.deltaMode === 2) return e.deltaY * target.clientHeight;
    return e.deltaY;
  }

  const app = new QUI({
    id: "app",
    data: {
      params: params,
      pin: pinnedChats.map((id) => ({ id: id })),
      collapsed: isWide() ? config.left_tab_collapsed : "1",
      left_tab: leftTab,
      feature: feature,
      configTab: configTab,
      monitorTab: monitorTab,
      usageTab: usageTab,
    },
    event: {
      show_tab: function () {
        const dom = $(".left-tab");
        const collapsed = dom.dataset.collapsed === "1" ? "0" : "1";
        dom.dataset.collapsed = collapsed;
        if (!isWide()) {
          return;
        }
        config.left_tab_collapsed = collapsed;
        writeConfig(config);
      },
      feature_tab_click: function (e) {
        const header = e.target.closest("header");
        const name = e.target.name;
        header.dataset.selected = name;
      },
      chat_input: function () {
        this.nextElementSibling.textContent = this.value + "\n";
        renderSendMode(panelSession(this));
      },
      chat_keydown: function (e) {
        if (e.key !== "Enter" || e.shiftKey || e.isComposing) {
          return;
        }
        e.preventDefault();
        submit(this);
      },
      send_click: function () {
        if (this.dataset.mode === "stop") {
          const sid = panelSession(this) || currentSessionId;
          if (pendingTask && pendingTask.sessionId === sid) {
            cancelPending(sid);
          } else {
            stopRunning(sid);
          }
          return;
        }
        submit(this);
      },
      harness_click: function (e) {
        if (voiceDisabled) return;

        const dom = e.target.closest("button");
        if (!dom) return;

        const on = dom.dataset.selected == null;
        const mark = function (state) {
          if (state) {
            dom.dataset.selected = "1";
          } else {
            delete dom.dataset.selected;
          }
          config.harness_enable = state;
          writeConfig(config);
        };

        mark(on);
        toggleVoice(on).catch(function () {
          mark(false);
        });
      },
      chat_wheel: function (e) {
        const dom = chatMessages();
        if (dom.contains(e.target) || dom.scrollHeight <= dom.clientHeight) {
          return;
        }

        const box = e.target.closest("textarea");
        if (box && box.scrollHeight > box.clientHeight) {
          return;
        }
        dom.scrollTop += wheelDelta(e, dom);
        e.preventDefault();
      },
      workdir_pick: function () {
        openWorkDirPrompt();
      },
      persona_edit: function () {
        openPersonaPopup(panelSession(this));
      },
      usage_open: function () {
        window.location.href = usageLink("24h", panelSession(this) || currentSessionId);
      },
      history_open: function () {
        window.location.href = historyLink(panelSession(this) || currentSessionId, 0);
      },
      history_range: function () {
        rangeSubmit("History", "history");
      },
      history_keydown: function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          rangeSubmit("History", "history");
        }
      },
      lesson_range: function () {
        rangeSubmit("Lessons", "lesson");
      },
      lesson_keydown: function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          rangeSubmit("Lessons", "lesson");
        }
      },
      skill_pick: function () {
        openSkillPicker();
      },
      role_save: function () {
        saveRole();
      },
      role_reset: function () {
        resetRole();
      },
      role_delete: function () {
        deleteEditingRole();
      },
      schedule_save: function () {
        commitSchedule();
      },
      schedule_reset: function () {
        resetSchedule();
      },
      schedule_delete: function () {
        deleteEditingSchedule();
      },
      schedule_test: function () {
        testSchedule();
      },
      skill_source: function () {
        openSkillSource();
      },
      skill_source_save: function () {
        saveSkillSource();
      },
      skill_config: function () {
        openSkillConfig();
      },
      skill_delete: function () {
        deleteSkillTab();
      },
      skill_open: function () {
        openSkillFolder();
      },
      role_pick: function () {
        openRolePicker();
      },
      model_pick: function () {
        openModelPicker();
      },
      reasoning_pick: function () {
        openReasoningPicker();
      },
      memory_pick: function () {
        openMemoryPicker(panelSession(this) || currentSessionId);
      },
      task_pick: function () {
        openTaskPicker(panelSession(this) || currentSessionId);
      },
      pin_add: function () {
        addPinChat(panelSession(this));
      },
      pin_remove: function () {
        removePinChat(panelSession(this));
      },
      model_add: function () {
        selectProviderAdd();
      },
      model_filter: function (e) {
        modelFilterChange(e);
      },
      daemon_range: function () {
        daemonSubmit();
      },
      daemon_keydown: function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          daemonSubmit();
        }
      },
      mcp_reset: function () {
        resetMcp();
      },
      mcp_save: function () {
        saveMcp();
      },
      mcp_delete: function () {
        deleteEditingMcp();
      },
      mcp_reconnect: function () {
        reconnectMcp();
      },
      mcp_login: function () {
        startMcpLogin(mcpEditing);
      },
      mcp_logout: function () {
        clearMcpOAuth(mcpEditing);
      },
      mcp_transport: function (e) {
        mcpTransportChange(e);
      },
      mcp_auth: function () {
        mcpAuthChange();
      },
      keychain_reset: function () {
        resetKeychain();
      },
      keychain_save: function () {
        saveKeychain();
      },
      keychain_delete: function () {
        deleteEditingKeychain();
      },
      admin_channel: function () {
        saveAdminChannel(this.value);
      },
      routing_change: function () {
        saveRoutingModel(this.dataset.kind, this.value);
      },
      dispatcher_beta_toggle: function () {
        saveTypesafeToggle("dispatcher_beta", this.dataset.on !== "1");
      },
      system_lang: function () {
        saveSystemLang();
      },
      system_startup: function () {
        saveSystemStartup();
      },
      system_official_guide: function () {
        saveSystemOfficialGuide();
      },
      system_output: function () {
        saveSystemOutput();
      },
      system_output_open: function () {
        openOutputDir();
      },
      system_update: function () {
        runSystemUpdate();
      },
      system_output_keydown: function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          saveSystemOutput();
        }
      },
    },
    when: {
      before_render: function () {
        // 停止渲染
      },
      rendered: function () {
        setTimeout(() => {
          document.body.dataset.rendered = "1";
        }, 0);

        let lastWidth = document.documentElement.clientWidth;
        window.addEventListener("resize", function (e) {
          const vw = document.documentElement.clientWidth;
          if (vw === lastWidth) {
            return;
          }
          lastWidth = vw;
          const dom = $(".left-tab");

          let timer;
          clearTimeout(timer);
          document.body.dataset.rendered = "0";
          timer = setTimeout(() => {
            document.body.dataset.rendered = "1";
            clearTimeout(timer);
          }, 500);

          if (dom.dataset.collapsed === "1" || vw >= LEFT_TAB_WIDE) {
            return;
          }
          dom.dataset.collapsed = "1";
        });

        if (!(params.page === "chat" && params.chat)) {
          subscribe("");
        }
        bindSelectPicker();
        bindFileLink();
        bindCodeCopy();
        bindInputDrop();
        renderChatList();

        if (params.page === "chat") {
          if (!params.chat) {
            clearChatDraft();
          }
          setSession(params.chat);
          subscribe(params.chat);
          getModelList(params.chat, models);
          getReasoningList(params.chat);
          getRoleList();
          renderWorkDirMark();
          renderChat(params.chat);
          renderPendingHint(params.chat);
          renderTaskButton(params.chat);

          for (const pinned of pinnedChats) {
            renderChat(pinned);
            renderPendingHint(pinned);
            renderTaskButton(pinned);
          }

          if (!voiceDisabled && config.harness_enable) {
            const harness = $("section.chat button.harness");
            if (harness) {
              harness.dataset.selected = "1";
            }
            initVoice().catch(function (err) {
              console.error("voice: start deferred to the first gesture", err?.message || err);
              document.addEventListener(
                "pointerdown",
                function () {
                  initVoice().catch(function (retry) {
                    console.error("voice", retry?.message || retry);
                  });
                },
                { once: true },
              );
            });
          }
        }

        if (params.page === "config") {
          if (params.tab === "Model") {
            renderModel();
          }
          if (params.tab === "MCP") {
            resetMcp();
            renderMcp();
          }
          if (params.tab === "Keychain") {
            resetKeychain();
            renderKeychain();
          }
          if (params.tab === "System") {
            renderChannel();
            renderSystem();
          }
        }

        if (params.page === "monitor") {
          if (params.tab === "Usage") {
            renderUsagePage(params.chat);
          }
          if (params.tab === "History") {
            renderHistoryPage(params.target || "", Number(params.offset) || 0);
          }
          if (params.tab === "Lessons") {
            renderLessonPage(params.target || "", Number(params.offset) || 0);
          }
          if (params.tab === "Details") {
            renderDetailsPage(params.target || "", params.hash || "", params.item || "");
          }
          if (params.tab === "Daemon") {
            renderDaemonPage();
          }
        }

        if (params.page === "features") {
          if (params.tab === "Roles") {
            resetRole();
            renderRole();
          }
          if (params.tab === "Skills") {
            renderSkillTab();
            if (!params.target) {
              openSkillConfig();
            }
          }
          if (params.tab === "Schedule") {
            resetSchedule();
            renderSchedule();
          }
        }
      },
      before_update: function () {
        // 停止更新
      },
      updated: function () {
        // 已更新
      },
      before_destroy: function () {
        // 停止銷毀
      },
      destroyed: function () {
        // 已銷毀
      },
    },
  });
});

function revealIconsWhenReady() {
  const reveal = () => {
    document.body.dataset.icons = "1";
  };
  if (!document.fonts) {
    reveal();
    return;
  }
  // * detection can never leave the icons invisible: a stalled font load still reveals
  setTimeout(reveal, 3000);
  document.fonts.ready
    .then(() => document.fonts.load('24px "Material Symbols Outlined"'))
    .finally(reveal);
}

if ("serviceWorker" in navigator) {
  window.addEventListener("load", function () {
    navigator.serviceWorker.register("/sw.js").catch(function (err) {
      console.error(err);
    });
  });
}

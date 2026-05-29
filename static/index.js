document.addEventListener("DOMContentLoaded", (_) => {
  const url = new URL(location.href);
  const currentLanguage = navigator.language || navigator.userLanguage;
  const lang = url.searchParams.get("lang");
  let isZh = /^zh/i.test(currentLanguage);

  if (lang != null) {
    isZh = /^zh/i.test(lang);
  }

  const page = new QUI({
    id: "page",
    i18n: {
      zh: {
        title: "讓 AI 真正為你工作的個人 AI 助理",
        // 區塊1
        one_command_title: "一鍵部署好簡單",
        one_command: "複製、貼上、不到 30 秒",
        one_command_footer: "還要 Discord / Telegram? 再加十秒",
        // 區塊2
        scheduler_title: "以 SKILL 為基底的排程",
        scheduler_content: "與 SKILL 相同邏輯文檔撰寫，輕鬆設計自己的排程工作流。",
        // 區塊3
        co_work_title: "多 Session 協同工作",
        co_work_content: "多個 Sessions 之間可互相調用對方分配任務，協同完成複雜任務。",
        // 區塊4
        tool_generator_title: "工具自動生成",
        tool_generator_content: "只需提出需求，並依據問題回答，即可自動生成工具，無需編碼。",
        // 區塊5
        plan_mode_title: "預規劃模式",
        plan_mode_content: "只需提出需求，並依據問題回答，Agent 會依據需求自動規劃任務流程後再開始執行。",
        // 區塊6
        extension_market_title: "工具擴充市集",
        extension_market_content: "社群開發的工具擴充，經過驗證後即可上架，讓你一鍵安裝使用。",
      },
      en: {
        title: "Your Personal AI Assistant That Actually Works",
        // 區塊1
        one_command_title: "One-Command Deployment Made Easy",
        one_command: "Copy, paste, and you're done in under 30 seconds",
        one_command_footer: "Need Discord / Telegram? Just add ten more seconds",
        // 區塊2
        scheduler_title: "SKILL-based Scheduler",
        scheduler_content: "Same logic document writing as SKILL, easily design your own scheduling workflow.",
        // 區塊3
        co_work_title: "Multi-Session Co-Working",
        co_work_content:
          "Multiple sessions can call each other to assign tasks and work together to complete complex tasks.",
        // 區塊4
        tool_generator_title: "Automatic Tool Generation",
        tool_generator_content:
          "Just ask for what you need, and the tool will be automatically generated based on the answer, no coding required.",
        // 區塊5
        plan_mode_title: "Pre-Planning Mode",
        plan_mode_content:
          "Just ask for what you need, and the Agent will automatically plan the task flow based on the requirements before starting execution.",
        // 區塊6
        extension_market_title: "Tool Extension Marketplace",
        extension_market_content:
          "Community-developed tool extensions, verified and listed for you to install and use with one click.",
      },
    },
    i18nLang: isZh ? "zh" : "en",
    data: {
      lang: isZh ? "zh" : "en",
      change: isZh ? "en" : "zh",
    },
    when: {
      rendered: (_) => {
        record = {
          "header-video": "7M6y5BO0kzo",
          "demo-scheduler": "bO9AMrW3L9c",
          "demo-co-work": "wM3NU4ARz4w",
          "demo-tool-generator": "wF3_q-iqsgg",
          "demo-plan-mode": "05rri8gNuTM",
          "demo-extension-market": "UrR5i7YAHRc",
        };
        i = 0;
        for (const e of Object.keys(record)) {
          i++;
          setTimeout(() => {
            const dom1 = new FPlyr({
              id: e,
              youtube: record[e],
              option: {
                panelType: "minimal",
              },
            });
          }, i * 300);
        }
      },
    },
  });
});

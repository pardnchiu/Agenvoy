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
        tagline_1: "一句話建工具，自動測試，直接呼叫。",
        tagline_2: "讓你現有的 Agent 也能自己造工具。",
        tagline_3: "你只需要說一句話，剩下的交給 Agent。",
        one_command_title: "一鍵安裝",
        one_command: "複製、貼上、不到 30 秒",
        one_command_footer: "還要 Discord / Telegram? 再加十秒",
        lookup_title: "查資料，沒工具就自己建",
        lookup_content: "問一句話，Agent 找資料、呼叫工具、給你答案。工具不存在？它自己建一個。",
        scheduler_title: "排程自動化",
        scheduler_content:
          "「每天早上八點報告台積電股價」——Agent 問你推送到哪、要什麼格式、何時執行，然後自動建立排程。",
        search_title: "搜尋你的檔案",
        search_content:
          "「找出去年所有發票」「哪份文件提到 Prompt guide？」——Agent 搜尋本地檔案，直接回答。",
        multistep_title: "多步驟任務",
        multistep_content:
          "「總結今天的 GitHub Commit 並生成進度報告」——Agent 拆解任務、呼叫工具、整合結果、回覆你。",
        mcp_title: "讓其他 Agent 也能造工具",
        mcp_content:
          "Agenvoy 也是 MCP server。Claude Code、Codex、OpenCode 連上後即可使用所有沙箱工具、自動建立新工具、跨 Agent 共享。一行設定，即時共享工具庫。",
      },
      en: {
        title: "Your Personal AI Assistant That Actually Works",
        tagline_1: "Build tools, test it, and call it.",
        tagline_2: "Give the Agent you already use the power to build its own tools.",
        tagline_3: "You say one sentence. The agent does the rest.",
        one_command_title: "One-Line Install",
        one_command: "Copy, paste, done in under 30 seconds",
        one_command_footer: "Need Discord / Telegram? Just add ten more seconds",
        lookup_title: "Look Things Up — No Tool? It Builds One",
        lookup_content:
          "Ask a question. The agent finds data, calls tools, and gives you the answer. If a tool doesn't exist, it builds one.",
        scheduler_title: "Set Up Automation",
        scheduler_content:
          '"Report TSMC stock price every morning at 8am" — The agent asks where to push, what format, when to run, then creates the schedule.',
        search_title: "Search Your Files",
        search_content:
          '"Find all invoices from last year" "Which document mentions Prompt guide?" — The agent searches your local files and answers directly.',
        multistep_title: "Handle Multi-Step Work",
        multistep_content:
          '"Summarize today\'s GitHub Commit and generate a progress report" — The agent breaks down the task, calls tools, combines results, and replies.',
        mcp_title: "Let Other Agents Build Tools Too",
        mcp_content:
          "Agenvoy is also an MCP server. Claude Code, Codex, OpenCode and other AI agents can connect to use all sandboxed tools, auto-build new tools, and share across agents. One line of config. Instant shared tool library.",
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
          "demo-lookup": "floMBsAfziY",
          "demo-scheduler": "5To3joKlFpU",
          "demo-search": "vqoQ6Qvl8qU",
          "demo-multistep": "nIV1xz_HIJg",
          "demo-mcp": "on5IaoxBO1E",
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

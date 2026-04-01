# Agenvoy — 架構參考

七張 Mermaid 圖，涵蓋從進入點到各子系統的完整系統結構。

## 1. 系統概覽

所有主要子系統的高層資料流。

```mermaid
graph TB
    subgraph Entry ["進入點"]
        App["cmd/app · TUI 管理介面"]
        subgraph Managed ["由 cmd/app 統一管理"]
            CLI["cmd/cli · will deprecate"]
            Discord["Discord Bot"]
            API["REST API · /v1/send · /v1/tools · /v1/tool/:name · /v1/key"]
        end
    end

    subgraph Engine ["執行引擎"]
        Run["exec.Run()"]
        Execute["exec.Execute()\n≤128 次迭代"]
    end

    subgraph Providers ["LLM Providers"]
        P["Copilot · OpenAI · Claude\nGemini · Nvidia · Compat"]
    end

    subgraph Security ["安全層"]
        S["沙箱 · 敏感路徑封鎖 · Keychain"]
    end

    subgraph Tools ["工具子系統"]
        T["檔案 · 網路 · API · Script\n排程器 · 錯誤記憶"]
    end

    subgraph Persistence ["持久化"]
        PS["Session 摘要 · 對話歷史"]
    end

    App --> CLI
    App --> Discord
    App --> API
    CLI --> Run
    Discord --> Run
    API --> Run
    Run --> Execute
    Execute -->|"Agent.Send()"| Providers
    Execute -->|"tool calls"| Security
    Security --> Tools
    Tools -->|"results"| Execute
    Execute --> Persistence
    Persistence -.->|"注入"| Execute
```

---

## 2. 執行引擎

`exec.Run()` 的內部流程：Skill/Agent 選擇、Token 裁剪，及工具呼叫迭代迴圈。

```mermaid
flowchart TD
    Run["exec.Run()"]

    subgraph Selection ["選擇階段"]
        SkillScan["SelectSkill()\n9 個掃描路徑（依優先順序）"]
        AgentScan["SelectAgent()\nPlanner LLM 挑選最適 Provider"]
    end

    subgraph Loop ["迭代迴圈 · exec.Execute()"]
        Assemble["assembleMessages()\n四個固定段：\nSystemPrompts · OldHistories · UserInput · ToolHistories"]
        ReactTrim{"Context 超限？"}
        TrimOld["裁剪 OldHistories\n或 ToolHistories\n（錯誤觸發，reactive）"]
        Send["Agent.Send()\n統一 Provider 介面"]
        Parse["解析回應\n擷取 tool_calls"]
        Dispatch["Dispatch tool calls\n並行執行"]
        Dedup["Hash 去重\n防止相同呼叫重複"]
        Accum["累積結果\n附加至訊息歷史"]
        Check{"停止條件？\n無 tool_calls 或\n迭代次數 ≥ 128"}
    end

    Done["回傳最終回應"]

    Run --> SkillScan
    Run --> AgentScan
    SkillScan -->|"注入 Skill prompt"| Loop
    AgentScan -->|"Provider 已選定"| Loop
    Assemble --> ReactTrim
    ReactTrim -->|"是"| TrimOld --> Assemble
    ReactTrim -->|"否"| Send
    Send --> Parse
    Parse --> Dispatch
    Dispatch --> Dedup
    Dedup --> Accum
    Accum --> Check
    Check -->|"繼續"| Assemble
    Check -->|"完成"| Done
```

---

## 3. Provider 路由

Planner LLM 如何選擇 Provider，以及各後端如何處理請求。

```mermaid
flowchart TD
    Planner["Planner LLM\n（SelectAgent）\n依任務類型評分各 Provider"]

    subgraph Providers ["Provider 後端"]
        Copilot["Copilot\ngithub.com token 認證\n401 自動重新登入"]
        OpenAI["OpenAI\nno_temperature 旗標\n（推理模型用）"]
        Claude["Claude\n多段 System Prompt 合併"]
        Gemini["Gemini\n多部分訊息修正"]
        Nvidia["Nvidia NIM\nOpenAI 相容協定"]
        Compat["Compat\nOllama / 任意 OpenAI 端點\n具名 compat[{name}] 實例"]
    end

    subgraph CopilotRouting ["Copilot 雙協定路由"]
        ModelCheck{"模型類型？"}
        ChatComp["Chat Completions API\n（預設路徑）"]
        RespAPI["Responses API\n（GPT-5.4 · Codex 模型）"]
        ImgNorm["圖片正規化\ndecode → re-encode 為 JPEG\n（PNG / GIF / WebP → JPEG）"]
    end

    subgraph ReasoningLevels ["推理層級（全 Provider）"]
        RL["各 Provider 可獨立設定\n推理層級\n（low / medium / high）"]
    end

    Planner --> Copilot
    Planner --> OpenAI
    Planner --> Claude
    Planner --> Gemini
    Planner --> Nvidia
    Planner --> Compat

    Copilot --> ModelCheck
    ModelCheck -->|"GPT-5.4 / Codex"| RespAPI
    ModelCheck -->|"其餘模型"| ChatComp
    Copilot --> ImgNorm

    Providers --> RL
```

---

## 4. 安全層

沙箱隔離、敏感路徑封鎖與憑證儲存。

```mermaid
flowchart TD
    ToolCall["進入的工具呼叫\n（來自 Execute 迴圈）"]

    subgraph PathValidation ["路徑驗證 · filesystem"]
        AbsPath["GetAbsPath()\nsymlink 安全解析"]
        HomeCheck{"在使用者\nHome 目錄內？"}
        Reject1["拒絕 · 路徑逃逸"]
    end

    subgraph DeniedPaths ["敏感路徑封鎖 · sandbox"]
        DenyMap["denied_map.json\n（嵌入式，依 OS 分類規則）"]
        DenyCheck{"符合封鎖規則？"}
        Reject2["拒絕 · 敏感路徑"]
    end

    subgraph SandboxExec ["Process 隔離"]
        OSCheck{"作業系統？"}
        Bwrap["bubblewrap · Linux\n--unshare-all namespace\n--new-session\n動態探測 + Graceful Fallback"]
        SandboxExecMac["sandbox-exec · macOS\nApple Seatbelt profile"]
    end

    subgraph Keychain ["憑證儲存 · filesystem"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service\nAPI Key 從不明文儲存"]
    end

    Allow["在沙箱中執行工具"]

    ToolCall --> AbsPath
    AbsPath --> HomeCheck
    HomeCheck -->|"在外部"| Reject1
    HomeCheck -->|"在內部"| DenyCheck
    DenyMap --> DenyCheck
    DenyCheck -->|"封鎖"| Reject2
    DenyCheck -->|"允許"| OSCheck
    OSCheck -->|"Linux"| Bwrap
    OSCheck -->|"macOS"| SandboxExecMac
    Bwrap --> Allow
    SandboxExecMac --> Allow
    KC -.->|"注入憑證"| Allow
```

---

## 5. 工具子系統

所有工具類別、發現路徑與自註冊機制。

```mermaid
flowchart TD
    Registry["自註冊 Tool Registry\n（取代 switch routing）"]

    subgraph FileTools ["檔案操作"]
        FT["read_file · write_file\npatch_edit · glob_files\nlist_files · search_content\nmove_to_trash · run_command"]
    end

    subgraph WebTools ["網路存取"]
        WT["fetch_page · 無頭 Chrome + stealth JS · Chrome 自動偵測\nsearch_web · Brave + DDG 並行 · SHA-256 快取 5 分鐘 TTL\ngoogle_rss · RSS 訂閱擷取 · 5 分鐘快取\ndownload_page · 原始頁面下載\nanalyze_youtube · metadata 擷取"]
    end

    subgraph APITools ["API Extension · apiAdapter"]
        AT["14+ 內嵌 JSON 定義\n（CoinGecko · Wikipedia · Open-Meteo\nYahoo Finance · YouTube · 等）"]
        UserAPI["使用者自訂 Extension\n~/.config/agenvoy/api_tools/*.json\n啟動時自動載入 · 無需重新編譯"]
    end

    subgraph ScriptTools ["Script Extension · scriptAdapter"]
        ST["掃描路徑\n~/.config/agenvoy/script_tools/\n<workdir>/.config/agenvoy/script_tools/"]
        Manifest["tool.json manifest\nname · description · parameters schema"]
        Runner["script.js / script.py\nstdin/stdout JSON 協定\nscript_ 前綴自動註冊"]
    end

    subgraph SkillTools ["Skill Git 工具"]
        SGT["skill_git_commit\nskill_git_log\nskill_git_rollback\n（作用於 Skill 儲存庫路徑）"]
    end

    subgraph SchedulerTools ["排程器 · scheduler"]
        SchT["Cron 任務 · 週期性\n一次性任務\nJSON 持久化 · 重啟後恢復\n完成時 Discord 回傳"]
        SchCRUD["add_task · update_task · delete_task\nadd_cron · update_cron · delete_cron"]
    end

    subgraph ErrorMemTools ["錯誤記憶"]
        EMT["工具呼叫失敗 →\n持久化至 tool_errors/{SHA-256}.json\nsearch_errors · 回溯過去失敗\nremember_error · 持久化解決方案\n跨 Session 學習"]
    end

    subgraph ExternalAgentTools ["外部 Agent 工具"]
        EAT["call_external_agent · 委派任務至指定外部 Agent\nverify_with_external_agent · 並行交叉驗證（所有已宣告 Agent）\nreview_result · 內部優先序模型審查\n（claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet）"]
    end

    subgraph SearchTools ["延遲工具 Registry · searchTools"]
        SRT["search_tools · AlwaysLoad=true · ReadOnly=true\n關鍵字模糊搜尋 + 'select:<name>' 直接啟用\n'+term' 必要詞語語法 · max_results 可設定\n將匹配工具的完整 schema 注入當前請求 Context"]
    end

    Registry --> FileTools
    Registry --> WebTools
    Registry --> APITools
    Registry --> ScriptTools
    Registry --> SkillTools
    Registry --> SchedulerTools
    Registry --> ErrorMemTools
    Registry --> ExternalAgentTools
    Registry --> SearchTools

    AT --> UserAPI
    ST --> Manifest
    Manifest --> Runner
    SchT --> SchCRUD
```

---

## 6. 持久化與記憶

Session 摘要深度合併、對話歷史裁剪與錯誤記憶。

```mermaid
flowchart TD
    subgraph SessionSummary ["Session 摘要 · sessionManager"]
        SumExtract["摘要擷取\n3 個獨立 regex pattern\n· fenced block\n· XML &lt;summary&gt; tag\n· [summary] bracket"]
        SumMerge["mergeSummary()\n跨輪次深度合併\n新條目附加\n已有條目原地更新"]
        SumInject["注入至下一次 Execute()\n作為 system context 置於 history 之前"]
    end

    subgraph HistoryTrim ["對話歷史 · trimMessages()"]
        Budget["MaxInputTokens()\n各 Provider token 預算"]
        Preserve["永遠保留\n· system prompt\n· 注入的 summary\n· 最新使用者訊息"]
        Trim["從最舊輪次開始裁剪\n直到符合預算\n插入省略號標記"]
        SearchHist["search_history 工具\n關鍵字觸發式回溯\n（非全量重播）"]
    end

    subgraph ErrorMemory ["錯誤記憶 · errorMemory"]
        ErrHash["SHA-256(tool_name + args)\n每個 Session 的唯一 key"]
        ErrStore["tool_errors/{hash}.json\n持久化至檔案系統"]
        ErrRecall["search_errors\n模糊關鍵字比對\n跨所有 Session"]
        ErrResolve["remember_error\n持久化解決方案決策\n跨 Session 重用"]
    end

    subgraph UsageTracking ["用量追蹤 · usageManager"]
        UT["逐模型 token 用量\n跨所有工具呼叫迭代累計\n（每次請求）"]
    end

    SumExtract --> SumMerge --> SumInject
    Budget --> Preserve --> Trim
    Trim --> SearchHist
    ErrHash --> ErrStore
    ErrStore --> ErrRecall
    ErrRecall --> ErrResolve
```

---

## 7. REST API 層

HTTP 端點路由、Handler dispatch，以及 SSE 與非 SSE 回應路徑。

```mermaid
flowchart TD
    Client["外部呼叫端\n（script tool · skill · 瀏覽器）"]

    subgraph Router ["Gin Router · internal/routes"]
        R1["GET  /v1/tools"]
        R2["POST /v1/tool/:name"]
        R3["POST /v1/send"]
        R4["GET  /v1/key"]
        R5["POST /v1/key"]
    end

    subgraph Handlers ["Handlers · internal/routes/handler"]
        H1["ListTools()\n列舉已註冊工具\nname · description · parameters"]
        H2["CallTool()\n驗證工具存在\n透過 tools.Execute() 執行"]
        H3SSE["SendSSE()\nstreaming token 輸出\nContent-Type: text/event-stream\n（exclude_tools → 每次請求獨立過濾工具清單）"]
        H3JSON["Send()\n收集完整回應\n回傳 JSON {text}\n（model 欄位設定時略過 SelectAgent）\n（exclude_tools → 每次請求獨立過濾工具清單）"]
        H4["GetKey()\n從 OS Keychain 讀取"]
        H5["SaveKey()\n寫入 OS Keychain"]
    end

    subgraph Core ["核心層"]
        Executor["tools.NewExecutor()\n載入所有已註冊工具"]
        Execute["tools.Execute()\n執行單一工具呼叫"]
        Run["exec.Run()\n完整 Agent 執行迴圈"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service"]
    end

    SSECheck{"sse: true?"}

    Client --> R1 & R2 & R3 & R4 & R5
    R1 --> H1
    R2 --> H2
    R3 --> SSECheck
    SSECheck -->|"是"| H3SSE
    SSECheck -->|"否"| H3JSON
    R4 --> H4
    R5 --> H5

    H1 --> Executor
    H2 --> Executor --> Execute
    H3SSE --> Run
    H3JSON --> Run
    H4 --> KC
    H5 --> KC
```

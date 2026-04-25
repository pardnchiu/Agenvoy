# Agenvoy — 架構

> 返回 [README](./README.zh.md)

九張 Mermaid 圖涵蓋從進入點到各子系統的完整系統結構。

## 1. 系統概覽

所有主要子系統之間的高階資料流。

```mermaid
graph TB
    subgraph Entry ["進入點"]
        App["cmd/app · 統一 TUI 應用\n(CLI · TUI · Discord · REST API)"]
    end

    subgraph Engine ["執行引擎"]
        Run["exec.Run()"]
        Execute["exec.Execute()\n≤128 iterations"]
    end

    subgraph Providers ["LLM Providers"]
        P["Copilot · OpenAI · Codex · Claude\nGemini · Nvidia · Compat"]
    end

    subgraph Security ["安全層"]
        S["Sandbox · Denied Paths · Keychain"]
    end

    subgraph Tools ["工具子系統"]
        T["File · Web · API · Script\nScheduler · Error Memory · Sub-Agent"]
    end

    subgraph Memory ["記憶層"]
        PS["ToriiDB Store\nSession Summary"]
    end

    App --> Run
    Run --> Execute
    Execute -->|"Agent.Send()"| Providers
    Execute -->|"tool calls"| Security
    Security --> Tools
    Tools -->|"results"| Execute
    Execute --> Memory
    Memory -.->|"inject"| Execute
```

---

## 2. 執行引擎

流程順序：`exec.Run()` 先偵測 `/skill-name` 前綴（僅標記，不啟用），接著 `SelectAgent()` 挑選 provider，然後交給 `Execute()`。Skill 啟用發生在**迭代迴圈內**，以工具呼叫形式完成 — 絕非獨立前置步驟。

```mermaid
flowchart TD
    Run["exec.Run()"]
    PrefixDetect["scanner.MatchSkillCall()\n僅偵測 '/skill-name' 前綴\n標記 matchedSkill 並剝除前綴"]
    AgentScan["SelectAgent()\nPlanner LLM 挑選最佳 provider\n（以 matchedSkill 作為 hint）"]
    Enter["exec.Execute() 進入點"]

    subgraph Preseed ["Pre-loop · 僅當 matchedSkill != nil"]
        AssignSynth["assignSkill()\n合成 activate_skill 的\ntool_call + tool_result\n寫入 ToolHistories\n（skill body + 執行指引）"]
    end

    subgraph Loop ["迭代迴圈 · exec.Execute()"]
        Assemble["assembleMessages()\n4 段固定區塊：\nSystemPrompts · OldHistories · UserInput · ToolHistories\n（system prompt 透過\nskillTool.ListBlock\n攜帶 '## Skills' 清單）"]
        ReactTrim{"超過 context length？"}
        TrimOld["裁剪 OldHistories\n或 ToolHistories\n（錯誤時反應式）"]
        Send["Agent.Send()\n統一 provider 介面"]
        Parse["解析回應\n抽取 tool_calls"]
        Dispatch["派送 tool calls\n並行執行"]
        SkillToolCall["activate_skill handler\nRenderActivation(skill)\n作為 tool_result 回傳\n（LLM 主動名稱比對路徑）"]
        Dedup["Hash 去重\n避免相同呼叫重跑"]
        Accum["累積結果\n附加至訊息歷史"]
        Check{"停止條件？\n無 tool_calls\n或 iteration ≥ 128"}
    end

    Done["回傳最終回應"]

    Run --> PrefixDetect
    PrefixDetect --> AgentScan
    AgentScan --> Enter
    Enter --> AssignSynth
    Enter -.->|"無前綴命中"| Loop
    AssignSynth --> Loop
    Assemble --> ReactTrim
    ReactTrim -->|"是"| TrimOld --> Assemble
    ReactTrim -->|"否"| Send
    Send --> Parse
    Parse --> Dispatch
    Dispatch -->|"name == activate_skill"| SkillToolCall
    SkillToolCall --> Accum
    Dispatch --> Dedup
    Dedup --> Accum
    Accum --> Check
    Check -->|"繼續"| Assemble
    Check -->|"結束"| Done
```

---

## 3. Provider 路由

Planner LLM 如何挑選 provider，以及各後端如何處理請求。

```mermaid
flowchart TD
    Planner["Planner LLM\n(SelectAgent)\n依任務類型為各 provider 評分"]

    subgraph Providers ["Provider 後端"]
        Copilot["Copilot\ngithub.com token auth\n401 自動重新登入"]
        OpenAI["OpenAI\n推理模型標記\nno_temperature"]
        Codex["OpenAI Codex\nDevice Code Flow\n自動刷新"]
        Claude["Claude\n多 system prompt 合併"]
        Gemini["Gemini\nmultipart 訊息修正"]
        Nvidia["Nvidia NIM\nOpenAI 相容"]
        Compat["Compat\nOllama / 任一 OpenAI endpoint\n具名 compat[{name}]"]
    end

    subgraph CopilotRouting ["Copilot 雙協定"]
        ModelCheck{"模型類型？"}
        ChatComp["Chat Completions API\n(預設路徑)"]
        RespAPI["Responses API\n(GPT-5.4 · Codex 模型)"]
        ImgNorm["圖片正規化\n解碼 → 重編為 JPEG\n(PNG / GIF / WebP → JPEG)"]
    end

    subgraph ReasoningLevels ["Reasoning Level（所有 provider）"]
        RL["逐 provider 設定\nreasoning level\n(low / medium / high)"]
    end

    Planner --> Copilot
    Planner --> OpenAI
    Planner --> Codex
    Planner --> Claude
    Planner --> Gemini
    Planner --> Nvidia
    Planner --> Compat

    Copilot --> ModelCheck
    ModelCheck -->|"GPT-5.4 / Codex"| RespAPI
    ModelCheck -->|"其他"| ChatComp
    Copilot --> ImgNorm

    Providers --> RL
```

---

## 4. 安全層

Sandbox 隔離、敏感路徑拒絕與憑證儲存。

```mermaid
flowchart TD
    ToolCall["進入工具呼叫\n(來自 Execute 迴圈)"]

    subgraph PathValidation ["路徑驗證 · filesystem"]
        AbsPath["GetAbsPath()\nsymlink 安全解析"]
        HomeCheck{"是否在使用者 home？"}
        Reject1["拒絕 · 路徑逃逸"]
    end

    subgraph DeniedPaths ["敏感路徑拒絕 · go-utils/sandbox"]
        DenyMap["denied_map.json\n(嵌入，依 OS 規則)\n以 sandbox.New(configs.DeniedMap) 一次性載入"]
        DenyCheck{"命中 deny 規則？"}
        Reject2["拒絕 · 敏感路徑"]
    end

    subgraph SandboxExec ["Process 隔離 · go-utils/sandbox v0.6.0"]
        OSCheck{"OS？"}
        Bwrap["bubblewrap · Linux\n自動探測 --unshare-* namespace\n--new-session · --die-with-parent\nCheckDependence() 自動安裝 bwrap"]
        SandboxExecMac["sandbox-exec · macOS\nApple Seatbelt profile\n為 Security.framework 重新允許 keychain"]
    end

    subgraph Keychain ["憑證儲存 · filesystem/keychain"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service\nAPI key 從不以明文儲存"]
    end

    Allow["在 sandbox 中執行工具"]

    ToolCall --> AbsPath
    AbsPath --> HomeCheck
    HomeCheck -->|"在外"| Reject1
    HomeCheck -->|"在內"| DenyCheck
    DenyMap --> DenyCheck
    DenyCheck -->|"拒絕"| Reject2
    DenyCheck -->|"允許"| OSCheck
    OSCheck -->|"Linux"| Bwrap
    OSCheck -->|"macOS"| SandboxExecMac
    Bwrap --> Allow
    SandboxExecMac --> Allow
    KC -.->|"注入憑證"| Allow
```

---

## 5. 工具子系統

所有工具類別、其發現路徑與註冊機制。

```mermaid
flowchart TD
    Registry["自註冊 Tool Registry\n(取代 switch 路由)"]

    subgraph FileTools ["檔案操作"]
        FT["read_file · write_file · read_image\npatch_file · glob_files\nlist_files · search_content\nmove_to_trash · run_command"]
    end

    subgraph WebTools ["Web 存取（ToriiDB 快取）"]
        WT["fetch_page · headless Chrome + stealth JS\nsearch_web · Google + DDG 並行 · SHA-256 cache\nfetch_google_rss · RSS 抓取\nsave_page_to_file · 原始頁面下載\nfetch_youtube_transcript · metadata 抓取"]
    end

    subgraph APITools ["API 擴充 · apiAdapter"]
        AT["12+ 嵌入 JSON 定義\n(CoinGecko · Wikipedia · Open-Meteo\nYahoo Finance · YouTube · etc.)"]
        UserAPI["使用者擴充\n~/.config/agenvoy/api_tools/*.json\n啟動時載入 · 無需重新編譯"]
    end

    subgraph ScriptTools ["Script 擴充 · scriptAdapter"]
        ST["掃描路徑\n~/.config/agenvoy/script_tools/\n<workdir>/.config/agenvoy/script_tools/"]
        Manifest["tool.json manifest\nname · description · parameters schema"]
        Runner["script.js / script.py\nstdin/stdout JSON 協定\nscript_ 前綴註冊"]
    end

    subgraph SkillTools ["Skill 啟用與 Git 工具"]
        SST["activate_skill · AlwaysLoad=true · ReadOnly=true\n以精確名稱啟用 '## Skills' 清單中的 skill\n回傳 RenderActivation(skill)：body + 執行指引\n'/skill-name' 前綴由 assignSkill() 自動合成呼叫\n（合成 tool_call/tool_result 寫入 ToolHistories）"]
        SGT["skill_git_commit\nskill_git_log\nskill_git_rollback\n(對 skill repo 路徑操作)"]
    end

    subgraph SchedulerTools ["Scheduler · scheduler"]
        SchT["cron 任務 · 循環\n一次性任務\nJSON 持久化 · 重啟還原\n完成時 Discord callback"]
        SchCRUD["add_task · remove_task · list_tasks\nadd_cron · remove_cron · list_crons"]
    end

    subgraph ErrorMemTools ["錯誤記憶（ToriiDB）"]
        EMT["工具呼叫失敗 →\n持久化至 ToriiDB store\nsearch_error_memory · 回憶過往失敗\nremember_error · 持久化解法\n跨 session 學習"]
    end

    subgraph ExternalAgentTools ["外部 Agent 工具"]
        EAT["invoke_external_agent · 委派至具名外部 agent\ncross_review_with_external_agents · 平行跨驗證所有宣告 agent\nreview_result · 內部優先序覆核\n(claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet)"]
    end

    subgraph SubAgentTools ["In-Process 子 Agent · agents/subagent"]
        SAT["invoke_subagent · Concurrent=true · ReadOnly=true\n以 exec.Execute() in-process 派送 · 不走 HTTP\n獨立 temp-sub-* session · 1 小時 idle TTL\n可覆寫：model · system_prompt · exclude_tools\n強制排除 invoke_subagent 自身避免無限巢狀\nhost singleton (agents/host) 提供 Planner · Registry · Scanner\ncmd/app/main.go blank-import 註冊 · 避免 tools → subagent import cycle"]
    end

    subgraph SearchTools ["延遲工具註冊 · searchTools"]
        SRT["search_tools · AlwaysLoad=true · ReadOnly=true\nkeyword fuzzy + 'select:<name>' 直接啟用\n'+term' 必要關鍵字語法 · max_results 可設\n將符合的工具 schema 注入當前請求 context"]
    end

    Registry --> FileTools
    Registry --> WebTools
    Registry --> APITools
    Registry --> ScriptTools
    Registry --> SkillTools
    Registry --> SchedulerTools
    Registry --> ErrorMemTools
    Registry --> ExternalAgentTools
    Registry --> SubAgentTools
    Registry --> SearchTools
    SAT -.->|"重新進入"| Registry

    AT --> UserAPI
    ST --> Manifest
    Manifest --> Runner
    SchT --> SchCRUD
```

---

## 6. 延遲載入機制

兩條並行的 lazy-load 路徑共同壓低 system prompt 體積。**工具 schema**：executor 初始化時，非 `AlwaysLoad` 的工具以空 stub schema（`{"type":"object","properties":{}}`）曝露；首次呼叫觸發 `search_tools select:<name>` 啟用並回覆 `Re-invoke...` 而非執行。**Skill body**：system prompt 的 `## Skills` 清單僅攜帶 name + description（≤200 runes）；完整 body + 執行指引僅在 `activate_skill` 被呼叫時以 tool result 回傳。兩者同一 `索引 → 啟用 → 完整內容` 模式。

```mermaid
flowchart TD
    subgraph ToolLazy ["工具 Schema 延遲載入 · internal/tools/executor.go"]
        ExecInit["NewExecutor()"]
        Classify{"AlwaysLoad\n標記？"}
        AlwaysReal["曝露真實 schema\n(activate_skill · search_tools\n+ api_*、ReadOnly 類)"]
        Stub["曝露 stub schema\n{type:object, properties:{}}\n標記 StubTools[name]=true"]
        LLM1["LLM 首次呼叫\n常以空參數或\nbest-effort 參數"]
        StubHit["toolCall.go Pass 1 偵測\nStubTools[name] == true"]
        Activate["dispatch search_tools\nselect:<name>\n→ 將真實 schema 注入\n當前請求工具清單"]
        DeleteStub["delete StubTools[name]\n標記 activatedInBatch[name]"]
        ReInvoke["回覆：'[name] tool schema\njust loaded. Re-invoke...'\n跳過 validator + executor"]
        SameBatch["同輪重複 stub 呼叫：\nactivatedInBatch 短路\n回覆相同 Re-invoke"]
        LLM2["下一輪\nLLM 看到真實 schema\n以正確參數呼叫"]
    end

    subgraph SkillLazy ["Skill Body 延遲載入 · internal/tools/skillTool"]
        SysPrompt["system prompt ##Skills 清單\nskillTool.ListBlock(scanner)\nname + description ≤200 runes\n(maxDescLen 截斷)"]
        LLMPick["LLM 依使用者請求\n配對清單中的 skill 名稱\n或 '/skill-name' 前綴\n(scanner.MatchSkillCall)"]
        CallPath{"啟用\n路徑？"}
        LLMCall["LLM 主動呼叫\nactivate_skill(skill=name)"]
        AssignCall["assignSkill() 合成\ntool_call + tool_result\n寫入 ToolHistories"]
        Handler["activate_skill handler\nRenderActivation(skill)：\n啟用名稱 + 路徑\n+ SkillExecution 指引\n+ 完整 skill body"]
        ToolResult["以 tool_result 回傳\n作為後續迭代的\n繫結指令"]
    end

    ExecInit --> Classify
    Classify -->|"true"| AlwaysReal
    Classify -->|"false"| Stub
    Stub --> LLM1
    LLM1 --> StubHit
    StubHit --> Activate
    Activate --> DeleteStub
    DeleteStub --> ReInvoke
    ReInvoke --> LLM2
    LLM1 -.->|"同批重複"| SameBatch
    SameBatch --> ReInvoke

    SysPrompt --> LLMPick
    LLMPick --> CallPath
    CallPath -->|"LLM 主動"| LLMCall
    CallPath -->|"/skill-name 前綴"| AssignCall
    LLMCall --> Handler
    AssignCall --> Handler
    Handler --> ToolResult
```

---

## 7. 子 Agent 流程

`invoke_subagent` 的完整生命週期：主 agent 如何以 in-process 方式透過 `exec.Execute()` 派送子 agent、隔離 session、並接收最終回應 — 全程不跨越 HTTP 邊界。

```mermaid
flowchart TD
    subgraph Parent ["主 Agent · exec.Execute()"]
        PLoop["迭代迴圈\ntool_call 派送"]
        PCheck{"tool_name ==\ninvoke_subagent？"}
        PWait["等待子 agent 結果\n(Concurrent=true · 可併發 fan-out)"]
        PResult["接收子 agent 最終回應\n作為 tool result\n繼續迭代"]
    end

    subgraph Handler ["invoke_subagent Handler · internal/agents/subagent"]
        Args["解析 args\n· task（必填）\n· model? · system_prompt?\n· exclude_tools?"]
        Host["host singleton 查詢\nPlanner · Registry · Scanner\n(由 cmd/app/main.go 設定)"]
        Session["建立 temp-sub-{uuid} session\n獨立 history 與 context\n1 小時 idle TTL"]
        ForceEx["強制加入 invoke_subagent\n至 exclude_tools\n→ 防止無限巢狀"]
        Overrides["套用 overrides\n· 切換 model（若有指定）\n· 覆寫 system_prompt\n· 依 exclude_tools 過濾 Registry"]
    end

    subgraph Child ["子 Agent · exec.Execute() 重新進入"]
        CRun["獨立迭代迴圈\n≤128 iterations\n獨立錯誤記憶 · 獨立 tool 歷史"]
        CTools["已過濾 tool registry\n(invoke_subagent 已移除\n+ 使用者指定排除項)"]
        CFinal["最終回應文字\n以 string 回傳"]
    end

    subgraph Lifecycle ["Session 生命週期"]
        IdleGC["Idle watcher\n清除 temp-sub-* sessions\n閒置 1 小時後移除"]
        NoHTTP["不經 HTTP\n不開 subprocess\n→ CLI / TUI / App 行為一致"]
    end

    PLoop --> PCheck
    PCheck -->|"是"| Args
    Args --> Host
    Host --> Session
    Session --> ForceEx
    ForceEx --> Overrides
    Overrides --> CRun
    CRun --> CTools
    CTools --> CFinal
    CFinal --> PWait
    PWait --> PResult
    PResult --> PLoop
    Session -.->|"TTL 到期"| IdleGC
    Handler -.-> NoHTTP
```

---

## 8. 儲存與記憶

Session 摘要的分塊多階段生成、對話歷史裁剪與 ToriiDB-backed 錯誤記憶。

```mermaid
flowchart TD
    subgraph SessionSummary ["Session 摘要 · sessionManager"]
        SumCron["每小時 cron 觸發\n(非 per-request)"]
        SumChunk["分塊多階段生成\n長 session 避免 context 溢位"]
        SumExtract["summary 抽取\n3 組獨立 regex\n· fenced block\n· XML &lt;summary&gt; tag\n· [summary] bracket"]
        SumMerge["mergeSummary()\n跨輪 deep-merge\n新項目 append\n既有項目 in-place update"]
        SumInject["注入下一個 Execute()\n作為 history 之前的 system context"]
    end

    subgraph HistoryTrim ["對話歷史 · trimMessages()"]
        Budget["MaxInputTokens()\n逐 provider token 預算"]
        Preserve["一律保留\n· system prompt\n· 注入的 summary\n· 最新 user message"]
        Trim["由最舊輪次開始裁剪\n直到符合預算\n插入 ellipsis 標記"]
        SearchHist["search_conversation_history 工具\n(ToriiDB store)\n前置：剔除最新 MaxHistoryMessages 筆\n(已在 LLM context 的範圍)\n一律：keyword (8) ∪ semantic (8)\n· keyword：字面 substring + time_range\n· semantic：VSearch 餘弦 top-K（不套 time_range，\n  無 OPENAI_API_KEY 靜默回空）\n後處理：依 key 去重（上限 16）、升冪時間排序、\nRFC3339 · role 前綴"]
    end

    subgraph ToriiStore ["ToriiDB Store · filesystem/store"]
        TS["嵌入 KV store\n取代分散 JSON 檔案\n· session 歷史（寫入時 SetVector 內嵌向量）\n· 錯誤記憶\n· fetch_page / search_web / google_rss 快取\n· fetch_page 跳過清單"]
    end

    subgraph ErrorMemory ["錯誤記憶 · errorMemory"]
        ErrHash["SHA-256(tool_name + args)\n逐 session key"]
        ErrStore["ToriiDB store\n取代早期 tool_errors/*.json"]
        ErrRecall["search_error_memory\nfuzzy 關鍵字比對\n跨所有 session"]
        ErrResolve["remember_error\n持久化解法決策\n跨 session 重用"]
    end

    subgraph UsageTracking ["Usage Tracking · usageManager"]
        UT["逐模型 token 用量\n跨所有工具呼叫迭代\n累積於每次請求"]
    end

    SumCron --> SumChunk --> SumExtract --> SumMerge --> SumInject
    Budget --> Preserve --> Trim
    Trim --> SearchHist
    SearchHist --> TS
    ErrHash --> ErrStore
    ErrStore --> TS
    ErrStore --> ErrRecall
    ErrRecall --> ErrResolve
```

---

## 9. REST API 層

HTTP endpoint 路由、handler 派送以及 SSE vs 非 SSE 回應路徑。

```mermaid
flowchart TD
    Client["外部 client\n(script tool · skill · browser)"]

    subgraph Router ["Gin Router · internal/routes"]
        R1["GET  /v1/tools"]
        R2["POST /v1/tool/:name"]
        R3["POST /v1/send"]
        R4["GET  /v1/key"]
        R5["POST /v1/key"]
    end

    subgraph Handlers ["Handlers · internal/routes/handler"]
        H1["ListTools()\n列出註冊工具\nname · description · parameters"]
        H2["CallTool()\n驗證工具存在\n經由 tools.Execute() 派送"]
        H3SSE["SendSSE()\n串流 token chunks\nContent-Type: text/event-stream\n(exclude_tools → 逐請求過濾)"]
        H3JSON["Send()\n收集完整回應\n回傳 JSON {text}\n(model 欄位 → 繞過 SelectAgent)\n(exclude_tools → 逐請求過濾)"]
        H4["GetKey()\n由 OS Keychain 讀取"]
        H5["SaveKey()\n寫入 OS Keychain"]
    end

    subgraph Core ["核心層"]
        Executor["tools.NewExecutor()\n載入所有註冊工具"]
        Execute["tools.Execute()\n執行單一工具呼叫"]
        Run["exec.Run()\n完整 agent 執行迴圈"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service"]
    end

    SSECheck{"sse: true？"}

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

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)

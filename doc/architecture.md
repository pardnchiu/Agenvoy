# Agenvoy — Architecture

> Back to [README](../README.md)

Nine Mermaid diagrams covering the full system, from entry points down to individual subsystems.

## 1. System Overview

High-level data flow across all major subsystems.

```mermaid
graph TB
    subgraph Entry ["Entry Points"]
        App["cmd/app · Unified TUI App\n(CLI · TUI · Discord · REST API)"]
    end

    subgraph Engine ["Execution Engine"]
        Run["exec.Run()"]
        Execute["exec.Execute()\n≤128 iterations"]
    end

    subgraph Providers ["LLM Providers"]
        P["Copilot · OpenAI · Codex · Claude\nGemini · Nvidia · Compat"]
    end

    subgraph Security ["Security Layer"]
        S["Sandbox · Denied Paths · Keychain"]
    end

    subgraph Tools ["Tool Subsystem"]
        T["File · Web · API · Script\nScheduler · Error Memory · Sub-Agent"]
    end

    subgraph Memory ["Memory Layer"]
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

## 2. Execution Engine

Flow ordering: `exec.Run()` first detects any `/skill-name` prefix (flag only, no activation), then runs `SelectAgent()` to pick the provider, then hands off to `Execute()`. Skill activation happens **inside** the iteration loop as a tool call — never as a separate pre-call.

```mermaid
flowchart TD
    Run["exec.Run()"]
    PrefixDetect["scanner.MatchSkillCall()\n'/skill-name' prefix detect only\nflags matchedSkill, strips prefix"]
    AgentScan["SelectAgent()\nPlanner LLM picks best provider\n(takes matchedSkill as hint)"]
    Enter["exec.Execute() entry"]

    subgraph Preseed ["Pre-loop · only if matchedSkill != nil"]
        AssignSynth["assignSkill()\nsynthesize activate_skill\ntool_call + tool_result\ninto ToolHistories\n(skill body + execution guidance)"]
    end

    subgraph Loop ["Iteration Loop · exec.Execute()"]
        Assemble["assembleMessages()\n4 fixed segments:\nSystemPrompts · OldHistories · UserInput · ToolHistories\n(system prompt carries '## Skills' list\nvia skillTool.ListBlock)"]
        ReactTrim{"Context length\nexceeded?"}
        TrimOld["Trim OldHistories\nor ToolHistories\n(reactive, on error)"]
        Send["Agent.Send()\nunified provider interface"]
        Parse["Parse response\nextract tool_calls"]
        Dispatch["Dispatch tool calls\nparallel execution"]
        SkillToolCall["activate_skill handler\nRenderActivation(skill)\nreturned as tool_result\n(LLM-initiated name-match path)"]
        Dedup["Hash-based deduplication\nprevent identical repeat calls"]
        Accum["Accumulate results\nappend to message history"]
        Check{"Stop condition?\nno tool_calls OR\niteration ≥ 128"}
    end

    Done["Return final response"]

    Run --> PrefixDetect
    PrefixDetect --> AgentScan
    AgentScan --> Enter
    Enter --> AssignSynth
    Enter -.->|"no prefix match"| Loop
    AssignSynth --> Loop
    Assemble --> ReactTrim
    ReactTrim -->|"yes"| TrimOld --> Assemble
    ReactTrim -->|"no"| Send
    Send --> Parse
    Parse --> Dispatch
    Dispatch -->|"name == activate_skill"| SkillToolCall
    SkillToolCall --> Accum
    Dispatch --> Dedup
    Dedup --> Accum
    Accum --> Check
    Check -->|"continue"| Assemble
    Check -->|"done"| Done
```

---

## 3. Provider Routing

How the Planner LLM selects a provider and how each backend handles the request.

```mermaid
flowchart TD
    Planner["Planner LLM\n(SelectAgent)\nscores providers by task type"]

    subgraph Providers ["Provider Backends"]
        Copilot["Copilot\ngithub.com token auth\nauto-relogin on 401"]
        OpenAI["OpenAI\nno_temperature flag\nfor reasoning models"]
        Codex["OpenAI Codex\nDevice Code Flow\nauto-refresh"]
        Claude["Claude\nmulti-system-prompt merge"]
        Gemini["Gemini\nmultipart message fix"]
        Nvidia["Nvidia NIM\nOpenAI-compatible"]
        Compat["Compat\nOllama / any OpenAI endpoint\nnamed compat[{name}] instances"]
    end

    subgraph CopilotRouting ["Copilot Dual-Protocol"]
        ModelCheck{"Model type?"}
        ChatComp["Chat Completions API\n(default path)"]
        RespAPI["Responses API\n(GPT-5.4 · Codex models)"]
        ImgNorm["Image normalization\ndecode → re-encode as JPEG\n(PNG / GIF / WebP → JPEG)"]
    end

    subgraph ReasoningLevels ["Reasoning Levels (all providers)"]
        RL["Per-provider configurable\nreasoning level\n(low / medium / high)"]
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
    ModelCheck -->|"all others"| ChatComp
    Copilot --> ImgNorm

    Providers --> RL
```

---

## 4. Security Layer

Sandbox isolation, sensitive path denial, and credential storage.

```mermaid
flowchart TD
    ToolCall["Incoming tool call\n(from Execute loop)"]

    subgraph PathValidation ["Path Validation · filesystem"]
        AbsPath["GetAbsPath()\nsymlink-safe resolution"]
        HomeCheck{"Within user\nhome directory?"}
        Reject1["Reject · path escape"]
    end

    subgraph DeniedPaths ["Sensitive Path Denial · go-utils/sandbox"]
        DenyMap["denied_map.json\n(embedded, OS-specific rules)\nseeded once via sandbox.New(configs.DeniedMap)"]
        DenyCheck{"Matches deny rule?"}
        Reject2["Reject · sensitive path"]
    end

    subgraph SandboxExec ["Process Isolation · go-utils/sandbox v0.7.1"]
        OSCheck{"OS?"}
        Bwrap["bubblewrap · Linux\nauto-probed --unshare-* namespaces\n--new-session · --die-with-parent\nCheckDependence() auto-installs bwrap"]
        SandboxExecMac["sandbox-exec · macOS\nApple Seatbelt profile\nkeychain re-allow for Security.framework"]
    end

    subgraph Keychain ["Credential Storage · filesystem/keychain"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service\nAPI keys never stored in plaintext"]
    end

    Allow["Execute tool in sandbox"]

    ToolCall --> AbsPath
    AbsPath --> HomeCheck
    HomeCheck -->|"outside"| Reject1
    HomeCheck -->|"inside"| DenyCheck
    DenyMap --> DenyCheck
    DenyCheck -->|"denied"| Reject2
    DenyCheck -->|"allowed"| OSCheck
    OSCheck -->|"Linux"| Bwrap
    OSCheck -->|"macOS"| SandboxExecMac
    Bwrap --> Allow
    SandboxExecMac --> Allow
    KC -.->|"inject credentials"| Allow
```

---

## 5. Tool Subsystem

All tool categories, their discovery paths, and registration mechanism.

```mermaid
flowchart TD
    Registry["Self-registering Tool Registry\n(replaces switch routing)"]

    subgraph FileTools ["File Operations"]
        FT["read_file · write_file · read_image\npatch_file · glob_files\nlist_files · search_content\nmove_to_trash · run_command"]
    end

    subgraph WebTools ["Web Access (ToriiDB cached)"]
        WT["fetch_page · headless Chrome + stealth JS\nsearch_web · Google + DDG concurrent · SHA-256 cache\nfetch_google_rss · RSS feed fetch\nsave_page_to_file · raw page download\nfetch_youtube_transcript · metadata fetch"]
    end

    subgraph APITools ["API Extensions · apiAdapter"]
        AT["12+ embedded JSON definitions\n(CoinGecko · Wikipedia · Open-Meteo\nYahoo Finance · YouTube · etc.)"]
        UserAPI["User extensions\n~/.config/agenvoy/api_tools/*.json\nloaded at startup · no recompile"]
    end

    subgraph ScriptTools ["Script Extensions · scriptAdapter"]
        ST["Scan paths\n~/.config/agenvoy/script_tools/\n<workdir>/.config/agenvoy/script_tools/"]
        Manifest["tool.json manifest\nname · description · parameters schema"]
        Runner["script.js / script.py\nstdin/stdout JSON protocol\nscript_ prefix registration"]
    end

    subgraph SkillTools ["Skill Activation & Git Tools"]
        SST["activate_skill · AlwaysLoad=true · ReadOnly=true\nactivates skill by exact name from '## Skills' list\nreturns RenderActivation(skill): body + execution guidance\nauto-invoked for '/skill-name' prefix via assignSkill()\n(synthetic tool_call/tool_result into ToolHistories)"]
        SGT["skill_git_commit\nskill_git_log\nskill_git_rollback\n(operates on skill repo path)"]
    end

    subgraph SchedulerTools ["Scheduler · scheduler"]
        SchT["cron tasks · recurring\none-time tasks\nJSON persistence · restore on restart\nDiscord callback on complete"]
        SchCRUD["add_task · remove_task · list_tasks\nadd_cron · remove_cron · list_crons"]
    end

    subgraph ErrorMemTools ["Error Memory (ToriiDB)"]
        EMT["Tool call fails →\npersist to ToriiDB store\nsearch_error_memory · recall past failures\nremember_error · persist resolution\ncross-session learning"]
    end

    subgraph ExternalAgentTools ["External Agent Tools"]
        EAT["invoke_external_agent · delegate task to named external agent\ncross_review_with_external_agents · parallel cross-validation across all declared agents\nreview_result · internal priority-model review\n(claude-opus → gpt-5.4 → gemini-3.1-pro → claude-sonnet)"]
    end

    subgraph SubAgentTools ["In-Process Sub-Agent · agents/subagent"]
        SAT["invoke_subagent · Concurrent=true · ReadOnly=true\nin-process dispatch via exec.Execute() · no HTTP\nisolated temp-sub-* session · 1h idle TTL\noverrides: model · system_prompt · exclude_tools\nforce-excludes invoke_subagent to prevent recursion\nhost singleton (agents/host) for Planner · Registry · Scanner\nblank-imported in cmd/app/main.go to avoid tools → subagent cycle"]
    end

    subgraph SearchTools ["Deferred Tool Registry · searchTools"]
        SRT["search_tools · AlwaysLoad=true · ReadOnly=true\nkeyword fuzzy + 'select:<name>' direct activation\n'+term' required-match syntax · max_results configurable\ninjects matching tool schemas into current request context"]
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
    SAT -.->|"re-enter"| Registry

    AT --> UserAPI
    ST --> Manifest
    Manifest --> Runner
    SchT --> SchCRUD
```

---

## 6. Lazy-Load Mechanism

Two parallel lazy-load patterns keep the system prompt minimal. **Tool schemas**: non-`AlwaysLoad` tools expose an empty stub schema (`{"type":"object","properties":{}}`) on executor init; first call triggers activation via `search_tools select:<name>` and replies `Re-invoke...` instead of running. **Skill bodies**: `## Skills` list in the system prompt only carries name + description (≤200 runes); the full body + execution guidance loads only as the `activate_skill` tool result. Same `index → activate → full content` pattern.

```mermaid
flowchart TD
    subgraph ToolLazy ["Tool Schema Lazy-Load · internal/tools/executor.go"]
        ExecInit["NewExecutor()"]
        Classify{"AlwaysLoad\nflag?"}
        AlwaysReal["expose real schema\n(activate_skill · search_tools\n+ api_*, ReadOnly flagged ones)"]
        Stub["expose stub schema\n{type:object, properties:{}}\nmark StubTools[name]=true"]
        LLM1["LLM first call\noften with empty or\nbest-effort args"]
        StubHit["toolCall.go Pass 1 detects\nStubTools[name] == true"]
        Activate["dispatch search_tools\nselect:<name>\n→ injects real schema into\ncurrent request tool list"]
        DeleteStub["delete StubTools[name]\nmark activatedInBatch[name]"]
        ReInvoke["reply: '[name] tool schema\njust loaded. Re-invoke...'\nSKIP validator + executor"]
        SameBatch["same-turn dup stub calls:\nactivatedInBatch short-circuits\nto same Re-invoke reply"]
        LLM2["next iteration\nLLM sees real schema\ncalls with correct args"]
    end

    subgraph SkillLazy ["Skill Body Lazy-Load · internal/tools/skillTool"]
        SysPrompt["system prompt ##Skills list\nskillTool.ListBlock(scanner)\nname + description ≤200 runes\n(maxDescLen truncation)"]
        LLMPick["LLM matches user request\nto listed skill name\nOR '/skill-name' prefix match\n(scanner.MatchSkillCall)"]
        CallPath{"activation\npath?"}
        LLMCall["LLM calls\nactivate_skill(skill=name)"]
        AssignCall["assignSkill() synthesizes\ntool_call + tool_result\ninto ToolHistories"]
        Handler["activate_skill handler\nRenderActivation(skill):\nactive name + path\n+ SkillExecution guidance\n+ full skill body"]
        ToolResult["return as tool_result\nbinding for subsequent\niterations"]
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
    LLM1 -.->|"dup in same batch"| SameBatch
    SameBatch --> ReInvoke

    SysPrompt --> LLMPick
    LLMPick --> CallPath
    CallPath -->|"LLM-initiated"| LLMCall
    CallPath -->|"/skill-name prefix"| AssignCall
    LLMCall --> Handler
    AssignCall --> Handler
    Handler --> ToolResult
```

---

## 7. Sub-Agent Flow

End-to-end lifecycle of `invoke_subagent`: how the parent agent dispatches an in-process child via `exec.Execute()`, isolates its session, and receives its final response — all without crossing an HTTP boundary.

```mermaid
flowchart TD
    subgraph Parent ["Parent Agent · exec.Execute()"]
        PLoop["Iteration loop\ntool_call dispatch"]
        PCheck{"tool_name ==\ninvoke_subagent?"}
        PWait["Wait for child result\n(Concurrent=true · fan-out ok)"]
        PResult["Receive child final response\nas tool result\ncontinue iteration"]
    end

    subgraph Handler ["invoke_subagent Handler · internal/agents/subagent"]
        Args["Parse args\n· task (required)\n· model? · system_prompt?\n· exclude_tools?"]
        Host["host singleton lookup\nPlanner · Registry · Scanner\n(set by cmd/app/main.go)"]
        Session["Create temp-sub-{uuid} session\nisolated history & context\n1h idle TTL"]
        ForceEx["Force-add invoke_subagent\nto exclude_tools\n→ prevent infinite nesting"]
        Overrides["Apply overrides\n· swap model (if provided)\n· override system_prompt\n· filter Registry by exclude_tools"]
    end

    subgraph Child ["Child Agent · exec.Execute() re-entry"]
        CRun["Independent iteration loop\n≤128 iterations\nown error memory · own tool history"]
        CTools["Filtered tool registry\n(invoke_subagent absent\n+ user exclusions)"]
        CFinal["Final response text\nreturned as string"]
    end

    subgraph Lifecycle ["Session Lifecycle"]
        IdleGC["Idle watcher\npurges temp-sub-* sessions\nafter 1h inactivity"]
        NoHTTP["NO HTTP hop\nNO subprocess\n→ CLI / TUI / App parity"]
    end

    PLoop --> PCheck
    PCheck -->|"yes"| Args
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
    Session -.->|"TTL expiry"| IdleGC
    Handler -.-> NoHTTP
```

---

## 8. Persistence & Memory

Chunked multi-pass summary generation, conversation history trimming, and ToriiDB-backed error memory.

```mermaid
flowchart TD
    subgraph SessionSummary ["Session Summary · sessionManager"]
        SumCron["Hourly cron trigger\n(not per-request)"]
        SumChunk["Chunked multi-pass generation\navoids context overflow on long sessions"]
        SumExtract["Summary extraction\n3 independent regex patterns\n· fenced block\n· XML &lt;summary&gt; tag\n· [summary] bracket"]
        SumMerge["mergeSummary()\ndeep-merge across turns\nnew entries append\nexisting entries update in-place"]
        SumInject["Inject into next Execute()\nas system context before history"]
    end

    subgraph HistoryTrim ["Conversation History · trimMessages()"]
        Budget["MaxInputTokens()\nper-provider token budget"]
        Preserve["Always preserve\n· system prompt\n· injected summary\n· latest user message"]
        Trim["Trim oldest turns first\nuntil within budget\nellipsis markers inserted"]
        SearchHist["search_conversation_history tool\n(ToriiDB store)\npre: drop newest MaxHistoryMessages keys\n(already in LLM context window)\nalways: keyword (8) ∪ semantic (8)\n· keyword: literal substring + time_range\n· semantic: VSearch cosine top-K (no time_range,\n  silent empty when OPENAI_API_KEY missing)\npost: key dedup (max 16), chronological sort,\nRFC3339 · role prefix"]
    end

    subgraph ToriiStore ["ToriiDB Store · filesystem/store"]
        TS["Embedded KV store\nreplaces scattered JSON files\n· session history (SetVector on write)\n· error memory\n· fetch_page / search_web / google_rss cache\n· fetch_page skip list"]
    end

    subgraph ErrorMemory ["Error Memory · errorMemory"]
        ErrHash["SHA-256(tool_name + args)\nper-session key"]
        ErrStore["ToriiDB store\nreplaces earlier tool_errors/*.json"]
        ErrRecall["search_error_memory\nfuzzy keyword match\nacross all sessions"]
        ErrResolve["remember_error\npersist resolution decision\ncross-session reuse"]
    end

    subgraph UsageTracking ["Usage Tracking · usageManager"]
        UT["Per-model token usage\naccumulated across all\ntool-call iterations per request"]
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

## 9. REST API Layer

HTTP endpoint routing, handler dispatch, and SSE vs. non-SSE response paths.

```mermaid
flowchart TD
    Client["External Client\n(script tool · skill · browser)"]

    subgraph Router ["Gin Router · internal/routes"]
        R1["GET  /v1/tools"]
        R2["POST /v1/tool/:name"]
        R3["POST /v1/send"]
        R4["GET  /v1/key"]
        R5["POST /v1/key"]
        R6["GET  /v1/session/:session_id/status"]
        R7["GET  /v1/session/:session_id/log"]
    end

    subgraph Handlers ["Handlers · internal/routes/handler"]
        H1["ListTools()\nenumerate registered tools\nname · description · parameters"]
        H2["CallTool()\nvalidate tool exists\ndispatch via tools.Execute()"]
        H3SSE["SendSSE()\nstream token chunks\nContent-Type: text/event-stream\n(exclude_tools → filter per request)"]
        H3JSON["Send()\ncollect full response\nreturn JSON {text}\n(model field → bypass SelectAgent)\n(exclude_tools → filter per request)"]
        H4["GetKey()\nread from OS Keychain"]
        H5["SaveKey()\nwrite to OS Keychain"]
        H6["GetSessionStatus()\nread status.json → JSON {state, active, ended_at, limit, usage}\n404 if session dir missing"]
        H7["StreamSessionLog()\nSSE: backlog tail-100 + 1 s polling\nlast-line dedup · : ping after 15 quiet ticks"]
    end

    subgraph Core ["Core Layer"]
        Executor["tools.NewExecutor()\nload all registered tools"]
        Execute["tools.Execute()\nrun single tool call"]
        Run["exec.Run()\nfull agent execution loop"]
        KC["OS Keychain\nmacOS Keychain / Linux secret-service"]
    end

    SSECheck{"sse: true?"}

    Client --> R1 & R2 & R3 & R4 & R5 & R6 & R7
    R1 --> H1
    R2 --> H2
    R3 --> SSECheck
    SSECheck -->|"yes"| H3SSE
    SSECheck -->|"no"| H3JSON
    R4 --> H4
    R5 --> H5
    R6 --> H6
    R7 --> H7

    H1 --> Executor
    H2 --> Executor --> Execute
    H3SSE --> Run
    H3JSON --> Run
    H4 --> KC
    H5 --> KC
    H6 -.->|"poll"| StatusFile["sessions/&lt;sid&gt;/status.json"]
    H7 -.->|"tail"| ActionFile["sessions/&lt;sid&gt;/action.log"]
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://linkedin.com/in/pardnchiu)

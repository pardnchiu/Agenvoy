# Agenvoy — Architecture Reference

Six diagrams covering the full system, from entry points down to individual subsystems.

## 1. System Overview

High-level data flow across all major subsystems.

```mermaid
graph TB
    subgraph Entry ["Entry Points"]
        App["cmd/app · TUI Dashboard · WIP"]
        subgraph Managed ["Managed by cmd/app"]
            CLI["cmd/cli · will deprecate"]
            Discord["Discord Bot"]
            API["REST API · HTTP Endpoint · WIP"]
        end
    end

    subgraph Engine ["Execution Engine"]
        Run["exec.Run()"]
        Execute["exec.Execute()\n≤128 iterations"]
    end

    subgraph Providers ["LLM Providers"]
        P["Copilot · OpenAI · Claude\nGemini · Nvidia · Compat"]
    end

    subgraph Security ["Security Layer"]
        S["Sandbox · Denied Paths · Keychain"]
    end

    subgraph Tools ["Tool Subsystem"]
        T["File · Web · API · Script\nScheduler · Error Memory"]
    end

    subgraph Persistence ["Persistence"]
        PS["Session Summary · History"]
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
    Persistence -.->|"inject"| Execute
```

---

## 2. Execution Engine

Internal flow of `exec.Run()` through skill/agent selection, token trimming, and the tool-call iteration loop.

```mermaid
flowchart TD
    Run["exec.Run()"]

    subgraph Selection ["Selection Phase"]
        SkillScan["SelectSkill()\n9 scan paths in priority order"]
        AgentScan["SelectAgent()\nPlanner LLM picks best provider"]
    end

    subgraph Loop ["Iteration Loop · exec.Execute()"]
        Trim["trimMessages()\ntoken-budget enforcement\n(preserves system + summary + latest)"]
        Send["Agent.Send()\nunified provider interface"]
        Parse["Parse response\nextract tool_calls"]
        Dispatch["Dispatch tool calls\nparallel execution"]
        Dedup["Hash-based deduplication\nprevent identical repeat calls"]
        Accum["Accumulate results\nappend to message history"]
        Check{"Stop condition?\nno tool_calls OR\niteration ≥ 128"}
    end

    Done["Return final response"]

    Run --> SkillScan
    Run --> AgentScan
    SkillScan -->|"skill prompt injected"| Loop
    AgentScan -->|"provider selected"| Loop
    Trim --> Send
    Send --> Parse
    Parse --> Dispatch
    Dispatch --> Dedup
    Dedup --> Accum
    Accum --> Check
    Check -->|"continue"| Trim
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

    subgraph DeniedPaths ["Sensitive Path Denial · sandbox"]
        DenyMap["denied_map.json\n(embedded, OS-specific rules)"]
        DenyCheck{"Matches deny rule?"}
        Reject2["Reject · sensitive path"]
    end

    subgraph SandboxExec ["Process Isolation"]
        OSCheck{"OS?"}
        Bwrap["bubblewrap · Linux\n--unshare-all namespace\n--new-session\ndynamic probe + graceful fallback"]
        SandboxExecMac["sandbox-exec · macOS\nApple Seatbelt profile"]
    end

    subgraph Keychain ["Credential Storage · filesystem"]
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
        FT["read_file · write_file\npatch_edit · glob_files\nlist_files · search_content\nmove_to_trash · run_command"]
    end

    subgraph WebTools ["Web Access"]
        WT["fetch_page · headless Chrome + stealth JS\nsearch_web · Brave + DDG concurrent\ndownload_page · SHA-256 cache 1h TTL\nanalyze_youtube · metadata fetch"]
    end

    subgraph APITools ["API Extensions · apiAdapter"]
        AT["14+ embedded JSON definitions\n(CoinGecko · Wikipedia · Open-Meteo\nYahoo Finance · YouTube · etc.)"]
        UserAPI["User extensions\n~/.config/agenvoy/apis/*.json\nloaded at startup · no recompile"]
    end

    subgraph ScriptTools ["Script Extensions · scriptAdapter"]
        ST["Scan paths\n~/.config/agenvoy/script_tools/\n<workdir>/.config/agenvoy/script_tools/"]
        Manifest["tool.json manifest\nname · description · parameters schema"]
        Runner["script.js / script.py\nstdin/stdout JSON protocol\nscript_ prefix registration"]
    end

    subgraph SkillTools ["Skill Git Tools"]
        SGT["skill_git_commit\nskill_git_log\nskill_git_rollback\n(operates on skill repo path)"]
    end

    subgraph SchedulerTools ["Scheduler · scheduler"]
        SchT["cron tasks · recurring\none-time tasks\nJSON persistence · restore on restart\nDiscord callback on complete"]
        SchCRUD["add_task · update_task · delete_task\nadd_cron · update_cron · delete_cron"]
    end

    subgraph ErrorMemTools ["Error Memory"]
        EMT["Tool call fails →\npersist to tool_errors/{SHA-256}.json\nsearch_errors · recall past failures\nremember_error · persist resolution\ncross-session learning"]
    end

    Registry --> FileTools
    Registry --> WebTools
    Registry --> APITools
    Registry --> ScriptTools
    Registry --> SkillTools
    Registry --> SchedulerTools
    Registry --> ErrorMemTools

    AT --> UserAPI
    ST --> Manifest
    Manifest --> Runner
    SchT --> SchCRUD
```

---

## 6. Persistence & Memory

Session summary deep-merge, conversation history trimming, and error memory.

```mermaid
flowchart TD
    subgraph SessionSummary ["Session Summary · sessionManager"]
        SumExtract["Summary extraction\n3 independent regex patterns\n· fenced block\n· XML &lt;summary&gt; tag\n· [summary] bracket"]
        SumMerge["mergeSummary()\ndeep-merge across turns\nnew entries append\nexisting entries update in-place"]
        SumInject["Inject into next Execute()\nas system context before history"]
    end

    subgraph HistoryTrim ["Conversation History · trimMessages()"]
        Budget["MaxInputTokens()\nper-provider token budget"]
        Preserve["Always preserve\n· system prompt\n· injected summary\n· latest user message"]
        Trim["Trim oldest turns first\nuntil within budget\nellipsis markers inserted"]
        SearchHist["search_history tool\nkeyword-triggered recall\n(not full replay)"]
    end

    subgraph ErrorMemory ["Error Memory · errorMemory"]
        ErrHash["SHA-256(tool_name + args)\nper-session key"]
        ErrStore["tool_errors/{hash}.json\npersisted to filesystem"]
        ErrRecall["search_errors\nfuzzy keyword match\nacross all sessions"]
        ErrResolve["remember_error\npersist resolution decision\ncross-session reuse"]
    end

    subgraph UsageTracking ["Usage Tracking · usageManager"]
        UT["Per-model token usage\naccumulated across all\ntool-call iterations per request"]
    end

    SumExtract --> SumMerge --> SumInject
    Budget --> Preserve --> Trim
    Trim --> SearchHist
    ErrHash --> ErrStore
    ErrStore --> ErrRecall
    ErrRecall --> ErrResolve
```

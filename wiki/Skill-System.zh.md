# Skill 系統

> [English](https://github.com/agenvoy/Agenvoy/wiki/Skill-System)

Skill 是可載入的 markdown 指令包，讓 agent 切換到特定執行模式（例如 commit message 生成、code review、README 生成）。

## Skill 格式

Skill 是 markdown 檔，frontmatter 為 YAML metadata：

```markdown
---
name: code-reviewer
description: 深度 code review，涵蓋品質、安全、架構
version: 1.0.0
---

你現在是一個嚴格的 code reviewer...
```

frontmatter `name` 是觸發關鍵字。body 在 skill 啟用時渲染進 system prompt。

## 觸發路徑

### `/skill-name` 斜線指令

在輸入前加 `/<skill-name>`：

```
/code-reviewer 幫我審 PR diff
```

`MatchSkillCall` 命中時，agenvoy 合成 `activate_skill` 的 `tool_call` 與對應 `tool_result`（內含 skill body）直接注入 `ToolHistories` —— 與自然語言啟用路徑 byte-identical，保留 prefix cache。

使用者帶 args（`/code-reviewer 審 src/parser.go`）時，user message 剝掉 `/<skill-name>` 前綴只留 args。無 args 時 user message 保留字面 `/<skill-name>`，讓 LLM 仍看得到啟用上下文。

### 自然語言啟用

Agent 在執行中判斷任務需要某個 skill 時，直接呼叫 `activate_skill`。這是 LLM-initiated 路徑，與斜線版同 render 管線。

> Skill 啟用刻意設計為 **tool call**（lazy load），而非啟動時預先選擇 —— 避免為不需要 skill 的任務支付 skill body token。

### 同一對話多 Skill

同一對話可依序啟用多個 skill。每次 `activate_skill` 在既有指令堆上追加；後啟用的 skill 透過 system prompt section 順序覆蓋或補充先前的。

## User message 是 binding context

`skill_execution.md` Mandatory Principle #5：觸發 skill 的 user message 是 **binding context，不是雜訊**。LLM 把它當成 user-supplied parameters/hints 織進輸出。

具體：

- SKILL.md 描述 default behavior
- User message override／augment default
- 「SKILL.md 內步驟是 commands」**不是**單向剛性解讀

範例：`/readme-generate private MIT` —— SKILL.md 定義 README 結構；user message 指定 private 模式 + MIT 授權，兩者都 override default。

## Skill 位置

Skill 放 `extensions/skills/<name>/`：

```
extensions/skills/code-reviewer/
├── SKILL.md            # skill 定義（frontmatter + body）
└── ...                 # 選用的輔助 script / template
```

Agenvoy 啟動時掃描此目錄。System prompt 的 `## Skills` 區段由 `skillTool.ListBlock` 動態填入，讓 LLM 知道有哪些 skill 可用。

## Skill 執行 Prompt

執行迴圈由 `configs/prompts/skill_execution.md` 驅動，內含每個 skill 都遵守的規則（輸出紀律、tool name mapping、mandatory principles）。

Tool name mapping 範例：外部 skill 可能引用 Anthropic SDK 的 `AskUserQuestion`；agenvoy 透過 `skill_execution.md` 的 **Tool Name Mapping** 表自動映射至 `ask_user`，**不需**在 Go 端註冊 alias。

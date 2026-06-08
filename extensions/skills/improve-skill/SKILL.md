---
name: improve-skill
description: >
  Improve a skill's SKILL.md and scripts/*.md based on execution trace errors.
  Fixes tool-name references, unclear steps that caused failures, and wording issues.
  Writes corrected files to ~/.config/agenvoy/skills/<name>/.
---

# Improve Skill

根據執行軌跡中的錯誤，改進指定 skill 的 SKILL.md 與 scripts/*.md，寫入 `~/.config/agenvoy/skills/<name>/`。

## Input Format

使用者訊息包含：
- Skill name
- Skill source path
- Execution trace（tool 呼叫序列 + 錯誤）

## Steps

1. 從使用者輸入取得 skill name 與 source path
2. 使用 `read_file` 讀取 source path 下的 SKILL.md
3. 使用 `run_command` 執行 `find <source_path>/scripts -name '*.md' -type f 2>/dev/null` 列出 script 檔案
4. 若有 script 檔案，逐一使用 `read_file` 讀取
5. 分析 execution trace 中的錯誤，對照 skill 步驟找出問題
6. 依 **Improvement Rules** 修正檔案內容
7. 使用 `write_skill` 寫入 `<name>/SKILL.md` 與所有 `<name>/scripts/*.md`（路徑相對於 skills dir）
8. 僅輸出修改摘要

## Improvement Rules

### Based on Execution Trace

- 步驟引用的 tool name 不在 Built-in Tools 清單中 → 替換為正確名稱
- 步驟導致重複錯誤 → 加入 fallback 策略或移除不必要步驟
- 步驟描述不清導致 LLM 錯誤調用 → 改寫更明確的指令
- 步驟順序導致 dependency 缺失 → 調整順序

### Tool Name Mapping

| 錯誤引用 | 正確 tool name |
|---|---|
| Bash / bash / Shell / Terminal / run shell | `run_command` |
| AskUserQuestion / ask the user / prompt user | `ask_user` |
| Read / Read file / Read tool | `read_file` |
| Write / Write file / Write tool (skill files) | `write_skill` |
| Edit / Edit file / patch (skill files) | `patch_skill` |
| Write / Write file / Write tool (other files) | `write_file` |
| Edit / Edit file / patch (other files) | `patch_file` |
| List files | `list_files` |
| Find files / glob | `glob_files` |
| Search file content / grep / Grep | `search_content` |
| Search web / WebSearch | `search_web` |
| Fetch page / WebFetch | `fetch_page` |

**替換原則：**

- 出現在步驟指令中的工具名稱（如「使用 Bash 工具執行」）→ 替換為「使用 `run_command` 執行」
- 出現在 backtick 內的工具名稱（如 `` `Bash` ``）→ 替換為 `` `run_command` ``
- 出現在說明文字中的自然語言引用 → 替換為 backtick 包裹的正確 tool name
- YAML frontmatter 中的 description 如引用工具名稱 → 同樣修正
- 系統提示注入的 **Built-in Tools** 清單為權威來源；對照表未涵蓋的工具以該清單為準

### Wording Fixes

- 修正明顯的錯字、語法錯誤、語意不清的敘述
- 保留原始語言（中文保持中文、英文保持英文）
- 不改變 skill 的核心目的與功能
- 不新增 trace 中未暗示的功能

### Preserve

- YAML frontmatter 結構（name, description 欄位格式）
- `scripts/` 路徑引用保持原樣（runtime 自動解析）
- 步驟編號與層級
- 範例 code block 內的內容（除非 code block 本身引用了錯誤 tool name）

## Output

修改清單，格式：

```
improved <N> file(s) → ~/.config/agenvoy/skills/<name>/:
- <relative path>: <一句話說明改了什麼>
- <relative path>: <一句話說明改了什麼>
```

若所有檔案都無需修正，輸出：`no changes needed`

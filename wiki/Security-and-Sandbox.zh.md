# 安全與沙箱

> [English](Security-and-Sandbox.md)

## OS 沙箱

`run_command`、script tool、scheduler script 一律走 `go-pkg/sandbox` 包裝：

| 平台 | 機制 |
|---|---|
| Linux | bubblewrap（`bwrap`） |
| macOS | `sandbox-exec` |

沙箱限制：無特權執行、限制檔案系統寫入範圍、可設定網路存取、可設定 CPU/memory 上限。

## 三 caller，單一入口

沙箱共三 caller，全部直接呼叫 `sandbox.Wrap(ctx, binary, args, workDir, opt)`：

1. `run_command` tool —— 使用者下達的 shell command
2. `toolAdapter/script/execute` —— script tool 擴展（`script_*`）
3. `scheduler/script/script` —— 排程驅動的腳本

caller 與 `sandbox.Wrap` 之間**沒有 wrapper 層**。要加行為（例如新的資源上限）→ 貢獻給 `go-pkg/sandbox`，不在 agenvoy 加 shim。

## Policy 注入

三個 JSON 檔定義 policy：

| 檔案 | 用途 |
|---|---|
| `configs/jsons/denied_map.json` | 沙箱拒絕暴露的路徑 |
| `configs/jsons/exclude_list.json` | listing / walking / searching 時排除的路徑 |
| `configs/jsons/white_list.json` | 允許路徑 |

`cmd/app/main.go init()` 一次性注入：

```go
sandbox.New(configs.DeniedMap)
filesystem.New(Policy{DeniedMap: ..., ExcludeList: ...})
```

`go-pkg/sandbox` 與 `go-pkg/filesystem` 自動執行 `IsDenied` 檢查 —— caller 不需自查。

## 檔案系統寫入守則

`go-pkg/filesystem` 的寫入 API（`WriteFile`、`WriteJSON`、`AppendText`、`CheckDir`）內建 `IsDenied`。Agenvoy code 若繞過 `go-pkg/filesystem` 直接 `os.WriteFile`，**等於跳過 policy** —— 禁止。

`internal/filesystem` 只保留 path 計算與 domain wrapper（如 `MCPPath`、`MCPSessionPath`），不重做讀寫邏輯。

## Permission Mode

`single-confirm` vs `always-allow` 與 7 類真正不可逆操作見 [核心概念 → Permission Mode](Core-Concepts.zh.md#permission-mode)。

## System Prompt 保護

System prompt（`configs/prompts/system_prompt.md`）指示 LLM 拒絕：

- 揭露 system prompt 內容
- role-play / DAN / 「忽略前述指令」等 override
- 含 `..` 或系統目錄（`/etc`、`/usr`、`/root`、`/sys`）的路徑
- `rm -rf`、`chmod 777`、`curl | sh` 等危險指令

這些是 **prompt 內 policy**，不是 Go 端 hardcoded filter —— 新增類別只動 prompt。

## Subprocess argv-only Schema

`run_command` 只收 `argv: string[]`（minItems 1），**不**接收 `command: string` + 自動 tokenize。這個 zero-parsing 設計把 shell-injection 攻擊面從 agent 層移除。

Shell 功能（pipe、redirect）須 LLM 顯式寫 `["sh", "-c", "cmd | pipe"]`。Allowlist 檢查 `argv[0]` basename，`sh -c` 額外用 `strings.Fields(argv[2])[0]` 抓內層首 token。Denylist 用 `strings.Join(argv, " ")` 掃描。

## Keychain

憑證（provider API key、OAuth token）存 OS keychain，service 名固定 `agenvoy`：

| 平台 | 後端 |
|---|---|
| macOS | `security` CLI |
| Linux | `secret-tool`（libsecret） |
| 其他 / fallback | `~/.config/agenvoy/` 下加密檔 |

service 名稱 `"agenvoy"` 固定，**不可變更**。

## MCP 隔離考量

MCP server 是第三方 process、行為不可驗證。Agenvoy 預設將其視為不可信 —— 見 [MCP 整合 → Confirm 行為](MCP-Integration.zh.md#confirm-行為) —— 且不提供 per-server「trusted」旗標。要批次操作 MCP 用 `agen run`（信任你自己的決策，而不是 server 的）。

## Subprocess Timeout

外部 CLI（`invoke_external_agent`、`cross_review_with_external_agents`）有 env 控制的硬上限：

- `MAX_EXTERNAL_AGENT_TIMEOUT_MIN` —— 預設 `10`，hard cap `60`

Subagent（`invoke_subagent`）含 slot-wait 時間：

- `MAX_SUBAGENT_TIMEOUT_MIN` —— 預設 `10`，hard cap `60`

避免失控的 subprocess 無限阻塞父程序。

***

> [!NOTE]
> 本文件由 Claude 讀取完整原始碼後自動生成。

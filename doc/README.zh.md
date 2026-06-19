<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>跑在你電腦上的個人 AI Agent</strong>
</p>

<p align="center">
  一句話建工具，自動測試，直接呼叫。<br>
  讓你現有的 Agent 也能自己造工具。<br>
  你說一句話，剩下 Agent 幫你完成。
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/pardnchiu/agenvoy"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
  <a href="https://app.codecov.io/github/pardnchiu/agenvoy/tree/master"><img src="https://img.shields.io/codecov/c/github/pardnchiu/agenvoy/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
  <a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
  <a href="../LICENSE"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <a href="../README.md">English</a> · <strong>繁體中文</strong>
</p>

## 一鍵安裝

> MacBook 建議額外執行 `sudo pmset -c sleep 0`，避免休眠影響排程。

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

***

## 你可以這樣用它

<table>
<tr>
<td width="50%" valign="top">

### 查資料

> 台北天氣如何？
> 
> Agent 找資料、呼叫工具、整理結果後回答你。
> 
> 如果沒有工具，它會自己建立。

</td>
<td width="50%" valign="top">

### 建立自動化

> 每天早上 8 點報台積電股價
>
> Agent 會確認：
> - 要推送到哪裡
> - 要什麼格式
> - 什麼時間執行
> 
> 確認後自動建立排程。

</td>
</tr>
<tr>
<td>

[![](https://i.ytimg.com/vi/floMBsAfziY/maxresdefault.jpg)](https://youtu.be/floMBsAfziY)

</td>
<td>

[![](https://i.ytimg.com/vi/5To3joKlFpU/maxresdefault.jpg)](https://youtu.be/5To3joKlFpU)

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 搜尋你的文件

> 找出去年所有報價單
>
> 哪份文件提到 Prompt 指南？
>
> Agent 直接從你的文件搜尋答案。

</td>
<td width="50%" valign="top">

### 完成長流程工作

> 幫我整理今天 GitHub Commit 並產生進度摘要
>
> Agent 可以拆解任務、呼叫工具、整合結果，再回覆給你。

</td>
</tr>
<tr>
<td>

[![](https://i.ytimg.com/vi/vqoQ6Qvl8qU/maxresdefault.jpg)](https://youtu.be/vqoQ6Qvl8qU)

</td>
<td>

[![](https://i.ytimg.com/vi/nIV1xz_HIJg/maxresdefault.jpg)](https://youtu.be/nIV1xz_HIJg)

</td>
</tr>
</table>

### 讓你正在用的 AI Agent 擁有自建工具的能力

> Agenvoy 同時是 MCP server。
> 
> Claude Code、Codex、OpenCode 等 AI Agent 直接連上，就能：
> - 使用你所有的沙箱工具
> - 找不到工具時自動建立新的
> - 建好的工具所有 Agent 共用
> 
> 一行設定，工具庫即時共享。
> 影片中建立的範例：[`fetch_weather`](demo/fetch_weather/)、[`fetch_crypto_price`](demo/fetch_crypto_price/)

<table>
<tr>
<td width="33%" valign="top">

#### Claude Code 建立天氣工具 (1)

</td>
<td width="33%" valign="top">

#### Codex 複用天氣工具，再建加密貨幣工具 (2)

</td>
<td width="33%" valign="top">

#### Agenvoy 測試兩個工具 (3)

</td>
</tr>
<tr>
<td>

[![](https://i.ytimg.com/vi/on5IaoxBO1E/maxresdefault.jpg)](https://youtu.be/on5IaoxBO1E)

</td>
<td>

[![](https://i.ytimg.com/vi/2DDFCIcbnso/maxresdefault.jpg)](https://youtu.be/2DDFCIcbnso)

</td>
<td>

[![](https://i.ytimg.com/vi/KPs4o9xDFjM/maxresdefault.jpg)](https://youtu.be/KPs4o9xDFjM)

</td>
</tr>
</table>

***

## 核心能力

| 能力 | 說明 |
| :- | :- |
| 自動工具生成 | 缺工具時自行建立並保存 |
| 自我排程 | 一句話建立定時任務 |
| 長期記憶 | 保留重要資訊與上下文 |
| 文件搜尋 | 從本機文件回答問題 |
| Sub-Agent | 多 Agent 協作 |
| MCP client | 連接外部 MCP 服務 |
| MCP server | 讓任何 AI Agent 使用你的沙箱工具 |
| Tool Market | 分享與安裝工具 |
| 語音轉錄 | 音訊與影片轉文字 |
| 自我改進 | 執行失敗後自動修正 |

***

## 跟其他工具比

| | **Agenvoy** | OpenClaw | Hermes-agent |
|---|---|---|---|
| 安裝方式 | 一行指令，單一檔案 | pnpm monorepo | pip + docker |
| 多模型同時用 | 自動選 | 手動切 | 手動切 |
| 對話 UI | 按鈕／選單／modal | 純文字 | 純文字 |
| 自己生成工具 | ✅ | ❌ | ⚠️ 僅 skill |
| 聊天驗證 | 6 碼驗證碼 | 人工核准 | 人工核准 |
| 跨 session 推送 | ✅ | ❌ | ⚠️ 有限 |
| 文件搜尋 | 語意＋關鍵字 | 僅對話記憶 | 僅對話記憶 |

***

## 文件

- [新手入門](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Getting-Started.zh.md)
- [架構](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Architecture.zh.md)
- [核心概念](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Core-Concepts.zh.md)
- [Provider 設定](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Providers.zh.md)
- [工具系統](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Tools.zh.md)
- [記憶系統](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Memory-System.zh.md)
- [Skill 系統](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Skill-System.zh.md)
- [MCP 整合](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/MCP-Integration.zh.md)
- [安全與沙箱](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Security-and-Sandbox.zh.md)
- [命令列參考](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/CLI-Reference.zh.md)
- [設定檔](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Configuration.zh.md)
- [產品對照](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Comparison.zh.md)

## License

本專案以 [Apache License 2.0](../LICENSE) 授權。

## 社群貢獻者

<a href="https://github.com/pardnchiu/Agenvoy/issues/3">
  <img src="https://github.com/Azetry.png" width="40" height="40" alt="Azetry" style="border-radius:50%" />
</a>
<a href="https://github.com/pardnchiu/agenvoy/issues/49">
  <img src="https://github.com/oceanasd.png" width="40" height="40" alt="oceanasd" style="border-radius:50%" />
</a>

## Contributor

歡迎 [開 issue](https://github.com/pardnchiu/agenvoy/issues/new) 分享想法。

<a href="https://github.com/pardnchiu/agenvoy/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=pardnchiu/agenvoy&cache_bust=2026-05-12" alt="Agenvoy contributors" />
</a>

## Star History

<a href="https://star-history.com/#pardnchiu/agenvoy&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&theme=dark&cache_bust=2026-05-12" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
    <img alt="Agenvoy star history" src="https://api.star-history.com/svg?repos=pardnchiu/agenvoy&type=Date&cache_bust=2026-05-12" />
  </picture>
</a>

曲線往上走，就是我們想要的訊號。按 ★ 推一把。

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)

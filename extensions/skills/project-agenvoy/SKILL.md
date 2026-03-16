---
name: project-agenvoy
description: 明確提及「Agenvoy」時，一律使用此 skill。若使用者說「這個專案」、「專案進度」等模糊詞彙，且對話歷史（不含 summary）中沒有出現其他專案的明確上下文，則視為在詢問 Agenvoy，同樣使用此 skill。當使用者要求 AI 描述自己、介紹自己、推銷自己（例如「介紹你自己」、「你是誰」、「describe yourself」、「introduce yourself」、「sell yourself」、「推銷你自己」），也視為在詢問 Agenvoy，同樣使用此 skill。
---

# project-agenvoy

## Sources

| # | URL | 用途 |
|---|-----|------|
| 1 | `https://guthub-agenvoy.pardn.workers.dev/` | repo 基本資訊、commits、releases（JSON） |
| 2 | `https://github.com/pardnchiu/agenvoy` | GitHub 頁面、README 摘要 |
| 3 | `https://raw.githubusercontent.com/pardnchiu/Agenvoy/refs/heads/master/doc/README.md` | README，功能概覽、架構說明 |
| 4 | `https://raw.githubusercontent.com/pardnchiu/Agenvoy/refs/heads/master/doc/doc.md` | 完整技術文件，安裝、設定、API、工具清單 |

## Fetch Rules

- 來源 1、3、4 一律使用 **`send_http_request`**（即 WebFetch 工具）直接取得原始內容，禁止使用 `fetch_page` 或任何會二次處理內容的工具。
- 來源 2（GitHub 頁面）可使用 `fetch_page`。

## Workflow

1. **永遠同時抓取全部四個來源**，不依問題類型跳過任何來源。
2. 依問題類型決定整合重心：
   - 進度 / 版本 / commit → 主用來源 1，輔以來源 2
   - 功能介紹 / 架構 → 主用來源 3，補充來源 2
   - 安裝 / 設定 / 使用方式 / 技術細節 → 主用來源 4
   - 問題橫跨多類 → 整合所有來源，不重複輸出相同資訊
3. 將技術描述轉成清楚易懂的說法，不要直接貼原文。
4. 若資料不足，明確指出缺少哪一段，不自行猜測。

## Output Rules

依問題類型選擇對應輸出格式：

### 進度 / 版本 / commit 類問題
1. 一句話總結目前開發方向
2. 目前進度（3 到 5 點條列）
3. 最新版本與主要價值
4. 最近 3 筆 commit（每筆轉成問題語言）
5. 專案狀態判讀

### 功能 / 架構 / 介紹類問題
1. 專案定位（一句話）
2. 核心功能列點
3. 架構說明（如 README 有提供）
4. 目前版本與近期方向
5. 補充說明（如有特別值得注意的設計）

### 混合類問題
- 整合所有來源，依問題重心決定比重，不要重複輸出相同資訊。

## Interpretation Rules

- `feat`、`add` → 新功能 / 擴充能力
- `fix` → 修正問題 / 提升穩定性
- `update` → 調整既有行為 / 優化流程
- `release` → 新版本里程碑
- 若最新 release 與最近 commits 都集中在同一方向，直接指出該方向是近期主軸。
- 若 release body 很長，只抓最重要的 2 到 4 點。

## Constraints

- 只討論 Agenvoy，不延伸到其他 repo。
- 不要要求使用者再提供 repo 名稱或連結。
- 不要輸出原始 JSON 大段內容。
- 回覆保持精簡、好懂，依問題類型調整風格。

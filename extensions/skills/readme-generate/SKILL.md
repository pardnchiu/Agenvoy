---
name: readme-generate
description: 從原始碼分析自動生成雙語 README。當使用者請求為專案建立 README、需要從程式碼庫生成 README.md（英文）和 README.zh.md（中文）、或希望為其函式庫/套件建立一致的多語言文件時使用。
---

# README 產生器

透過分析專案原始碼來生成專業的雙語 README 文件。

## 指令語法

```
/readme-generate [private] [LICENSE_TYPE] [REPO_PATH] [--only <targets>]
```

### 參數（全部為選填）

| 參數 | 格式 | 範例 | 行為 |
|-----------|--------|---------|----------|
| `private` | 關鍵字 | `private` | 生成時不包含徽章和星標歷史 |
| `LICENSE_TYPE` | 授權識別碼 | `MIT`、`Apache-2.0` | 生成 LICENSE 檔案 |
| `REPO_PATH` | `github.com/{owner}/{repo}` | `github.com/foo/bar` | 覆蓋預設的擁有者/儲存庫 |
| `--only <targets>` | 逗號分隔的目標清單 | `--only readme`、`--only doc,architecture` | 僅重新生成指定檔案集，保留其他檔案與 LICENSE 不動 |

### `--only` 目標對應

| Target（不區分大小寫） | 重新生成檔案 |
|---|---|
| `readme` | `README.md` + `doc/README.zh.md` |
| `doc` | `doc/doc.md` + `doc/doc.zh.md` |
| `architecture` | `doc/architecture.md` + `doc/architecture.zh.md` |

**未指定 `--only` = 全部重新生成 + 必要時寫入 LICENSE。**

**`--only` 存在時的覆寫保護：**

- 明確未指定的目標檔案**不得讀取、不得覆寫**，包含 LICENSE
- 有 `--only` 時 **忽略 `LICENSE_TYPE`**（即使傳入也不產生 LICENSE 檔）
- `private` 與 `REPO_PATH` 仍套用到被重新生成的檔案上

### 參數偵測規則

| 模式 | 偵測為 |
|---------|-------------|
| `private`（不區分大小寫） | `PRIVATE_MODE` 旗標 |
| 包含 `github.com/` | `REPO_PATH` |
| 符合已知授權類型（不區分大小寫） | `LICENSE_TYPE` |
| `--only <targets>` 或 `--only=<targets>` | `ONLY_TARGETS`（逗號分隔，大小寫不敏感） |

**順序獨立**：位置參數可以任意順序出現；`--only` 與其值視為一組。

### 支援的 LICENSE 類型

| 類型 | 別名（不區分大小寫） |
|------|----------------------------|
| MIT | `mit` |
| Apache-2.0 | `apache`、`apache2`、`apache-2.0` |
| GPL-3.0 | `gpl`、`gpl3`、`gpl-3.0` |
| BSD-3-Clause | `bsd`、`bsd3`、`bsd-3-clause` |
| ISC | `isc` |
| Unlicense | `unlicense`、`public-domain` |
| Proprietary | `proprietary`（隱含 `private` 模式） |

### 範例

```bash
/readme-generate                                  # 全部重生成 + MIT（若無 LICENSE）
/readme-generate MIT                              # 全部重生成 + MIT LICENSE
/readme-generate private                          # 全部重生成，不含徽章/星標歷史
/readme-generate private MIT                      # 私有模式 + MIT LICENSE
/readme-generate proprietary                      # 私有模式 + Proprietary LICENSE
/readme-generate github.com/foo/bar               # 全部重生成，使用自訂儲存庫路徑
/readme-generate --only README                    # 只重生成 README.md + doc/README.zh.md
/readme-generate --only doc,architecture          # 只重生成 doc 與 architecture 雙語四檔
/readme-generate private --only README            # 僅 README + 套用 private 模式（不動 LICENSE）
```

---

## Step 0：作者設定（強制性 - 第一步）

**在執行任何動作前，必須先載入或建立作者設定。所有載入與建立都透過 `scripts/setup_config.py` 腳本完成。**

### 設定檔位置

```
~/.skill-readme-generate.json
```

### 設定檔格式

```json
{
  "author_name": "張三 John Doe",
  "author_email": "dev@example.com",
  "author_url": "https://linkedin.com/in/johndoe",
  "github_owner": "johndoe"
}
```

### 執行協議（必須嚴格遵守）

**Step 0.1：檢查設定檔**

```bash
python3 ~/.claude/skills/readme-generate/scripts/setup_config.py check
```

| Exit Code | stdout | 意義 | 下一步 |
|-----------|--------|------|--------|
| `0` | 單行 JSON | 設定存在且完整 | 解析 JSON，載入 `{author_name}`、`{author_email}`、`{author_url}`、`{github_owner}`，跳到 Step 1 |
| `1` | （無） | 設定缺失或欄位不完整 | 進入 Step 0.2 |

**Step 0.2：收集使用者輸入**

當 Step 0.1 回傳 exit code `1` 時，**必須使用 `AskUserQuestion` 工具**詢問下列四個欄位（順序固定，全部必填）：

| 欄位 | 提示文字 | 範例 |
|------|----------|------|
| `author_name` | 作者姓名（顯示於 README Author 區段） | `張三 John Doe` |
| `author_email` | 聯絡 Email | `dev@example.com` |
| `author_url` | 個人連結（LinkedIn、GitHub、個人網站皆可） | `https://linkedin.com/in/johndoe` |
| `github_owner` | GitHub 使用者名稱（用於預設 `{owner}` 與頭像） | `johndoe` |

**Step 0.3：寫入設定**

將使用者的四個答案按順序傳給腳本的 `write` 子指令：

```bash
python3 ~/.claude/skills/readme-generate/scripts/setup_config.py write \
    "{author_name}" "{author_email}" "{author_url}" "{github_owner}"
```

腳本會將設定寫入 `~/.skill-readme-generate.json` 並將該 JSON 印到 stdout（供 Claude 解析載入）。後續執行可直接由 Step 0.1 讀取，不再詢問。

### 手動初始化（使用者自行執行）

使用者可在終端直接執行腳本以互動方式建立設定（適合首次設定或手動重置）：

```bash
python3 ~/.claude/skills/readme-generate/scripts/setup_config.py
```

若檔案已存在，腳本會直接印出現有設定；若不存在則以 `input()` 逐欄詢問。

### 覆蓋機制

- 若指令列傳入 `REPO_PATH`（含 `github.com/{owner}/{repo}`），則 `{owner}` 使用指令列的值，其餘作者欄位仍從設定檔取得
- 使用者可隨時手動編輯 `~/.skill-readme-generate.json` 更新資訊，或刪除該檔以觸發重新設定

---

## 關鍵：必要輸出

**預設（無 `--only`）務必生成六個檔案：**

| 檔案 | 語言 | 用途 | Target 歸屬 |
|------|----------|---------|------|
| `README.md` | 英文 | 主要文件（精簡，特色驅動） | `readme` |
| `doc/README.zh.md` | 繁體中文（ZH-TW） | 中文文件（精簡，特色驅動） | `readme` |
| `doc/doc.md` | 英文 | 詳細技術文件（安裝、使用、參考） | `doc` |
| `doc/doc.zh.md` | 繁體中文（ZH-TW） | 中文詳細技術文件 | `doc` |
| `doc/architecture.md` | 英文 | 詳細架構圖（完整 Mermaid） | `architecture` |
| `doc/architecture.zh.md` | 繁體中文（ZH-TW） | 中文詳細架構圖 | `architecture` |

**README 負責吸引人，doc 負責教會人，architecture 負責畫清楚。**

**`--only` 指定時**：僅輸出指定 target 的對應檔案集；未指定的目標檔案維持原狀不得覆寫，LICENSE 亦不處理。

---

## 參數（從專案中提取）

| 參數 | 來源 | 範例 |
|-------|--------|---------|
| `{owner}` | `REPO_PATH` 覆蓋 > `~/.skill-readme-generate.json` 的 `github_owner` > `git remote` | `johndoe` |
| `{author_name}` | `~/.skill-readme-generate.json` 的 `author_name` | `張三 John Doe` |
| `{author_email}` | `~/.skill-readme-generate.json` 的 `author_email` | `dev@example.com` |
| `{author_url}` | `~/.skill-readme-generate.json` 的 `author_url` | `https://linkedin.com/in/johndoe` |
| `{avatar_url}` | `https://github.com/{owner}.png` | `https://github.com/johndoe.png` |
| `{repo}` | `REPO_PATH` 覆蓋或資料夾名稱或 `git remote get-url origin` | `go-scheduler` |
| `{package}` | `package.json` name、`go.mod` module、`pyproject.toml` name | `@aspect/utils` |
| `{year}` | 現有 README 年份或 `git log --reverse --format=%ai \| head -1` 或當前年份 | `2024` |

**優先順序**：指令列 `REPO_PATH` > `~/.skill-readme-generate.json` > 本地 git remote > 資料夾名稱

---

## 工作流程

```
0.  作者設定  →  `scripts/setup_config.py check` 讀取 ~/.skill-readme-generate.json；缺失則 AskUserQuestion + `write` 子指令建立
1.  解析      →  從指令中提取 PRIVATE_MODE、LICENSE_TYPE、REPO_PATH、ONLY_TARGETS
                 - 若 ONLY_TARGETS 非空 → `LICENSE_TYPE` 強制忽略；目標集 = 使用者指定
                 - 若 ONLY_TARGETS 空 → 目標集 = {readme, doc, architecture}
2.  分析      →  在目標專案上執行 analyze_project.py
3.  提取      →  從專案取得 {repo}、{package}、{year}（或使用 REPO_PATH 覆蓋）
4.  檢視      →  檢查現有文件、LICENSE、範例
5.  選特色    →  從分析結果中提煉出所有精妙且具代表性的專案特色
5.5 選圖示    →  WebFetch https://skillicon-list.pardn.dev/ 取得 skillicons ID 列表，從專案分析挑出對應 ID（可選區段，無對應則省略）
6.  生成 readme        →  僅當 `readme` ∈ 目標集：先建立 doc/README.zh.md，再翻譯為 README.md
7.  生成 doc           →  僅當 `doc` ∈ 目標集：先建立 doc/doc.zh.md，再翻譯為 doc/doc.md
8.  生成 architecture  →  僅當 `architecture` ∈ 目標集：先建立 doc/architecture.zh.md，再翻譯為 doc/architecture.md
9.  授權      →  僅當 ONLY_TARGETS 為空時執行：
                 - 若指定 LICENSE_TYPE → 使用指定類型
                 - 若無 LICENSE 檔案且未指定 → 預設生成 MIT LICENSE
                 - 有 ONLY_TARGETS 時完全跳過，不讀取、不覆寫 LICENSE
10. 驗證      →  僅驗證目標集對應的檔案；未在目標集的檔案視為「未觸碰」跳過
11. 儲存      →  README.md 寫入專案根目錄；其餘檔案寫入 doc/ 子目錄（自動建立）
```
```

---

## 步驟 1：分析專案

```bash
python3 /mnt/skills/user/readme-generator/scripts/analyze_project.py /path/to/project
```

輸出：包含語言、名稱、版本、類型、函式、相依性的 JSON。

---

## 步驟 2：提取參數

```bash
# 如果提供 REPO_PATH，解析它：
# github.com/owner/repo → {owner}=owner, {repo}=repo

# 否則回退到：

# 取得儲存庫名稱
basename $(pwd)
# 或
git remote get-url origin | sed 's/.*\/\([^\/]*\)\.git/\1/'

# 取得首次提交年份
git log --reverse --format=%ai | head -1 | cut -d'-' -f1

# 取得套件名稱（Node.js）
jq -r '.name' package.json

# 取得模組名稱（Go）
grep '^module' go.mod | awk '{print $2}'
```

---

## 步驟 3：提煉專案特色

**這是整份 README 的核心。** 從分析結果中識別出所有精妙且能區分此專案的特色。

### 特色選擇原則

| 優先級 | 類型 | 範例 |
|--------|------|------|
| 最高 | 解決的核心問題 | 「透過統一 Agent 介面支援 5 家 AI 後端」 |
| 高 | 獨特的技術手段 | 「安全指令執行：rm 自動移至 .Trash」 |
| 中 | 開發者體驗差異 | 「互動式確認機制，支援 --allow 跳過」 |
| 低 | 通用能力 | 「支援多種語言」（太泛，避免） |

### 規則

- **數量限制：3–5 個**。少於 3 個代表提煉不夠深入，超過 5 個會稀釋焦點
- 若候選特色超過 5 個，**依優先級表合併或刪除**，保留最具區分度的 3–5 個
- 每個特色：一行標題（≤15 字）+ **一句話說明**（簡潔扼要）
- **不提供 code snippet** — 純文字描述核心價值
- **不做細節展開** — 不列舉子功能、不解釋實作細節、不提供參數說明

---

## 步驟 4：生成中文版本

**使用精確的區段順序建立 `README.zh.md`：**

### 區段順序（強制性）

| 順序 | 區段 | 必要 | 公開模式 | 私有模式 |
|-------|---------|----------|-------------|--------------|
| 0 | LLM 生成通知 + `***` | **是** | ✓ | ✓ |
| 1 | 置中封面圖片 | 否 | ✓ | ✓ |
| 2 | 置中標語 + 置中徽章 + `***` | **是** | 標語 + 徽章 | **僅標語** |
| 3 | 簡短描述 | **是** | ✓ | ✓ |
| 4 | 目錄 | **是** | ✓ | ✓ |
| 5 | 功能特點（3–5 個精妙特色，list 呈現）| **是** | ✓ | ✓ |
| 6 | 技術堆疊（skillicons）| 否 | ✓ | ✓ |
| 7 | 架構（僅與特色相關）| 否 | ✓ | ✓ |
| 8 | 授權 | **是** | ✓ | ✓ |
| 9 | 作者 | **是** | ✓ | ✓ |
| 10 | 星標歷史 | 否 | ✓ | **跳過** |
| 11 | 版權頁尾 | 否 | ✓ | ✓ |

---

## 強制性區段（完全複製）

### 順序 0：LLM 生成通知 + 分隔線

**英文（README.md）：**
```markdown
> [!NOTE]
> This README was generated by [SKILL](https://github.com/pardnchiu/skill-readme-generate), get the ZH version from [here](./doc/README.zh.md).

***
```

**中文（README.zh.md）：**
```markdown
> [!NOTE]
> 此 README 由 [SKILL](https://github.com/pardnchiu/skill-readme-generate) 生成，英文版請參閱 [這裡](../README.md)。

***
```

**規則：**
- 通知後必須接 `***` 分隔線
- 若專案同時使用 coverage-generate，在通知中追加一行：`> Tests are generated by [SKILL](https://github.com/pardnchiu/skill-coverage-generate).`，使用 `<br>` 換行

### 順序 1：置中封面圖片（可選）

**若專案根目錄或 `doc/` 下存在 logo 圖片（如 `logo.svg`、`logo.png`），使用置中 HTML 格式：**
```markdown
<p align="center">
<picture>
<img src="./doc/logo.svg" alt="{repo}">
</picture>
</p>
```

### 順序 2：置中標語 + 置中徽章 + 分隔線

**公開模式：**
```markdown
<p align="center">
<strong>一句大寫英文標語描述專案核心價值</strong>
</p>

<p align="center">
<a href="{badge1_link}"><img src="https://img.shields.io/...?style=for-the-badge" alt="..."></a>
<a href="{badge2_link}"><img src="https://img.shields.io/...?style=for-the-badge" alt="..."></a>
</p>

***
```

**私有模式（僅標語）：**
```markdown
<p align="center">
<strong>一句大寫英文標語描述專案核心價值</strong>
</p>

***
```

**徽章規則：**
- **僅使用 `https://img.shields.io` 來源**，禁用其他徽章服務（如 `pkg.go.dev/badge`、`goreportcard.com/badge`）
- **必須使用 HTML `<a><img></a>` 格式**，不使用 markdown `[![]()]()`
- **所有徽章加上 `style=for-the-badge`**
- **所有徽章加上 `include_prereleases`**（如適用）
- 徽章放在同一個 `<p align="center">` 內，徽章之間用換行分隔
- 依語言追加對應的 shields.io 徽章（見下方「依語言的徽章範本」）
- 徽章後必須接 `***` 分隔線
- **標語規則：** 全大寫英文，簡短有力（如 `BUILD YOUR OWN OPENCLAW WITH AGENVOY!`），中文版也使用英文標語

### 順序 3：簡短描述

**使用固定格式，依據專案原始碼內容重新分析並生成，每次執行都必須更新。**

格式（英文）：
```
A [tech] [what it is] with [key feature 1], [key feature 2], and [key feature 3]
```
- 不超過 20 個英文單字，句尾不加句號
- `[tech]` = 主要技術框架或語言（e.g., `Go`, `Claude Code`, `Node.js`）
- `[what it is]` = 產品類型（e.g., `CLI tool`, `skill`, `library`）
- `[key feature 1~3]` = 從原始碼分析中提煉的 3 個最具代表性特色（名詞片語），用於簡短描述摘要

格式（中文）：
- 長度與英文版相當（約 20–26 個中文字）
- 結構對應英文，但以中文慣用語表達
- 句尾不加句號

```markdown
> A [tech] [what it is] with [key feature 1], [key feature 2], and [key feature 3]
```

```markdown
> [tech] [產品類型]，具備 [特色 1]、[特色 2] 與 [特色 3]
```

### 順序 4：目錄

**務必包含目錄區段。根據實際存在的區段動態生成。**

**中文（README.zh.md）：**
```markdown
## 目錄

- [功能特點](#功能特點)
- [技術堆疊](#技術堆疊)
- [架構](#架構)
- [授權](#授權)
- [Author](#author)
- [Stars](#stars)
```

**英文（README.md）：**
```markdown
## Table of Contents

- [Features](#features)
- [Built With](#built-with)
- [Architecture](#architecture)
- [License](#license)
- [Author](#author)
- [Stars](#stars)
```

**目錄生成規則：**

| 規則 | 描述 |
|------|-------------|
| 動態生成 | 根據文件中實際存在的區段生成目錄 |
| 錨點格式（EN） | 小寫，將空格替換為 `-`，移除特殊字元 |
| 錨點格式（ZH） | 保留原始中文字元作為錨點 |
| 跳過區段 | 不要在目錄中包含順序 0（LLM 通知）、順序 1（封面）、順序 11（頁尾） |
| 私有模式 | 從目錄中省略 `Stars` 項目 |

### 順序 5：功能特點（3–5 個精妙特色 — 核心區段）

**這是 README 最重要的區段。以 unordered list 呈現 3–5 個最具區分度的特色，每項用 `**粗體標題**` 讓項目明顯。純文字描述，不含 code snippet。**

格式（中文 README.zh.md）：

```markdown
## 功能特點

> `go install github.com/{owner}/{repo}/cmd/cli@latest` · [完整文件](./doc/README.zh.md)

- **特色標題 1**（≤15 字）— 一句話簡述此特色的核心價值。
- **特色標題 2** — 一句話簡述。
- **特色標題 3** — 一句話簡述。
- **特色標題 4** — 一句話簡述。（可選）
- **特色標題 5** — 一句話簡述。（可選）
```

格式（英文 README.md）：

```markdown
## Features

> `go install github.com/{owner}/{repo}/cmd/cli@latest` · [Documentation](./doc/doc.md)

- **Feature Title 1** — One-sentence description of core value.
- **Feature Title 2** — One-sentence description.
- **Feature Title 3** — One-sentence description.
- **Feature Title 4** — One-sentence description. (optional)
- **Feature Title 5** — One-sentence description. (optional)
```

**規則：**
- **數量：3–5 個**，少於 3 表示提煉不足，超過 5 會稀釋焦點
- 使用 markdown unordered list（`-`），每個 item 為一行
- 標題用 `**粗體**` 包起來提升可讀性，後接 em dash `—` 再接說明
- 純文字，不含 code snippet、不含 inline code 範例
- 每個特色僅一句話，聚焦在「解決什麼問題」而非「怎麼實作」
- 不要出現 Installation 或 Usage 的獨立區段
- 安裝指令（如需要）以單行 blockquote 形式放在 list 之前

### 順序 6：技術堆疊（skillicons - 可選）

**展示專案使用的技術堆疊、框架與工具，以置中的 [skillicons.dev](https://skillicons.dev) 圖示呈現。**

**中文（README.zh.md）：**
```markdown
## 技術堆疊

<a href="https://skillicons.dev">
  <img src="https://skillicons.dev/icons?i={ID1},{ID2},{ID3}&theme=light" />
</a>
```

**英文（README.md）：**
```markdown
## Built With

<a href="https://skillicons.dev">
  <img src="https://skillicons.dev/icons?i={ID1},{ID2},{ID3}&theme=light" />
</a>
```

**可用圖示 ID 列表**：

```
https://skillicon-list.pardn.dev/
```

**規則**：
- 執行前先 `WebFetch` 上述列表取得最新可用的 ID 集合
- ID 從專案分析結果推導：語言（go、ts、py、rust、swift、php…）、主要框架（react、vue、fastapi、gin…）、資料庫（postgres、redis、mongodb…）、容器（docker、kubernetes）、CI（githubactions）
- 僅選擇專案實際使用的工具，**不要湊數、不要列入無關的生態圈**
- 建議數量：**4–10 個**，超過 10 個視覺會過於擁擠
- 僅列出列表中存在的 ID；若專案使用的工具不在列表，**跳過該工具**（不自創 ID、不用別名）
- 若未偵測到可對應的 ID，**整個區段可省略**（此區段為可選，不強制輸出）
- 順序建議：語言 → 框架 → 資料庫 → 容器 / 部署 → CI / 工具
- 使用 `theme=light` 讓圖示在白底與 GitHub 深色底下都可讀

### 順序 7：架構（概覽版 + 連結詳細版）

**README 中的架構圖為精簡概覽版，詳細版放在 `doc/architecture.md`。**

**英文（README.md）：**
```markdown
## Architecture

> [Full Architecture](./doc/architecture.md)

\`\`\`mermaid
graph TB
    A[Input] --> B[Core]
    B --> C[Output]
\`\`\`
```

**中文（README.zh.md）：**
```markdown
## 架構

> [完整架構](./architecture.zh.md)

\`\`\`mermaid
graph TB
    A[輸入] --> B[核心]
    B --> C[輸出]
\`\`\`
```

**README 架構圖規則：**
- Node 數量 **≤ 10**，只畫主要模組間的關係
- 不展開內部細節，用單一 node 代表整個子系統
- 必須附上連結指向詳細版

### 順序 8：授權區段

**英文（README.md）：**
```markdown
## License

This project is licensed under the [MIT LICENSE](LICENSE).
```

**中文（README.zh.md）：**
```markdown
## 授權

本專案採用 [MIT LICENSE](LICENSE)。
```

### 順序 9：作者區段（從 `~/.skill-readme-generate.json` 套用）

**在兩個檔案中使用此格式，並以 `~/.skill-readme-generate.json` 的值替換 placeholder：**
```markdown
## Author

<img src="https://github.com/{owner}.png" align="left" width="96" height="96" style="margin-right: 0.5rem;">

<h4 style="padding-top: 0">{author_name}</h4>

<a href="mailto:{author_email}">{author_email}</a><br>
<a href="{author_url}">{author_url}</a>
```

**規則：**
- 頭像使用 `https://github.com/{owner}.png`（GitHub 自動產生）
- Email 與個人連結皆以**純文字超連結**呈現於姓名下方，不使用圖示
- 兩行之間以 `<br>` 換行，避免 markdown list 或段落間距
- ZH 與 EN 版本使用相同區段（`## Author` 不翻譯，與業界慣例一致）

### 順序 10：星標歷史區段（僅公開模式）

**在私有模式中完全跳過。**

```markdown
## Stars

[![Star](https://api.star-history.com/svg?repos={owner}/{repo}&type=Date)](https://www.star-history.com/#{owner}/{repo}&Date)
```

### 順序 11：版權頁尾

```markdown
***

©️ {year} [{author_name}]({author_url})
```

---

## 依語言的徽章範本（僅公開模式）

**所有徽章使用 HTML `<a><img></a>` 格式，加上 `include_prereleases&style=for-the-badge`。**

### Go
```html
<a href="https://pkg.go.dev/github.com/{owner}/{repo}"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
<a href="https://app.codecov.io/github/{owner}/{repo}/tree/master"><img src="https://img.shields.io/codecov/c/github/{owner}/{repo}/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
```

### Node.js
```html
<a href="https://www.npmjs.com/package/{package}"><img src="https://img.shields.io/npm/v/{package}?include_prereleases&style=for-the-badge" alt="npm"></a>
<a href="https://www.npmjs.com/package/{package}"><img src="https://img.shields.io/npm/dm/{package}?include_prereleases&style=for-the-badge" alt="Downloads"></a>
```

### Python
```html
<a href="https://pypi.org/project/{package}"><img src="https://img.shields.io/pypi/v/{package}?include_prereleases&style=for-the-badge" alt="PyPI"></a>
<a href="https://pypi.org/project/{package}"><img src="https://img.shields.io/pypi/pyversions/{package}?include_prereleases&style=for-the-badge" alt="Python"></a>
```

### 通用（在公開模式中始終包含）
```html
<a href="LICENSE"><img src="https://img.shields.io/github/v/tag/{owner}/{repo}?include_prereleases&style=for-the-badge" alt="Version"></a>
<a href="https://github.com/{owner}/{repo}/releases"><img src="https://img.shields.io/github/license/{owner}/{repo}?include_prereleases&style=for-the-badge" alt="License"></a>
```

---

## 翻譯指南

### ZH-TW 慣例（README.zh.md）

| 元素 | 規則 | 範例 |
|---------|------|---------|
| 技術術語 | 英文 + 中文註解（首次使用） | `Worker 池（Pool）` |
| 後續使用 | 僅中文 | `Worker 池` |
| 函式名稱 | 保持原樣 | `Enqueue()`、`Shutdown()` |
| 程式碼區塊 | 不變，僅翻譯註解 | `// 啟動佇列` |
| 區段標題 | 翻譯 | `## 安裝` 對應 `## Installation` |

### EN 慣例（README.md）

| 規則 | 範例 |
|------|---------|
| 主動語態 | "The queue processes tasks" 而非 "Tasks are processed" |
| 祈使語氣 | "Run the command" 而非 "You should run" |
| 一致的術語 | 整份文件使用相同術語 |
| 無括號翻譯 | "queue" 後不加 "（佇列）" |

---

---

## doc.md 詳細技術文件生成

**README 精簡呈現核心特色吸引讀者，doc.md 則提供完整的技術細節讓使用者上手。**

### doc.md 與 README 的分工

| 內容 | README | doc.md |
|------|--------|--------|
| 專案是什麼、為什麼用 | ✓ | ✗ |
| 核心特色（純文字） | ✓ | ✗ |
| 安裝步驟（詳細） | ✗ | ✓ |
| 使用方式（完整範例） | ✗ | ✓ |
| CLI/API/設定參考 | ✗ | ✓ |
| 環境變數 | ✗ | ✓ |
| 進階用法 | ✗ | ✓ |

### doc.md 區段順序（強制性）

| 順序 | 區段 | 必要 | 說明 |
|-------|---------|----------|------|
| 0 | 標題 + 返回連結 | **是** | 連結回 README |
| 1 | 前置需求 | **是** | 語言版本、系統需求、相依服務 |
| 2 | 安裝 | **是** | 完整安裝步驟含所有方式 |
| 3 | 設定 | 條件性 | 環境變數、設定檔（有才寫） |
| 4 | 使用方式 | **是** | 基礎 → 進階的完整範例 |
| 5 | 參考 | **是** | 依專案類型選擇對應的參考區段 |
| 6 | 版權頁尾 | 否 | 同 README |

### 順序 0：標題 + 返回連結

**英文（doc.md）：**
```markdown
# {repo} - Documentation

> Back to [README](../README.md)
```

**中文（doc.zh.md）：**
```markdown
# {repo} - 技術文件

> 返回 [README](./README.zh.md)
```

### 順序 1：前置需求

列出使用此專案所需的所有前置條件。

```markdown
## Prerequisites

- Go 1.20 or higher
- PostgreSQL 15+
- At least one AI agent credential (GitHub Copilot subscription or API key)
```

**規則：**
- 從 `go.mod`、`package.json`、`pyproject.toml` 等提取版本需求
- 列出外部服務相依（資料庫、API、第三方服務）
- 不解釋如何安裝這些前置條件（假設讀者知道）

### 順序 2：安裝

提供所有可用的安裝方式。

```markdown
## Installation

### From Source

\`\`\`bash
git clone https://github.com/{owner}/{repo}.git
cd {repo}
go build -o {repo} cmd/cli/main.go
\`\`\`

### Using go install

\`\`\`bash
go install github.com/{owner}/{repo}/cmd/cli@latest
\`\`\`
```

**規則：**
- 列出所有安裝方式（source、package manager、binary release）
- 每種方式都必須是可直接複製執行的完整指令
- 從 `Makefile`、`Dockerfile`、`docker-compose.yml` 等推斷安裝方式

### 順序 3：設定（條件性）

**僅在專案有環境變數、設定檔或需要初始化設定時才包含。**

```markdown
## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | No | OpenAI API key |
| `ANTHROPIC_API_KEY` | No | Anthropic API key |
| `DATABASE_URL` | Yes | PostgreSQL connection string |

### Config File

Copy `.env.example` and fill in the values:

\`\`\`bash
cp .env.example .env
\`\`\`
```

**規則：**
- 從 `.env.example`、`config.yaml`、原始碼中的 `os.Getenv()` 等提取
- 標明 Required / Optional
- 提供預設值（如有）

### 順序 4：使用方式

**從基礎到進階的完整使用範例。**

```markdown
## Usage

### Basic

\`\`\`bash
./agent-skills list
\`\`\`

### Advanced

\`\`\`bash
./agent-skills run readme-generate "generate readme" --allow
\`\`\`
```

**規則：**
- 程式碼範例必須是完整可執行的
- 包含預期輸出（如有意義）
- 從 Basic → Advanced 漸進
- 生產範例必須包含 error handling
- 始終指定語言識別碼以進行語法突顯
- 包含 import/require 陳述式
- ZH 版本中翻譯註解，程式碼不變

### 順序 5：參考

**依專案類型選擇對應的參考區段。這是 doc.md 中最重要的區段。**

#### 專案類型偵測

| 偵測信號 | 專案類型 |
|----------|----------|
| `main()` + `flag`/`cobra`/`argparse` | CLI 工具 |
| 僅匯出類型，無 `main()` | 函式庫 |
| `plugin`/`middleware`/`hook` 模式 | 框架 |
| `.config.json`/`.yaml` 範本 | 設定驅動 |
| `Info.plist` / `AppDelegate` | 桌面/行動應用 |

#### 依專案類型的參考區段標題

| 專案類型 | EN 標題 | ZH-TW 標題 |
|--------------|----------|-------------|
| 函式庫/SDK | `## API Reference` | `## API 參考` |
| CLI 工具 | `## CLI Reference` | `## 命令列參考` |
| 框架 | `## Interface Reference` | `## 介面參考` |
| 基於設定 | `## Configuration Reference` | `## 設定參考` |
| 桌面應用程式 | `## Preferences Reference` | `## 偏好設定參考` |

#### 參考區段內容

| 專案類型 | 應包含的內容 |
|----------|-------------|
| 函式庫/SDK | 匯出的類型、函式、方法及其簽章、參數、回傳值 |
| CLI 工具 | 指令表、旗標/選項表、子命令說明、環境變數 |
| 框架 | 生命週期鉤子、中介軟體介面、外掛 API |
| 基於設定 | 設定檔案結構及預設值和驗證規則 |

**CLI 工具參考範例：**

```markdown
## CLI Reference

### Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| `list` | `./app list` | List all installed skills |
| `run` | `./app run <skill> <input> [--allow]` | Execute the specified skill |

### Flags

| Flag | Description |
|------|-------------|
| `--allow` | Skip interactive confirmation prompts |

### Built-in Tools

| Tool | Parameters | Description |
|------|------------|-------------|
| `read_file` | `path` | Read file content at the specified path |
| `write_file` | `path`, `content` | Write or create a file |
```

**函式庫 API 參考範例：**

```markdown
## API Reference

### Agent Interface

\`\`\`go
type Agent interface {
    Send(ctx context.Context, messages []Message, toolDefs []tools.Tool) (*Output, error)
    Execute(ctx context.Context, skill *skill.Skill, userInput string, output io.Writer, allowAll bool) error
}
\`\`\`

`Send` handles a single API call. `Execute` manages the complete skill execution loop with up to 128 tool call iterations.

### NewScanner

\`\`\`go
func NewScanner() *Scanner
\`\`\`

Creates a skill scanner that concurrently scans all configured paths.
```

**規則：**
- 從 analyze_project.py 的 types 和 functions 輸出中提取
- 僅列出 exported / public API
- 每個函式/方法都需要簽章和一句話說明
- Table 格式優先，保持掃描性
- 複雜的 interface 可用 code block 展示完整簽章

### 順序 6：版權頁尾

```markdown
***

©️ {year} [{author_name}]({author_url})
```

---

## README 中連結 doc.md

**在 README 的功能特點區段安裝指令旁，加入 doc 連結：**

**中文（README.zh.md）：**
```markdown
## 功能特點

> `go install github.com/{owner}/{repo}/cmd/cli@latest` · [完整文件](./doc.zh.md)
```

**英文（README.md）：**
```markdown
## Features

> `go install github.com/{owner}/{repo}/cmd/cli@latest` · [Documentation](./doc/doc.md)
```

---

## Mermaid 圖表類型

| 類型 | 指令 | 使用案例 |
|------|-----------|----------|
| 流程圖（TB） | `graph TB` | 由上而下的流程 |
| 流程圖（LR） | `graph LR` | 由左至右的流程 |
| 序列圖 | `sequenceDiagram` | 互動序列 |
| 狀態機 | `stateDiagram` | 狀態轉換 |
| 類別圖 | `classDiagram` | 類型關係 |

---

## architecture.md 詳細架構文件生成

**README 的架構圖是概覽，architecture.md 負責完整呈現專案的模組關係與資料流。**

### architecture.md 與 README 架構的分工

| 內容 | README 架構 | architecture.md |
|------|-------------|-----------------|
| 主要模組概覽 | ✓（≤10 nodes） | ✓ |
| 模組內部結構 | ✗ | ✓ |
| 資料流 / 請求流程 | 簡化 | 完整 |
| 介面 / 型別關係 | ✗ | ✓ |
| 狀態轉換 | ✗ | ✓（如適用） |
| 第三方整合 | ✗ | ✓ |

### architecture.md 區段順序（強制性）

| 順序 | 區段 | 必要 | 說明 |
|-------|---------|----------|------|
| 0 | 標題 + 返回連結 | **是** | 連結回 README |
| 1 | 概覽圖 | **是** | 全域 Mermaid 圖（與 README 架構圖相同或稍詳細） |
| 2 | 模組詳細圖 | **是** | 每個核心模組一張 Mermaid 圖 |
| 3 | 資料流 | 條件性 | 請求 / 資料處理流程（如適用） |
| 4 | 狀態機 | 條件性 | 有限狀態機（如適用） |
| 5 | 版權頁尾 | 否 | 同 README |

### 順序 0：標題 + 返回連結

**英文（architecture.md）：**
```markdown
# {repo} - Architecture

> Back to [README](../README.md)
```

**中文（architecture.zh.md）：**
```markdown
# {repo} - 架構

> 返回 [README](./README.zh.md)
```

### 順序 1：概覽圖

**與 README 順序 7 的概覽圖相同或稍詳細，作為 architecture.md 的入口。**

```markdown
## Overview

\`\`\`mermaid
graph TB
    ...
\`\`\`
```

### 順序 2：模組詳細圖

**針對每個核心模組，各自畫一張 Mermaid 圖，展現內部結構與對外介面。**

```markdown
## Module: {ModuleName}

{一句話描述此模組的職責}

\`\`\`mermaid
graph TB
    subgraph {ModuleName}
        A[Component A] --> B[Component B]
        B --> C[Component C]
    end
    External[External Dependency] --> A
\`\`\`
```

**規則：**
- 每個模組一個 `## Module:` 區段
- 使用 `subgraph` 框出模組邊界
- 標示對外的 input / output 與相依
- 可使用 `classDiagram` 展示型別關係（如函式庫專案）

### 順序 3：資料流（條件性）

**當專案有明確的請求 / 資料處理流程時，使用序列圖或流程圖呈現。**

```markdown
## Data Flow

\`\`\`mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Service
    participant DB
    Client->>Handler: Request
    Handler->>Service: Process
    Service->>DB: Query
    DB-->>Service: Result
    Service-->>Handler: Response
    Handler-->>Client: Reply
\`\`\`
```

### 順序 4：狀態機（條件性）

**當專案有狀態管理邏輯時，使用狀態圖呈現。**

```markdown
## State Machine

\`\`\`mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Running: Start
    Running --> Done: Complete
    Running --> Failed: Error
    Failed --> Idle: Retry
    Done --> [*]
\`\`\`
```

### architecture.md 規則總結

- **不限 node 數量**，但每張圖保持可讀性（建議單圖 ≤ 30 nodes）
- 過大的圖拆成多張，每張聚焦一個模組或流程
- ZH 版本中 Mermaid node label 使用中文，圖表結構不變
- 從 `analyze_project.py` 的 types / functions 輸出推斷模組結構
- 先建立 `architecture.zh.md`（中文），再翻譯為 `architecture.md`

---

## LICENSE 生成

### 預設行為（無 LICENSE 檔案且未指定 LICENSE_TYPE）

**當專案目錄中不存在 LICENSE 檔案且使用者未指定 LICENSE_TYPE 時：**

1. **自動生成 MIT LICENSE 檔案**並儲存到專案根目錄
2. **在 README 授權區段使用預設文字**

### 明確指定 LICENSE_TYPE

**當使用者提供 `LICENSE_TYPE` 參數時，使用指定的授權類型。**

### LICENSE 範本

**開源授權的原文存放於 `scripts/licenses/`，來源為 [github/choosealicense.com](https://github.com/github/choosealicense.com) 的標準範本（SPDX 對應）。生成時讀取對應檔案並替換下列 placeholder 即可：**

| Placeholder | 替換來源 |
|-------------|----------|
| `{year}` | 專案首次提交年份（見步驟 2） |
| `{author_name}` | `~/.skill-readme-generate.json` 的 `author_name` |
| `{author_email}` | `~/.skill-readme-generate.json` 的 `author_email`（僅 Proprietary 使用） |

### 授權類型對應表

| LICENSE_TYPE | 檔案路徑 | 含 placeholder |
|--------------|----------|----------------|
| MIT | `scripts/licenses/mit.txt` | `{year}`、`{author_name}` |
| Apache-2.0 | `scripts/licenses/apache-2.0.txt` | `{year}`、`{author_name}`（於 APPENDIX） |
| GPL-3.0 | `scripts/licenses/gpl-3.0.txt` | 無（固定條款） |
| BSD-3-Clause | `scripts/licenses/bsd-3-clause.txt` | `{year}`、`{author_name}` |
| ISC | `scripts/licenses/isc.txt` | `{year}`、`{author_name}` |
| Unlicense | `scripts/licenses/unlicense.txt` | 無 |
| Proprietary | 內嵌於本檔（見下方） | `{year}`、`{author_name}`、`{author_email}` |

**執行方式：**

```bash
sed -e "s/{year}/$YEAR/g" \
    -e "s/{author_name}/$AUTHOR/g" \
    scripts/licenses/mit.txt > LICENSE
```

或讀檔後於記憶體替換再寫入。

### Proprietary（自動啟用私有模式，非開源不外放檔案）

```
Proprietary License

Copyright (c) {year} {author_name}. All rights reserved.

This software and associated documentation files (the "Software") are
proprietary and confidential. Unauthorized copying, modification,
distribution, or use of this Software, via any medium, is strictly
prohibited.

The Software is provided for internal use only and may not be shared
with third parties without prior written consent from the copyright
holder.

For licensing inquiries, contact: {author_email}
```

---

## 驗證檢查清單（必須全部通過）

**`--only` 存在時**：僅驗證指定 target 對應的檢查項；未在目標集的章節（README / doc / architecture / 共通中的 LICENSE）視為跳過，且**不得讀取未指定目標的檔案**以避免誤判覆寫。

完成前，請驗證：

### README（README.md + README.zh.md）
- [ ] `doc/README.zh.md` 已建立並儲存
- [ ] `README.md` 已建立並儲存
- [ ] **順序 0**：LLM 生成通知存在，後接 `***` 分隔線
- [ ] **順序 1**：置中封面圖片（如有 logo）
- [ ] **順序 2**：置中標語 + 置中 HTML 徽章（`style=for-the-badge`）+ `***`；私有模式省略徽章
- [ ] **順序 3**：引用格式的一句話描述
- [ ] **順序 4**：目錄存在且具有正確的錨點
- [ ] **順序 5**：功能特點為 3–5 個 list items（`- **標題** — 說明`），純文字無 code snippet，無 h3 子區段
- [ ] **順序 5**：安裝指令 + doc 連結以 blockquote 形式嵌入
- [ ] **順序 6**：技術堆疊 skillicons（可選）— 若包含，ID 全部來自 https://skillicon-list.pardn.dev/
- [ ] **順序 8**：授權區段存在
- [ ] **順序 9**：作者區段使用文字格式（姓名下方以 `<br>` 分隔的兩行純文字超連結）
- [ ] **順序 10**：星標歷史包含（公開）或省略（私有）
- [ ] **無獨立的 Installation、Usage、API Reference 區段**

### doc（doc.md + doc.zh.md）
- [ ] `doc/doc.zh.md` 已建立並儲存
- [ ] `doc/doc.md` 已建立並儲存
- [ ] **順序 0**：標題 + 返回 README 連結
- [ ] **順序 1**：前置需求存在
- [ ] **順序 2**：安裝區段存在，含所有可用方式
- [ ] **順序 3**：設定區段存在（如專案有環境變數/設定檔）
- [ ] **順序 4**：使用方式存在，從 Basic → Advanced
- [ ] **順序 5**：參考區段存在，標題符合專案類型
- [ ] 所有程式碼範例完整可執行，含 error handling
- [ ] ZH 版本中程式碼註解已翻譯

### architecture（architecture.md + architecture.zh.md）
- [ ] `doc/architecture.zh.md` 已建立並儲存
- [ ] `doc/architecture.md` 已建立並儲存
- [ ] **順序 0**：標題 + 返回 README 連結
- [ ] **順序 1**：概覽圖存在
- [ ] **順序 2**：每個核心模組各有一張詳細 Mermaid 圖
- [ ] **順序 3**：資料流圖存在（如適用）
- [ ] **順序 4**：狀態機圖存在（如適用）
- [ ] ZH 版本中 Mermaid node label 已翻譯為中文
- [ ] README 順序 6 架構區段已附上連結指向此文件

### 共通
- [ ] 所有 `{owner}`、`{repo}`、`{package}`、`{year}` 佔位符已替換
- [ ] README 與 doc 的安裝指令一致
- [ ] LICENSE 檔案存在且內容正確
- [ ] [如果指定 REPO_PATH] 所有 URL 使用覆蓋的 owner/repo

---

## 範例輸出結構

完整的 README 與 doc.md 輸出範例已拆分為獨立檔案，存放於 `scripts/examples/`，使用時請依模式讀取對應檔案作為生成藍本（placeholder 如 `{owner}`、`{author_name}` 等會在生成時被替換）。

| 檔案 | 用途 |
|------|------|
| [`scripts/examples/readme-public.md`](./scripts/examples/readme-public.md) | 公開模式 README.zh.md 完整範例（Go 專案為例，含徽章、Stars、完整作者區） |
| [`scripts/examples/readme-private.md`](./scripts/examples/readme-private.md) | 私有模式 README.zh.md 完整範例（省略徽章與 Stars） |
| [`scripts/examples/doc-cli.md`](./scripts/examples/doc-cli.md) | doc.zh.md 完整範例（以 CLI 工具為例，含前置需求、安裝、設定、使用、命令列參考） |

**使用流程**：
1. 依 `PRIVATE_MODE` 讀取 `readme-public.md` 或 `readme-private.md` 作為 README 藍本
2. 依專案類型調整 `doc-cli.md` 為藍本（函式庫 / 框架 / 設定驅動專案需改寫對應的「參考」區段）
3. 按實際專案資料替換所有 `{...}` placeholder
4. 依分析結果填入真實的功能特色、架構圖、程式碼範例

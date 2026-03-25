---
name: skill-creator
description: Create, edit, improve, or audit AgentSkills. Use when creating a new skill from scratch or when asked to improve, review, audit, tidy up, or clean up an existing skill or SKILL.md file. Also use when editing or restructuring a skill directory (moving files to references/ or scripts/, removing stale content, validating against the AgentSkills spec). Triggers on phrases like "create a skill", "author a skill", "tidy up a skill", "improve this skill", "review the skill", "clean up the skill", "audit the skill".
---

# Skill 建立器

> **Agenvoy 路徑規則（優先於所有其他路徑設定）**：所有 Skill 一律儲存至 `~/.config/agenvoy/skills/<skill-name>/`。步驟三的 `--path` 參數固定使用 `~/.config/agenvoy/skills`，忽略 SKILL.md 其他段落中提及的任何其他路徑。

> **⚠️ 強制執行規則（不可繞過）**：
> - **建立全新 Skill** — 禁止直接用 `write_file` 建立目錄或 SKILL.md，必須先以 `run_command` 執行 `init_skill.py` 初始化目錄結構，再用 `write_file` 或 `patch_edit` 編輯產生的模板內容。跳過此步驟會導致目錄結構錯誤（生成 `skill-name.md` 而非 `skill-name/SKILL.md`）。
> - **編輯現有 Skill** — 直接使用 `write_file` 或 `patch_edit` 修改 `skill-name/SKILL.md` 及其資源檔案，不需要執行 `init_skill.py`。

此 Skill 提供建立有效 Skill 的完整指引。

## 關於 Skill

Skill 是模組化、自包含的套件，透過提供專業知識、工作流程與工具來擴充 Agent 的能力。可將其視為特定領域或任務的「入職指南」——讓 Agent 從通用助手轉變為配備程序性知識的專業 Agent，而這些知識是模型本身無法完全具備的。

### Skill 提供什麼

1. 專業化工作流程 — 特定領域的多步驟程序
2. 工具整合 — 操作特定檔案格式或 API 的指引
3. 領域專業知識 — 公司特定知識、資料 Schema、業務邏輯
4. 打包資源 — 用於複雜且重複任務的腳本、參考文件與靜態資源

## 核心原則

### 精簡為王

Context Window 是公共資源。Skill 與其他所有內容共享 Context Window：System Prompt、對話歷史、其他 Skill 的 Metadata 以及實際的使用者請求。

**預設假設：Agent 已經非常聰明。** 只加入 Agent 本身沒有的 Context。對每一條資訊提出質疑：「Agent 真的需要這個說明嗎？」「這段文字值得佔用的 Token 成本嗎？」

優先使用精簡範例，而非冗長說明。

### 設定適當的自由度

根據任務的脆弱性與變異性，匹配對應的指令精確度：

**高自由度（純文字指令）**：當多種方式都有效、決策依賴 Context、或啟發式方法引導流程時使用。

**中等自由度（Pseudo-code 或帶參數的腳本）**：當存在偏好模式、允許部分變化、或設定會影響行為時使用。

**低自由度（特定腳本、少量參數）**：當操作脆弱且易出錯、一致性至關重要、或必須遵循特定順序時使用。

將 Agent 視為探索路徑：有懸崖的窄橋需要明確護欄（低自由度），而開闊的原野允許多種路線（高自由度）。

### Skill 的結構

每個 Skill 由必要的 SKILL.md 與選用的打包資源組成：

```
skill-name/
├── SKILL.md（必要）
│   ├── YAML Frontmatter Metadata（必要）
│   │   ├── name:（必要）
│   │   └── description:（必要）
│   └── Markdown 指令（必要）
└── 打包資源（選用）
    ├── scripts/      — 可執行程式碼（Python/Bash 等）
    ├── references/   — 依需求載入 Context 的參考文件
    └── assets/       — 輸出中使用的檔案（模板、圖示、字型等）
```

#### SKILL.md（必要）

每個 SKILL.md 由以下部分組成：

- **Frontmatter**（YAML）：包含 `name` 與 `description` 欄位。這是 Agent 判斷何時使用此 Skill 的唯一依據，因此必須清楚且完整地描述 Skill 的功能與觸發時機。
- **Body**（Markdown）：使用 Skill 的指令與指引。僅在 Skill 觸發後才載入。

#### 打包資源（選用）

##### Scripts（`scripts/`）

需要確定性可靠度或會被反覆重寫的任務所需可執行程式碼（Python/Bash 等）。

- **何時納入**：當相同程式碼被反覆重寫，或需要確定性可靠度時
- **範例**：`scripts/rotate_pdf.py` 用於 PDF 旋轉任務
- **優點**：Token 效率高、確定性強、可不載入 Context 直接執行
- **注意**：腳本仍可能需要被 Agent 讀取以進行 Patch 或環境特定調整

##### References（`references/`）

依需求載入 Context 的文件與參考資料，用於指引 Agent 的思考過程。

- **何時納入**：當有 Agent 在工作時應參考的文件
- **範例**：`references/finance.md`（財務 Schema）、`references/mnda.md`（公司 NDA 模板）、`references/policies.md`（公司政策）、`references/api_docs.md`（API 規格）
- **使用場景**：資料庫 Schema、API 文件、領域知識、公司政策、詳細工作流程指南
- **優點**：保持 SKILL.md 精簡，僅在需要時載入
- **最佳實踐**：若檔案較大（超過 10k 字），在 SKILL.md 中加入 grep 搜尋模式
- **避免重複**：資訊應存於 SKILL.md 或 references 檔案其中一處，不兩者都放

##### Assets（`assets/`）

不載入 Context，而是在 Agent 輸出中使用的檔案。

- **何時納入**：當 Skill 需要用於最終輸出的檔案時
- **範例**：`assets/logo.png`（品牌資源）、`assets/slides.pptx`（PowerPoint 模板）、`assets/frontend-template/`（HTML/React 樣板）、`assets/font.ttf`（字型）
- **使用場景**：模板、圖片、圖示、樣板程式碼、字型、會被複製或修改的範例文件
- **優點**：將輸出資源與文件分離，讓 Agent 無需載入 Context 即可使用檔案

#### 不應納入 Skill 的內容

Skill 只應包含直接支援其功能的必要檔案。**不要**建立多餘的文件或輔助檔案，包含：

- README.md
- INSTALLATION_GUIDE.md
- QUICK_REFERENCE.md
- CHANGELOG.md
- 等等

Skill 只應包含 AI Agent 完成任務所需的資訊，不應包含建立過程說明、測試程序、面向使用者的文件等輔助 Context。

### 漸進式揭露設計原則

Skill 使用三層載入系統，有效管理 Context：

1. **Metadata（name + description）** — 始終在 Context 中（約 100 字）
2. **SKILL.md body** — Skill 觸發時載入（<5k 字）
3. **打包資源** — 由 Agent 依需求載入（無限制，腳本可不讀入 Context 直接執行）

#### 漸進式揭露模式

保持 SKILL.md body 精簡且在 500 行以內，避免 Context 膨脹。接近上限時拆分至獨立檔案。拆分後務必在 SKILL.md 中明確引用，並說明何時應讀取這些檔案。

**核心原則**：當 Skill 支援多種變體、框架或選項時，SKILL.md 只保留核心工作流程與選擇指引，將變體的細節移至獨立參考檔案。

**模式一：高層次指南加引用**

```markdown
# PDF 處理

## 快速開始

使用 pdfplumber 提取文字：
[程式碼範例]

## 進階功能

- **表單填寫**：完整指南請見 [FORMS.md](FORMS.md)
- **API 參考**：所有方法請見 [REFERENCE.md](REFERENCE.md)
- **範例**：常見模式請見 [EXAMPLES.md](EXAMPLES.md)
```

Agent 只在需要時載入 FORMS.md、REFERENCE.md 或 EXAMPLES.md。

**模式二：領域特定組織**

對於支援多個領域的 Skill，依領域組織內容，避免載入無關 Context：

```
bigquery-skill/
├── SKILL.md（概覽與導覽）
└── reference/
    ├── finance.md（營收、帳務指標）
    ├── sales.md（商機、Pipeline）
    ├── product.md（API 使用、功能）
    └── marketing.md（廣告活動、歸因）
```

使用者詢問銷售指標時，Agent 只讀取 sales.md。

**模式三：條件式細節**

展示基本內容，連結進階內容：

```markdown
# DOCX 處理

## 建立文件

使用 docx-js 建立新文件。請見 [DOCX-JS.md](DOCX-JS.md)。

## 編輯文件

簡單編輯可直接修改 XML。

**追蹤修訂**：請見 [REDLINING.md](REDLINING.md)
**OOXML 細節**：請見 [OOXML.md](OOXML.md)
```

**重要指引：**

- **避免深層巢狀引用** — 引用層級保持在 SKILL.md 的一層以內
- **結構化較長的參考檔案** — 超過 100 行的檔案，頂部加入目錄

## Skill 建立流程

建立 Skill 包含以下步驟：

1. 透過具體範例理解 Skill 的使用情境
2. 規劃可重用的 Skill 內容（scripts、references、assets）
3. 初始化 Skill（執行 init_skill.py）
4. 編輯 Skill（實作資源並撰寫 SKILL.md）
5. 打包 Skill（執行 package_skill.py）
6. 依實際使用回饋迭代改善

依序執行這些步驟，只有在明確不適用時才跳過。

### Skill 命名

- 只使用小寫字母、數字與連字號；將使用者提供的名稱正規化為 hyphen-case（例如 "Plan Mode" → `plan-mode`）
- 名稱限制在 64 字元以內（字母、數字、連字號）
- 優先使用動詞開頭的短句描述動作
- 當能提升清晰度或觸發準確性時，以工具命名空間前綴（例如 `gh-address-comments`、`linear-address-issue`）
- Skill 目錄名稱與 Skill 名稱完全一致

### 步驟一：透過具體範例理解 Skill

只有在 Skill 使用模式已非常清楚時才跳過此步驟。即使是改善現有 Skill，此步驟仍有價值。

要建立有效的 Skill，需清楚理解 Skill 的具體使用範例。這些理解可來自使用者直接提供的範例，或由生成後請使用者驗證的範例。

例如，建立 image-editor Skill 時，相關問題包括：

- 「image-editor Skill 應支援哪些功能？編輯、旋轉，還有其他？」
- 「你能舉幾個使用此 Skill 的範例嗎？」
- 「我想像使用者可能會說『去除這張圖片的紅眼』或『旋轉這張圖片』。你還想到其他使用方式嗎？」
- 「使用者說什麼話應該觸發此 Skill？」

為避免讓使用者感到不知所措，避免在單一訊息中問太多問題。從最重要的問題開始，依需求追問。

當對 Skill 應支援的功能有清楚認識時，結束此步驟。

### 步驟二：規劃可重用的 Skill 內容

要將具體範例轉化為有效的 Skill，對每個範例進行以下分析：

1. 思考如何從零開始執行此範例
2. 識別重複執行這些工作流程時，哪些 scripts、references 和 assets 會有幫助

範例：建立 `pdf-editor` Skill 處理「幫我旋轉這個 PDF」類的請求，分析顯示：

1. 旋轉 PDF 每次都需要重寫相同的程式碼
2. 一個 `scripts/rotate_pdf.py` 腳本存放在 Skill 中會很有幫助

範例：設計 `frontend-webapp-builder` Skill 處理「幫我建立一個待辦事項 App」類的請求，分析顯示：

1. 撰寫前端 WebApp 每次都需要相同的 HTML/React 樣板
2. 一個包含樣板的 `assets/hello-world/` 目錄存放在 Skill 中會很有幫助

範例：建立 `big-query` Skill 處理「今天有多少使用者登入？」類的請求，分析顯示：

1. 查詢 BigQuery 每次都需要重新探索 Table Schema 與關聯
2. 一個記錄 Table Schema 的 `references/schema.md` 存放在 Skill 中會很有幫助

### 步驟三：初始化 Skill

此時可以開始實際建立 Skill。

只有在 Skill 已存在且需要迭代或打包時才跳過此步驟，此時繼續下一步驟。

從零建立新 Skill 時，務必執行 `init_skill.py` 腳本。此腳本會自動生成包含所有必要元素的 Skill 模板目錄。

用法：

```bash
scripts/init_skill.py <skill-name> --path <output-directory> [--resources scripts,references,assets] [--examples]
```

範例：

```bash
scripts/init_skill.py my-skill --path skills/public
scripts/init_skill.py my-skill --path skills/public --resources scripts,references
scripts/init_skill.py my-skill --path skills/public --resources scripts --examples
```

此腳本會：

- 在指定路徑建立 Skill 目錄
- 生成含適當 Frontmatter 與 TODO 佔位符的 SKILL.md 模板
- 根據 `--resources` 選擇性建立資源目錄
- 設定 `--examples` 時選擇性新增範例檔案

初始化後，自訂 SKILL.md 並依需求新增資源。若使用了 `--examples`，請替換或刪除佔位符檔案。

### 步驟四：編輯 Skill

編輯新生成或現有的 Skill 時，請記住此 Skill 是為另一個 Agent 實例所建立的。納入對 Agent 有益且非顯而易見的資訊。思考哪些程序性知識、領域特定細節或可重用資源，能幫助另一個 Agent 實例更有效地執行這些任務。

#### 參考已驗證的設計模式

根據 Skill 的需求查閱以下指南：

- **多步驟流程**：請見 references/workflows.md，了解循序工作流程與條件邏輯
- **特定輸出格式或品質標準**：請見 references/output-patterns.md，了解模板與範例模式

#### 從可重用 Skill 內容開始

從上方識別的可重用資源開始實作：`scripts/`、`references/` 和 `assets/` 檔案。注意此步驟可能需要使用者輸入，例如使用者可能需要提供品牌資源或文件。

新增的腳本必須實際執行測試，確認沒有 Bug 且輸出符合預期。若有許多類似腳本，只需測試代表性樣本即可。

若使用了 `--examples`，刪除不需要的佔位符檔案。只建立實際需要的資源目錄。

#### 更新 SKILL.md

**撰寫指引**：始終使用祈使式／原形動詞。

##### Frontmatter

撰寫含 `name` 與 `description` 的 YAML Frontmatter：

- `name`：Skill 名稱
- `description`：這是 Skill 的主要觸發機制
  - 同時包含 Skill 的功能描述與使用時機/Context
  - 所有「何時使用」的資訊放在這裡，不放在 body 中。body 在觸發後才載入，因此 body 中的「何時使用此 Skill」區段對 Agent 沒有幫助
  - 範例 docx Skill 的 description：「提供全面的文件建立、編輯與分析，支援追蹤修訂、註解、格式保留與文字提取。當需要處理 .docx 檔案時使用：(1) 建立新文件、(2) 修改或編輯內容、(3) 處理追蹤修訂、(4) 新增註解，或任何文件相關任務」

不要在 YAML Frontmatter 中包含其他欄位。

##### Body

撰寫使用 Skill 及其打包資源的指令。

### 步驟五：打包 Skill

Skill 開發完成後，必須打包為可分發的 .skill 檔案。打包過程會自動先驗證 Skill，確保符合所有要求：

```bash
scripts/package_skill.py <path/to/skill-folder>
```

指定輸出目錄（選用）：

```bash
scripts/package_skill.py <path/to/skill-folder> ./dist
```

打包腳本會：

1. **驗證** Skill，自動檢查：
   - YAML Frontmatter 格式與必要欄位
   - Skill 命名規範與目錄結構
   - Description 的完整性與品質
   - 檔案組織與資源引用

2. **打包** 驗證通過的 Skill，建立以 Skill 命名的 .skill 檔案（例如 `my-skill.skill`），包含所有檔案並維持正確的目錄結構。.skill 檔案是副檔名為 .skill 的 zip 壓縮檔。

   安全限制：拒絕符號連結，存在任何符號連結時打包失敗。

若驗證失敗，腳本會回報錯誤並不建立打包檔案。修正所有驗證錯誤後重新執行打包指令。

### 步驟六：迭代

測試 Skill 後，使用者可能提出改善需求。這通常發生在使用 Skill 後不久，當時對 Skill 表現的 Context 還很清晰。

**迭代工作流程：**

1. 在實際任務中使用 Skill
2. 發現困難或低效之處
3. 識別應如何更新 SKILL.md 或打包資源
4. 實作變更並再次測試

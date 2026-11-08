> [!NOTE]
> English version: [README.md](README.md)

# Web UI 介面差異比較（old / new）

> 從 v1.3.4 開始，Web UI 的介面風格會優先交由 Claude Opus 優化，因為開發者真的不太喜歡構思介面。
> 本次改版只調整樣式（`page/src/sass` 與 Usage 圖表配色），不更動任何功能邏輯與操作流程。

## 整體方向

| 項目 | old | new |
| --- | --- | --- |
| 主題 | 僅淺色 | 深色為預設，依系統 `prefers-color-scheme` 自動切換淺色 |
| 色彩 | 黑白灰為主，主按鈕純黑 | 冷調中性灰階＋單一藍色強調色（淺色 `#2563eb`、深色 `#4f8cff`），全部改為 design token |
| 背景 | 純色 | 側欄與預覽面板加上點狀網格；主面板為左上圓角的「紙張」，貼齊右側與底部 |
| 卡片與表單 | 淡灰底、無邊框 | 白底＋1px 邊框＋輕陰影，hover 加深邊框 |
| 按鈕 | 高度與圓角各處不一 | 統一 2.25rem 高、0.5rem 圓角、500 字重；主要動作為深色按鈕 |
| 字體 | 系統預設字體、偏細字重（300）、10px 小字 | SF Pro／Inter／Segoe UI Variable＋蘋方-繁／Noto Sans TC；最低 400 字重、最小 12px；統一等寬字體；表格數字等寬對齊 |
| 動態 | 各處 transition 不一 | 統一 0.15s 與同一條 easing 曲線 |

## 逐頁比較

### Chat 首頁

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-chat-0.png) | ![](new-chat-0.png) | ![](new-chat-dark-0.png) |

- 送出按鈕由黑色改為藍色強調色並帶光暈。
- 輸入框改為邊框＋分層陰影，聚焦時顯示藍色外環；語音錄音的光圈維持不變。
- 側欄群組標題改為小型大寫（`CHATS`、`TERMINALS`），項目行高加大。

### Chat 對話

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-chat-1.png) | ![](new-chat-1.png) | ![](new-chat-dark-1.png) |

- 頂部工具列改為毛玻璃膠囊，帶邊框與陰影；沒有對話時隱藏。
- 使用者訊息泡泡改為不對稱圓角。
- 側欄選中項目由灰底改為白色卡片。

### 檔案預覽

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-subview.png) | ![](new-subview.png) | ![](new-subview-dark.png) |

- 預覽面板以陰影（淺色）或背景色（深色）與對話區分隔，取代單純的分隔線。
- Markdown 內文行高 1.7、字距收斂，程式碼區塊配色依主題切換。

### Skills

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-skill.png) | ![](new-skill.png) | ![](new-skill-dark.png) |

- 頂部分頁由底線樣式改為圓角色塊，選中為藍色。
- 左側清單改為獨立卡片；勾選清單整列 hover 有底色。

### Schedule

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-schedule.png) | ![](new-schedule.png) | ![](new-schedule-dark.png) |

- 欄位標題加粗為 600，欄位間距加大。
- `SKILL.md` 編輯區改用等寬字體與獨立底色。
- 表單底部加上分隔線，送出按鈕與測試資訊集中在下方。

### Model

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-model.png) | ![](new-model.png) | ![](new-model-dark.png) |

- Provider 與設定卡片統一為白底＋邊框，標題 600 字重。
- Fallback 順序卡片加大內距與高度，排列更整齊。

### System

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-system.png) | ![](new-system.png) | ![](new-system-dark.png) |

- 卡片內的輸入框與按鈕統一高度並對齊。
- 輸入框改為白底＋邊框，與卡片背景區分。

### Usage

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-usage.png) | ![](new-usage.png) | ![](new-usage-dark.png) |

- 圖表配色改讀主題色彩變數（input 藍、output 綠、cache read 淺灰），切換深淺色時即時重繪。
- 統計卡片與表格數字改為等寬對齊，表格列 hover 有底色。

### Lessons

| old | new（淺色） | new（深色） |
| --- | --- | --- |
| ![](old-lesson.png) | ![](new-lesson.png) | ![](new-lesson-dark.png) |

- 每筆紀錄改為帶邊框的卡片，內距加大，標籤改為圓角膠囊。
- 篩選列與分頁按鈕統一為 2.25rem 高。

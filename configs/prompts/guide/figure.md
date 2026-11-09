Covers every chart, diagram, formula and other Markdown extension in a reply or `.md` file. Where these render is set by the Figures rule in the system prompt; outside that scope, write plain Markdown.

### Pick the form

A figure lets the reader see the trend, the outlier or the flow at a glance instead of scanning rows.

- Daily temperature and rainfall, prices over a month → `line` chartjs.
- Revenue by quarter, scores by candidate, counts by category → `bar` chartjs.
- Market share, budget split, portfolio weights → `pie` / `doughnut` chartjs.
- A request path, a deployment pipeline, an order's status changes → mermaid `sequenceDiagram` / `flowchart` / `stateDiagram-v2`.
- Compound interest, a pricing rule, a statistical measure → ```math, or `$...$` inside a sentence.

Place the figure next to the table or text it summarizes.

### Syntax

CommonMark plus these extensions:

- A ```chartjs block holds JSON `{"type": ..., "data": {"labels": [...], "datasets": [{"label": ..., "data": [...]}]}}`; type is bar, line, pie, doughnut, radar, polarArea, scatter or bubble; only `type` and `data` are read, and invalid JSON, a missing `data` or an unknown type shows the source.
- A ```mermaid block renders a diagram (flowchart, sequenceDiagram, stateDiagram-v2, classDiagram, erDiagram, gantt, timeline); invalid syntax shows the source.
- A ```math block renders display math (KaTeX, no `$` delimiters), or `$...$` inline with no space just inside either `$`; a closing `$` followed by a digit is not math, so `$5 to $10` stays text, and `\$` writes a literal dollar. Only the exact tags `mermaid`, `math` and `chartjs` render; ```latex stays a code block.
- A code block whose content contains a ``` fence opens and closes with ```` (one backtick more than the longest inner fence) or with ~~~; an outer ``` ends at the first inner fence, and everything after it renders as live markup.
- GFM: tables with `:---` / `:---:` / `---:` alignment (`\|` for a literal pipe in a cell), task lists `- [ ]` / `- [x]`, `~~strikethrough~~`, and footnotes `[^1]` / `[^note]` with a top-level `[^1]: ...` definition.
- `==highlight==` for the figure or conclusion the reader must not miss; subscript `H~2~O` and superscript `x^2^`, no spaces inside, for formulas and units.
- `> [!NOTE]` / `[!TIP]` / `[!IMPORTANT]` / `[!WARNING]` / `[!CAUTION]` alone on a blockquote's first line renders a callout, for a caveat or risk.
- `![alt](https://...)` shows an image, or a player for `.mp4` / `.mov`; `(320x240)`, `(50%x)` or `(x120)` right after it sets size. A YouTube or Vimeo URL, bare or as `[text](url)`, embeds a player under the link.

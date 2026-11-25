One self-contained HTML file — report, dashboard, chart, diagram: no build step, no local assets, no server.

### 1. Gallery first

- **List** before writing any tag: `html_template(mode="list")` returns one entry per page with `name`, `category`, `title`, `desc`.
- **Match on shape, not subject**: what the reader does with the page — skim a status, compare options, follow a flow, click through a mock, answer questions before merging. `unknowns/*` entries surface what the reader has not decided yet.
- **Something fits** (close enough counts) → §3.
- **Nothing fits** → §4, after naming the two closest entries and why each falls short.
- **`html_template` failed** → §4 without comment.

### 2. Script and data — every page, both branches

Plain HTML and inline script; no framework, no build step.

- **Figures come from tool output**: every number, row, label and link on the page traces back to a tool result; formatted strings are written out in full (`"$429.9M"`, `"+1.23%"`, `"2026-09-15 17:34 UTC"`).
- **Fetched text is escaped**: markup a script builds from fetched strings goes through `textContent` or an escape helper, never raw into `innerHTML`.
- **DOM work starts after the document exists**: one `DOMContentLoaded` handler (or a script at the end of `<body>`) draws charts and initialises highlight.js, KaTeX and Mermaid.
- **Every `T.init` id has a container** in the markup or created before the call; a missing container draws nothing and raises no error.
- **Sortable tables**: a click on `<th data-k="field">` sorts the rows by a raw value kept beside its display string (`score` beside `scoreText`) and redraws the `<tbody>`.

### 3. An example fits — copy it

1. `html_template(mode="read", name=...)`.
2. Copy its `<link rel="stylesheet">` tag into your page.
3. Replace the content in place, keeping class names and block order: every figure, row and label is replaced with real data, and repeated blocks repeat the example's markup.
4. Save with `write_result`; the content starts with `<!DOCTYPE html>`.

- **Zero new CSS**: no `<style>` block, custom property or font stack; the example's own `style=` attributes stay as they are. Redefining one variable (`--accent`, `--ink`) repaints the whole page.
- **Zero new class names**: content that fits no component goes into the nearest existing one (stat block, card, list, callout). Nothing can hold it → the wrong entry was picked; back to §1.
- **Zero sample data**: the example's brand and figures are fictional.
- **Boilerplate only**: doctype, charset, viewport meta, `<title>`, that one `<link>`, CDN libraries the content uses and the inline script. Typography, palette, spacing and breakpoints come with the sheet.

### 4. Nothing fits — build it

Applies only on this branch: you write the stylesheet and a small `T` runtime into the file.

- **Document**: `<html>`, `<head>`, viewport meta, a real `<title>`, one inline `<style>`, an inline `<script>` defining `T` before any script that calls it.
- **Charts** through `T.init(elementId, option)`, container given a height (`<div class="plot" id="gex" style="aspect-ratio:2/1">`). Chart everything chartable; tables are for data that resists plotting.
- **`T` helpers**: colours `T.series[0..3]`, `T.up`/`T.down`, `T.good`/`T.critical`, `T.ink`/`T.muted`/`T.rule`/`T.surface`, `T.tone(n)`; numbers `T.fmt`/`T.signed`/`T.si`; `T.sortable(...)` sorts the rows and redraws the `<tbody>`.
- **Layout classes**: `.masthead`, `.band`, `.stats/.stat`, `.exhibit`, `.plot`, `.note`, `.tw`, `.cols`, `.pill`, `.flag`, `ul.pts`, `.grid/.card`.
- **Sources** as `<a href>` with a URL that appeared in tool output; a constructed URL is a fabricated source.

#### Libraries

Load one only when the content uses it. These five are the whole set; a chart ECharts cannot draw is a chart to reconsider.

| Need | Load | CDN |
|---|---|---|
| Charts | ECharts 5.6.0 | `https://cdn.jsdelivr.net/npm/echarts@5.6.0/dist/echarts.min.js` |
| Icons | Font Awesome 6.7.2 | `https://cdn.jsdelivr.net/npm/@fortawesome/fontawesome-free@6.7.2/css/all.min.css` |
| Code blocks | highlight.js 11.12.0 | `https://cdn.jsdelivr.net/npm/@highlightjs/cdn-assets@11.12.0/highlight.min.js` + `styles/github.min.css` |
| Maths | KaTeX 0.16.11 | `https://cdn.jsdelivr.net/npm/katex@0.16.11/dist/katex.min.js` + `katex.min.css` + `contrib/auto-render.min.js` |
| Flow / sequence / Gantt | Mermaid 11.17.2 | `https://cdn.jsdelivr.net/npm/mermaid@11.17.2/dist/mermaid.esm.min.mjs` (ESM) |

- **Versions**: exact paths as listed, character for character; another version can 404.
- **Stylesheets are required**: KaTeX without its sheet shows raw boxes.
- **KaTeX**: both scripts, then one `renderMathInElement(document.body, {delimiters:[...]})`.
- **Mermaid**: the ESM entry (30 KB against 3.5 MB UMD), `mermaid.initialize({startOnLoad:false})`, then `await mermaid.run()`.

#### Responsive

Mobile-first: base styles are the phone, each query widens it — `min-width: 480px` (two-up stat tiles, larger type), `640px` (side-by-side content, persistent nav), `1024px` (multi-column grid, fixed max-width).

- **No horizontal page scroll**: wide tables, code blocks and timelines scroll inside their own `overflow-x: auto` container.
- **Charts follow their container**: ECharts measures once, so call `chart.resize()` from a `ResizeObserver`. The SVG renderer prints better than canvas.
- **Heights work at 480px** for every chart and canvas; a percentage height in an auto-height parent collapses to zero.
- **Media** (`img`, `svg`, `canvas`, `iframe`) carries `max-width: 100%`; type and spacing use `clamp()`.
- **Focus ring** stays visible on anything interactive — sortable headers, links, controls.

#### Visual direction

An undecided page lands on the same default — system sans, blue accent, rounded white cards on grey — which reads as unconsidered however correct the data. The brief wins on every axis it pins, in its own words; the open axes are yours, chosen from the subject's own world (its instruments, artifacts, vernacular) rather than a palette that fits any dataset.

- **Typography**: a display face and a body face chosen as a pair, with deliberate weights, widths and scale.
- **Palette**: 4–6 named values — one accent, a neutral ramp, semantic colours kept apart from the accent — written once into `T` and read back as `T.*`, so no hex literal appears in the markup. Financial, scientific, editorial and monitoring content do not share a palette.
- **Spacing**: one rhythm (4px or 8px base) for padding, gaps and vertical spacing alike.
- **Background**: flat white is a default, not a decision; consider a tinted surface or a dark canvas.
- **Motion**: one orchestrated moment (a load sequence, a scroll reveal), not micro-interactions scattered across the page. Honour `prefers-reduced-motion`.
- **Signature**: the one element the page is remembered by; spend boldness there and keep the rest quiet.
- **Structure carries information**: numbering, eyebrows, dividers and labels encode something true or they only decorate. `01 / 02 / 03` fits a real sequence and misleads on a set of peers.
- **Execution matches direction**: maximalist needs elaborate follow-through; minimal needs precision in spacing, type and detail.
- **Labels** name what the reader recognises, not what the pipeline produced; an empty panel says what is missing and what would fill it.

#### Plan, critique, build

1. **Token block before markup**: palette values, type roles (display / body / utility), the layout concept in one sentence, the signature element; state the direction in one line.
2. **Critique** it against the brief: a part that would come out identical for any other page on this subject is a default — change it and name what changed.
3. **Build** from the revised plan; every colour and type decision traces back to it. Keep selector specificity flat: a class rule and an element rule on the same box (`.section` against `section`) cancel each other, and spacing between blocks silently goes missing.

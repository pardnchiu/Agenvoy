# read_files: With and Without the Decision Model

Last updated: 2026-11-22

> Back to [Documentation](../doc.md)

This document compares measured results of `read_files` without a decision model and with decision-model filtering. It focuses on three questions:

1. **Relevance**: are files unrelated to the task withheld?
2. **Content filtering**: are unrelated parts removed before returning, while the key content survives?
3. **Reduction**: how much less content actually reaches the agent?

## Summary

| Metric | Without | With |
| --- | --- | --- |
| Unrelated files withheld | 0 / 12 | **12 / 12** |
| Related files wrongly withheld | 0 / 12 | **0 / 12** |
| Key content kept for related tasks | 12 / 12 | **12 / 12** |
| Content sent for related tasks (342 KB) | 342 KB | **189 KB, 44.8% less** |
| Content sent for related tasks, files of 16K characters or more (288 KB) | 288 KB | **134 KB, 53.3% less** |
| Content sent for unrelated tasks (342 KB) | 342 KB | **2.2 KB** (notice only) |
| Average time per read (24 reads) | 19 ms | 245 ms |

Without a decision model every read returns the whole file, whatever the task. With one, unrelated files come back as a short notice, long related files lose the blocks that do not bear on the task, and short related files come back whole.

## How the two modes work

| | Without | With |
| --- | --- | --- |
| Files under 16K characters | Returned whole | Judged as a whole; unrelated files are replaced by a withheld notice, related files are returned whole |
| Files of 16K characters or more | Returned whole | Split into blocks at blank lines and judged block by block; unrelated blocks are replaced by a row-range marker, and a file with no related block is replaced by a withheld notice |
| Getting the whole file | Always | Read again with `full: true`, which skips the filter |
| Writing after a filtered read | — | `write_file` rejects content that still contains a marker and asks for a `full: true` read |

The marker names the rows it replaced, for example `[unrelated: rows 120-188 not loaded; read it again with full: true to load the whole file]`, so the agent can see the gap and knows how to load it. Block judgments that fail or time out keep their blocks, and with no decision model configured every read behaves as in the "Without" column.

## 1. Relevance: unrelated files are withheld

Each file is paired with one related task and one unrelated task (`How do I bake sourdough bread at home?`). A related task should not be withheld; an unrelated task should get a withheld notice.

| File | Size | Path | Related | Unrelated |
| --- | --- | --- | --- | --- |
| `README.md` | 26 KB | blocks | returned | withheld |
| `doc/doc.md` | 135 KB | blocks | returned | withheld |
| `doc/architecture.md` | 39 KB | blocks | returned | withheld |
| `internal/agents/exec/toolCall.go` | 34 KB | blocks | returned | withheld |
| `internal/agents/exec/execute.go` | 34 KB | blocks | returned | withheld |
| `internal/runtime/tui/viewRender.go` | 20 KB | blocks | returned | withheld |
| `CLAUDE.md` | 24 KB | whole file | returned | withheld |
| `configs/configs.go` | 10 KB | whole file | returned | withheld |
| `internal/tools/file/boundary/boundary.go` | 8.2 KB | whole file | returned | withheld |
| `internal/tools/file/relevance.go` | 5.0 KB | whole file | returned | withheld |
| `go.mod` | 4.7 KB | whole file | returned | withheld |
| `makefile` | 1.8 KB | whole file | returned | withheld |

Sizes are bytes of the `read_files` output, row numbers included. `CLAUDE.md` is 24 KB but under 16K characters because most of it is Chinese, so it takes the whole-file path.

## 2. Content filtering: unrelated parts are removed before returning

Related tasks only. Each task lists one item that must appear in the result to count as kept.

| File / task | Key content | With | Markers |
| --- | --- | --- | --- |
| `README.md` / how to install Agenvoy | `install.sh` | kept | 14 |
| `doc/doc.md` / decision model call timeout | `times out after 3 seconds` | kept | 17 |
| `doc/architecture.md` / which `official_guides` layers are injected | `_vendor_` | kept | 6 |
| `toolCall.go` / how the `read_files` cache is invalidated after a write | `invalidateReadFileCache` | kept | 12 |
| `execute.go` / when `alreadyCall` is reset | `alreadyCall = make` | kept | 10 |
| `viewRender.go` / how the TUI renders a `write_file` diff | `writeDiffLine` | kept | 9 |
| `CLAUDE.md` / why Go builds use `-tags fts5` | `FTS5` | kept, whole file | — |
| `configs.go` / value of `MAX_DECISION_STATE_RUNES` | `20_000` | kept, whole file | — |
| `boundary.go` / how `ResolveIn` resolves a path | `func ResolveIn` | kept, whole file | — |
| `relevance.go` / the relevance size threshold | `MIN_RELEVANCE_RUNES` | kept, whole file | — |
| `go.mod` / which `go-llm-router` version is used | `go-llm-router v` | kept, whole file | — |
| `makefile` / how the binary is built | `fts5` | kept, whole file | — |

- All 12 key items are kept. The 6 long files were filtered block by block and left 6-17 markers each; the 6 short files were returned whole.
- Without a decision model all 12 items are present too, since nothing is ever removed; the difference is the size of what comes with them.

## 3. How much content is removed

The baseline is the `full: true` output, which is what every read returns without a decision model. Related tasks only; for unrelated tasks the decision model always returns a notice of about 170-210 B.

| File | Without | With | Reduction |
| --- | --- | --- | --- |
| `README.md` | 26 KB | 9.6 KB | 63.3% |
| `doc/doc.md` | 135 KB | 59 KB | 56.7% |
| `doc/architecture.md` | 39 KB | 28 KB | 27.1% |
| `internal/agents/exec/toolCall.go` | 34 KB | 15 KB | 56.9% |
| `internal/agents/exec/execute.go` | 34 KB | 13 KB | 61.8% |
| `internal/runtime/tui/viewRender.go` | 20 KB | 10 KB | 47.8% |
| 6 files under 16K characters | 55 KB | 55 KB | 0% |
| **Total** | **342 KB** | **189 KB** | **44.8%** |

- At roughly 4 bytes per token: about 86K tokens without, about 47K with.
- `doc/architecture.md` loses the least because its paragraphs are long and the task touches several of them.
- `doc/doc.md` still returns 59 KB, the largest filtered output; the task matches many blocks across a 135 KB file.

## Timing

| Scope | Without average | With average | Difference |
| --- | --- | --- | --- |
| Files of 16K characters or more (12 reads) | 28 ms | 289 ms | +261 ms |
| Files under 16K characters (12 reads) | 10 ms | 201 ms | +191 ms |

Without a decision model a read is a local file read. The extra time is the decision call: one call for a short file, and block batches run in parallel for a long one.

## Method and limitations

- Version: git `bb0de320`.
- Both modes were called through the tool entry `toolregister.Dispatch("read_files")` in one test process: "Without" with `full: true`, which skips the filter exactly as a missing decision model does, and "With" with `full: false`; decision model `typesafe@jev-latest`; run one after the other.
- Each case ran once. The decision model's answers can vary between runs, so borderline blocks may come and go.
- Key content was checked by string match, which shows the content is present but not that the surrounding context is enough. The sample is 12 files from this repository, weighted toward Go source and English documentation.
- The baseline is the `read_files` output with row-number prefixes, read with the default limit of 2,048 lines.

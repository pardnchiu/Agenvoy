<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>Agenvoy · <code>linebot</code> branch</strong>
</p>

<p align="center">
  A permanent downstream fork that adds a LINE bot runtime — and, by design, is never merged back.
</p>

***

## What this branch is

This is the `linebot` branch of [Agenvoy](https://github.com/pardnchiu/agenvoy). It carries everything on the upstream `develop` line **plus** a LINE bot runtime (`internal/runtime/line/`): an inbound webhook channel that answers questions, with whitelist + 6-digit verification gating identical to the Telegram / Discord channels.

For the full product overview, feature comparison, install instructions, and tool reference, see the [upstream README on `master`](https://github.com/pardnchiu/agenvoy/blob/master/README.md).

## Why it is never merged back

LINE's messaging platform (as exposed through [pardnchiu/go-bot/line](https://github.com/pardnchiu/go-bot)) has **no interactive UI primitives**. The mainline Agenvoy runtime depends on those primitives for core behaviour:

| Capability the main runtime relies on | Telegram / Discord | LINE |
|---|---|---|
| Inline buttons / select menus / modals | ✅ | ❌ |
| Tool-confirm gate (approve / reject each tool call) | ✅ interactive | ❌ |
| `ask_user` (picker / multi-select / masked input) | ✅ | ❌ |
| Live status message edit / delete | ✅ | ❌ |
| Cross-channel send tools | ✅ | ❌ |

Because LINE can only do **Q&A in always-allow mode** (`AllowAll=true`, no pending listener, no confirm, no `ask_user`), this branch diverges from the mainline interaction model at the runtime level. Folding it into `develop` would mean either degrading the interactive channels to LINE's lowest common denominator or carrying a permanently special-cased path in the shared runtime. Neither is acceptable.

**Policy:** as long as LINE does not support interactive UI, this branch stays a one-way downstream — it pulls the latest upstream changes in via merge, but is **never merged into `develop` or `master`**.

## Staying in sync with upstream

```bash
git fetch github develop
git merge github/develop   # resolve conflicts, keep the LINE runtime
```

The LINE runtime lives in `internal/runtime/line/` and a handful of integration points (`cmd/app/main.go`, `internal/agents/exec/systemPrompt.go`, `internal/filesystem/path.go`, TUI `/line` command). Conflicts during a merge are almost always at these seams; resolve them in favour of preserving the LINE channel.

## License

This project is licensed under the [Apache License 2.0](LICENSE).

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)

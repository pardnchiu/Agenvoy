<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>A personal AI Agent that runs on your machine</strong>
</p>

<p align="center">
  Look things up, build tools, organize files, set up schedules.<br>
  You say one sentence. The agent does the rest.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/pardnchiu/agenvoy"><img src="https://img.shields.io/badge/GO-REFERENCE-blue?include_prereleases&style=for-the-badge" alt="Go Reference"></a>
  <a href="https://app.codecov.io/github/pardnchiu/agenvoy/tree/master"><img src="https://img.shields.io/codecov/c/github/pardnchiu/agenvoy/master?include_prereleases&style=for-the-badge" alt="Coverage"></a>
  <a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <strong>English</strong> · <a href="./doc/README.zh.md">繁體中文</a>
</p>

---

## What you can do with it

### Look things up

> What's the weather in Taipei?

The agent finds data, calls tools, and gives you the answer.

If a tool doesn't exist, it builds one.

***

### Set up automation

> Report TSMC stock price every morning at 8am

The agent asks:

- Where to push results
- What format you want
- When to run

Then creates the schedule automatically.

***

### Search your files

> Find all invoices from last year<br>
> Which document mentions OAuth?

The agent searches your local files and answers directly.

***

### Handle multi-step work

> Summarize this week's GitHub Issues and generate a progress report

The agent breaks down the task, calls tools, combines results, and replies.

***

## Why Agenvoy

### You don't pick the model

Coding, research, document analysis, video processing.

The agent picks the best model for the job.

***

### No tool? It builds one

The agent can:

- Find an API
- Generate a tool
- Test it
- Fix errors
- Save it

Built once.

Used forever.

***

### Long-term memory

The agent remembers more than the current conversation.

It also remembers:

- Key information
- Work progress
- Long-term preferences

No need to re-explain context every time.

***

### Your files are a knowledge base

Supported:

- PDF
- Markdown
- TXT
- Source code

Ask questions about your files in natural language.

***

### Use it anywhere

Same agent.

Same memory.

Same tools.

Works on:

* Telegram
* Discord
* Terminal

---

## One-line install

> On MacBook, also run `sudo pmset -c sleep 0` to prevent sleep from interrupting schedules.

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

---

## Core capabilities

| Capability | Description |
| :- | :- |
| Auto tool generation | Builds and saves tools when they're missing |
| Self-scheduling | Create cron jobs with a single sentence |
| Long-term memory | Retains key info and context |
| File search | Answers from your local files |
| Sub-Agent | Multi-agent collaboration |
| MCP | Connect to external services |
| Tool Market | Share and install tools |
| Transcription | Audio and video to text |
| Self-improvement | Auto-fixes after execution failures |

---

## Demo

| Auto tool generation | Skill-based scheduler |
| :-: | :-: |
| [![](https://i.ytimg.com/vi/Fj0ooIij8TM/maxresdefault.jpg)](https://youtu.be/Fj0ooIij8TM) | [![](https://i.ytimg.com/vi/bO9AMrW3L9c/maxresdefault.jpg)](https://www.youtube.com/watch?v=bO9AMrW3L9c) |
| **Sub-agent collaboration** | **Install tools from market** |
| [![](https://i.ytimg.com/vi/wM3NU4ARz4w/maxresdefault.jpg)](https://www.youtube.com/watch?v=wM3NU4ARz4w) | [![](https://i.ytimg.com/vi/UrR5i7YAHRc/maxresdefault.jpg)](https://www.youtube.com/watch?v=UrR5i7YAHRc) |

---

## How it compares

| | **Agenvoy** | OpenClaw | Hermes-agent |
|---|---|---|---|
| Install | One command, single binary | pnpm monorepo | pip + docker |
| Multi-model | Auto-picks | Manual switch | Manual switch |
| Chat UI | Buttons / menus / modals | Text only | Text only |
| Builds its own tools | ✅ | ❌ | ⚠️ Skill only |
| Chat verification | 6-digit code | Manual approval | Manual approval |
| Cross-session push | ✅ | ❌ | ⚠️ Limited |
| File search | Semantic + keyword | Chat memory only | Chat memory only |

---

## Docs

- [Getting Started](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Getting-Started.md)
- [Architecture](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Architecture.md)
- [Core Concepts](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Core-Concepts.md)
- [Providers](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Providers.md)
- [Tools](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Tools.md)
- [Memory System](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Memory-System.md)
- [Skill System](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Skill-System.md)
- [MCP Integration](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/MCP-Integration.md)
- [Security and Sandbox](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Security-and-Sandbox.md)
- [CLI Reference](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/CLI-Reference.md)
- [Configuration](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Configuration.md)
- [Comparison](https://github.com/pardnchiu/Agenvoy/blob/master/doc/wiki/Comparison.md)

## License

This project is licensed under the [Apache License 2.0](LICENSE).

## Community Contributors

<a href="https://github.com/pardnchiu/Agenvoy/issues/3">
  <img src="https://github.com/Azetry.png" width="40" height="40" alt="Azetry" style="border-radius:50%" />
</a>
<a href="https://github.com/pardnchiu/agenvoy/issues/49">
  <img src="https://github.com/oceanasd.png" width="40" height="40" alt="oceanasd" style="border-radius:50%" />
</a>

## Contributor

Just [open an issue](https://github.com/pardnchiu/agenvoy/issues/new) to share an idea.

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

When the curve trends up — that's the signal we want to see. Hit ★ to push it along.

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)

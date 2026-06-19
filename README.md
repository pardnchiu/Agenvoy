<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>A personal AI Agent that runs on your machine</strong>
</p>

<p align="center">
  Build tools, test it, and call it.<br>
  Give the Agent you already use the power to build its own tools.<br>
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

## One-line install

> On MacBook, also run `sudo pmset -c sleep 0` to prevent sleep from interrupting schedules.

```bash
curl -fsSL https://cloud.agenvoy.com/install.sh | bash
```

***

## What you can do with it

<table>
<tr>
<td width="50%" valign="top">

### Look things up

> What's the weather in Taipei?
>
> The agent finds data, calls tools, and gives you the answer.
>
> If a tool doesn't exist, it builds one.

</td>
<td width="50%" valign="top">

### Set up automation

> Report TSMC stock price every morning at 8am
>
> The agent asks:
> - Where to push results
> - What format you want
> - When to run
>
> Then creates the schedule automatically.

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

### Search your files

> Find all invoices from last year
>
> Which document mentions Prompt guide?
>
> The agent searches your local files and answers directly.

</td>
<td width="50%" valign="top">

### Handle multi-step work

> Summarize today's GitHub Commit and generate a progress report
>
> The agent breaks down the task, calls tools, combines results, and replies.

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

### Give the Agent you already use the power to build its own tools

> Agenvoy is also an MCP server.
>
> Claude Code, Codex, OpenCode and other AI agents can connect and:
> - Use all your sandboxed tools
> - Auto-build new tools when none exist
> - Share every tool across all agents
>
> One line of config. Instant shared tool library.
> Tools created in the demo: [`fetch_weather`](doc/demo/fetch_weather/) · [`fetch_crypto_price`](doc/demo/fetch_crypto_price/)

<table>
<tr>
<td width="33%" valign="top">

#### Claude Code creates a weather tool (1)

</td>
<td width="33%" valign="top">

#### Codex reuses it and creates a crypto tool (2)

</td>
<td width="33%" valign="top">

#### Agenvoy tests both tools (3)

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

## Core capabilities

| Capability | Description |
| :- | :- |
| Auto tool generation | Builds and saves tools when they're missing |
| Self-scheduling | Create cron jobs with a single sentence |
| Long-term memory | Retains key info and context |
| File search | Answers from your local files |
| Sub-Agent | Multi-agent collaboration |
| MCP client | Connect to external MCP services |
| MCP server | Expose sandboxed tools to any MCP-compatible agent |
| Tool Market | Share and install tools |
| Transcription | Audio and video to text |
| Self-improvement | Auto-fixes after execution failures |

***

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

***

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

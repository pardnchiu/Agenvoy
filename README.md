<p align="center">
  <picture style="margin-down: 1rem">
    <img src="./doc/logo.svg" alt="Agenvoy" width="320">
  </picture>
</p>

<p align="center">
  <strong>Your private, local AI agent — running 24/7 on your own machine</strong>
</p>

<p align="center">
  Not a VM in someone else's cloud — Agenvoy runs as a daemon on your computer and keeps memory, schedules, tools and credentials on your own disk.<br>
  Say what you need in one sentence; it builds the tool, sandbox-tests it, and shares it with Claude Code, Codex and any MCP agent — all from a single Go binary.
</p>

<p align="center">
<a href="https://trendshift.io/repositories/41899?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-41899" target="_blank" rel="noopener noreferrer">
<img src="https://trendshift.io/api/badge/trendshift/repositories/41899/daily?language=Go" alt="agenvoy%2FAgenvoy | Trendshift" width="250" height="55"/>
</a>
</p>

<p align="center">
  <a href="https://github.com/pardnchiu/agenvoy/releases"><img src="https://img.shields.io/github/v/tag/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/pardnchiu/agenvoy?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

<p align="center">
  <strong>English</strong> · <a href="./doc/README.zh.md">繁體中文</a> · <a href="https://agenvoy.com/">Docs</a>
</p>

> [!NOTE]
> Agenvoy is currently in a review-and-cleanup period. File-by-file progress: [Optimization Progress](./doc/optimization.md).

## Quick start

1. **Install** (macOS / Linux; on Windows, run it inside WSL; on Android, run it in the built-in Terminal or inside Ubuntu on Termux — see below):

   ```bash
   curl -fsSL https://agenvoy.com/scripts/install.sh | bash
   ```

   The installer opens the TUI and starts the daemon when it finishes; run `agen` to open it again later.
2. **Add a model** — type `/model add` and sign in with a subscription you already have (Copilot, ChatGPT, Claude) or paste an API key.

Then just ask. New here? [From Copilot sign-in to first answer in 5 minutes](https://agenvoy.com/quickstart).

<details>
<summary><strong>macOS</strong></summary>

If you run schedules on a MacBook, also run the following so sleep does not interrupt them:

```bash
sudo pmset -c sleep 0
```

</details>

<details>
<summary><strong>Windows (via WSL)</strong></summary>

Open PowerShell as an administrator, then list and install a Linux distribution:

```powershell
wsl --online --list
wsl --install <distribution-name>
```

After installation, restart your computer, open a WSL terminal, and run the install command above.

</details>

<details>
<summary><strong>Android</strong></summary>

On Android 15 QPR2 / Android 16 or later with the built-in Terminal app (Pixel, Galaxy Z Flip 7, ...), enable **Developer options → Linux development environment**, open Terminal, install `curl`, then run the install command above:

```bash
sudo apt update && sudo apt install -y curl
```

On other devices, use Termux. Termux cannot run the Linux binary directly. Install Ubuntu with `proot-distro` in Termux and log in to it:

```bash
pkg install proot-distro
proot-distro install ubuntu
proot-distro login ubuntu
```

Inside Ubuntu, install `curl`, then run the install command above:

```bash
apt update && apt install -y curl
```

</details>

## What a session looks like

```text
❯ What's the weather in Taipei?

⏺ Checking for an existing weather tool, then fetching current conditions.

  Ran 2 tool calls

⏺ Currently 28°C in Taipei, partly cloudy with 72% humidity.
  Wind from the northeast at 12 km/h.
  ⎿ 3.2s(41.5t/s)  ↑ 21.4K(11%) ↓ 133  08:12

❯ Report TSMC stock price every morning at 8am on Telegram

⏺ Loading the finance tool and creating a daily 08:00 schedule.

  Ran 3 tool calls

⏺ Done. Scheduled daily at 08:00 — results push to Telegram.
  ⎿ 5.8s(36.2t/s)  ↑ 34.0K(17%) ↓ 210  08:13
```

The first request finds an existing script tool and answers with live data; the second binds a tool to a cron schedule whose result is pushed to Telegram.

## Taiwan-developed AI Agent Harness

Agenvoy is a Taiwan-developed AI Agent Harness by [Pardn Chiu](https://pardn.io/), built to turn conversation into completed work on your computer. It coordinates models, context, tools, real-time data, task routing, memory, schedules, and execution in one workflow, while keeping control of your files and environment in your hands. Through the Web interface, it also supports hands-free voice interaction with natural speech, wake-word detection, and interruptible spoken replies; the full result remains available in the chat.

### Why Agenvoy

A chat can give you an answer; work needs a result. Agenvoy breaks requests into steps, calls tools, and delivers outcomes on your computer—while you retain control of files, tools, schedules, and working context.

- **Turns conversation into deliverable work** — Research live data, organize files, and complete multi-step tasks with an agent that acts and reports the result.
- **Fills capability gaps itself** — Finds a keyless public API and calls it directly; when none fits, creates, tests, and keeps a new tool, ready to reuse next time.
- **Shares one tool library across agents** — Agenvoy, Claude Code, Codex, and other agents use the same sandboxed tools instead of rebuilding them.
- **Keeps automation running** — Create schedules in one sentence; recurring work runs in your environment and pushes the result.
- **Makes every step visible and controllable** — Command output streams to the TUI and Web dashboard, while sensitive paths and restricted actions still require confirmation.
- **Connects models and external services freely** — Route models by task and configure image generation, STT, TTS, stdio/HTTP MCP servers, and OAuth.
- **Provides private access from anywhere** — The local daemon connects outward to Telegram and Discord, without making your host public or opening inbound ports.

### Keep your keys and data on your machine

Meta Muse, OpenAI dots and Grok Bot each run your agent on a cloud VM the vendor operates, with your logins inside it. Agenvoy runs on the computer you already own, so you decide what leaves it.

| Guarantee | Details |
| :-- | :-- |
| **Data on your disk** | Sessions, memory and schedules live in SQLite under `~/.config/agenvoy`. No Agenvoy account, no hosted copy |
| **Credentials in your keychain** | API keys and OAuth tokens sit in the OS keychain and are used only by the local daemon; secrets in tool output are masked before the model or logs see them |
| **Sandboxed by default** | Commands run under `sandbox-exec` on macOS or `bwrap` on Linux with network off unless you approve it for that call, and `/config` → Command network can block it altogether; management endpoints answer localhost only unless you turn on LAN access in `/config` |
| **Your choice of model** | Use the provider you trust, or point Agenvoy at Ollama or LM Studio so prompts never leave your machine |

## What you can do with it

<details>
<summary><strong>Ask live questions and get live answers (Web Search / Tool Generate)</strong></summary>

> What's the weather in Taipei?
>
> The agent finds current data, calls tools, and gives you the answer.
>
> If a tool doesn't exist, it calls a public API or builds one.

<p align="center">
  <a href="https://youtu.be/floMBsAfziY">
    <img src="https://img.youtube.com/vi/floMBsAfziY/maxresdefault.jpg" alt="Web Search / Tool Generate demo" width="640">
  </a>
</p>

</details>

<details>
<summary><strong>Turn one sentence into automation (Scheduler)</strong></summary>

> Report TSMC stock price every morning at 8am
>
> The agent asks:
>
> - Where to push results
> - What format you want
> - When to run
>
> Then creates the schedule automatically.

<p align="center">
  <a href="https://youtu.be/5To3joKlFpU">
    <img src="https://img.youtube.com/vi/5To3joKlFpU/maxresdefault.jpg" alt="Scheduler demo" width="640">
  </a>
</p>

</details>

<details>
<summary><strong>Ask questions about your local files (File Search / RAG)</strong></summary>

> Find all invoices from last year
>
> Which document mentions Prompt guide?
>
> The agent searches your local files and answers directly.

<p align="center">
  <a href="https://youtu.be/vqoQ6Qvl8qU">
    <img src="https://img.youtube.com/vi/vqoQ6Qvl8qU/maxresdefault.jpg" alt="File Search / RAG demo" width="640">
  </a>
</p>

</details>

<details>
<summary><strong>Finish multi-step work (Skills / Sub-agents)</strong></summary>

> Summarize today's GitHub commits and generate a progress report
>
> The agent breaks down the task, calls tools, combines results, and replies.

<p align="center">
  <a href="https://youtu.be/nIV1xz_HIJg">
    <img src="https://img.youtube.com/vi/nIV1xz_HIJg/maxresdefault.jpg" alt="Skills / Sub-agents demo" width="640">
  </a>
</p>

</details>

## No plugin to wait for — it builds the tool

When no tool fits a live-data request, the agent first calls a **keyless public API** directly; when none fits, or your own API needs a key, it writes a script tool, tests it in the sandbox, and saves it for every agent to reuse. No code required.

```mermaid
flowchart LR
  A[Detect gap] --> B[Find API]
  B -->|fits| D[Call and share]
  B -->|none fits or needs a key| C[Write and test]
  C --> D
```

### Give other agents the same ability (MCP Server)

Agenvoy is also an MCP server, so this ability is not only its own. Claude Code, Codex, OpenCode, and other AI agents can connect and:

- Use all your sandboxed tools
- Auto-build new tools when none exist
- Share every tool across all agents

One line of config. Instant shared tool library. Tools created in the demo: [`fetch_weather`](doc/demo/fetch_weather/) · [`fetch_crypto_price`](doc/demo/fetch_crypto_price/)

<details>
<summary><strong>1. Claude Code creates a weather tool</strong></summary>

<p align="center">
  <a href="https://youtu.be/on5IaoxBO1E">
    <img src="https://img.youtube.com/vi/on5IaoxBO1E/maxresdefault.jpg" alt="Claude Code creates a weather tool" width="640">
  </a>
</p>

</details>

<details>
<summary><strong>2. Codex reuses it and creates a crypto tool</strong></summary>

<p align="center">
  <a href="https://youtu.be/2DDFCIcbnso">
    <img src="https://img.youtube.com/vi/2DDFCIcbnso/maxresdefault.jpg" alt="Codex creates a crypto tool" width="640">
  </a>
</p>

</details>

<details>
<summary><strong>3. Agenvoy tests both tools</strong></summary>

<p align="center">
  <a href="https://youtu.be/KPs4o9xDFjM">
    <img src="https://img.youtube.com/vi/KPs4o9xDFjM/maxresdefault.jpg" alt="Agenvoy tests both tools" width="640">
  </a>
</p>

</details>

## Everything in one Go binary

OpenClaw, Hermes Agent, DeepSeek Harness and pi are TypeScript or Python projects; Agenvoy's core is a single Go binary, with auto-built tools and a built-in MCP server at the center.

| Capability | What it does |
| :-- | :-- |
| Auto tool generation | Calls a public API, or writes, sandbox-tests and registers a tool when none fits |
| MCP server & client | Shares every sandboxed tool with Claude Code, Codex and any MCP client; calls external MCP servers over stdio or HTTP with live tool refresh and OAuth |
| Multi-model dispatch | Eleven providers behind one interface plus any OpenAI-compatible endpoint; each task is routed to the model best suited to it |
| Scheduler | One-shot and cron tasks bound to skills, created from a single sentence |
| 3-tier memory | Context window, semantic vector search and full-text SQLite archive; tool failures and their fixes persist across sessions |
| Sandboxed execution | Every command and script runs in an OS-native sandbox — bubblewrap on Linux, sandbox-exec on macOS |
| Skills & sub-agents | Loadable markdown instruction packs and multi-agent collaboration for multi-step work |
| Lifecycle hooks | Python or JavaScript scripts at seven points of a task (before send, around each tool call, after `ask_user`, on done / cancel) can block a call or stop the task; the agent writes and tests them |
| Self-improvement | Rewrites and retries a failing tool; every file the tools touch is versioned, so a version or a whole task can be restored |
| Realtime voice | The Web dashboard talks with you live through Gemini Live and lets you cut in mid-sentence |
| Live output & rich chat | Command progress streams to the TUI and Web dashboard; Web chat renders charts, Mermaid diagrams, math and highlighted code |
| Secret masking | Keys and tokens in tool output are masked before the model or logs see them |
| Transcription & images | Audio and video to text; image generation through a configured provider |

**39 built-in tools**: related actions sit behind one name and a `mode` argument, and only seven schemas ride along with every run — the rest are fetched on demand, so the prompt cache survives the whole session. See [Built-in Tools](https://agenvoy.com/built-in-tools).

## Talk to your agent anywhere

Same agent, same tools, same memory — from the terminal, chat apps, HTTP or other AI agents.

| Channel | How you use it |
| :-- | :-- |
| TUI | Interactive terminal with slash commands and session switching |
| Web dashboard | Ships inside the binary at `http://127.0.0.1:17989`; manage sessions, tools, schedules and memory, with the interface in 16 languages that follows your reply language |
| Telegram / Discord | The local daemon connects outward with a bot token — no inbound port, no reverse proxy. New messages steer a running task; `/new <message>` starts a separate one |
| REST API | OpenAI-compatible chat completions endpoint; `agen -p` runs one-shot prompts from the shell |
| MCP server | Expose tools to Claude Code, Codex, OpenCode and Cursor |

<p align="center">
  <a href="https://youtu.be/oaXxrTNvLaU">
    <img src="https://img.youtube.com/vi/oaXxrTNvLaU/maxresdefault.jpg" alt="Agenvoy Web Dashboard demo" width="640">
  </a>
</p>

See the [Web UI comparison (old / new, light / dark)](doc/diff/README.md) for the v1.3.4 interface refresh.

## Providers

Eleven model providers behind one interface, plus as many OpenAI-compatible endpoints as you want. Subscription routes — Claude Code, Codex, Copilot, xAI — sign in with the plan you already pay for instead of an API key.

| Provider | Models / route |
| :-- | :-- |
| Anthropic | Opus · Sonnet · Claude Code |
| OpenAI | GPT-5.x · Codex OAuth |
| Gemini | Pro · Flash |
| GitHub Copilot | GitHub OAuth |
| NVIDIA NIM | NIM API |
| xAI Grok | API key · xAI OAuth · searches X directly |
| DeepSeek | Flash · Pro |
| Mistral | API key |
| Ollama Cloud | API key |
| OpenRouter | 200+ models |
| Cloudflare | Workers AI |
| + OpenAI-compatible | Ollama · LM Studio · vLLM or self-hosted |

## Run one prompt from the shell

```bash
agen -p "Summarize today's commits" --stream
```

`agen -p` runs one prompt in its own process and prints the reply. It is read-only and sandboxed by default; add `--allow-edit` to let it write and `--skip-sandbox` to let it leave `$HOME`. `--skip-sandbox` is very dangerous: it allows `sudo` and paths outside `$HOME` with no confirmation. It exists only on the command line; `/v1/send` cannot turn the sandbox off. See the [docs](doc/doc.md#one-shot-prompt) for every flag.

## Recommended setup

> [!NOTE]
> **No free plan is recommended.** Since 2026-11-09 (5:40 PM PT), Ollama Cloud's Free plan is pay-as-you-go: hosted models such as `gemma4:31b`, `gpt-oss:120b`, `gpt-oss:20b`, `nemotron-3-super` and `nemotron-3-ultra` need purchased credits, so Ollama Cloud no longer offers a free way to try Agenvoy. Running models locally with Ollama stays free, but small local models call tools unreliably. The lowest-cost recommendation is GitHub Copilot ($10/mo) below.
>
> NVIDIA NIM's free API is not recommended either: NIM has not offered `gpt-oss-120b` since 2026-09-04, and since then most of the remaining models time out or respond slowly, and none meets the minimum response speed and basic reasoning an agent needs.

A cost-effective model setup to get started:

1. Choose a subscription model for everyday primary use, such as:
   - GitHub Copilot ($10/mo) — pick the `luna` model: its quota drains slowly enough for daily use and it is capable enough for most work
   - OpenAI ChatGPT Plus ($20/mo)
   - SuperGrok ($30/mo)

2. **Strongly recommended:** Add TypeSafe's Jev as the decision model: create a key in the [TypeSafe Console](https://console.typesafe.ai/keys), add it with `/model add` → **API Key** → **TypeSafe**, then pick `typesafe@jev-latest` under `/model` → **decision**. It is billed only on input tokens ($42 per billion, output free, per [TypeSafe's model page](https://docs.typesafe.ai/models) on 2026-11-11) and keeps irrelevant skills, memory entries, search results, page boilerplate, and unrelated file and history reads out of each turn; on the author's machine it left 95.8% of skill entries out of the turn context. Setup and measurements: [Quickstart](https://agenvoy.com/quickstart); measured page and file filtering: [fetch_page](doc/decision/fetchPage.md), [read_files](doc/decision/readFiles.md).

## Who it's for

If you want AI to complete work—not merely respond—within an environment you control, Agenvoy is for you:

- People who want to turn research, file work, and recurring reporting into reusable automation
- Developers who want a self-hosted agent with local data control and sandbox guardrails
- Teams that want Claude Code, Codex, and other agents to share tools instead of rebuilding them
- Technical operators who need private access to a local agent through the Web, Telegram, or Discord

## Validated designs

Agenvoy has already implemented the design directions below, and other teams have since adopted similar approaches, confirming that these directions solve real problems agents face in practice. Every milestone links to a public commit or release, so you can check it yourself.

<details>
<summary><strong>Show the timeline</strong></summary>

| Shipped in Agenvoy | Design | Same direction elsewhere |
| :-- | :-- | :-- |
| 2025-06-28 [Implement core application](https://github.com/agenvoy/cim-prototype/commit/df4fa95a01)<br>2025-06-29 [add traditional memory structure](https://github.com/agenvoy/cim-prototype/commit/d9a109c270) | Summary-based memory for unbounded context | 2025-11-24 [Claude client-side compaction](https://platform.claude.com/docs/en/release-notes/overview#november-24-2025) |
| 2026-01-04 [bubblewrap sandbox](https://github.com/pardnchiu/HakoRun/commit/87af1ba486)<br>2026-02-01 [harden bubblewrap sandbox](https://github.com/pardnchiu/HakoRun/commit/0cae0ed8c5)<br>2026-03-07 [block sensitive paths and credentials](https://github.com/agenvoy/Agenvoy/commit/fdaaa62b3b)<br>2026-03-18 [sandbox execution with bubblewrap](https://github.com/agenvoy/Agenvoy/commit/1f458d3596) | Sandbox mode for secure agents | 2026-03-16 [NVIDIA NemoClaw](https://blogs.nvidia.com.tw/blog/nvidia-announces-nemoclaw) |
| 2026-02-07 [Copilot device code login](https://github.com/agenvoy/Agenvoy/commit/3d4020dfb2)<br>2026-04-04 [OpenAI Codex (OAuth) provider](https://github.com/agenvoy/Agenvoy/commit/748b551ce1) | Sign in with an existing AI subscription | 2026-09-29 [Pi 0.99.0 Sign in with ChatGPT](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/providers.md#authenticate-interactively) |
| 2026-02-27 [LLM-driven agent routing](https://github.com/agenvoy/Agenvoy/commit/0514cc1127)<br>2026-03-10 [configurable dispatcher model selection](https://github.com/agenvoy/Agenvoy/commit/e4274818e7)<br>2026-04-15 [strengthen agent tier routing](https://github.com/agenvoy/Agenvoy/commit/055af67e44)<br>2026-06-10 [tiered routing across providers](https://github.com/agenvoy/Agenvoy/commit/d3c5d9f63a)<br>2026-09-11 [configurable model tiers for agent and subagent routing](https://github.com/agenvoy/Agenvoy/commit/2032737d23)<br>2026-09-11 [model tier controls on fallback priority](https://github.com/agenvoy/Agenvoy/commit/b393d22f12) | Route each task to the model best suited to it | 2026-09-14 [Copilot auto model selection tiers: efficiency / balance / intelligence](https://github.blog/changelog/2026-09-14-configure-cost-and-quality-in-copilot-auto-model-selection/)<br>2026-09-29 [Pi 0.99.0 virtual models: per-request model and thinking level](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/virtual-models.md)<br>2026-09-29 [Pi 0.99.0 classifier models](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/llama-cpp.md#classification) |
| 2026-03-23 [script tool support with sandboxed execution](https://github.com/agenvoy/Agenvoy/commit/eb789ef4be)<br>2026-05-14 [script tool scaffolding skill](https://github.com/agenvoy/Agenvoy/commit/33a62c9161)<br>2026-06-05 [script tool runtime metadata](https://github.com/agenvoy/Agenvoy/commit/74ac5bf7eb) | Custom JS / Python tools run in the sandbox | 2026-09-29 [Pi 0.99.0 codemode: model-written JavaScript in a QuickJS sandbox](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/cli.md#enable-codemode) |
| 2026-04-02 [deferred tool loading with search_tools](https://github.com/agenvoy/Agenvoy/commit/650711b764)<br>2026-05-05 [MCP client adapter with stdio/HTTP transports](https://github.com/agenvoy/Agenvoy/commit/bda9b15a08)<br>2026-06-07 [tool search registry](https://github.com/agenvoy/Agenvoy/commit/679d5544c7)<br>2026-08-08 [MCP OAuth flow](https://github.com/agenvoy/Agenvoy/commit/751e97c541) | Load tools on demand instead of declaring them all | 2026-09-29 [Pi 0.99.0 built-in MCP + tool_search](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md) |
| 2026-04-17 [invoke_subagent tool](https://github.com/agenvoy/Agenvoy/commit/db690a51d4)<br>2026-04-25 [session action logs](https://github.com/agenvoy/Agenvoy/commit/a78ae4e4c1)<br>2026-04-25 [session log streaming endpoint](https://github.com/agenvoy/Agenvoy/commit/78806be270)<br>2026-04-28 [subagent name dispatch](https://github.com/agenvoy/Agenvoy/commit/fb20bab34a) | Invoke sub-agents and track their logs | 2026-08-03 [Claude Code cross-session messaging](https://code.claude.com/docs/en/whats-new/2026-w32) |
| 2026-05-28 [extension tool loading](https://github.com/agenvoy/Agenvoy/commit/d92d37512b)<br>2026-05-28 [extension install and upload skills](https://github.com/agenvoy/Agenvoy/commit/4566ef49ae)<br>2026-05-29 [extension marketplace](https://github.com/agenvoy/Agenvoy/commit/ddc9b02903)<br>2026-06-08 [ext_ extension tools and scaffolding](https://github.com/agenvoy/Agenvoy/commit/2f2ba6e3ae) | Everything is a plugin | 2026-08-13 [DeepSeek Harness: everything is a plugin](https://www.deepseek.com/harness/en/) |
| 2026-06-03 [ask_user interruption async resume](https://github.com/agenvoy/Agenvoy/commit/137b322243)<br>2026-06-04 [preserve in-progress action memory across interruptions](https://github.com/agenvoy/Agenvoy/commit/a6b5e98f39)<br>2026-06-29 [pending task resume](https://github.com/agenvoy/Agenvoy/commit/757493e53b)<br>2026-07-03 [write_todo checklist flow](https://github.com/agenvoy/Agenvoy/commit/3b7c69cd05) | Resume long tasks from a checkpoint | 2026-08-04 [pi harness v2 in-memory storage](https://github.com/earendil-works/pi/commit/1d0c974713)<br>2026-08-05 [pi indexed harness recovery queries](https://github.com/earendil-works/pi/commit/591f22a618)<br>2026-08-05 [pi validate harness recovery record logs](https://github.com/earendil-works/pi/commit/a5953d2e15)<br>2026-08-05 [pi harness v2 jsonl backend](https://github.com/earendil-works/pi/commit/651d5d6a53)<br>2026-08-06 [pi atomic writes + torn-tail truncation](https://github.com/earendil-works/pi/commit/a838c069e6)<br>2026-08-14 [pi AgentHarness R3 generation recovery](https://github.com/earendil-works/pi/commit/a606448360)<br>2026-08-14 [pi AgentHarness R4 tool execution](https://github.com/earendil-works/pi/commit/d411d7b05e) |

</details>

## Docs

Full documentation at **[agenvoy.com](https://agenvoy.com/)**

- [Getting Started](https://agenvoy.com/getting-started)
- [Sessions & Agents](https://agenvoy.com/sessions)
- [Providers](https://agenvoy.com/providers)
- [Built-in Tools](https://agenvoy.com/built-in-tools)
- [MCP Server](https://agenvoy.com/mcp-server)
- [MCP Client](https://agenvoy.com/mcp-client)
- [Memory System](https://agenvoy.com/memory-system)
- [Skill System](https://agenvoy.com/skill-basics)
- [Sandbox](https://agenvoy.com/sandbox)
- [Architecture](https://agenvoy.com/architecture)

## License

This project is dual-licensed.

- **Open source** — [GNU Affero General Public License v3.0](LICENSE). You may use, modify and distribute it, provided derivative works and any network-accessible service built on it are released under the same license, source included.
- **Commercial** — for use that cannot meet the AGPL-3.0 source-disclosure requirement, a commercial license is available. It covers rights to the software only; any work on the project requested from the developer is quoted and charged separately. See [COMMERCIAL.md](COMMERCIAL.md).

Under section 7(b) of the AGPL-3.0, the **License** and **Source** entries in the `Other` group of the web interface's `Config > System` tab must be preserved: they carry the copyright notice, the warranty disclaimer, the license statement, and the corresponding-source link that section 13 requires. They may be relocated or restyled while remaining prominently visible, but not removed or hidden. A commercial license removes this requirement.

## Author

Just [open an issue](https://github.com/pardnchiu/agenvoy/issues/new) to share an idea.

<a href="https://github.com/pardnchiu/agenvoy/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=pardnchiu/agenvoy&cache_bust=2026-05-12" alt="Agenvoy contributors" />
</a>

---

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)

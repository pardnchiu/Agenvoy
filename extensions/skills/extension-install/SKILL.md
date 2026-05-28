---
name: extension-install
description: Install an Agenvoy extension from pkg.agenvoy.com registry (browse/pick) or local tarball into ~/.config/agenvoy/tools/.extension/<type>/<name>@<version>/. Extracts tar.gz, validates manifest (email field, type api/script only), installs deps, stores keychain keys, atomically moves staged dir. Collisions handled by Overwrite/Rename/Cancel popup.
---

# Extension Installer

Takes a packager-produced tarball, installs it as an extension visible to the runtime scanner.

## Input

`tarball` (**optional**): absolute path to a tar.gz.

- **Provided** → skip to §1 and extract the local file (offline / already-downloaded case)
- **Missing** → run §0 list + pick + download, then proceed to §1

`pkg.agenvoy.com` is the fixed registry endpoint. **Never** `ask_user` for a URL or switch to another source.

## Flow

### 0. Browse and download from registry (when no tarball)

#### 0.1 GET /list — fetch the catalog

Call `send_http_request`:

```json
{
  "url": "https://pkg.agenvoy.com/list?limit=100",
  "method": "GET",
  "content_type": "json"
}
```

Expect 200 with body `{"ok":true,"items":[{...}],"count":N,"limit":100,"offset":0}`.

Each `item` carries `name` / `type` / `email` / `version` / `summary` / `description` / `dependence` / `api_key_name` / `files` / `r2_key` / `size_bytes` / `sha256` / `created_at`.

`status_code != 200` or `items` empty → abort with "registry unavailable or no packages".

#### 0.2 ask_user singleSelect to pick a package

Convert `items` into display strings, one per line:

```
<item.name>@<item.version> (<item.email>) · <item.type> · <item.summary>
```

Example: `yt_dlp_youtube_downloader@1.0.0 (chiu@example.com) · script · Download a YouTube video...`

`ask_user` (singleSelect):

```
Pick the extension to install (N total):
```

`options` are the display strings. Record the user's chosen item index → extract that item's `r2_key` / `name` / `email` / `version`.

User cancel → abort.

#### 0.3 GET /download — pull the tar locally

Call `download_file` (**not** `send_http_request` — binary doesn't belong in a string body):

```json
{
  "url": "https://pkg.agenvoy.com/download?key=<selected item.r2_key>",
  "output_file": "~/.config/agenvoy/download/<name>@<version>.tar.gz",
  "timeout": 300
}
```

Response: `{ok, output_file, size_bytes, sha256, ...}`.

Verify the download:
- `size_bytes > 0`
- If the response carries `sha256`, compare to the picked item's `sha256`; mismatch → abort and `rm` the file
- Any other failure → abort with the error

Use `output_file` as the `tarball` variable and **proceed to §1**.

### 1. Extract into staging

Fixed staging directory: `~/.config/agenvoy/tools/.extension/.staging/` (**cleaned and recreated** on every install).

```bash
rm -rf ~/.config/agenvoy/tools/.extension/.staging
```

```bash
mkdir -p ~/.config/agenvoy/tools/.extension/.staging
```

```bash
tar -xzf <tarball> -C ~/.config/agenvoy/tools/.extension/.staging
```

After extraction, the staging directory should contain exactly one subdirectory `<original-basename>/` (the packager uses `-C <parent>` to keep the outer dir in the tarball). `run_command: ls ~/.config/agenvoy/tools/.extension/.staging` gets `<original-basename>`.

If extraction fails, or staging contains 0 / >1 subdirectories, abort with:
```
❌ Tarball contents are invalid (cannot find a single root dir). Verify the tarball was produced by the extension-upload skill.
```

### 2. Read and validate manifest.json

`read_file: ~/.config/agenvoy/tools/.extension/.staging/<original-basename>/manifest.json`

Missing file → abort with "manifest.json missing in tarball, refuse to install".

Validate each field (**any failure aborts** — do not `ask_user` to fix):

| Field | Condition |
|---|---|
| `name` | non-empty, matches `^[a-z0-9][a-z0-9_-]*$` |
| `type` | ∈ `{api, script}` (worker rejects mcp) |
| `version` | strict semver `^\d+\.\d+\.\d+$` |
| `summary` | non-empty |
| `email` | non-empty, matches `^[^@\s]+@[^@\s]+\.[^@\s]+$`, already lowercase (worker normalizes) |
| `dependence` | array |
| `api_key_name` | array, each element matches `[A-Z][A-Z0-9_]*_API_KEY` |
| `files` | array, length ≥ 1, must include `tool.json` |

Every path in `files` must exist in the staging subdirectory; any missing file → abort.

### 3. Install system dependencies

For each `<dep>` in `manifest.dependence`, call:

```
install_dependence(name="<dep>")
```

The tool internally:
- `exec.LookPath(<dep>)` already present → returns `already_installed:true`, this step is satisfied
- macOS → `brew install <dep>` (brew doesn't need sudo)
- Linux → probes `apt-get/dnf/yum/pacman/apk` (first found) and runs `sudo <pm> install -y <dep>`; the TUI suspends the alt-screen so sudo can take the tty for the password prompt
- After install, `LookPath` re-checks

Response is JSON `{"ok": true/false, ...}`. `ok:false` or tool error → **abort immediately** and `rm -rf .staging`.

Each call triggers a `KindToolConfirm` popup (`AlwaysAllow=false`); the user sees "About to run `brew install ffmpeg` — confirm?". User decline → tool fails → skill aborts.

> Note: `install_dependence` is only registered in the TUI / `agen cli` / `agen run` processes; Telegram / Discord / HTTP-API / subagent see it filtered out via `ExcludeTools`. This skill is also only usable from those visible channels.

### 4. Check and fill keychain keys

For each `<KEY>` in `manifest.api_key_name`, call:

```
store_secret(key="<KEY>", prompt="<extension name> needs <KEY>; enter the value (re-enter to overwrite an existing one):")
```

`store_secret` calls `keychain.Set` internally. User cancels / empty value → tool returns error → skill **aborts** and removes staging.

> Note: Agenvoy has no read-only key check tool, so **even an existing key is re-prompted**. The user can re-enter the same value or a new one; cancelling (empty) aborts the install.

### 5. Derive install directory

Directory name (**no author/email prefix**):

```
<manifest.name>@<manifest.version>
```

Examples: `yt-dlp-info@1.0.0`, `yt_dlp_youtube_downloader@1.0.0`.

Full install path:

```
~/.config/agenvoy/tools/.extension/<manifest.type>/<manifest.name>@<manifest.version>/
```

**Never rewrite `tool.json::name`** — keep the value the manifest already carries. The runtime tool registry uses that value as the key.

Collisions (same name + version from two authors) are resolved uniformly in §6 collision check (Overwrite / Rename / Cancel popup).

### 6. Collision check

Define `<final-dir>` = `<name>@<version>` (the default install directory name derived in §5).

```bash
ls ~/.config/agenvoy/tools/.extension/<type>/<final-dir> 2>/dev/null
```

If the directory already exists → `ask_user` singleSelect:

```
<final-dir> is already installed.
- Overwrite · remove the old version and install the new one
- Rename · pick a new directory name (keep both on disk)
- Cancel · abort install
```

| Choice | Action |
|---|---|
| `Cancel` | Clean staging and abort |
| `Overwrite` | `rm -rf` the existing dir → keep `<final-dir>` → proceed to §7 |
| `Rename` | Proceed to §6.1 to collect a new name |

#### 6.1 Rename — collect a new directory name

Default suggestion = `<final-dir>-2`. If `-2` is also taken, increment to `-3`, `-4`, … until non-conflicting.

`ask_user` (free-text):

```
Enter a new directory name (must end with `@<version>`; allowed chars [A-Za-z0-9_.+\-@]):
default: <suggested name>
```

Reply validation:

| Condition | Action |
|---|---|
| Blank → use the default suggestion | Pass; set `<final-dir>` = default |
| Matches `^[A-Za-z0-9][A-Za-z0-9_.+\-]*@\d+\.\d+\.\d+(-[A-Za-z0-9_.+\-]+)?$` AND does not collide | Pass; set `<final-dir>` = new name |
| Format invalid / still colliding | Re-prompt; abort after 3 attempts with "rename cancelled, install aborted" |

> Note: rename only affects the install directory name (two versions coexist on disk). `tool.json::name` is not rewritten; the runtime tool registry uses that name as the key. When two versions are loaded with the same name, **the later-loaded one shadows the earlier**, so the runtime still exposes only one. Rename is purely a disk-level coexistence escape hatch for rollback / diff, not a runtime version switcher.

### 7. Move staging to the install path

```bash
mkdir -p ~/.config/agenvoy/tools/.extension/<type>
```

```bash
mv ~/.config/agenvoy/tools/.extension/.staging/<original-basename> ~/.config/agenvoy/tools/.extension/<type>/<final-dir>
```

```bash
rm -rf ~/.config/agenvoy/tools/.extension/.staging
```

### 8. Final report

`tools.NewExecutor` re-scans `.extension/<type>/*` on every incoming user message, so **the new extension is loaded automatically on the next message**. No daemon restart needed; do not run `agen stop`.

```
✅ installed
- name:    <manifest.name>
- email:   <manifest.email>
- version: <manifest.version>
- type:    <manifest.type>
- path:    ~/.config/agenvoy/tools/.extension/<type>/<final-dir>/
- tool:    <tool.json::name> (kept verbatim from manifest, not rewritten)
- deps:    <installed binary list or "all present">
- keys:    <stored KEY list or "none">
- next:    your next message will see the new tool
```

## Forbidden

- Never `ask_user` for a different registry endpoint in §0; the endpoint is fixed at `https://pkg.agenvoy.com`
- Never fetch the tar binary via `send_http_request` in §0.3; binary belongs to `download_file` (cannot go through a string body)
- Never skip the §0.3 sha256 comparison (when the response carries `sha256`); mismatch means the tar is corrupted or substituted
- Never extract directly into the install path; always isolate through `.staging/`. Any failure in validation / deps / key steps must `rm -rf .staging`
- Never `ask_user` to patch a missing manifest field in step 2; validation failure means the tarball is broken at the packager side — abort
- Never skip the dual `command -v <dep>` check in step 3 (once before install, once after)
- Never alter the step 4 `store_secret` flow; do not `ask_user` for a plaintext key and then forward it to store_secret (that pulls the value into LLM context)
- Never hardcode a single package manager; always use `uname -s` + probe order
- Never rewrite `tool.json::name`; keep whatever the manifest carries. The runtime tool registry uses it as the key. (Authors must ensure the name matches Gemini / Vertex AI rules `[a-zA-Z_][a-zA-Z0-9_.:-]*` at publish time.)
- Never add author / email / safe-email prefixes to the install dir; the name is fixed at `<manifest.name>@<manifest.version>`; collisions are resolved via the §6 popup
- Never skip the step 6 collision check; the Overwrite/Rename/Cancel three-way decision must come from the user
- Never accept a renamed directory in §6.1 that doesn't end with `@<version>`; that violates the collision-safe naming convention
- Never accept a renamed name in §6.1 that still collides with an existing directory; re-prompt
- Never let §6.1 rename touch `tool.json::name`; rename is purely disk-level coexistence — the tool registry key always stays as the manifest value
- Never rename `.staging/` to `.tmp/` or another path (the runtime scanner skips `.` prefixes; `.staging` is the scan-safe staging location)
- Never run `agen stop` / `kill <pid>` / `pkill agen` / any daemon-restart command in §8. `tools.NewExecutor` re-scans on every user message, so the new extension auto-loads on the next message. Restarting only kills your own session.
</content>
</invoke>
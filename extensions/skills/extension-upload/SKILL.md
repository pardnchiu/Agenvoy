---
name: extension-upload
description: Package a script tool under ~/.config/agenvoy/tools/script/ into a tar.gz and publish to pkg.agenvoy.com registry. Keyword picker, dep/key detection, config-stored email (ask + lowercase + persist), ask version, email verification gate, multipart upload with downgrade/unique guards.
---

# Extension Uploader

Packages an Agenvoy script tool directory into a marketplace tarball and uploads it to pkg.agenvoy.com. The source root is fixed at `~/.config/agenvoy/tools/script/`. If the user provides a keyword (e.g. `yt`), only matching subdirectories are listed.

## Input

`keyword` (**optional**): substring filter (case-insensitive) over subdirectories under `~/.config/agenvoy/tools/script/`. Examples: `yt`, `dlp`, `tts`.

- **With keyword** → list filtered matches
- **Without keyword** (e.g. plain `/extension-upload`) → **list ALL subdirectories for the user to pick**; do not ask for a keyword

### 0. Select `extension_dir` (picker)

Fixed source root:

```
SCRIPT_ROOT=~/.config/agenvoy/tools/script
```

`list_files` lists all first-level subdirectories under `SCRIPT_ROOT` (non-recursive). **Skip names starting with `.` or `_`.**

Branch by input:

| Has keyword? | Candidate set |
|---|---|
| No | All subdirectories |
| Yes | Subdirectories whose name (lowercased) contains `keyword` (lowercased) |

Branch by candidate count:

| Count | Action |
|---|---|
| 0 | Abort. With keyword: "No directory matching `<keyword>` under `SCRIPT_ROOT`". Without keyword: "`SCRIPT_ROOT` is empty — run `script-tool-add` first to create a script tool" |
| 1 | Use that directory as `extension_dir` directly; report "auto-selected `<basename>`" |
| ≥ 2 | `ask_user` singleSelect listing all candidates; user picks one as `extension_dir` |

`extension_dir` is the **absolute path** `<SCRIPT_ROOT>/<basename>`, used by every step below.

> Note: this skill only packages script tools (fixed `type: "script"`). Packaging api tools requires a separate skill. The worker does not accept `mcp` type.

## Flow

### 1. Read the directory

`list_files` enumerates every file under `extension_dir` (relative paths, recursive).

**Collect into `raw_files`, excluding:**
- `.DS_Store`, `Thumbs.db`, `.git*`
- `*.tar`, `*.tar.gz`, `*.tgz`, `*.zip`
- An existing `manifest.json` (this skill will regenerate it)

`read_file` reads (if present):
- `tool.json` (**required** — if missing, abort with "tool.json missing in <extension_dir>, refuse to package")
- All `script.{js,py,sh}` / `*.js` / `*.py` / `*.sh`

### 1.5 Generic structure check (gate)

Check root-level files under `extension_dir`. Abort immediately on any violation:

| Rule | Condition |
|---|---|
| tool.json must exist | `extension_dir/tool.json` exists and is a regular file |
| script mutual exclusion | `script.py` and `script.js` **must not coexist** (both present → abort) |

Abort message:
```
❌ Structure check failed: script.py and script.js cannot coexist (keep only one).
```

### 2. Infer type

Fixed `type: "script"` (this skill only packages tools under `~/.config/agenvoy/tools/script/`).

### 2.5 Type-specific structure check (gate)

For `type: script`:

| Rule | Condition |
|---|---|
| script must exist | `script.py` or `script.js` must exist (exactly one); both missing → abort |

Abort message:
```
❌ Structure check failed: type:script requires script.py or script.js.
```

### 2.7 Health check (gate)

Verify the script can be parsed by its interpreter so consumers don't install something that immediately crashes. **Any failure aborts** and prints the stderr.

Branch on the script file confirmed in §2.5:

- `script.py`:
  ```bash
  python3 -m py_compile <extension_dir>/script.py
  ```
- `script.js`:
  ```bash
  node --check <extension_dir>/script.js
  ```

exit code ≠ 0 → abort:
```
❌ Health check failed: script cannot be parsed by the interpreter.
<first line of stderr>
```

> Note: only syntax-level. Top-level code is not executed; import errors / runtime errors / missing API keys are not checked. The author must test runtime behavior before packaging.

### 3. Detect `dependence` and `api_key_name`

#### Dependence (system binary names)

Scan every script file for these patterns to extract binaries:

- JavaScript: `spawn("X"` / `exec("X"` / `execSync("X"` / `spawnSync("X"`
- Python: `subprocess.run(["X"` / `subprocess.Popen(["X"` / `os.system("X "` / `shell=True` + `"X "`
- Shell: the first non-reserved-word token

**Exclude:**
- Shell builtins: `cd`, `echo`, `test`, `[`, `export`, `set`, `shift`, `pwd`, `true`, `false`
- Interpreters themselves: `node`, `python`, `python3`, `bash`, `sh`, `/usr/bin/env`
- Tools whitelisted by default: `ls`, `cat`, `head`, `tail`, `mkdir`, `cp`, `mv`, `rm`, `grep`, `sed`, `awk`, `find`, `jq`, `which`, `date`, `git` (almost always present; not counted as a dependence)

What remains (e.g. `yt-dlp`, `ffmpeg`, `imagemagick`, `pandoc`, `tesseract`) is the `dependence` candidate set.

#### Api_key_name (keychain key names)

Scan every script file for these patterns to extract key names:

- `localhost:17989/v1/key?key=([A-Z][A-Z0-9_]*_API_KEY)`
- `process\.env\.([A-Z][A-Z0-9_]*_API_KEY)`
- `os\.environ\[["']([A-Z][A-Z0-9_]*_API_KEY)["']\]`
- `os\.environ\.get\(["']([A-Z][A-Z0-9_]*_API_KEY)["']`
- `\$([A-Z][A-Z0-9_]*_API_KEY)` (shell scripts)

Dedupe and sort all hits.

#### Confirm detection results

Call `ask_user`. Put the detection list in the `detail` field (hint-style subtitle); keep `question` short:

```json
{
  "questions": [
    {
      "question": "Are the detections above correct?",
      "detail": "dependence: [yt-dlp, ffmpeg]\napi_key_name: []",
      "options": ["Yes, continue", "No, let me edit dependence", "No, let me edit api_key_name", "No, edit both"]
    }
  ]
}
```

`detail` is multi-line hint text; `question` is the short prompt. The popup renders `detail` with hint style (subtitle) and `question` with bold (title). **Never** stuff `detail` content into `question` — they will render together and become unreadable.

### 4. Take `name` and `summary` from tool.json

Use `tool.json` as the source of truth — **do not** ask_user to confirm:

- `manifest.name` = `tool.json::name` verbatim
- `manifest.summary` = first line of `tool.json::description`, truncated to 120 chars

If either fails §6 validation (name pattern / summary length), §6 will ask_user to fix. Do not prompt proactively here.

### 4.5 Ask version

Pre-read `<extension_dir>/manifest.json` if it exists and use its `version` as the default; otherwise default to `1.0.0`.

`ask_user` (single free-text):

```
Enter version (semver `MAJOR.MINOR.PATCH`, e.g. 1.0.0; blank = use default <default>):
default: <existing manifest version or 1.0.0>
```

Reply handling:

| Reply | Action |
|---|---|
| Blank / whitespace-only / empty | Use default as `manifest.version` — **not an error**, do not re-prompt |
| Matches `^\d+\.\d+\.\d+$` | Accept as-is |
| Anything else (incl. `v` prefix, pre-release like `1.0.0-beta`, build metadata `+sha`, etc.) | Re-prompt; abort after 3 attempts with "version format invalid, upload cancelled" |

**Never treat blank as an error** — a deliberate blank means "use default".

### 5. Get registry email (from config; ask if missing)

#### 5.1 Read config

Call `get_registry_email`:

- Returns `{"email": "<stored value>"}` → use directly, **do not re-prompt**, jump to §6
- Returns `{"email": ""}` → fall through to §5.2 first-time setup

#### 5.2 First-time setup

`ask_user` (single free-text):

```
First publish needs a marketplace registry email (stored in ~/.config/agenvoy/config.json, reused next time):
```

Validate against `^[^@\s]+@[^@\s]+\.[^@\s]+$`:

- Pass → **lowercase first**, then call `set_registry_email(email=<lowercased>)` to persist (worker normalizes to lowercase, client must match)
- Fail → re-prompt; abort after 3 attempts with "email format invalid"
- Blank / cancel → abort with "no email provided, cannot upload"

**The entire username / git config user.name chain is gone** — marketplace identity uses email only. The manifest uses the `email` field (not `author`); its value is the lowercased pure email string.

### 6. Fill in any missing manifest fields

Assemble the candidate manifest:

```json
{
  "name": "<tool.json::name>",
  "type": "script",
  "version": "<from §4.5>",
  "summary": "<first line of tool.json::description, truncated to 120>",
  "email": "<§5.1/5.2 registry email, lowercased>",
  "dependence": <confirmed in §3>,
  "api_key_name": <confirmed in §3>,
  "files": <raw_files from §1>
}
```

Validate field by field; **any failure** triggers `ask_user` to fix that field:

| Field | Condition |
|---|---|
| `name` | non-empty, matches `^[a-z0-9][a-z0-9_-]*$` |
| `type` | ∈ `{api, script}` (worker rejects mcp) |
| `version` | strict semver `^\d+\.\d+\.\d+$` (no pre-release suffix) |
| `summary` | non-empty, ≤ 120 chars |
| `email` | non-empty, matches `^[^@\s]+@[^@\s]+\.[^@\s]+$` (already guaranteed by §5) |
| `dependence` | array, elements non-empty (empty array OK) |
| `api_key_name` | array, each element matches `[A-Z][A-Z0-9_]*_API_KEY` (empty array OK) |
| `files` | array, length ≥ 1, must include `tool.json` |

Re-validate after each fix; only proceed once everything passes.

### 7. Write manifest.json and package

Fixed output directory: `~/.config/agenvoy/tools/.extension/.package/` — **not** `$HOME/Downloads`, **not** `~/.config/agenvoy/download/`, **not** the current work dir, **not** the source dir.

Fixed filename format: `<name>@<version>.tar.gz` (e.g. `yt-dlp-info@1.0.0.tar.gz`).

`write_file`:

- Path: `<extension_dir>/manifest.json`
- Content: the manifest that passed §6 validation, pretty-printed (2-space indent), trailing newline

`run_command`:

```bash
mkdir -p ~/.config/agenvoy/tools/.extension/.package
```

```bash
tar --no-xattrs -czf ~/.config/agenvoy/tools/.extension/.package/<name>@<version>.tar.gz -C <extension_dir>/.. <basename(extension_dir)>/
```

Where `<basename>` is the last segment of `extension_dir` (e.g. `yt_dlp_youtube_downloader`).

- `--no-xattrs`: **mandatory**. macOS bsdtar tries to read xattrs (e.g. `com.apple.quarantine`) by default; the sandbox can't read them and prints "Operation not permitted" warnings. Marketplace packages shouldn't carry OS-local metadata anyway.
- `-C <parent>`: points to the parent so the tarball stores `<basename>/<file>...` rather than absolute paths.

#### 7.5 Disk verify (gate — **the only** success check)

`run_command` returns a merged stdout+stderr string to the LLM. **The LLM cannot see the exit code by itself**, and **must not** guess from stderr substrings like "not permitted" / "error" / "warning". `tar` can print xattr/ACL warnings yet still exit 0 with a valid tarball.

The only reliable check:

```bash
ls -l ~/.config/agenvoy/tools/.extension/.package/<name>@<version>.tar.gz
```

| ls result | Verdict |
|---|---|
| File exists, size > 0 bytes | **Packaging succeeded.** Record the size, proceed to §8. **Ignore** every stderr warning from the tar step. |
| `No such file or directory` or size 0 | Actually failed. Print the tar stderr to the user and abort. |

Do not branch on "stderr contains a warning → failure". A tarball on disk = success.

If `tar` is not in the whitelist / blocked by the sandbox, tell the user:

```
⚠️ tar is not in the whitelist. Run /allow-cmd tar and retry, or have the maintainer add tar to configs/jsons/white_list.json.
```

### 8. Upload to pkg.agenvoy.com (registry)

Fixed endpoint: `https://pkg.agenvoy.com/upload` (**do not** let the user change the URL; **never** `ask_user` for an endpoint).

`manifest.email` is the registration email (pure email string); keep it for the §9 report. `read_file` `<extension_dir>/manifest.json` to obtain the **full JSON string** for `fields.manifest` below (the worker will `JSON.parse(manifest)` and re-validate).

#### 8.1 First POST — trigger verification email

Call `send_http_request`. **All four fields are required** (`url` / `method` / `content_type` / `body`); missing any one of them and the worker returns `multipart parse failed`:

```json
{
  "url": "https://pkg.agenvoy.com/upload",
  "method": "POST",
  "content_type": "multipart",
  "body": {
    "fields": {
      "manifest": "<full JSON string written in §7>"
    },
    "files": [
      {
        "name": "tar",
        "path": "~/.config/agenvoy/tools/.extension/.package/<name>@<version>.tar.gz",
        "content_type": "application/gzip"
      }
    ]
  }
}
```

**Do not simplify the payload:**

- Never omit `content_type: "multipart"` (defaults to `json`, worker won't see multipart, fails)
- Never omit `body` (must contain both `fields` and `files`)
- Never put the manifest JSON into `files[]` (manifest is a text field, goes under `fields.manifest`)
- Never put tar bytes into `fields` (tar is binary, goes under `files[].path` and is read from disk by the handler)

Response is the `send_http_request` envelope: `{status_code, headers, body}`. **`status_code` is the only branching signal** — do not guess from the body string.

| status_code | Expected body | Action |
|---|---|---|
| 202 | `{"ok":false,"error":"verification_sent","email":"...","ttl_seconds":60}` | Proceed to §8.2 |
| 400 | schema error | Abort, print the error in the body |
| 413 | `tar_too_large` | Abort |
| 502 | `email_send_failed` | Abort |
| Other | Shouldn't happen on first POST without a code (always expect 202) | Abort, print the raw body |

#### 8.2 ask_user for the verification code

`ask_user` (single free-text):

```
A verification code was sent to <email> (valid 60s). Enter the 6-digit code:
```

If blank or not 6 digits → re-prompt up to 3 times; abort with "verification code format invalid, upload cancelled".

**Do not** swap `ask_user` for code guessing / pre-fill / `popupSecret` — the code is not a secret, expires in 60s, and plaintext echo helps the user paste it correctly.

#### 8.3 Second POST with the code

Call `send_http_request` — **same four-field structure as §8.1**, only difference is `fields` now also has `code`:

```json
{
  "url": "https://pkg.agenvoy.com/upload",
  "method": "POST",
  "content_type": "multipart",
  "body": {
    "fields": {
      "manifest": "<same as 8.1>",
      "code": "<6-digit code>"
    },
    "files": [
      {
        "name": "tar",
        "path": "<same as 8.1>",
        "content_type": "application/gzip"
      }
    ]
  }
}
```

| status_code | Action |
|---|---|
| 200 | Success — parse body for `r2_key` / `sha256` / `size_bytes`, proceed to §9 |
| 401 | `verification_failed` (wrong / expired) → loop back to §8.2; abort after 3 retries |
| 409 | `version_already_exists` or `type_mismatch` → **abort**, print body `existing` info, suggest the user run `/version-generate` to bump or align type |
| 422 | `downgrade_not_allowed` → **abort**, print body `latest`, ask user to bump version |
| 413 | `tar_too_large` → abort |
| 5xx | `internal` / `email_send_failed` → abort, print raw body |
| Other | Abort, print raw body |

### 9. Final report

Success (§8.3 returned 200):

```
✅ packaged & published
- manifest: <extension_dir>/manifest.json
- tarball:  ~/.config/agenvoy/tools/.extension/.package/<name>@<version>.tar.gz
- size:     <bytes>
- registry: pkg.agenvoy.com
- r2_key:   <body.r2_key>
- sha256:   <body.sha256>
```

`size` was captured by `ls` in §7.5 — do not re-run.

Upload-stage failure (§8.1 / §8.2 / §8.3) → show `✅ packaged` plus `❌ publish failed` with the worker error; the local tarball stays in `.package/` so the user can fix and retry.

§7.5 disk-verify failure (tarball missing or size 0) → show `❌ packaging failed` with the tar stderr; **do not** proceed to §8.

## Forbidden

- Never hardcode `email`; it must come from `get_registry_email` (and §5.2 ask_user + `set_registry_email` if missing)
- Never touch `git config user.name` / `git config user.email`; marketplace identity uses only the config registry email
- Never use an `author` field in the manifest; the worker expects `email` (a pure email string, not `<name> (<email>)`)
- Never skip the §5.2 lowercase normalize; the worker normalizes email to lowercase — mismatched case breaks both KV verification lookup and D1 lookup
- Never re-ask the user for email; once §5.1 returns a non-empty value, use it
- Never run username sanitization in §5, never derive a `<safe-author>` / `<email-local>` prefix; the filename is just `<name>@<version>.tar.gz` with no prefix
- Never add items to `dependence` / `api_key_name` that weren't detected in the scripts; the user can add them in §3, but the LLM must not embellish
- Never list `manifest.json` itself in the `files` array (the marketplace client fetches the manifest separately)
- Never omit `-C <parent>` in §7 — the tarball would contain absolute paths otherwise
- Never omit `--no-xattrs` in §7 — macOS sandbox can't read xattrs and would flood warnings
- Never skip §7.5 disk verify; `ls -l <tarball>` is the **only** success check
- Never infer failure from stderr substrings ("not permitted" / "error" / "warning" / "Operation not permitted"); `run_command` returns merged stdout+stderr and the LLM has no exit code — disk state is the source of truth
- Never claim `✅ packaged` and `❌ publish failed` while stopping at the packaging step — that's a contradiction; once §7.5 passes, packaging succeeded and §8 must run
- Never drop the tarball in the current work dir, `~/Downloads`, `~/.config/agenvoy/download/`, the source dir, `tmp`, or any path from `ask_user`; the output location is **fixed** at `~/.config/agenvoy/tools/.extension/.package/`
- Never add prefixes like `<safe-author>-` / `<email-local>-` to the filename; fixed `<name>@<version>.tar.gz`
- Never substitute `zip` for `tar.gz` (the marketplace only accepts tar.gz)
- Never lower §6 standards by accepting `1.0`, `v1.0.0`, `1.0.0-beta` etc.
- Never skip §1.5 or §2.5 structure checks (tool.json must exist, script.py and script.js are mutually exclusive, type:script requires a script)
- Never bypass the §0 picker by guessing `extension_dir`; the source root is **fixed** at `~/.config/agenvoy/tools/script/` — do not scan `.extension/` / `api/` / anywhere else
- Never force `ask_user` to collect a keyword when the skill is invoked without one — list all subdirectories directly (the user explicitly wants to browse everything)
- Never fall back to "list everything" when a provided keyword yields zero hits — abort and ask for a more precise keyword
- Never override §2 `type` via path inference or `ask_user`; this skill only packages `type:script`
- Never skip §2.7 health check; a syntax failure means the tool is broken — shipping it would crash on install
- Never replace `py_compile` / `node --check` in §2.7 with "run the whole script" — top-level reads on stdin would hang
- Never `ask_user` for `name` or `summary` in §4; `tool.json` is the source of truth, §6 handles validation fallback
- Never skip §4.5; the user must confirm `version` in the main flow — do not hardcode `1.0.0` or rely on §6 fallback
- Never accept `v` prefix, pre-release suffix, or build metadata (`+sha`) in §4.5; strict `^\d+\.\d+\.\d+$`
- Never treat a blank reply as an error in §4.5; blank = "use default", accept directly and do not re-prompt
- Never change the §8 endpoint `https://pkg.agenvoy.com/upload`; do not `ask_user` for a URL or fall back to staging / custom domains
- Never skip the first §8.1 POST (the one that triggers the email) and jump to §8.3 with a guessed code; the code must come from the worker email and be entered by the user
- Never use `popupSecret` to collect the code in §8.2; the code is not a secret, expires in 60s, plaintext echo helps the user paste it
- Never use `run_command` with `curl` / `wget`; uploads must use `send_http_request` with `content_type=multipart`, binary read from `files[].path`
- Never simplify the §8 `send_http_request` payload — **all four fields (`url`/`method`/`content_type`/`body`) are required**, and `body` must contain both `fields` and `files`
- Never omit `content_type: "multipart"` (defaults to `json`, worker won't see multipart)
- Never put manifest JSON into `files[]` (it's a text field, goes under `fields.manifest`); never put tar bytes into `fields` (binary goes under `files[].path` and the handler reads from disk)
- Never guess `status_code`; use the `send_http_request` envelope `status_code` as the only branch signal
- Never auto-bump version and re-POST after 409 / 422; both codes signal "user-side mistake" — go back through `/version-generate` or manual adjustment, then re-run the whole skill
- Never upload tarball + manifest to any endpoint other than §8 (raw GitHub / S3 / any other worker variant)
</content>
</invoke>
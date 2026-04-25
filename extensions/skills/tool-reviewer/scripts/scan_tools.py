#!/usr/bin/env python3
"""
Scan Agenvoy tool definitions and apply deterministic checks against the
four tool design rules from CLAUDE.md.

Sources:
  - Built-in Go tools  : internal/tools/**/*.go (toolRegister.Regist blocks)
  - API extensions     : extensions/apis/*.json
  - Script extensions  : extensions/scripts/*/tool.json

Output: JSON to stdout
  {
    "tools": [
      {
        "source": "builtin" | "api" | "script",
        "name": "...",
        "description": "...",
        "parameters": { <name>: { "type": ..., "description": ..., "default": ... } },
        "required": [ ... ],
        "file": "<path>",
        "line": <int|null>
      }
    ],
    "deterministic_violations": [
      { "tool": "<name>", "source": "...", "rule": "R3_NON_ENGLISH_DESCRIPTION", "detail": "...", "file": "...", "line": <int|null> }
    ]
  }
"""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path
from typing import Any


# ---------- CJK / non-English detection ----------

CJK_RANGES = [
    (0x3000, 0x303F),   # CJK Symbols and Punctuation (full-width 、。「」)
    (0x3040, 0x309F),   # Hiragana
    (0x30A0, 0x30FF),   # Katakana
    (0x3400, 0x4DBF),   # CJK Extension A
    (0x4E00, 0x9FFF),   # CJK Unified Ideographs
    (0xAC00, 0xD7AF),   # Hangul Syllables
    (0xF900, 0xFAFF),   # CJK Compatibility Ideographs
    (0xFF00, 0xFFEF),   # Halfwidth and Fullwidth Forms
]


def has_cjk(text: str) -> bool:
    if not isinstance(text, str):
        return False
    for ch in text:
        cp = ord(ch)
        for lo, hi in CJK_RANGES:
            if lo <= cp <= hi:
                return True
    return False


# ---------- Description heuristics ----------

RE_BOLD = re.compile(r"\*\*[^*\n]+\*\*|__[^_\n]+__")
RE_NUMBERED = re.compile(r"(?:^|\n)\s*(?:\(\d+\)|\d+\.)\s")
RE_TOOL_COMPARISON = re.compile(
    r"\bvs\.?\s|\bprefer\s+over\s|\binstead\s+of\s+\w+_\w+",
    re.IGNORECASE,
)


def desc_violations(desc: str) -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    if not desc:
        return out
    if RE_BOLD.search(desc):
        out.append(("R2_BOLD_MARKDOWN", "Description contains **bold** / __bold__ markdown"))
    if RE_NUMBERED.search(desc):
        out.append(("R2_NUMBERED_TRIGGER", "Description contains numbered trigger conditions"))
    paragraphs = [p for p in desc.strip().split("\n\n") if p.strip()]
    if len(paragraphs) > 2:
        out.append(("R2_MULTI_PARAGRAPH", f"Description has {len(paragraphs)} paragraphs (> 2)"))
    if RE_TOOL_COMPARISON.search(desc):
        out.append(("R2_TOOL_COMPARISON", "Description compares against another tool"))
    return out


# ---------- Parameter normalization ----------

def normalize_params(raw_params: Any) -> tuple[dict[str, dict[str, Any]], list[str]]:
    """
    Normalize parameter definitions across the three formats:
      - Built-in / script tool: { "type": "object", "properties": {...}, "required": [...] }
      - API tool: { "<name>": { "type": ..., "description": ..., "required": bool, "default": ... } }
    Returns (properties_map, required_list).
    """
    if not isinstance(raw_params, dict):
        return {}, []

    if raw_params.get("type") == "object" and isinstance(raw_params.get("properties"), dict):
        props = raw_params["properties"]
        required = raw_params.get("required") or []
        return props, list(required)

    props: dict[str, dict[str, Any]] = {}
    required: list[str] = []
    for name, meta in raw_params.items():
        if not isinstance(meta, dict):
            continue
        meta_copy = dict(meta)
        if meta_copy.pop("required", False):
            required.append(name)
        props[name] = meta_copy
    return props, required


def param_violations(name: str, props: dict[str, dict[str, Any]], required: list[str]) -> list[tuple[str, str]]:
    out: list[tuple[str, str]] = []
    req_set = set(required)
    for pname, meta in props.items():
        if not isinstance(meta, dict):
            continue
        desc = meta.get("description", "")
        if has_cjk(desc):
            out.append(("R3_NON_ENGLISH_PARAM", f"Parameter '{pname}' description is non-English"))
        is_required = pname in req_set
        has_default = "default" in meta
        if not is_required and not has_default:
            out.append(("R4_OPTIONAL_NO_DEFAULT", f"Optional parameter '{pname}' has no default"))
        if is_required and has_default:
            out.append(("R4_REQUIRED_HAS_DEFAULT", f"Required parameter '{pname}' carries a default"))
    return out


# ---------- Source loaders ----------

def load_api_tools(repo: Path) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    api_dir = repo / "extensions" / "apis"
    if not api_dir.is_dir():
        return out
    for f in sorted(api_dir.glob("*.json")):
        try:
            data = json.loads(f.read_text(encoding="utf-8"))
        except Exception as e:
            sys.stderr.write(f"[skip] {f}: {e}\n")
            continue
        name = data.get("name", "")
        out.append({
            "source": "api",
            "name": f"api_{name}" if name else "(missing)",
            "raw_name": name,
            "description": data.get("description", ""),
            "raw_parameters": data.get("parameters", {}),
            "file": str(f.relative_to(repo)),
            "line": None,
        })
    return out


def load_script_tools(repo: Path) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    script_dir = repo / "extensions" / "scripts"
    if not script_dir.is_dir():
        return out
    for sub in sorted(script_dir.iterdir()):
        if not sub.is_dir():
            continue
        tool_json = sub / "tool.json"
        if not tool_json.is_file():
            continue
        try:
            data = json.loads(tool_json.read_text(encoding="utf-8"))
        except Exception as e:
            sys.stderr.write(f"[skip] {tool_json}: {e}\n")
            continue
        name = data.get("name", "")
        out.append({
            "source": "script",
            "name": f"script_{name}" if name else "(missing)",
            "raw_name": name,
            "description": data.get("description", ""),
            "raw_parameters": data.get("parameters", {}),
            "file": str(tool_json.relative_to(repo)),
            "line": None,
        })
    return out


# Built-in Go tool parser:
# Looks for `toolRegister.Regist(toolRegister.Def{ ... })` blocks and pulls
# Name / Description / Parameters via brace-depth scan + minimal Go literal eval.

RE_REGIST_OPEN = re.compile(r"toolRegister\.Regist\s*\(\s*toolRegister\.Def\s*\{")
RE_NAME = re.compile(r'Name\s*:\s*"([^"]+)"')
RE_NAME_IDENT = re.compile(r'Name\s*:\s*([A-Za-z_][A-Za-z0-9_.]*)\b')
RE_DESC_BACKTICK = re.compile(r"Description\s*:\s*`([^`]*)`", re.DOTALL)
RE_DESC_DQUOTE = re.compile(r'Description\s*:\s*"((?:\\.|[^"\\])*)"', re.DOTALL)


def find_balanced(text: str, open_idx: int, open_ch: str = "{", close_ch: str = "}") -> int:
    depth = 0
    i = open_idx
    n = len(text)
    in_str = False
    str_ch = ""
    while i < n:
        ch = text[i]
        if in_str:
            if ch == "\\":
                i += 2
                continue
            if ch == str_ch:
                in_str = False
            i += 1
            continue
        if ch in ('"', "`"):
            in_str = True
            str_ch = ch
            i += 1
            continue
        if ch == open_ch:
            depth += 1
        elif ch == close_ch:
            depth -= 1
            if depth == 0:
                return i
        i += 1
    return -1


def _unescape_go_dquote(raw: str) -> str:
    # Handle the common Go double-quoted escapes WITHOUT going through
    # unicode_escape (which would re-decode UTF-8 bytes as latin-1 and
    # turn CJK into mojibake).
    out: list[str] = []
    i = 0
    n = len(raw)
    while i < n:
        ch = raw[i]
        if ch == "\\" and i + 1 < n:
            nxt = raw[i + 1]
            mapping = {"n": "\n", "t": "\t", "r": "\r", '"': '"', "\\": "\\", "`": "`", "'": "'", "0": "\0"}
            if nxt in mapping:
                out.append(mapping[nxt])
                i += 2
                continue
            if nxt == "u" and i + 5 < n:
                try:
                    out.append(chr(int(raw[i + 2 : i + 6], 16)))
                    i += 6
                    continue
                except ValueError:
                    pass
            out.append(nxt)
            i += 2
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def extract_go_string(s: str, key: str) -> str:
    # Backtick (raw) string first — multi-line, no escapes.
    m = re.search(rf'{key}\s*:\s*`([^`]*)`', s, re.DOTALL)
    if m:
        return m.group(1).strip()
    m = re.search(rf'{key}\s*:\s*"((?:\\.|[^"\\])*)"', s, re.DOTALL)
    if m:
        return _unescape_go_dquote(m.group(1))
    return ""


# Parse the Parameters: map[string]any{...} block into {properties, required} where possible.
# Best-effort — Go literals can be deeply nested, we focus on the shapes Agenvoy actually uses.

def extract_params_block(body: str) -> str:
    m = re.search(r"Parameters\s*:\s*map\[string\]any\s*\{", body)
    if not m:
        return ""
    open_brace = body.find("{", m.start())
    if open_brace == -1:
        return ""
    end = find_balanced(body, open_brace)
    if end == -1:
        return ""
    return body[open_brace : end + 1]


def parse_required(params_block: str) -> list[str]:
    """
    Find the OUTER `"required": []string{...}` — the one at depth 1 inside
    params_block's outermost braces. Inner schemas (nested item required)
    are at deeper levels and must be skipped.
    """
    if not params_block.startswith("{"):
        return []
    depth = 0
    i = 0
    n = len(params_block)
    in_str = False
    str_ch = ""
    while i < n:
        ch = params_block[i]
        if in_str:
            if ch == "\\":
                i += 2
                continue
            if ch == str_ch:
                in_str = False
            i += 1
            continue
        # Check for "required" literal BEFORE entering string mode for its opening quote.
        if depth == 1 and params_block.startswith('"required"', i):
            m = re.match(r'"required"\s*:\s*\[\]string\s*\{([^}]*)\}', params_block[i:], re.DOTALL)
            if m:
                return [s.strip().strip('"') for s in m.group(1).split(",") if s.strip().strip('"')]
        if ch in ('"', "`"):
            in_str = True
            str_ch = ch
            i += 1
            continue
        if ch == "{":
            depth += 1
            i += 1
            continue
        if ch == "}":
            depth -= 1
            i += 1
            continue
        i += 1
    return []


def parse_properties(params_block: str) -> dict[str, dict[str, Any]]:
    """
    Walk the "properties": map[string]any{ "<name>": map[string]any{ ... }, ... } block.
    Capture each property's description, default presence.
    """
    pm = re.search(r'"properties"\s*:\s*map\[string\]any\s*\{', params_block)
    if not pm:
        return {}
    open_brace = params_block.find("{", pm.start())
    end = find_balanced(params_block, open_brace)
    if end == -1:
        return {}
    inner = params_block[open_brace + 1 : end]

    out: dict[str, dict[str, Any]] = {}
    i = 0
    n = len(inner)
    while i < n:
        # Find next  "<name>": map[string]any{
        m = re.search(r'"([A-Za-z0-9_]+)"\s*:\s*map\[string\]any\s*\{', inner[i:])
        if not m:
            break
        prop_name = m.group(1)
        open_inner = inner.find("{", i + m.start())
        if open_inner == -1:
            break
        end_inner = find_balanced(inner, open_inner)
        if end_inner == -1:
            break
        prop_body = inner[open_inner + 1 : end_inner]
        meta: dict[str, Any] = {}
        desc = extract_go_string(prop_body, '"description"')
        if desc:
            meta["description"] = desc
        if re.search(r'"default"\s*:', prop_body):
            meta["default"] = "<set>"
        out[prop_name] = meta
        i = end_inner + 1
    return out


def load_builtin_tools(repo: Path) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    tools_dir = repo / "internal" / "tools"
    if not tools_dir.is_dir():
        return out
    for path in sorted(tools_dir.rglob("*.go")):
        try:
            text = path.read_text(encoding="utf-8")
        except Exception:
            continue
        for m in RE_REGIST_OPEN.finditer(text):
            open_brace = text.find("{", m.start())
            close_brace = find_balanced(text, open_brace)
            if close_brace == -1:
                continue
            body = text[open_brace : close_brace + 1]
            name = ""
            nm = RE_NAME.search(body)
            if nm:
                name = nm.group(1)
            else:
                ident = RE_NAME_IDENT.search(body)
                if ident:
                    name = f"<ident:{ident.group(1)}>"
            description = extract_go_string(body, "Description")
            params_block = extract_params_block(body)
            required = parse_required(params_block) if params_block else []
            properties = parse_properties(params_block) if params_block else {}
            line = text.count("\n", 0, m.start()) + 1
            out.append({
                "source": "builtin",
                "name": name,
                "raw_name": name,
                "description": description,
                "raw_parameters": {
                    "type": "object",
                    "properties": properties,
                    "required": required,
                },
                "file": str(path.relative_to(repo)),
                "line": line,
            })
    return out


# ---------- Main ----------

def main() -> int:
    if len(sys.argv) < 2:
        sys.stderr.write("usage: scan_tools.py <repo_root>\n")
        return 2
    repo = Path(sys.argv[1]).resolve()
    if not repo.is_dir():
        sys.stderr.write(f"not a directory: {repo}\n")
        return 2

    tools: list[dict[str, Any]] = []
    tools.extend(load_builtin_tools(repo))
    tools.extend(load_api_tools(repo))
    tools.extend(load_script_tools(repo))

    violations: list[dict[str, Any]] = []
    output_tools: list[dict[str, Any]] = []
    for t in tools:
        props, required = normalize_params(t.get("raw_parameters", {}))
        desc = t.get("description", "") or ""
        name = t.get("name", "") or "(missing)"

        if has_cjk(desc):
            violations.append({
                "tool": name,
                "source": t["source"],
                "rule": "R3_NON_ENGLISH_DESCRIPTION",
                "detail": "Tool description contains CJK characters",
                "file": t["file"],
                "line": t["line"],
            })
        for rule, detail in desc_violations(desc):
            violations.append({
                "tool": name,
                "source": t["source"],
                "rule": rule,
                "detail": detail,
                "file": t["file"],
                "line": t["line"],
            })
        for rule, detail in param_violations(name, props, required):
            violations.append({
                "tool": name,
                "source": t["source"],
                "rule": rule,
                "detail": detail,
                "file": t["file"],
                "line": t["line"],
            })

        output_tools.append({
            "source": t["source"],
            "name": name,
            "description": desc,
            "parameters": props,
            "required": required,
            "file": t["file"],
            "line": t["line"],
        })

    json.dump(
        {
            "tools": output_tools,
            "deterministic_violations": violations,
            "summary": {
                "tool_count": len(output_tools),
                "violation_count": len(violations),
                "by_source": {
                    src: sum(1 for x in output_tools if x["source"] == src)
                    for src in ("builtin", "api", "script")
                },
            },
        },
        sys.stdout,
        ensure_ascii=False,
        indent=2,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())

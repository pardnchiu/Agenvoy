#!/usr/bin/env python3
"""
Scheduler Skill Initializer - Creates a new scheduler-triggered skill from template.

Usage:
    init_scheduler_skill.py <short-name>

Examples:
    init_scheduler_skill.py daily-hn-digest
    init_scheduler_skill.py meeting-reminder

Output:
    ~/.config/agenvoy/skills/scheduler/<short-name>-<hash8>/SKILL.md

Notes:
    - <short-name> is normalized to lowercase, hyphen-case.
    - A random 8-char hex suffix is appended to avoid collisions across
      independent scheduling requests.
    - No 'scheduler-' prefix anywhere: dir, frontmatter name, and add_task /
      add_cron skill_name argument all use the suffixed short name.
    - On hash collision (vanishingly rare) the script exits non-zero; rerun.
"""

import argparse
import re
import secrets
import sys
from pathlib import Path

MAX_NAME_LENGTH = 64
HASH_BYTES = 4  # 8 hex chars
ROOT = Path.home() / ".config" / "agenvoy" / "skills" / "scheduler"

TEMPLATE = """---
name: {name}
description: [TODO: 一句話描述何時觸發、做什麼。例：抓取 X、提醒 Y、彙整 Z]
---

# {title}

## 任務

[TODO: 描述被觸發時要做的具體行為。
- 引用要呼叫的 tool 名稱與必要參數
- 步驟以祈使式列出
- 不假設對話上下文（subagent session 從零開始）]

## 輸出格式

[TODO: 期望輸出形式。
- 例：1 行總結 + 條列 5 筆 + 結尾時間戳
- 例：JSON object with fields ...]
"""


def normalize(name: str) -> str:
    s = name.strip().lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = re.sub(r"-{2,}", "-", s).strip("-")
    return s


def title_case(name: str) -> str:
    return " ".join(word.capitalize() for word in name.split("-"))


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Initialize a scheduler-triggered skill directory.",
    )
    parser.add_argument(
        "short_name",
        help="Short name (kebab-case). No 'scheduler-' prefix.",
    )
    args = parser.parse_args()

    raw = args.short_name
    base = normalize(raw)
    if not base:
        print("[ERROR] short name must contain at least one letter or digit", file=sys.stderr)
        return 1

    suffix = secrets.token_hex(HASH_BYTES)
    full = f"{base}-{suffix}"
    if len(full) > MAX_NAME_LENGTH:
        print(
            f"[ERROR] full name '{full}' too long ({len(full)} > {MAX_NAME_LENGTH})",
            file=sys.stderr,
        )
        return 1

    skill_dir = ROOT / full
    skill_md = skill_dir / "SKILL.md"

    if raw != base:
        print(f"note: normalized '{raw}' -> '{base}'")

    if skill_md.exists():
        print(f"[ERROR] collision on {full}; rerun to roll a fresh suffix", file=sys.stderr)
        return 1

    skill_dir.mkdir(parents=True, exist_ok=True)
    skill_md.write_text(TEMPLATE.format(name=full, title=title_case(base)))

    print(f"[OK] created   : {skill_md}")
    print(f"[OK] skill name: {full}")
    print()
    print("Next: edit SKILL.md to fill in TODOs (description + body).")
    print("Then: add_task(time, skill_name='{}') or".format(full))
    print("      add_cron(time, skill_name='{}')".format(full))
    return 0


if __name__ == "__main__":
    sys.exit(main())

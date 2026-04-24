#!/usr/bin/env python3
"""readme-generate skill: initialize or read author config.

Config path: ~/.skill-readme-generate.json

Usage:
    setup_config.py                 Interactive: prompt if missing, otherwise print
    setup_config.py check           Print config as JSON; exit 1 if missing
    setup_config.py write NAME EMAIL URL OWNER
                                    Non-interactive write (for agent use)
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

CONFIG_PATH = Path.home() / ".skill-readme-generate.json"
REQUIRED_FIELDS = ("author_name", "author_email", "author_url", "github_owner")


def load_config() -> dict | None:
    if not CONFIG_PATH.exists():
        return None
    try:
        data = json.loads(CONFIG_PATH.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return None
    if not isinstance(data, dict):
        return None
    if not all(isinstance(data.get(k), str) and data.get(k) for k in REQUIRED_FIELDS):
        return None
    return data


def write_config(config: dict) -> None:
    CONFIG_PATH.write_text(
        json.dumps(config, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def prompt_interactive() -> dict:
    if not sys.stdin.isatty():
        print(
            "ERROR: stdin is not a TTY. "
            "Run this script in a terminal, or use "
            "`setup_config.py write NAME EMAIL URL OWNER` for non-interactive mode.",
            file=sys.stderr,
        )
        sys.exit(2)

    print(f"Initializing readme-generate config at {CONFIG_PATH}", file=sys.stderr)
    print("All fields are required.", file=sys.stderr)
    print("", file=sys.stderr)

    def ask(label: str, example: str) -> str:
        while True:
            value = input(f"{label} (e.g., {example}): ").strip()
            if value:
                return value
            print("  value is required, please try again.", file=sys.stderr)

    return {
        "author_name": ask("Author name", "John Doe 張三"),
        "author_email": ask("Email", "dev@example.com"),
        "author_url": ask("Personal URL", "https://linkedin.com/in/johndoe"),
        "github_owner": ask("GitHub username", "johndoe"),
    }


def cmd_check() -> int:
    config = load_config()
    if config is None:
        print("MISSING", file=sys.stderr)
        return 1
    print(json.dumps(config, ensure_ascii=False))
    return 0


def cmd_write(args: list[str]) -> int:
    if len(args) != 4:
        print(
            "Usage: setup_config.py write <author_name> <author_email> <author_url> <github_owner>",
            file=sys.stderr,
        )
        return 2
    config = dict(zip(REQUIRED_FIELDS, args))
    if not all(config[k] for k in REQUIRED_FIELDS):
        print("ERROR: all four fields must be non-empty.", file=sys.stderr)
        return 2
    write_config(config)
    print(f"Config saved to {CONFIG_PATH}", file=sys.stderr)
    print(json.dumps(config, ensure_ascii=False))
    return 0


def cmd_default() -> int:
    config = load_config()
    if config is not None:
        print(json.dumps(config, ensure_ascii=False))
        return 0
    config = prompt_interactive()
    write_config(config)
    print(f"Config saved to {CONFIG_PATH}", file=sys.stderr)
    print(json.dumps(config, ensure_ascii=False))
    return 0


def main() -> int:
    if len(sys.argv) == 1:
        return cmd_default()
    cmd = sys.argv[1]
    if cmd == "check":
        return cmd_check()
    if cmd == "write":
        return cmd_write(sys.argv[2:])
    if cmd in ("-h", "--help", "help"):
        print(__doc__)
        return 0
    print(f"Unknown command: {cmd}", file=sys.stderr)
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Entry point for code-reviewer.

Detects the project's primary language (Go / Python / JavaScript / TypeScript)
and dispatches to the corresponding analyzer. Emits a JSON report on stdout.
"""
import json
import sys
from dataclasses import asdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import analyze_go
import analyze_js_ts
import analyze_python
from common import Issue, ProjectAnalysis

_LANGUAGE_MARKERS: tuple[tuple[str, tuple[str, ...]], ...] = (
    ("go", ("go.mod",)),
    ("typescript", ("tsconfig.json",)),
    ("javascript", ("package.json",)),
    ("python", ("pyproject.toml", "setup.py", "requirements.txt", "Pipfile")),
)

_EXTENSION_MAP = {
    ".go": "go",
    ".ts": "typescript",
    ".tsx": "typescript",
    ".js": "javascript",
    ".jsx": "javascript",
    ".mjs": "javascript",
    ".cjs": "javascript",
    ".py": "python",
}

_SEVERITY_ORDER = {"critical": 0, "high": 1, "medium": 2, "low": 3}


def detect_language(root: Path) -> str:
    for lang, files in _LANGUAGE_MARKERS:
        if any((root / f).exists() for f in files):
            return lang

    counts: dict[str, int] = {}
    for f in root.rglob("*"):
        if not f.is_file():
            continue
        lang = _EXTENSION_MAP.get(f.suffix)
        if lang:
            counts[lang] = counts.get(lang, 0) + 1
    return max(counts, key=counts.get) if counts else "unknown"


def _dispatch(lang: str, root: Path, script_dir: Path) -> ProjectAnalysis:
    if lang == "go":
        return analyze_go.analyze(root, script_dir / "go_ast.go")
    if lang == "python":
        return analyze_python.analyze(root)
    if lang in ("javascript", "typescript"):
        return analyze_js_ts.analyze(root)

    analysis = ProjectAnalysis(language=lang, name=root.name)
    analysis.issues.append(Issue(
        severity="low",
        category="quality",
        title="不支援的語言",
        description=f"此 skill 僅支援 Go / Python / JavaScript / TypeScript；偵測到: {lang}",
        file="(system)",
        suggestion="切換至支援語言的專案",
    ))
    return analysis


def _build_output(analysis: ProjectAnalysis) -> dict:
    analysis.issues.sort(key=lambda x: _SEVERITY_ORDER.get(x.severity, 4))

    counts = {"critical": 0, "high": 0, "medium": 0, "low": 0}
    for issue in analysis.issues:
        counts[issue.severity] = counts.get(issue.severity, 0) + 1

    return {
        "language": analysis.language,
        "name": analysis.name,
        "file_count": len(analysis.files),
        "function_count": len(analysis.functions),
        "files": sorted(analysis.files),
        "functions": [asdict(f) for f in analysis.functions],
        "issues": [asdict(i) for i in analysis.issues],
        "issue_counts": counts,
        "metrics": asdict(analysis.metrics),
        "dependencies": analysis.dependencies,
    }


def main() -> int:
    if len(sys.argv) < 2:
        print("Usage: analyze_code.py <project_path>", file=sys.stderr)
        return 1

    root = Path(sys.argv[1]).resolve()
    if not root.exists():
        print(json.dumps({"error": f"Path does not exist: {sys.argv[1]}"}))
        return 1

    lang = detect_language(root)
    script_dir = Path(__file__).resolve().parent
    analysis = _dispatch(lang, root, script_dir)
    output = _build_output(analysis)
    print(json.dumps(output, indent=2, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())

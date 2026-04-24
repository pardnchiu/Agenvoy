"""JavaScript/TypeScript analyzer. Delegates to project-local eslint when available."""
import json
import subprocess
from pathlib import Path

from common import (
    IGNORE_DIRS,
    Issue,
    ProjectAnalysis,
    detect_command_injection,
    detect_commented_code,
    detect_hardcoded_credentials,
    detect_sql_injection,
)


_ESLINT_CANDIDATES = (
    "node_modules/.bin/eslint",
    "node_modules/.bin/eslint.cmd",
)

_ESLINT_SEVERITY_MAP = {
    2: "high",
    1: "medium",
}


def _find_eslint(root: Path) -> Path | None:
    for candidate in _ESLINT_CANDIDATES:
        p = root / candidate
        if p.exists() and p.is_file():
            return p
    return None


def _run_eslint(eslint: Path, root: Path) -> list[dict]:
    try:
        result = subprocess.run(
            [str(eslint), ".", "--format", "json"],
            cwd=str(root),
            capture_output=True,
            timeout=300,
            text=True,
        )
        if result.stdout.strip():
            return json.loads(result.stdout)
    except (FileNotFoundError, subprocess.TimeoutExpired, json.JSONDecodeError):
        pass
    return []


def _map_eslint_messages(eslint_data: list[dict], root: Path) -> list[Issue]:
    issues: list[Issue] = []
    for file_result in eslint_data:
        file_path = file_result.get("filePath", "")
        try:
            rel = str(Path(file_path).relative_to(root))
        except ValueError:
            rel = file_path
        for msg in file_result.get("messages", []):
            sev = _ESLINT_SEVERITY_MAP.get(msg.get("severity", 1), "low")
            rule = msg.get("ruleId") or "syntax"
            issues.append(Issue(
                severity=sev,
                category="quality",
                title=f"eslint: {rule}",
                description=msg.get("message", ""),
                file=rel,
                line=msg.get("line", 0),
                code_snippet=(msg.get("source") or "").strip()[:120],
                suggestion="依 eslint 規則建議修正",
            ))
    return issues


def _detect_language(root: Path) -> str:
    if (root / "tsconfig.json").exists():
        return "typescript"
    return "javascript"


def _parse_package_json(root: Path) -> tuple[str, list[str]]:
    pkg_json = root / "package.json"
    if not pkg_json.exists():
        return "", []
    try:
        data = json.loads(pkg_json.read_text())
    except (OSError, json.JSONDecodeError):
        return "", []
    name = data.get("name", "") or ""
    deps = list((data.get("dependencies") or {}).keys())
    return name, deps


def _iter_source_files(root: Path, lang: str):
    exts = (".ts", ".tsx") if lang == "typescript" else (".js", ".jsx", ".mjs", ".cjs")
    for ext in exts:
        for f in root.rglob(f"*{ext}"):
            if any(p in f.parts for p in IGNORE_DIRS):
                continue
            if f.name.endswith(".d.ts"):
                continue
            if any(suffix in f.name for suffix in (".spec.", ".test.")):
                continue
            yield f


def analyze(root: Path) -> ProjectAnalysis:
    lang = _detect_language(root)
    analysis = ProjectAnalysis(language=lang, name=root.name)

    name, deps = _parse_package_json(root)
    if name:
        analysis.name = name
    analysis.dependencies = deps

    total_lines = 0
    for src_file in _iter_source_files(root, lang):
        rel = str(src_file.relative_to(root))
        analysis.files.append(rel)

        try:
            content = src_file.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue

        total_lines += content.count('\n') + 1
        analysis.issues.extend(detect_hardcoded_credentials(content, rel))
        analysis.issues.extend(detect_sql_injection(content, rel))
        analysis.issues.extend(detect_command_injection(content, rel))
        analysis.issues.extend(detect_commented_code(content, rel, lang))

    analysis.metrics.total_lines = total_lines

    eslint = _find_eslint(root)
    if eslint is not None:
        eslint_data = _run_eslint(eslint, root)
        analysis.issues.extend(_map_eslint_messages(eslint_data, root))
    else:
        analysis.issues.append(Issue(
            severity="low",
            category="quality",
            title="eslint 不可用",
            description="未找到 node_modules/.bin/eslint；僅執行字串模式掃描",
            file="(system)",
            suggestion="於專案執行 `npm install eslint --save-dev` 並設定 eslint config 以啟用完整分析",
        ))

    return analysis

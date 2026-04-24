"""Go project analyzer. Combines AST analysis (via go_ast.go) with string-level scans."""
import json
import re
import subprocess
from pathlib import Path

from common import (
    FunctionInfo,
    IGNORE_DIRS,
    Issue,
    ProjectAnalysis,
    detect_command_injection,
    detect_commented_code,
    detect_hardcoded_credentials,
    detect_sql_injection,
)


def _run_gofmt(path: Path) -> None:
    try:
        subprocess.run(
            ["gofmt", "-s", "-w", str(path)],
            capture_output=True,
            timeout=30,
            check=False,
        )
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass


def _run_go_ast(helper: Path, root: Path) -> dict | None:
    try:
        result = subprocess.run(
            ["go", "run", str(helper), str(root)],
            capture_output=True,
            timeout=120,
            text=True,
        )
        if result.returncode != 0:
            return None
        return json.loads(result.stdout)
    except (FileNotFoundError, subprocess.TimeoutExpired, json.JSONDecodeError):
        return None


_MODULE_RE = re.compile(r"^module\s+(.+)$", re.MULTILINE)
_BLOCK_DEP_RE = re.compile(r'([^\s]+)\s+v')
_SINGLE_DEP_RE = re.compile(r'require\s+([^\s]+)\s+v')


def _parse_go_mod(go_mod: Path) -> tuple[str, list[str]]:
    try:
        content = go_mod.read_text()
    except OSError:
        return "", []

    name = ""
    if m := _MODULE_RE.search(content):
        name = m.group(1).strip().split("/")[-1]

    deps: list[str] = []
    in_require = False
    for raw in content.splitlines():
        line = raw.strip()
        if not line:
            continue
        if in_require:
            if line == ")":
                in_require = False
                continue
            if m := _BLOCK_DEP_RE.match(line):
                deps.append(m.group(1))
            continue
        if line.startswith("require ("):
            in_require = True
            continue
        if line.startswith("require "):
            if m := _SINGLE_DEP_RE.match(line):
                deps.append(m.group(1))
    return name, deps


def _merge_ast_output(data: dict, analysis: ProjectAnalysis) -> None:
    for f in data.get("functions", []):
        analysis.functions.append(FunctionInfo(
            name=f["name"],
            signature=f["signature"],
            file=f["file"],
            line=f["line"],
            line_count=f["line_count"],
            has_doc=f["has_doc"],
        ))
    for i in data.get("issues", []):
        analysis.issues.append(Issue(
            severity=i["severity"],
            category=i["category"],
            title=i["title"],
            description=i["description"],
            file=i["file"],
            line=i["line"],
            code_snippet=i.get("code_snippet", ""),
            suggestion=i.get("suggestion", ""),
        ))
    analysis.metrics.max_nesting_depth = max(
        analysis.metrics.max_nesting_depth,
        data.get("max_nesting_depth", 0),
    )


def _apply_go_mod(root: Path, analysis: ProjectAnalysis) -> None:
    go_mod = root / "go.mod"
    if not go_mod.exists():
        return
    name, deps = _parse_go_mod(go_mod)
    if name:
        analysis.name = name
    analysis.dependencies = deps


def _iter_go_sources(root: Path):
    for go_file in root.rglob("*.go"):
        if any(p in go_file.parts for p in IGNORE_DIRS):
            continue
        if go_file.name.endswith("_test.go"):
            continue
        yield go_file


def _scan_source_file(path: Path, rel: str, analysis: ProjectAnalysis) -> int:
    _run_gofmt(path)
    try:
        content = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return 0

    analysis.issues.extend(detect_hardcoded_credentials(content, rel))
    analysis.issues.extend(detect_sql_injection(content, rel))
    analysis.issues.extend(detect_command_injection(content, rel))
    analysis.issues.extend(detect_commented_code(content, rel, "go"))
    return content.count('\n') + 1


def _scan_sources(root: Path, analysis: ProjectAnalysis) -> None:
    total_lines = 0
    for go_file in _iter_go_sources(root):
        rel = str(go_file.relative_to(root))
        analysis.files.append(rel)
        total_lines += _scan_source_file(go_file, rel, analysis)
    analysis.metrics.total_lines = total_lines


def _run_ast_helper(helper: Path | None, root: Path, analysis: ProjectAnalysis) -> None:
    if helper is None or not helper.exists():
        analysis.issues.append(Issue(
            severity="low",
            category="quality",
            title="Go AST helper 不可用",
            description="找不到 go_ast.go helper；僅執行字串模式掃描",
            file="(system)",
            suggestion="確認 skill 安裝完整",
        ))
        return

    data = _run_go_ast(helper, root)
    if data is None:
        analysis.issues.append(Issue(
            severity="low",
            category="quality",
            title="Go AST 分析失敗",
            description="go run go_ast.go 執行失敗；僅保留字串模式掃描結果",
            file="(system)",
            suggestion="確認 go toolchain ≥ 1.21 且可執行",
        ))
        return
    _merge_ast_output(data, analysis)


def _finalize_function_metrics(analysis: ProjectAnalysis) -> None:
    lengths = [f.line_count for f in analysis.functions]
    if not lengths:
        return
    analysis.metrics.avg_function_length = sum(lengths) / len(lengths)
    analysis.metrics.max_function_length = max(lengths)


def analyze(root: Path, helper: Path | None = None) -> ProjectAnalysis:
    analysis = ProjectAnalysis(language="go", name=root.name)
    _apply_go_mod(root, analysis)
    _scan_sources(root, analysis)
    _run_ast_helper(helper, root, analysis)
    _finalize_function_metrics(analysis)
    return analysis

"""Python project analyzer using the built-in ast module."""
import ast
import re
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


_NESTING_NODES = (
    ast.If, ast.For, ast.While, ast.Try, ast.With,
    ast.AsyncFor, ast.AsyncWith,
)

LONG_FUNCTION_THRESHOLD = 50
DEEP_NESTING_THRESHOLD = 3


class _NestingVisitor(ast.NodeVisitor):
    def __init__(self) -> None:
        self.max_depth = 0
        self._current = 0

    def generic_visit(self, node: ast.AST) -> None:
        if isinstance(node, _NESTING_NODES):
            self._current += 1
            if self._current > self.max_depth:
                self.max_depth = self._current
            super().generic_visit(node)
            self._current -= 1
        else:
            super().generic_visit(node)


def _attribute_root(node: ast.Attribute) -> str | None:
    cur: ast.AST = node
    while isinstance(cur, ast.Attribute):
        cur = cur.value
    return cur.id if isinstance(cur, ast.Name) else None


def _referenced_name(node: ast.AST) -> str | None:
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        return _attribute_root(node)
    return None


def _collect_used_names(tree: ast.AST) -> set[str]:
    used: set[str] = set()
    for node in ast.walk(tree):
        if name := _referenced_name(node):
            used.add(name)
    return used


def _unused_import_issue(rel: str, line: int, description: str) -> Issue:
    return Issue(
        severity="low",
        category="quality",
        title="未使用的 import",
        description=description,
        file=rel,
        line=line,
        suggestion="移除未使用的 import",
    )


def _expand_import(node: ast.Import):
    for alias in node.names:
        bound = alias.asname or alias.name.split('.')[0]
        yield node, bound, f"import '{alias.name}' 未被使用"


def _expand_import_from(node: ast.ImportFrom):
    module = node.module or ""
    for alias in node.names:
        if alias.name == "*":
            continue
        bound = alias.asname or alias.name
        yield node, bound, f"from {module} import {alias.name} 未被使用"


def _iter_import_aliases(tree: ast.AST):
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            yield from _expand_import(node)
        elif isinstance(node, ast.ImportFrom):
            yield from _expand_import_from(node)


def _check_unused_imports(tree: ast.AST, rel: str, issues: list[Issue]) -> None:
    used = _collect_used_names(tree)
    for node, bound, description in _iter_import_aliases(tree):
        if bound in used:
            continue
        issues.append(_unused_import_issue(rel, node.lineno, description))


def _check_bare_except(tree: ast.AST, rel: str, issues: list[Issue]) -> None:
    for node in ast.walk(tree):
        if isinstance(node, ast.ExceptHandler) and node.type is None:
            issues.append(Issue(
                severity="medium",
                category="quality",
                title="裸 except",
                description="`except:` 會攔截所有例外（含 KeyboardInterrupt / SystemExit）",
                file=rel,
                line=node.lineno,
                suggestion="改為 `except Exception:` 或更具體的例外型別",
            ))


def _has_docstring(node: ast.FunctionDef | ast.AsyncFunctionDef) -> bool:
    if not node.body:
        return False
    first = node.body[0]
    return (
        isinstance(first, ast.Expr)
        and isinstance(first.value, ast.Constant)
        and isinstance(first.value.value, str)
    )


def _build_function_info(
    node: ast.FunctionDef | ast.AsyncFunctionDef, rel: str,
) -> FunctionInfo:
    start = node.lineno
    end = getattr(node, 'end_lineno', start) or start
    args = [a.arg for a in node.args.args]
    prefix = "async def " if isinstance(node, ast.AsyncFunctionDef) else "def "
    return FunctionInfo(
        name=node.name,
        signature=f"{prefix}{node.name}({', '.join(args)})",
        file=rel,
        line=start,
        line_count=end - start + 1,
        has_doc=_has_docstring(node),
    )


def _check_function_length(info: FunctionInfo, rel: str) -> Issue | None:
    if info.line_count <= LONG_FUNCTION_THRESHOLD:
        return None
    return Issue(
        severity="medium",
        category="quality",
        title="過長的函式",
        description=f"函式 '{info.name}' 有 {info.line_count} 行",
        file=rel,
        line=info.line,
        suggestion="拆分為多個小函式，遵循單一職責原則",
    )


def _measure_nesting(node: ast.FunctionDef | ast.AsyncFunctionDef) -> int:
    visitor = _NestingVisitor()
    for child in node.body:
        visitor.visit(child)
    return visitor.max_depth


def _check_function_nesting(
    name: str, depth: int, rel: str, line: int,
) -> Issue | None:
    if depth <= DEEP_NESTING_THRESHOLD:
        return None
    return Issue(
        severity="medium",
        category="quality",
        title="過深的巢狀結構",
        description=f"函式 '{name}' 巢狀深度 {depth} 層",
        file=rel,
        line=line,
        suggestion="使用 early return 或抽出子函式降低巢狀深度",
    )


def _analyze_function(
    node: ast.FunctionDef | ast.AsyncFunctionDef,
    rel: str,
    analysis: ProjectAnalysis,
) -> None:
    info = _build_function_info(node, rel)
    analysis.functions.append(info)

    if issue := _check_function_length(info, rel):
        analysis.issues.append(issue)

    depth = _measure_nesting(node)
    if depth > analysis.metrics.max_nesting_depth:
        analysis.metrics.max_nesting_depth = depth
    if issue := _check_function_nesting(info.name, depth, rel, info.line):
        analysis.issues.append(issue)


def _analyze_file(path: Path, rel: str, analysis: ProjectAnalysis) -> int:
    try:
        content = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return 0

    try:
        tree = ast.parse(content, filename=str(path))
    except SyntaxError:
        return content.count('\n') + 1

    analysis.issues.extend(detect_hardcoded_credentials(content, rel))
    analysis.issues.extend(detect_sql_injection(content, rel))
    analysis.issues.extend(detect_command_injection(content, rel))
    analysis.issues.extend(detect_commented_code(content, rel, "python"))

    _check_unused_imports(tree, rel, analysis.issues)
    _check_bare_except(tree, rel, analysis.issues)

    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            _analyze_function(node, rel, analysis)

    return content.count('\n') + 1


_REQ_SPLIT = re.compile(r'[=<>!~\s;]')
_PYPROJECT_DEP = re.compile(r'^\s*"([A-Za-z0-9_.-]+)(?:[=<>!~\s]|")', re.MULTILINE)


def _parse_requirements_txt(path: Path) -> list[str]:
    try:
        text = path.read_text()
    except OSError:
        return []
    deps: list[str] = []
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith('#'):
            continue
        name = _REQ_SPLIT.split(line, 1)[0]
        if name:
            deps.append(name)
    return deps


def _parse_pyproject_toml(path: Path) -> list[str]:
    try:
        content = path.read_text()
    except OSError:
        return []
    deps: list[str] = []
    for m in _PYPROJECT_DEP.finditer(content):
        name = m.group(1)
        if name and name not in deps:
            deps.append(name)
    return deps


def _parse_dependencies(root: Path) -> list[str]:
    req = root / "requirements.txt"
    if req.exists():
        return _parse_requirements_txt(req)
    pyproject = root / "pyproject.toml"
    if pyproject.exists():
        return _parse_pyproject_toml(pyproject)
    return []


def analyze(root: Path) -> ProjectAnalysis:
    analysis = ProjectAnalysis(language="python", name=root.name)
    analysis.dependencies = _parse_dependencies(root)

    total_lines = 0
    for py_file in root.rglob("*.py"):
        if any(p in py_file.parts for p in IGNORE_DIRS):
            continue
        rel = str(py_file.relative_to(root))
        analysis.files.append(rel)
        total_lines += _analyze_file(py_file, rel, analysis)

    lengths = [f.line_count for f in analysis.functions]
    analysis.metrics.total_lines = total_lines
    if lengths:
        analysis.metrics.avg_function_length = sum(lengths) / len(lengths)
        analysis.metrics.max_function_length = max(lengths)

    return analysis

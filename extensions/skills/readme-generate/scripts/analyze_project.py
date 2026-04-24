#!/usr/bin/env python3
"""
Analyze project structure and extract API information for README generation.

Full extraction: Python (ast), Go (regex), JavaScript, TypeScript (regex).
Detected only (file listing): PHP, Swift.
"""

import ast
import json
import re
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path


@dataclass
class FunctionInfo:
    name: str
    signature: str
    doc: str = ""
    exported: bool = False
    file: str = ""
    line: int = 0


@dataclass
class TypeInfo:
    name: str
    kind: str  # struct, interface, class, type
    fields: list[dict] = field(default_factory=list)
    doc: str = ""
    file: str = ""


@dataclass
class ProjectAnalysis:
    language: str
    name: str
    description: str = ""
    version: str = ""
    types: list[TypeInfo] = field(default_factory=list)
    functions: list[FunctionInfo] = field(default_factory=list)
    files: list[str] = field(default_factory=list)
    dependencies: list[str] = field(default_factory=list)
    entry_points: list[str] = field(default_factory=list)


IGNORE_DIRS = {
    ".git",
    "node_modules",
    "vendor",
    ".idea",
    ".vscode",
    "__pycache__",
    ".pytest_cache",
    "dist",
    "build",
    "target",
    ".next",
    ".nuxt",
    "coverage",
    ".nyc_output",
}

IGNORE_FILES = {
    ".DS_Store",
    "Thumbs.db",
    ".gitignore",
    ".gitattributes",
    "package-lock.json",
    "yarn.lock",
    "go.sum",
    "Pipfile.lock",
    "poetry.lock",
    "composer.lock",
}

LANGUAGE_INDICATORS = {
    "go": ["go.mod", "go.sum"],
    "python": ["pyproject.toml", "setup.py", "requirements.txt", "Pipfile"],
    "javascript": ["package.json"],
    "typescript": ["tsconfig.json"],
    "php": ["composer.json"],
    "swift": ["Package.swift", "*.xcodeproj"],
}

EXT_LANGUAGE_MAP = {
    ".go": "go",
    ".py": "python",
    ".js": "javascript",
    ".ts": "typescript",
    ".php": "php",
    ".swift": "swift",
}

# Pre-compiled regex patterns (avoid rebuilding on every call).
GO_MODULE_RE = re.compile(r"^module\s+(.+)$", re.MULTILINE)
GO_DEP_RE = re.compile(r"^\t([^\s]+)\s+v[\d.]+", re.MULTILINE)
GO_TYPE_RE = re.compile(
    r"(?://\s*(.+)\n)?type\s+(\w+)\s+(struct|interface)\s*\{([^}]*)\}"
)
GO_FIELD_RE = re.compile(r"(\w+)\s+(\S+)(?:\s+`([^`]+)`)?")
GO_FUNC_RE = re.compile(
    r"(?://\s*(.+)\n)?func\s+(?:\((\w+)\s+\*?(\w+)\)\s+)?(\w+)\s*\(([^)]*)\)\s*(?:\(([^)]*)\)|(\w+))?"
)

PY_NAME_RE = re.compile(r'name\s*=\s*["\']([^"\']+)["\']')
PY_VERSION_RE = re.compile(r'version\s*=\s*["\']([^"\']+)["\']')
PY_DESC_RE = re.compile(r'description\s*=\s*["\']([^"\']+)["\']')
PY_DEP_BLOCK_RE = re.compile(r"dependencies\s*=\s*\[([^\]]*)\]", re.DOTALL)
PY_DEP_RE = re.compile(r'["\']([^"\'<>=!~;\s]+)')

JS_EXPORT_FUNC_RE = re.compile(
    r"export\s+(?:async\s+)?function\s+(\w+)\s*(?:<[^>]+>)?\s*\(([^)]*)\)"
)
JS_EXPORT_CLASS_RE = re.compile(r"export\s+class\s+(\w+)")


def _pattern_exists(root: Path, pattern: str) -> bool:
    if "*" in pattern:
        return any(root.glob(pattern))
    return (root / pattern).exists()


def _normalize_detected_language(root: Path, lang: str) -> str:
    if lang == "javascript" and (root / "tsconfig.json").exists():
        return "typescript"
    return lang


def _detect_by_indicators(root: Path) -> str | None:
    for lang, files in LANGUAGE_INDICATORS.items():
        for pattern in files:
            if not _pattern_exists(root, pattern):
                continue
            return _normalize_detected_language(root, lang)
    return None


def _detect_by_extensions(root: Path) -> str:
    ext_count: dict[str, int] = {}
    for f in root.rglob("*"):
        if not f.is_file():
            continue
        lang = EXT_LANGUAGE_MAP.get(f.suffix)
        if lang is None:
            continue
        ext_count[lang] = ext_count.get(lang, 0) + 1
    return max(ext_count, key=ext_count.get) if ext_count else "unknown"


def detect_language(root: Path) -> str:
    """Detect primary project language."""
    detected = _detect_by_indicators(root)
    if detected is not None:
        return detected
    return _detect_by_extensions(root)


def _parse_go_mod(go_mod: Path) -> tuple[str | None, list[str]]:
    content = go_mod.read_text()
    name: str | None = None
    if m := GO_MODULE_RE.search(content):
        name = m.group(1).split("/")[-1]
    deps = [m.group(1) for m in GO_DEP_RE.finditer(content)]
    return name, deps


def _iter_go_source_files(root: Path):
    for go_file in root.rglob("*.go"):
        if any(p in go_file.parts for p in IGNORE_DIRS):
            continue
        if "_test.go" in go_file.name:
            continue
        yield go_file


def _extract_go_types(content: str, rel_path: str) -> list[TypeInfo]:
    types: list[TypeInfo] = []
    for m in GO_TYPE_RE.finditer(content):
        doc, name, kind, body = m.groups()
        if not name[0].isupper():
            continue
        fields = [
            {"name": fname, "type": ftype, "tag": tag or ""}
            for fname, ftype, tag in (fm.groups() for fm in GO_FIELD_RE.finditer(body))
        ]
        types.append(
            TypeInfo(
                name=name,
                kind=kind,
                fields=fields,
                doc=doc.strip() if doc else "",
                file=rel_path,
            )
        )
    return types


def _extract_go_functions(content: str, rel_path: str) -> list[FunctionInfo]:
    functions: list[FunctionInfo] = []
    for m in GO_FUNC_RE.finditer(content):
        doc, recv_name, recv_type, func_name, params, ret_multi, ret_single = m.groups()
        if not func_name[0].isupper():
            continue
        if recv_type:
            sig = f"func ({recv_name} *{recv_type}) {func_name}({params})"
        else:
            sig = f"func {func_name}({params})"
        ret = ret_multi or ret_single or ""
        if ret:
            sig += f" {ret}" if ret_single else f" ({ret})"
        functions.append(
            FunctionInfo(
                name=func_name,
                signature=sig,
                exported=True,
                doc=doc.strip() if doc else "",
                file=rel_path,
            )
        )
    return functions


def extract_go_info(root: Path) -> ProjectAnalysis:
    """Extract Go project information."""
    analysis = ProjectAnalysis(language="go", name=root.name)

    go_mod = root / "go.mod"
    if go_mod.exists():
        name, deps = _parse_go_mod(go_mod)
        if name:
            analysis.name = name
        analysis.dependencies.extend(deps)

    for go_file in _iter_go_source_files(root):
        rel_path = str(go_file.relative_to(root))
        analysis.files.append(rel_path)
        try:
            content = go_file.read_text()
        except (OSError, UnicodeDecodeError):
            continue
        analysis.types.extend(_extract_go_types(content, rel_path))
        analysis.functions.extend(_extract_go_functions(content, rel_path))

    return analysis


def _format_py_args(args: ast.arguments) -> str:
    parts: list[str] = []
    positional = args.args
    num_positional = len(positional)
    num_defaults = len(args.defaults)
    default_start = num_positional - num_defaults

    for i, arg in enumerate(positional):
        piece = arg.arg
        if arg.annotation is not None:
            piece += f": {ast.unparse(arg.annotation)}"
        if i >= default_start:
            piece += f" = {ast.unparse(args.defaults[i - default_start])}"
        parts.append(piece)

    if args.vararg is not None:
        piece = f"*{args.vararg.arg}"
        if args.vararg.annotation is not None:
            piece += f": {ast.unparse(args.vararg.annotation)}"
        parts.append(piece)
    elif args.kwonlyargs:
        parts.append("*")

    for arg, default in zip(args.kwonlyargs, args.kw_defaults):
        piece = arg.arg
        if arg.annotation is not None:
            piece += f": {ast.unparse(arg.annotation)}"
        if default is not None:
            piece += f" = {ast.unparse(default)}"
        parts.append(piece)

    if args.kwarg is not None:
        piece = f"**{args.kwarg.arg}"
        if args.kwarg.annotation is not None:
            piece += f": {ast.unparse(args.kwarg.annotation)}"
        parts.append(piece)

    return ", ".join(parts)


def _build_py_function(
    node: ast.FunctionDef | ast.AsyncFunctionDef, file: str
) -> FunctionInfo:
    prefix = "async def" if isinstance(node, ast.AsyncFunctionDef) else "def"
    args_str = _format_py_args(node.args)
    ret_str = f" -> {ast.unparse(node.returns)}" if node.returns is not None else ""
    signature = f"{prefix} {node.name}({args_str}){ret_str}"
    return FunctionInfo(
        name=node.name,
        signature=signature,
        doc=ast.get_docstring(node) or "",
        exported=True,
        file=file,
        line=node.lineno,
    )


def _collect_py_class_fields(item: ast.stmt) -> list[dict]:
    if isinstance(item, ast.AnnAssign) and isinstance(item.target, ast.Name):
        fname = item.target.id
        if fname.startswith("_"):
            return []
        ftype = ast.unparse(item.annotation) if item.annotation is not None else ""
        return [{"name": fname, "type": ftype, "tag": ""}]
    if isinstance(item, ast.Assign):
        return [
            {"name": tgt.id, "type": "", "tag": ""}
            for tgt in item.targets
            if isinstance(tgt, ast.Name) and not tgt.id.startswith("_")
        ]
    return []


def _detect_py_class_kind(decorators: list[str]) -> str:
    for dec in decorators:
        if (
            dec == "dataclass"
            or dec.endswith(".dataclass")
            or dec.startswith("dataclass(")
        ):
            return "dataclass"
    return "class"


def _build_py_class(node: ast.ClassDef, file: str) -> TypeInfo:
    fields: list[dict] = []
    for item in node.body:
        fields.extend(_collect_py_class_fields(item))
    decorators = [ast.unparse(d) for d in node.decorator_list]
    return TypeInfo(
        name=node.name,
        kind=_detect_py_class_kind(decorators),
        fields=fields,
        doc=ast.get_docstring(node) or "",
        file=file,
    )


def _parse_pyproject(pyproject: Path, analysis: ProjectAnalysis) -> None:
    content = pyproject.read_text()
    if m := PY_NAME_RE.search(content):
        analysis.name = m.group(1)
    if m := PY_VERSION_RE.search(content):
        analysis.version = m.group(1)
    if m := PY_DESC_RE.search(content):
        analysis.description = m.group(1)
    if dep_block := PY_DEP_BLOCK_RE.search(content):
        for m in PY_DEP_RE.finditer(dep_block.group(1)):
            analysis.dependencies.append(m.group(1))


def _iter_python_files(root: Path):
    for py_file in root.rglob("*.py"):
        if any(p in py_file.parts for p in IGNORE_DIRS):
            continue
        yield py_file


def _build_public_symbol(
    node: ast.stmt, rel_path: str
) -> tuple[str, TypeInfo | FunctionInfo] | None:
    """Classify a module-level node into a public TypeInfo / FunctionInfo, or None."""
    if isinstance(node, ast.ClassDef):
        if node.name.startswith("_"):
            return None
        return "type", _build_py_class(node, rel_path)
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
        if node.name.startswith("_"):
            return None
        return "function", _build_py_function(node, rel_path)
    return None


def _analyze_python_file(
    path: Path, rel_path: str
) -> tuple[list[TypeInfo], list[FunctionInfo]] | None:
    try:
        source = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return None
    try:
        tree = ast.parse(source, filename=str(path))
    except SyntaxError:
        return None

    types: list[TypeInfo] = []
    functions: list[FunctionInfo] = []
    # Module-level definitions only; nested classes/functions are internal.
    for node in tree.body:
        built = _build_public_symbol(node, rel_path)
        if built is None:
            continue
        kind, symbol = built
        if kind == "type":
            types.append(symbol)
        else:
            functions.append(symbol)
    return types, functions


def extract_python_info(root: Path) -> ProjectAnalysis:
    """Extract Python project information via the `ast` module."""
    analysis = ProjectAnalysis(language="python", name=root.name)

    pyproject = root / "pyproject.toml"
    if pyproject.exists():
        _parse_pyproject(pyproject, analysis)

    for py_file in _iter_python_files(root):
        rel_path = str(py_file.relative_to(root))
        analysis.files.append(rel_path)
        result = _analyze_python_file(py_file, rel_path)
        if result is None:
            continue
        types, functions = result
        analysis.types.extend(types)
        analysis.functions.extend(functions)

    return analysis


def _parse_package_json(pkg_json: Path, analysis: ProjectAnalysis) -> None:
    try:
        pkg = json.loads(pkg_json.read_text())
    except (OSError, json.JSONDecodeError):
        return
    analysis.name = pkg.get("name", analysis.name)
    analysis.version = pkg.get("version", "")
    analysis.description = pkg.get("description", "")
    analysis.dependencies = list(pkg.get("dependencies", {}).keys())


def _is_skip_script(name: str) -> bool:
    return ".d.ts" in name or ".spec." in name or ".test." in name


def _iter_script_files(root: Path, ext: str):
    for src_file in root.rglob(ext):
        if any(p in src_file.parts for p in IGNORE_DIRS):
            continue
        if _is_skip_script(src_file.name):
            continue
        yield src_file


def _scan_script_symbols(
    content: str, rel_path: str
) -> tuple[list[TypeInfo], list[FunctionInfo]]:
    functions = [
        FunctionInfo(
            name=name,
            signature=f"function {name}({params})",
            exported=True,
            file=rel_path,
        )
        for name, params in (m.groups() for m in JS_EXPORT_FUNC_RE.finditer(content))
    ]
    types = [
        TypeInfo(name=m.group(1), kind="class", file=rel_path)
        for m in JS_EXPORT_CLASS_RE.finditer(content)
    ]
    return types, functions


def extract_js_ts_info(root: Path, lang: str) -> ProjectAnalysis:
    """Extract JavaScript/TypeScript project information."""
    analysis = ProjectAnalysis(language=lang, name=root.name)

    pkg_json = root / "package.json"
    if pkg_json.exists():
        _parse_package_json(pkg_json, analysis)

    ext = "*.ts" if lang == "typescript" else "*.js"
    for src_file in _iter_script_files(root, ext):
        rel_path = str(src_file.relative_to(root))
        analysis.files.append(rel_path)
        try:
            content = src_file.read_text()
        except (OSError, UnicodeDecodeError):
            continue
        types, functions = _scan_script_symbols(content, rel_path)
        analysis.types.extend(types)
        analysis.functions.extend(functions)

    return analysis


def _list_generic_files(root: Path) -> list[str]:
    files: list[str] = []
    for f in root.rglob("*"):
        if not f.is_file():
            continue
        if any(p in f.parts for p in IGNORE_DIRS):
            continue
        if f.name in IGNORE_FILES:
            continue
        files.append(str(f.relative_to(root)))
    return files


ANALYZERS = {
    "go": extract_go_info,
    "python": extract_python_info,
    "javascript": lambda root: extract_js_ts_info(root, "javascript"),
    "typescript": lambda root: extract_js_ts_info(root, "typescript"),
}


def analyze_project(root_path: str) -> dict:
    """Main entry point for project analysis."""
    root = Path(root_path).resolve()

    if not root.exists():
        return {"error": f"Path does not exist: {root_path}"}

    lang = detect_language(root)
    analyzer = ANALYZERS.get(lang)
    if analyzer is not None:
        analysis = analyzer(root)
    else:
        analysis = ProjectAnalysis(language=lang, name=root.name)
        analysis.files.extend(_list_generic_files(root))

    return {
        "language": analysis.language,
        "name": analysis.name,
        "description": analysis.description,
        "version": analysis.version,
        "files": sorted(analysis.files),
        "types": [asdict(t) for t in analysis.types],
        "functions": [asdict(f) for f in analysis.functions],
        "dependencies": analysis.dependencies,
    }


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: analyze_project.py <project_path>", file=sys.stderr)
        sys.exit(1)

    result = analyze_project(sys.argv[1])
    print(json.dumps(result, indent=2, ensure_ascii=False))

#!/usr/bin/env python3
"""
Analyze project structure and extract API information for README generation.

Full extraction: Python (ast), Go (regex), JavaScript, TypeScript (regex).
Detected only (file listing): PHP, Swift.
"""

import ast
import json
import os
import re
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Optional


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


def detect_language(root: Path) -> str:
    """Detect primary project language."""
    indicators = {
        "go": ["go.mod", "go.sum"],
        "python": ["pyproject.toml", "setup.py", "requirements.txt", "Pipfile"],
        "javascript": ["package.json"],
        "typescript": ["tsconfig.json"],
        "php": ["composer.json"],
        "swift": ["Package.swift", "*.xcodeproj"],
    }

    for lang, files in indicators.items():
        for pattern in files:
            if "*" in pattern:
                if list(root.glob(pattern)):
                    return lang
            elif (root / pattern).exists():
                # Check for TypeScript in package.json
                if lang == "javascript" and (root / "tsconfig.json").exists():
                    return "typescript"
                return lang

    # Fallback: count file extensions
    ext_count = {}
    ext_map = {
        ".go": "go",
        ".py": "python",
        ".js": "javascript",
        ".ts": "typescript",
        ".php": "php",
        ".swift": "swift",
    }

    for f in root.rglob("*"):
        if f.is_file() and f.suffix in ext_map:
            lang = ext_map[f.suffix]
            ext_count[lang] = ext_count.get(lang, 0) + 1

    return max(ext_count, key=ext_count.get) if ext_count else "unknown"


def extract_go_info(root: Path) -> ProjectAnalysis:
    """Extract Go project information."""
    analysis = ProjectAnalysis(language="go", name=root.name)

    # Parse go.mod
    go_mod = root / "go.mod"
    if go_mod.exists():
        content = go_mod.read_text()
        if m := re.search(r"^module\s+(.+)$", content, re.MULTILINE):
            analysis.name = m.group(1).split("/")[-1]

        # Extract dependencies
        for m in re.finditer(r"^\t([^\s]+)\s+v[\d.]+", content, re.MULTILINE):
            analysis.dependencies.append(m.group(1))

    # Parse .go files
    for go_file in root.rglob("*.go"):
        if any(p in go_file.parts for p in IGNORE_DIRS):
            continue
        if "_test.go" in go_file.name:
            continue

        rel_path = str(go_file.relative_to(root))
        analysis.files.append(rel_path)

        try:
            content = go_file.read_text()
        except:
            continue

        # Extract types (struct, interface)
        type_pattern = (
            r"(?://\s*(.+)\n)?type\s+(\w+)\s+(struct|interface)\s*\{([^}]*)\}"
        )
        for m in re.finditer(type_pattern, content):
            doc, name, kind, body = m.groups()
            if name[0].isupper():  # Exported
                fields = []
                for field_m in re.finditer(r"(\w+)\s+(\S+)(?:\s+`([^`]+)`)?", body):
                    fname, ftype, tag = field_m.groups()
                    fields.append({"name": fname, "type": ftype, "tag": tag or ""})

                analysis.types.append(
                    TypeInfo(
                        name=name,
                        kind=kind,
                        fields=fields,
                        doc=doc.strip() if doc else "",
                        file=rel_path,
                    )
                )

        # Extract functions
        func_pattern = r"(?://\s*(.+)\n)?func\s+(?:\((\w+)\s+\*?(\w+)\)\s+)?(\w+)\s*\(([^)]*)\)\s*(?:\(([^)]*)\)|(\w+))?"
        for m in re.finditer(func_pattern, content):
            doc, recv_name, recv_type, func_name, params, ret_multi, ret_single = (
                m.groups()
            )

            if func_name[0].isupper():  # Exported
                if recv_type:
                    sig = f"func ({recv_name} *{recv_type}) {func_name}({params})"
                else:
                    sig = f"func {func_name}({params})"

                ret = ret_multi or ret_single or ""
                if ret:
                    sig += f" {ret}" if ret_single else f" ({ret})"

                analysis.functions.append(
                    FunctionInfo(
                        name=func_name,
                        signature=sig,
                        exported=True,
                        doc=doc.strip() if doc else "",
                        file=rel_path,
                    )
                )

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


def _build_py_class(node: ast.ClassDef, file: str) -> TypeInfo:
    fields: list[dict] = []
    for item in node.body:
        if isinstance(item, ast.AnnAssign) and isinstance(item.target, ast.Name):
            fname = item.target.id
            if fname.startswith("_"):
                continue
            ftype = ast.unparse(item.annotation) if item.annotation is not None else ""
            fields.append({"name": fname, "type": ftype, "tag": ""})
        elif isinstance(item, ast.Assign):
            for tgt in item.targets:
                if isinstance(tgt, ast.Name) and not tgt.id.startswith("_"):
                    fields.append({"name": tgt.id, "type": "", "tag": ""})

    decorators = [ast.unparse(d) for d in node.decorator_list]
    kind = "class"
    for dec in decorators:
        if dec == "dataclass" or dec.endswith(".dataclass") or dec.startswith("dataclass("):
            kind = "dataclass"
            break

    return TypeInfo(
        name=node.name,
        kind=kind,
        fields=fields,
        doc=ast.get_docstring(node) or "",
        file=file,
    )


def extract_python_info(root: Path) -> ProjectAnalysis:
    """Extract Python project information via the `ast` module."""
    analysis = ProjectAnalysis(language="python", name=root.name)

    pyproject = root / "pyproject.toml"
    if pyproject.exists():
        content = pyproject.read_text()
        if m := re.search(r'name\s*=\s*["\']([^"\']+)["\']', content):
            analysis.name = m.group(1)
        if m := re.search(r'version\s*=\s*["\']([^"\']+)["\']', content):
            analysis.version = m.group(1)
        if m := re.search(r'description\s*=\s*["\']([^"\']+)["\']', content):
            analysis.description = m.group(1)
        # Naive [project.dependencies] extraction
        dep_block = re.search(
            r"dependencies\s*=\s*\[([^\]]*)\]", content, re.DOTALL
        )
        if dep_block:
            for m in re.finditer(r'["\']([^"\'<>=!~;\s]+)', dep_block.group(1)):
                analysis.dependencies.append(m.group(1))

    for py_file in root.rglob("*.py"):
        if any(p in py_file.parts for p in IGNORE_DIRS):
            continue

        rel_path = str(py_file.relative_to(root))
        analysis.files.append(rel_path)

        try:
            source = py_file.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue

        try:
            tree = ast.parse(source, filename=str(py_file))
        except SyntaxError:
            continue

        # Module-level definitions only; nested classes/functions are internal.
        for node in tree.body:
            if isinstance(node, ast.ClassDef):
                if node.name.startswith("_"):
                    continue
                analysis.types.append(_build_py_class(node, rel_path))
            elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
                if node.name.startswith("_"):
                    continue
                analysis.functions.append(_build_py_function(node, rel_path))

    return analysis


def extract_js_ts_info(root: Path, lang: str) -> ProjectAnalysis:
    """Extract JavaScript/TypeScript project information."""
    analysis = ProjectAnalysis(language=lang, name=root.name)

    # Parse package.json
    pkg_json = root / "package.json"
    if pkg_json.exists():
        try:
            pkg = json.loads(pkg_json.read_text())
            analysis.name = pkg.get("name", root.name)
            analysis.version = pkg.get("version", "")
            analysis.description = pkg.get("description", "")
            analysis.dependencies = list(pkg.get("dependencies", {}).keys())
        except:
            pass

    ext = "*.ts" if lang == "typescript" else "*.js"
    for src_file in root.rglob(ext):
        if any(p in src_file.parts for p in IGNORE_DIRS):
            continue
        if (
            ".d.ts" in src_file.name
            or ".spec." in src_file.name
            or ".test." in src_file.name
        ):
            continue

        rel_path = str(src_file.relative_to(root))
        analysis.files.append(rel_path)

        try:
            content = src_file.read_text()
        except:
            continue

        # Extract exported functions/classes
        export_func = (
            r"export\s+(?:async\s+)?function\s+(\w+)\s*(?:<[^>]+>)?\s*\(([^)]*)\)"
        )
        for m in re.finditer(export_func, content):
            name, params = m.groups()
            analysis.functions.append(
                FunctionInfo(
                    name=name,
                    signature=f"function {name}({params})",
                    exported=True,
                    file=rel_path,
                )
            )

        export_class = r"export\s+class\s+(\w+)"
        for m in re.finditer(export_class, content):
            analysis.types.append(
                TypeInfo(name=m.group(1), kind="class", file=rel_path)
            )

    return analysis


def analyze_project(root_path: str) -> dict:
    """Main entry point for project analysis."""
    root = Path(root_path).resolve()

    if not root.exists():
        return {"error": f"Path does not exist: {root_path}"}

    lang = detect_language(root)

    if lang == "go":
        analysis = extract_go_info(root)
    elif lang == "python":
        analysis = extract_python_info(root)
    elif lang in ("javascript", "typescript"):
        analysis = extract_js_ts_info(root, lang)
    else:
        analysis = ProjectAnalysis(language=lang, name=root.name)
        for f in root.rglob("*"):
            if f.is_file() and not any(p in f.parts for p in IGNORE_DIRS):
                if f.name not in IGNORE_FILES:
                    analysis.files.append(str(f.relative_to(root)))

    # Convert to dict for JSON output
    result = {
        "language": analysis.language,
        "name": analysis.name,
        "description": analysis.description,
        "version": analysis.version,
        "files": sorted(analysis.files),
        "types": [asdict(t) for t in analysis.types],
        "functions": [asdict(f) for f in analysis.functions],
        "dependencies": analysis.dependencies,
    }

    return result


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: analyze_project.py <project_path>", file=sys.stderr)
        sys.exit(1)

    result = analyze_project(sys.argv[1])
    print(json.dumps(result, indent=2, ensure_ascii=False))

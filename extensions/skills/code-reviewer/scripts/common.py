"""Shared types and detection utilities for code-reviewer analyzers."""
import math
import re
from collections import Counter
from dataclasses import dataclass, field


@dataclass
class Issue:
    severity: str
    category: str
    title: str
    description: str
    file: str
    line: int = 0
    code_snippet: str = ""
    suggestion: str = ""


@dataclass
class FunctionInfo:
    name: str
    signature: str
    file: str
    line: int = 0
    line_count: int = 0
    has_doc: bool = False


@dataclass
class CodeMetrics:
    total_lines: int = 0
    code_lines: int = 0
    avg_function_length: float = 0.0
    max_function_length: int = 0
    max_nesting_depth: int = 0


@dataclass
class ProjectAnalysis:
    language: str
    name: str
    files: list[str] = field(default_factory=list)
    functions: list[FunctionInfo] = field(default_factory=list)
    issues: list[Issue] = field(default_factory=list)
    metrics: CodeMetrics = field(default_factory=CodeMetrics)
    dependencies: list[str] = field(default_factory=list)


IGNORE_DIRS = frozenset({
    ".git", "node_modules", "vendor", ".idea", ".vscode",
    "__pycache__", ".pytest_cache", "dist", "build", "target",
    ".next", ".nuxt", "coverage", ".nyc_output", "venv", ".venv",
})


CREDENTIAL_KEYWORDS = re.compile(
    r'(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?key|auth[_-]?token|private[_-]?key|token)'
    r'\s*[=:]\s*["\']([^"\']{8,})["\']',
    re.IGNORECASE,
)

HIGH_ENTROPY_CANDIDATE = re.compile(r'["\']([A-Za-z0-9+/_=-]{32,})["\']')

UUID_PATTERN = re.compile(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
    re.IGNORECASE,
)

HASH_PATTERNS = (
    re.compile(r'^[0-9a-f]{32}$', re.IGNORECASE),   # MD5
    re.compile(r'^[0-9a-f]{40}$', re.IGNORECASE),   # SHA1
    re.compile(r'^[0-9a-f]{64}$', re.IGNORECASE),   # SHA256
)

ENTROPY_THRESHOLD = 4.0


def shannon_entropy(s: str) -> float:
    if not s:
        return 0.0
    counts = Counter(s)
    length = len(s)
    return -sum((c / length) * math.log2(c / length) for c in counts.values())


def _looks_like_credential(value: str) -> bool:
    if UUID_PATTERN.match(value):
        return False
    for p in HASH_PATTERNS:
        if p.match(value):
            return False
    return shannon_entropy(value) >= ENTROPY_THRESHOLD


_COMMENT_PREFIXES = ("#", "//", "/*", "*")


def _is_comment_line(stripped: str) -> bool:
    return stripped.startswith(_COMMENT_PREFIXES)


def detect_hardcoded_credentials(content: str, file_path: str) -> list[Issue]:
    issues: list[Issue] = []
    is_test = "test" in file_path.lower() or "_test." in file_path

    for i, line in enumerate(content.split('\n'), 1):
        stripped = line.strip()
        if not stripped or _is_comment_line(stripped):
            continue
        if is_test:
            continue

        if m := CREDENTIAL_KEYWORDS.search(line):
            issues.append(Issue(
                severity="critical",
                category="security",
                title="硬編碼密鑰",
                description="偵測到疑似硬編碼的密鑰或密碼",
                file=file_path,
                line=i,
                code_snippet=stripped[:120],
                suggestion="改用環境變數或 secret management 服務（Vault / AWS Secrets Manager 等）",
            ))
            continue

        for m in HIGH_ENTROPY_CANDIDATE.finditer(line):
            value = m.group(1)
            if _looks_like_credential(value):
                issues.append(Issue(
                    severity="high",
                    category="security",
                    title="可疑的高熵字串",
                    description=f"長度 {len(value)} 且 Shannon entropy ≥ {ENTROPY_THRESHOLD}，疑似硬編碼密鑰",
                    file=file_path,
                    line=i,
                    code_snippet=stripped[:120],
                    suggestion="確認是否為密鑰；若是，改用環境變數或 secret management",
                ))
                break

    return issues


SQL_PATTERNS = (
    (re.compile(
        r'(?:query|exec|execute|raw)\s*\([^)]*\+[^)]*["\'][^"\']*'
        r'(?:SELECT|INSERT|UPDATE|DELETE|DROP)', re.IGNORECASE), "字串拼接"),
    (re.compile(
        r'f["\'][^"\']*(?:SELECT|INSERT|UPDATE|DELETE|DROP)[^"\']*\{',
        re.IGNORECASE), "f-string"),
    (re.compile(
        r'(?:query|exec|execute|raw)\s*\([^)]*%\s*[^)]+["\']', re.IGNORECASE), "% 格式化"),
)


def detect_sql_injection(content: str, file_path: str) -> list[Issue]:
    issues: list[Issue] = []
    for i, line in enumerate(content.split('\n'), 1):
        stripped = line.strip()
        if _is_comment_line(stripped):
            continue
        for pattern, kind in SQL_PATTERNS:
            if pattern.search(line):
                issues.append(Issue(
                    severity="high",
                    category="security",
                    title=f"潛在 SQL Injection（{kind}）",
                    description="偵測到疑似以字串操作建構 SQL，需人工確認輸入來源是否可信",
                    file=file_path,
                    line=i,
                    code_snippet=stripped[:120],
                    suggestion="改用參數化查詢（Prepared Statement / placeholder）",
                ))
                break
    return issues


CMD_PATTERNS = (
    re.compile(
        r'(?:os\.system|subprocess\.(?:call|run|Popen)|exec\.Command|child_process\.exec)'
        r'\s*\([^)]*\+[^)]*\)', re.IGNORECASE),
    re.compile(
        r'(?:os\.system|subprocess\.(?:call|run)|child_process\.exec)\s*\(\s*f["\']',
        re.IGNORECASE),
)


def detect_command_injection(content: str, file_path: str) -> list[Issue]:
    issues: list[Issue] = []
    for i, line in enumerate(content.split('\n'), 1):
        stripped = line.strip()
        if _is_comment_line(stripped):
            continue
        for pattern in CMD_PATTERNS:
            if pattern.search(line):
                issues.append(Issue(
                    severity="high",
                    category="security",
                    title="潛在 Command Injection",
                    description="偵測到以字串拼接建構系統指令，需人工確認輸入來源",
                    file=file_path,
                    line=i,
                    code_snippet=stripped[:120],
                    suggestion="避免拼接；若必要，使用 argv 陣列形式並對輸入做白名單驗證",
                ))
                break
    return issues


_COMMENT_LINE_PATTERNS = {
    "go": re.compile(r'^\s*//'),
    "python": re.compile(r'^\s*#'),
    "javascript": re.compile(r'^\s*//'),
    "typescript": re.compile(r'^\s*//'),
}

COMMENT_BLOCK_THRESHOLD = 10


def detect_commented_code(content: str, file_path: str, lang: str) -> list[Issue]:
    pattern = _COMMENT_LINE_PATTERNS.get(lang)
    if pattern is None:
        return []

    issues: list[Issue] = []
    block_start = 0
    count = 0

    def emit() -> None:
        if count >= COMMENT_BLOCK_THRESHOLD:
            issues.append(Issue(
                severity="low",
                category="quality",
                title="大量連續註解",
                description=f"偵測到 {count} 行連續註解，可能是被註解掉的程式碼",
                file=file_path,
                line=block_start,
                suggestion="若為廢棄程式碼請移除；若為文件請考慮移至獨立文件",
            ))

    for i, line in enumerate(content.split('\n'), 1):
        if pattern.match(line):
            if count == 0:
                block_start = i
            count += 1
        else:
            emit()
            count = 0
    emit()
    return issues

"""Read-only repository scanning for code-target candidates (no execution, no network)."""
from __future__ import annotations

import os
import re
import subprocess
from pathlib import Path

CODE_SUFFIXES = {".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".kt", ".go", ".rb", ".rs", ".php", ".cs", ".scala", ".sql", ".swift"}
SKIP_DIRS = {".git", ".venv", "node_modules", "__pycache__", "dist", "build", ".tdd-agent", "target", ".idea", ".mypy_cache"}
DEF_RE = re.compile(r"^\s*(?:export\s+)?(?:async\s+)?(?:def|class|function|func|fn|public|private|protected|static|interface|type)\b[^\n(:{=]*?([A-Za-z_][A-Za-z0-9_]*)\s*[(:{=<]")
MAX_FILE_BYTES = 400_000


def tracked_files(root: Path) -> list[Path]:
    try:
        out = subprocess.run(["git", "-C", str(root), "ls-files", "-z"], capture_output=True, timeout=30, check=True).stdout.decode()
        files = [root / p for p in out.split("\0") if p]
        if files:
            return [f for f in files if f.suffix in CODE_SUFFIXES and f.is_file()]
    except (OSError, subprocess.SubprocessError):
        pass
    found = []
    for base, dirs, names in os.walk(root):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        found.extend(Path(base) / n for n in names if Path(n).suffix in CODE_SUFFIXES)
    return sorted(found)


def split_identifier(name: str) -> list[str]:
    spaced = re.sub(r"([a-z0-9])([A-Z])", r"\1 \2", name)
    return [t for t in re.split(r"[^A-Za-z0-9]+", spaced.lower()) if t]


def scan(root: Path, literals: list[str], name_tokens: list[str], limit: int = 40) -> list[dict]:
    """Candidates whose text contains an id literal, or whose symbol name contains all of ``name_tokens``."""
    root = Path(root)
    hits: list[dict] = []
    wanted = [t.lower() for t in name_tokens if len(t) > 2]
    for path in tracked_files(root):
        try:
            if path.stat().st_size > MAX_FILE_BYTES:
                continue
            lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
        except OSError:
            continue
        symbol, symbol_line = None, 0
        for n, line in enumerate(lines, 1):
            m = DEF_RE.match(line)
            if m:
                symbol, symbol_line = m.group(1), n
                if wanted and set(wanted) <= set(split_identifier(symbol)):
                    hits.append({"path": str(path.relative_to(root)), "symbol": symbol, "line": n, "match": "symbol-name"})
            for lit in literals:
                if lit and lit in line:
                    hits.append({"path": str(path.relative_to(root)), "symbol": symbol, "line": n, "symbol_line": symbol_line, "match": f"literal:{lit}"})
        if len(hits) >= limit * 4:
            break
    seen, out = set(), []
    for h in sorted(hits, key=lambda h: (0 if h["match"].startswith("literal") else 1, h["path"], h["line"])):
        key = (h["path"], h["symbol"])
        if key in seen:
            continue
        seen.add(key); out.append(h)
    return out[:limit]

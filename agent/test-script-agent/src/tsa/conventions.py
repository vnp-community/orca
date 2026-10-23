"""What the test repo already offers: framework, test folder, fixtures, a few sample tests. Read from the worktree only."""
from __future__ import annotations

import ast
import configparser
import tomllib
from dataclasses import dataclass, field
from pathlib import Path

from .errors import UnsupportedFramework

SAMPLE_COUNT = 3
SAMPLE_MAX_CHARS = 4000
SKIP_DIRS = {".git", ".venv", "venv", "node_modules", "__pycache__", ".tox", "build", "dist"}


@dataclass
class Conventions:
    framework: str
    tests_dir: str
    fixtures: list[str] = field(default_factory=list)
    samples: list[tuple[str, str]] = field(default_factory=list)


def _files(root: Path, pattern: str) -> list[Path]:
    return sorted(p for p in root.rglob(pattern) if not (set(p.relative_to(root).parts[:-1]) & SKIP_DIRS))


def _is_pytest(root: Path) -> bool:
    if (root / "pytest.ini").is_file() or _files(root, "conftest.py"):
        return True
    pp = root / "pyproject.toml"
    if pp.is_file():
        try:
            if "pytest" in tomllib.loads(pp.read_text(encoding="utf-8")).get("tool", {}):
                return True
        except (OSError, ValueError):
            pass
    cfg = root / "setup.cfg"
    if cfg.is_file():
        cp = configparser.ConfigParser()
        try:
            cp.read(cfg, encoding="utf-8")
        except configparser.Error:
            return False
        return cp.has_section("tool:pytest")
    return bool(_files(root, "test_*.py"))


def _fixture_names(path: Path) -> list[str]:
    try:
        tree = ast.parse(path.read_text(encoding="utf-8"))
    except (OSError, SyntaxError, ValueError):
        return []
    out = []
    for node in tree.body:
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            for d in node.decorator_list:
                target = d.func if isinstance(d, ast.Call) else d
                if ast.unparse(target).endswith("fixture"):
                    out.append(node.name)
    return out


def detect(root: Path, generated_dir: str = "tsa") -> Conventions:
    root = Path(root)
    if not _is_pytest(root):
        raise UnsupportedFramework(f"no pytest setup found in {root}; this version of TSA generates pytest scripts only")
    tests_dir = "tests" if (root / "tests").is_dir() else "."
    fixtures = sorted({name for c in _files(root, "conftest.py") for name in _fixture_names(c)})
    samples = []
    for p in _files(root, "test_*.py"):
        rel = p.relative_to(root)
        if generated_dir in rel.parts[:-1]:
            continue
        text = p.read_text(encoding="utf-8", errors="replace")
        samples.append((str(rel), text[:SAMPLE_MAX_CHARS]))
        if len(samples) == SAMPLE_COUNT:
            break
    return Conventions("pytest", tests_dir, fixtures, samples)

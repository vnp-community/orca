"""Static checks of a generated pytest file: compiles, no undefined names (ruff when available), fixtures that exist, no secrets."""
from __future__ import annotations

import ast
import json
import re
import shutil
import subprocess
from pathlib import Path

from .conformance import ERROR, WARNING, Finding

BUILTIN_FIXTURES = {"request", "tmp_path", "tmp_path_factory", "tmpdir", "tmpdir_factory", "monkeypatch", "capsys", "capsysbinary", "capfd",
                    "capfdbinary", "caplog", "pytestconfig", "recwarn", "record_property", "record_testsuite_property", "record_xml_attribute",
                    "cache", "doctest_namespace", "pytester", "testdir"}
SECRET_RE = re.compile(r"(?i)(password|passwd|secret|api[_-]?key|token)\s*=\s*['\"][^'\"]{4,}['\"]")
CARD_RE = re.compile(r"(?<!\d)(?:\d[ -]?){13,19}(?!\d)")
RUFF_RULES = "E9,F63,F7,F82"   # syntax errors and undefined names: catches helpers the model invented


def _local_fixtures(tree: ast.Module) -> set[str]:
    out = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            for d in node.decorator_list:
                if ast.unparse(d.func if isinstance(d, ast.Call) else d).endswith("fixture"):
                    out.add(node.name)
    return out


def check_file(root: Path, rel: str, known_fixtures: set[str]) -> list[Finding]:
    path = Path(root) / rel
    try:
        source = path.read_text(encoding="utf-8")
    except OSError:
        return [Finding("missing-file", ERROR, "the generator did not write this file", rel)]
    try:
        tree = ast.parse(source)
        compile(source, rel, "exec")
    except SyntaxError as exc:
        return [Finding("syntax", ERROR, f"syntax error: {exc.msg}", rel, exc.lineno)]
    out: list[Finding] = []
    fixtures = known_fixtures | BUILTIN_FIXTURES | _local_fixtures(tree)
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name.startswith("test"):
            for arg in node.args.args:
                if arg.arg not in fixtures and arg.arg != "self":
                    out.append(Finding("unknown-fixture", WARNING, f"fixture {arg.arg!r} is not defined in conftest.py or this file "
                                       "(may come from a plugin)", rel, node.lineno))
    for i, line in enumerate(source.splitlines(), 1):
        if SECRET_RE.search(line):
            out.append(Finding("hardcoded-secret", ERROR, "credential written in the script; read it from the environment", rel, i))
        elif CARD_RE.search(line) and "@exp" not in line:
            out.append(Finding("card-like-number", WARNING, "long digit run looks like a card/account number; generate it instead", rel, i))
    out += _ruff(root, rel)
    return out


def _ruff(root: Path, rel: str) -> list[Finding]:
    ruff = shutil.which("ruff")
    if not ruff:
        return []
    proc = subprocess.run([ruff, "check", "--no-cache", "--isolated", "--select", RUFF_RULES, "--output-format", "json", rel],
                          cwd=root, capture_output=True, text=True)
    try:
        items = json.loads(proc.stdout or "[]")
    except ValueError:
        return []
    return [Finding(f"ruff-{i.get('code')}", ERROR, i.get("message", ""), rel, (i.get("location") or {}).get("row")) for i in items]

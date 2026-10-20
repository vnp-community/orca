"""``tdd-agent doctor``: what a command would need, checked without calling a model.

Every check says what it found and, when it fails, the command that fixes it, so the first change request does not stop at the fifth gate on
something that was knowable at the start.
"""
from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

PROVIDER_COMMANDS = {"claude_code": ("claude", "llm.claude_code.command"), "codex_cli": ("codex", "llm.codex_cli.command")}


def _version(command: str) -> str | None:
    try:
        run = subprocess.run([command, "--version"], capture_output=True, text=True, timeout=15)
    except (OSError, subprocess.TimeoutExpired):
        return None
    out = (run.stdout or run.stderr).strip().splitlines()
    return out[0][:80] if run.returncode == 0 and out else None


def providers_in_use(config) -> dict[str, list[str]]:
    """provider -> the roles it serves (``default`` for the ones nothing else claims)."""
    llm = config.section("llm")
    used: dict[str, list[str]] = {llm["default"]: ["default"]}
    for role, name in (llm.get("roles") or {}).items():
        if name:
            used.setdefault(name, []).append(role)
    return used


def check(config, *, for_change: bool = False) -> list[dict]:
    """The checks as ``{name, ok, detail, fix}`` (``fix`` only when not ok)."""
    from . import project
    root = config.base_dir
    out: list[dict] = []

    def add(name: str, ok: bool, detail: str, fix: str = "", level: str = "error") -> None:
        out.append({"name": name, "ok": ok, "detail": detail, **({"fix": fix, "level": level} if not ok else {})})

    for provider, roles in sorted(providers_in_use(config).items()):
        command, key = PROVIDER_COMMANDS.get(provider, (provider, ""))
        command = config.get(key) or command
        found = shutil.which(command)
        version = _version(command) if found else None
        add(f"agent {provider}", bool(found and version), f"{command} {version or 'not found'} ({', '.join(roles)})",
            f"install the `{command}` command and sign in (or set llm.{provider}.command), or choose another agent in [llm]")
    spec = shutil.which("openspec")
    add("openspec", bool(spec), f"openspec {_version('openspec') or 'not found'}", "install the OpenSpec CLI (npm install -g @fission-ai/openspec) - the change bundle is validated with it")
    state = root / project.STATE_DIR
    add("project", (state / project.CONFIG_NAME).is_file(), f"{project.STATE_DIR}/{project.CONFIG_NAME}", "run `tdd-agent init`")
    missing = []
    for role, raw in sorted(config.section("project")["sources"].items()):
        if raw and not ((root / raw) if not Path(raw).is_absolute() else Path(raw)).is_file():
            missing.append(f"{role}: {raw}")
    add("documents", not missing, "URD/SRS/SAD files found" if not missing else "missing " + ", ".join(missing), "fix [project.sources] in .tdd-agent/config.toml")
    ignore = state / ".gitignore"
    add("git ignore", ignore.is_file() and "*.db" in ignore.read_text(encoding="utf-8"), f"{project.STATE_DIR}/.gitignore keeps the local databases out of version control",
        f"add *.db to {project.STATE_DIR}/.gitignore", level="warning")
    if for_change:
        last = project._read_state(root)
        add("baseline", bool(last.get("sources", {}).get("SRS") and last.get("sources", {}).get("SAD")), "a TDD was generated from the SRS and the SAD", "run `tdd-agent init` first (a change request is read against what the TDD was generated from)")
    mode = config.approval_mode()
    add("approvals", True, f"mode {mode}" + (f", people required at {', '.join(config.get('approvals.required'))}" if mode == "auto" and config.get("approvals.required") else ""))
    return out


def failures(checks: list[dict]) -> list[dict]:
    return [c for c in checks if not c["ok"] and c.get("level") == "error"]


def lines(checks: list[dict]) -> list[str]:
    out = []
    for c in checks:
        mark = "ok  " if c["ok"] else ("FAIL" if c.get("level") == "error" else "warn")
        out.append(f"[{mark}] {c['name']}: {c['detail']}")
        if not c["ok"]:
            out.append(f"       → {c['fix']}")
    return out

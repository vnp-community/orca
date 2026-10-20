"""OpenSpec baseline of the requirements a change modifies or removes.

``MODIFIED``/``REMOVED`` deltas of an OpenSpec change act on a requirement that already stands in ``openspec/specs/<capability>/spec.md``.  A project that
adopted the runtime has its requirements in the SRS and the knowledge graph but no such file, so the readiness rules (``TS-02``/``TS-10``) stop the change.  The baseline
is written from the statements of the SRS revision the change starts from (what the TDD was generated from); it records, it does not add content.
"""
from __future__ import annotations

import re
from pathlib import Path

from ..errors import TddAgentError

NORMATIVE = re.compile(r"\b(SHALL|MUST)\b")


def _block(ident: str, statement: str, acceptance: str = "") -> str:
    """A requirement as OpenSpec wants it (it must say SHALL or MUST); a statement that does not (any language but English) is carried whole after a fixed lead-in."""
    body = statement.strip() if NORMATIVE.search(statement) else f"The system SHALL satisfy this requirement as the SRS states it: {statement.strip()}"
    if acceptance.strip():   # what the SRS row itself says is the acceptance of the requirement
        return f"### Requirement: {ident}\n{body}\n\n#### Scenario: acceptance stated in the SRS\n- **WHEN** the requirement applies\n- **THEN** {acceptance.strip()}\n"
    return (f"### Requirement: {ident}\n{body}\n\n#### Scenario: current behaviour\n"
            "- **WHEN** the requirement applies\n- **THEN** the system behaves as the requirement states\n")


def baseline_path(repo_root: str | Path, capability: str) -> Path:
    return Path(repo_root) / "openspec" / "specs" / capability / "spec.md"


def missing_requirements(repo_root: str | Path, capability: str, ids: list[str]) -> list[str]:
    path = baseline_path(repo_root, capability)
    text = path.read_text(encoding="utf-8") if path.is_file() else ""
    return [i for i in ids if f"### Requirement: {i}" not in text]


def ensure_baseline(repo_root: str | Path, capability: str, statements: dict[str, str], *, force: bool = False, acceptance: dict[str, str] | None = None) -> dict:
    """Write the requirements ``statements`` (id -> the statement as it stood before the change) into the baseline of ``capability``.

    A requirement already there is kept unless ``force``.  Returns ``{"path", "written", "kept", "unwritable"}``; ``unwritable`` lists those that cannot be
    stated as a normative OpenSpec requirement (no statement in the SRS revision the change starts from) with the reason, so the person decides instead of text being invented.
    """
    path = baseline_path(repo_root, capability)
    text = path.read_text(encoding="utf-8") if path.is_file() else ""
    written, kept, unwritable, blocks = [], [], [], []
    for ident, statement in statements.items():
        if not statement.strip():
            unwritable.append({"id": ident, "reason": "the SRS revision the change starts from has no statement for it"})
        elif f"### Requirement: {ident}" in text and not force:
            kept.append(ident)
        else:
            blocks.append(_block(ident, statement, (acceptance or {}).get(ident, ""))); written.append(ident)
    if blocks:
        if force and text:
            for ident in written:
                text = re.sub(rf"### Requirement: {re.escape(ident)}\n.*?(?=\n### Requirement: |\Z)", "", text, flags=re.S)
        if not text.strip():
            text = f"# {capability} Specification\n\n## Purpose\nThe requirements this change modifies, as the SRS stated them before it (written by `tdd-agent baseline`).\n\n## Requirements\n"
        elif "## Requirements" not in text:
            raise TddAgentError(f"{path} has no '## Requirements' section to add to", code="BASELINE_MALFORMED")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text.rstrip() + "\n\n" + "\n".join(blocks), encoding="utf-8")
    return {"path": str(path), "written": written, "kept": kept, "unwritable": unwritable}

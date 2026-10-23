"""Which test cases go into which generated file: one file per test set (else per level), under <tests>/tsa/<feature>/."""
from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import PurePosixPath

from .conventions import Conventions

SLUG = re.compile(r"[^a-z0-9]+")


def slug(text: str) -> str:
    return SLUG.sub("_", text.lower()).strip("_") or "x"


@dataclass
class FilePlan:
    path: str          # relative to the worktree, posix
    group: str
    cases: list[dict]


def output_dir(conv: Conventions, feature: str) -> str:
    return str(PurePosixPath(conv.tests_dir) / "tsa" / slug(feature))


def plan(cases: list[dict], conv: Conventions, feature: str) -> list[FilePlan]:
    groups: dict[str, list[dict]] = {}
    for c in cases:
        sets = (c.get("trace") or {}).get("testSet") or []
        groups.setdefault(sets[0] if sets else f"level-{c.get('level', 'x')}", []).append(c)
    out_dir = PurePosixPath(output_dir(conv, feature))
    # The feature is in the file name too: pytest's default import mode needs unique basenames across folders without __init__.py.
    return [FilePlan(str(out_dir / f"test_tsa_{slug(feature)}_{slug(g)}.py"), g, sorted(cs, key=lambda c: c["id"]))
            for g, cs in sorted(groups.items())]

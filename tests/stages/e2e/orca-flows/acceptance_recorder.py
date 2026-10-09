"""Test execution and result bookkeeping.

PASS     the behaviour matches the acceptance criterion.
FAIL     the product misbehaves (or a precondition the product itself should guarantee is broken).
BLOCKED  the environment lacks something the test needs (AI key, agent, docker); says nothing about product quality.
SKIP     opt-in test that was not enabled.
"""
from __future__ import annotations

import time
import traceback
from dataclasses import asdict, dataclass
from typing import Callable


class Blocked(Exception):
    """Raised by a test when a precondition outside the product is missing."""


class Skipped(Exception):
    pass


@dataclass
class Outcome:
    suite: str
    test_id: str
    title: str
    status: str
    detail: str = ""
    seconds: float = 0.0


class AcceptanceRecorder:
    def __init__(self) -> None:
        self.outcomes: list[Outcome] = []
        self.current_suite = ""

    def run(self, test_id: str, title: str, fn: Callable[[], None]) -> str:
        started = time.time()
        status, detail = "PASS", ""
        try:
            fn()
        except Blocked as exc:
            status, detail = "BLOCKED", str(exc)
        except Skipped as exc:
            status, detail = "SKIP", str(exc)
        except AssertionError as exc:
            status, detail = "FAIL", str(exc) or "assertion failed"
        except Exception as exc:  # noqa: BLE001 - any other error is a finding, not a crash of the run
            status, detail = "FAIL", f"{type(exc).__name__}: {exc}"
            if not str(exc):
                detail += "\n" + traceback.format_exc(limit=3)
        out = Outcome(self.current_suite, test_id, title, status, detail[:600], round(time.time() - started, 2))
        self.outcomes.append(out)
        mark = {"PASS": "  ok ", "FAIL": " FAIL", "BLOCKED": "block", "SKIP": " skip"}[status]
        print(f"[{mark}] {test_id} {title}" + (f"\n         -> {detail[:300]}" if detail else ""), flush=True)
        return status

    def counts(self) -> dict[str, int]:
        c = {"PASS": 0, "FAIL": 0, "BLOCKED": 0, "SKIP": 0}
        for o in self.outcomes:
            c[o.status] += 1
        return c

    def as_dicts(self) -> list[dict]:
        return [asdict(o) for o in self.outcomes]

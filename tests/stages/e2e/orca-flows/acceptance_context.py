"""State shared by the suites of one run, and the cleanup stack."""
from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Any, Callable

from acceptance_recorder import AcceptanceRecorder
from orca_session import OrcaSession
from orca_stack_config import StackConfig
from ws_rpc_client import WsRpcClient


@dataclass
class AcceptanceContext:
    cfg: StackConfig
    session: OrcaSession
    rec: AcceptanceRecorder
    state: dict[str, Any] = field(default_factory=dict)
    cleanups: list[tuple[str, Callable[[], None]]] = field(default_factory=list)

    @property
    def ws(self) -> WsRpcClient:
        return self.session.ws()

    def on_cleanup(self, label: str, fn: Callable[[], None]) -> None:
        self.cleanups.append((label, fn))

    def run_cleanups(self) -> list[str]:
        problems = []
        for label, fn in reversed(self.cleanups):
            try:
                fn()
            except Exception as exc:  # noqa: BLE001 - cleanup must try everything
                problems.append(f"{label}: {exc}")
        self.cleanups.clear()
        return problems


def wait_until(predicate: Callable[[], Any], timeout: float = 30.0, interval: float = 0.5, what: str = "condition"):
    """Poll until `predicate` returns a truthy value; raise AssertionError naming `what` on timeout."""
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        last = predicate()
        if last:
            return last
        time.sleep(interval)
    raise AssertionError(f"timed out after {timeout:.0f}s waiting for {what}")

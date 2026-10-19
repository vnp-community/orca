"""Wait until each named dev server is connected, approving it as the stack's admin (a fresh dev server starts pending)."""
from __future__ import annotations

import sys
from dataclasses import replace
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "orca-flows"))

from ai_stub_agent_switch import StubAgentError, wait_until_connected  # noqa: E402
from orca_stack_config import load_config  # noqa: E402


def main(names: list[str]) -> int:
    if not names:
        print("usage: devserver_ready.py NAME [NAME...]", file=sys.stderr)
        return 2
    cfg = load_config("split-ready")
    for name in names:
        try:
            wait_until_connected(replace(cfg, dev_server_name=name), timeout=120, approve=True)
        except StubAgentError as exc:
            print(f"    {name}: {exc}", file=sys.stderr)
            print(f"    logs: docker logs orca-e2e-{name.removeprefix('e2e-')}", file=sys.stderr)
            return 1
        print(f"    {name}: connected and approved")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

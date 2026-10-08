"""Integration test cho 13-agent-ws."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "13-agent-ws"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 13-agent-ws
    ctx.skip("13-agent-ws", "Chưa có test tích hợp cho 13-agent-ws")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Integration test cho 03-agent."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "03-agent"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 03-agent
    ctx.skip("03-agent", "Chưa có test tích hợp cho 03-agent")

if __name__ == "__main__":
    run_single(SUITE, run)

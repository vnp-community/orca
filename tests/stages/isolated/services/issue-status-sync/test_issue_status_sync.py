"""Test cho issue-status-sync."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single

SUITE = "issue-status-sync"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for issue-status-sync
    ctx.skip("issue-status-sync", "Chưa có test cho issue-status-sync")

if __name__ == "__main__":
    run_single(SUITE, run)

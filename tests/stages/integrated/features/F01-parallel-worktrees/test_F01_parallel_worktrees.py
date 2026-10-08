"""Integration test cho F01-parallel-worktrees."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F01-parallel-worktrees"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F01-parallel-worktrees
    ctx.skip("F01-parallel-worktrees", "Chưa có test tích hợp cho F01-parallel-worktrees")

if __name__ == "__main__":
    run_single(SUITE, run)

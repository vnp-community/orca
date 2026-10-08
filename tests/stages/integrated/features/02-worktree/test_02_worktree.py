"""Integration test cho 02-worktree."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "02-worktree"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 02-worktree
    ctx.skip("02-worktree", "Chưa có test tích hợp cho 02-worktree")

if __name__ == "__main__":
    run_single(SUITE, run)

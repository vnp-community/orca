"""Integration test cho F37-task-graph-management."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F37-task-graph-management"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F37-task-graph-management
    ctx.skip("F37-task-graph-management", "Chưa có test tích hợp cho F37-task-graph-management")

if __name__ == "__main__":
    run_single(SUITE, run)

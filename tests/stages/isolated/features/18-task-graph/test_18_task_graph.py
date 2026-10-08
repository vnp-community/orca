"""Test cho 18-task-graph."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "18-task-graph"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 18-task-graph
    ctx.skip("18-task-graph", "Chưa có test cho 18-task-graph")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Integration test cho F36-multi-server-workflow-orchestration."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F36-multi-server-workflow-orchestration"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F36-multi-server-workflow-orchestration
    ctx.skip("F36-multi-server-workflow-orchestration", "Chưa có test tích hợp cho F36-multi-server-workflow-orchestration")

if __name__ == "__main__":
    run_single(SUITE, run)

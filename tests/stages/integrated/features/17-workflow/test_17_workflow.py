"""Integration test cho 17-workflow."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "17-workflow"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 17-workflow
    ctx.skip("17-workflow", "Chưa có test tích hợp cho 17-workflow")

if __name__ == "__main__":
    run_single(SUITE, run)

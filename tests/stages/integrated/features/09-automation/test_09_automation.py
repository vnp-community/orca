"""Integration test cho 09-automation."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "09-automation"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 09-automation
    ctx.skip("09-automation", "Chưa có test tích hợp cho 09-automation")

if __name__ == "__main__":
    run_single(SUITE, run)

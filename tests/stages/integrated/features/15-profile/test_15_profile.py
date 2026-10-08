"""Integration test cho 15-profile."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "15-profile"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 15-profile
    ctx.skip("15-profile", "Chưa có test tích hợp cho 15-profile")

if __name__ == "__main__":
    run_single(SUITE, run)

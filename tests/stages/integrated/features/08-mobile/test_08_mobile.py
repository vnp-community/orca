"""Integration test cho 08-mobile."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "08-mobile"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 08-mobile
    ctx.skip("08-mobile", "Chưa có test tích hợp cho 08-mobile")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Integration test cho 10-design-browser."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "10-design-browser"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 10-design-browser
    ctx.skip("10-design-browser", "Chưa có test tích hợp cho 10-design-browser")

if __name__ == "__main__":
    run_single(SUITE, run)

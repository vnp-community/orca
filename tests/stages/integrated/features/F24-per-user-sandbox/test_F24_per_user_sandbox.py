"""Integration test cho F24-per-user-sandbox."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F24-per-user-sandbox"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F24-per-user-sandbox
    ctx.skip("F24-per-user-sandbox", "Chưa có test tích hợp cho F24-per-user-sandbox")

if __name__ == "__main__":
    run_single(SUITE, run)

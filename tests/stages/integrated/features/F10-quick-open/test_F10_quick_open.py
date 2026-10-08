"""Integration test cho F10-quick-open."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F10-quick-open"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F10-quick-open
    ctx.skip("F10-quick-open", "Chưa có test tích hợp cho F10-quick-open")

if __name__ == "__main__":
    run_single(SUITE, run)

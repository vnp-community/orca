"""Integration test cho F13-text-search."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F13-text-search"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F13-text-search
    ctx.skip("F13-text-search", "Chưa có test tích hợp cho F13-text-search")

if __name__ == "__main__":
    run_single(SUITE, run)

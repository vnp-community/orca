"""Integration test cho F03-mobile-companion."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F03-mobile-companion"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F03-mobile-companion
    ctx.skip("F03-mobile-companion", "Chưa có test tích hợp cho F03-mobile-companion")

if __name__ == "__main__":
    run_single(SUITE, run)

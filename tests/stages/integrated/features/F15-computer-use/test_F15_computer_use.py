"""Integration test cho F15-computer-use."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F15-computer-use"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F15-computer-use
    ctx.skip("F15-computer-use", "Chưa có test tích hợp cho F15-computer-use")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Test cho 06-code-review."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "06-code-review"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 06-code-review
    ctx.skip("06-code-review", "Chưa có test cho 06-code-review")

if __name__ == "__main__":
    run_single(SUITE, run)

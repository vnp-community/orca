"""Test cho F21-auto-update."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F21-auto-update"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F21-auto-update
    ctx.skip("F21-auto-update", "Chưa có test cho F21-auto-update")

if __name__ == "__main__":
    run_single(SUITE, run)

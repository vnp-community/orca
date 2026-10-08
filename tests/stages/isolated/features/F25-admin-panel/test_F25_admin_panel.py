"""Test cho F25-admin-panel."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F25-admin-panel"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F25-admin-panel
    ctx.skip("F25-admin-panel", "Chưa có test cho F25-admin-panel")

if __name__ == "__main__":
    run_single(SUITE, run)

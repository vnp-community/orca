"""Test cho 01-auth."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "01-auth"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 01-auth
    ctx.skip("01-auth", "Chưa có test cho 01-auth")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Test cho F26-multi-database."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F26-multi-database"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F26-multi-database
    ctx.skip("F26-multi-database", "Chưa có test cho F26-multi-database")

if __name__ == "__main__":
    run_single(SUITE, run)

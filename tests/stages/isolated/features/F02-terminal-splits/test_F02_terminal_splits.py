"""Test cho F02-terminal-splits."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F02-terminal-splits"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F02-terminal-splits
    ctx.skip("F02-terminal-splits", "Chưa có test cho F02-terminal-splits")

if __name__ == "__main__":
    run_single(SUITE, run)

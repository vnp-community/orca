"""Test cho 04-terminal."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "04-terminal"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 04-terminal
    ctx.skip("04-terminal", "Chưa có test cho 04-terminal")

if __name__ == "__main__":
    run_single(SUITE, run)

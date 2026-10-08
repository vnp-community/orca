"""Test cho 05-remote-dev."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "05-remote-dev"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 05-remote-dev
    ctx.skip("05-remote-dev", "Chưa có test cho 05-remote-dev")

if __name__ == "__main__":
    run_single(SUITE, run)

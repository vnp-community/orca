"""Test cho F22-web-server-mode."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F22-web-server-mode"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F22-web-server-mode
    ctx.skip("F22-web-server-mode", "Chưa có test cho F22-web-server-mode")

if __name__ == "__main__":
    run_single(SUITE, run)

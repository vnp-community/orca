"""Test cho F39-remote-git-ui."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F39-remote-git-ui"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F39-remote-git-ui
    ctx.skip("F39-remote-git-ui", "Chưa có test cho F39-remote-git-ui")

if __name__ == "__main__":
    run_single(SUITE, run)

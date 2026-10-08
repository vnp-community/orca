"""Test cho F38-project-workspace."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F38-project-workspace"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F38-project-workspace
    ctx.skip("F38-project-workspace", "Chưa có test cho F38-project-workspace")

if __name__ == "__main__":
    run_single(SUITE, run)

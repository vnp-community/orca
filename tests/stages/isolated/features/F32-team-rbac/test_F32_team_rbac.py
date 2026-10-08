"""Test cho F32-team-rbac."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F32-team-rbac"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F32-team-rbac
    ctx.skip("F32-team-rbac", "Chưa có test cho F32-team-rbac")

if __name__ == "__main__":
    run_single(SUITE, run)

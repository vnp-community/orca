"""Integration test cho F33-user-profile-hierarchy."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F33-user-profile-hierarchy"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F33-user-profile-hierarchy
    ctx.skip("F33-user-profile-hierarchy", "Chưa có test tích hợp cho F33-user-profile-hierarchy")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Integration test cho F23-multi-user-auth."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F23-multi-user-auth"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F23-multi-user-auth
    ctx.skip("F23-multi-user-auth", "Chưa có test tích hợp cho F23-multi-user-auth")

if __name__ == "__main__":
    run_single(SUITE, run)

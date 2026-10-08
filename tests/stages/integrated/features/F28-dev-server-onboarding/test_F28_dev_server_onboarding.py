"""Integration test cho F28-dev-server-onboarding."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F28-dev-server-onboarding"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F28-dev-server-onboarding
    ctx.skip("F28-dev-server-onboarding", "Chưa có test tích hợp cho F28-dev-server-onboarding")

if __name__ == "__main__":
    run_single(SUITE, run)

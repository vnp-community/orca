"""Integration test cho F42-contextual-onboarding-tours."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F42-contextual-onboarding-tours"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F42-contextual-onboarding-tours
    ctx.skip("F42-contextual-onboarding-tours", "Chưa có test tích hợp cho F42-contextual-onboarding-tours")

if __name__ == "__main__":
    run_single(SUITE, run)

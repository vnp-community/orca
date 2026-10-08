"""Integration test cho F11-notifications."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F11-notifications"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F11-notifications
    ctx.skip("F11-notifications", "Chưa có test tích hợp cho F11-notifications")

if __name__ == "__main__":
    run_single(SUITE, run)

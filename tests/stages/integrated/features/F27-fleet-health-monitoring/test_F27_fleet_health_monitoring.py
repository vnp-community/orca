"""Integration test cho F27-fleet-health-monitoring."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F27-fleet-health-monitoring"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F27-fleet-health-monitoring
    ctx.skip("F27-fleet-health-monitoring", "Chưa có test tích hợp cho F27-fleet-health-monitoring")

if __name__ == "__main__":
    run_single(SUITE, run)

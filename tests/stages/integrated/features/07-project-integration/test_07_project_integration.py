"""Integration test cho 07-project-integration."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "07-project-integration"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 07-project-integration
    ctx.skip("07-project-integration", "Chưa có test tích hợp cho 07-project-integration")

if __name__ == "__main__":
    run_single(SUITE, run)

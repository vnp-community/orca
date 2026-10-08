"""Integration test cho 14-remote-integration."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "14-remote-integration"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for 14-remote-integration
    ctx.skip("14-remote-integration", "Chưa có test tích hợp cho 14-remote-integration")

if __name__ == "__main__":
    run_single(SUITE, run)

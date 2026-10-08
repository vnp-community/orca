"""Test cho workflow-service."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single

SUITE = "workflow-service"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for workflow-service
    ctx.skip("workflow-service", "Chưa có test cho workflow-service")

if __name__ == "__main__":
    run_single(SUITE, run)

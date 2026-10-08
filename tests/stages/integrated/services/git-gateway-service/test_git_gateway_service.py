"""Test cho git-gateway-service."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "git-gateway-service"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for git-gateway-service
    ctx.skip("git-gateway-service", "Chưa có test cho git-gateway-service")

if __name__ == "__main__":
    run_single(SUITE, run)

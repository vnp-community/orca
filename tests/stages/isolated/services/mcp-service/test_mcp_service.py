"""Test cho mcp-service."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from check_framework import Context, run_single

SUITE = "mcp-service"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for mcp-service
    ctx.skip("mcp-service", "Chưa có test cho mcp-service")

if __name__ == "__main__":
    run_single(SUITE, run)

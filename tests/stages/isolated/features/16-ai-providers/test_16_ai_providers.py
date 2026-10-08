"""Test cho 16-ai-providers."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "16-ai-providers"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for 16-ai-providers
    ctx.skip("16-ai-providers", "Chưa có test cho 16-ai-providers")

if __name__ == "__main__":
    run_single(SUITE, run)

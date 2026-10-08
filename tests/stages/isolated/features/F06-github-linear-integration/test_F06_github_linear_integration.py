"""Test cho F06-github-linear-integration."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F06-github-linear-integration"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F06-github-linear-integration
    ctx.skip("F06-github-linear-integration", "Chưa có test cho F06-github-linear-integration")

if __name__ == "__main__":
    run_single(SUITE, run)

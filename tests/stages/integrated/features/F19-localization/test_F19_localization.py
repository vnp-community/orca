"""Integration test cho F19-localization."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F19-localization"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F19-localization
    ctx.skip("F19-localization", "Chưa có test tích hợp cho F19-localization")

if __name__ == "__main__":
    run_single(SUITE, run)

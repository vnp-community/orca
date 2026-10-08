"""Test cho F16-rich-repo-previews."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F16-rich-repo-previews"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F16-rich-repo-previews
    ctx.skip("F16-rich-repo-previews", "Chưa có test cho F16-rich-repo-previews")

if __name__ == "__main__":
    run_single(SUITE, run)

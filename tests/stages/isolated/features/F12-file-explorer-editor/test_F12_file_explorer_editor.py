"""Test cho F12-file-explorer-editor."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F12-file-explorer-editor"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F12-file-explorer-editor
    ctx.skip("F12-file-explorer-editor", "Chưa có test cho F12-file-explorer-editor")

if __name__ == "__main__":
    run_single(SUITE, run)

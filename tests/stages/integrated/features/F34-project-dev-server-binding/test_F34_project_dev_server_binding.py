"""Integration test cho F34-project-dev-server-binding."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F34-project-dev-server-binding"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement integration test for F34-project-dev-server-binding
    ctx.skip("F34-project-dev-server-binding", "Chưa có test tích hợp cho F34-project-dev-server-binding")

if __name__ == "__main__":
    run_single(SUITE, run)

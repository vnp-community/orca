"""Test cho F31-fleet-provisioning."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F31-fleet-provisioning"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F31-fleet-provisioning
    ctx.skip("F31-fleet-provisioning", "Chưa có test cho F31-fleet-provisioning")

if __name__ == "__main__":
    run_single(SUITE, run)

"""Test cho F18-ephemeral-vm."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F18-ephemeral-vm"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F18-ephemeral-vm
    ctx.skip("F18-ephemeral-vm", "Chưa có test cho F18-ephemeral-vm")

if __name__ == "__main__":
    run_single(SUITE, run)

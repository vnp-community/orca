"""Test cho F20-speech-input."""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

from check_framework import Context, run_single

SUITE = "F20-speech-input"

def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    # TODO: Implement tests for F20-speech-input
    ctx.skip("F20-speech-input", "Chưa có test cho F20-speech-input")

if __name__ == "__main__":
    run_single(SUITE, run)

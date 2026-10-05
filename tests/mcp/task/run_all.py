#!/usr/bin/env python3
"""Chạy bộ thử nghiệm tạo task qua MCP trên project đích (mặc định Vnp-asm).

    python3 run_all.py                    # tất cả suite
    python3 run_all.py fields hierarchy   # một số suite
    python3 run_all.py --list

Cấu hình đọc từ tests/mcp/.env. Đổi project: ORCA_MCP_TASK_PROJECT=<tên hoặc uuid>. Cần gateway bật pack ghi
(MCP_TOOL_PACKS_ENABLED=1,2) và websocket-client (để xoá task thử nghiệm ở cuối). Task thử nghiệm mang tiền tố
ORCA_TEST_PREFIX; mã thoát khác 0 nếu có FAIL.
"""
from __future__ import annotations

import argparse
import importlib
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mcp_check_framework import build_context, run_suite, summarize  # noqa: E402

SUITES = [
    ("lifecycle", "check_task_lifecycle"),
    ("fields", "check_task_create_fields"),
    ("hierarchy", "check_task_create_hierarchy"),
    ("update", "check_task_create_update"),
    ("from-source", "check_task_create_from_source"),
    ("concurrency-access", "check_task_create_concurrency_access"),
]


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("suites", nargs="*")
    ap.add_argument("--list", action="store_true")
    args = ap.parse_args()
    if args.list:
        for name, mod in SUITES:
            print(f"{name:20s} {mod}.py")
        return 0
    unknown = [s for s in args.suites if s not in {n for n, _ in SUITES}]
    if unknown:
        print(f"suite không tồn tại: {unknown}. Dùng --list.", file=sys.stderr)
        return 2

    ctx = build_context()
    print(f"MCP: {ctx.cfg.mcp_url}   mcp_mounted={ctx.mcp_enabled}")
    if "unreachable" in ctx.state or (not ctx.mcp_enabled and ctx.cfg.expect_enabled):
        ctx.suite = "preflight"
        ctx.check("MCP truy cập được", False, ctx.state.get("unreachable", "/.well-known trả 404 (MCP_ENABLED=false?)"))
        return summarize(ctx)
    for name, mod in SUITES:
        if args.suites and name not in args.suites:
            continue
        run_suite(ctx, name, importlib.import_module(mod).run)
    return summarize(ctx)


if __name__ == "__main__":
    raise SystemExit(main())

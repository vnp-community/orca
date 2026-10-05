#!/usr/bin/env python3
"""Chạy toàn bộ bộ kiểm tra giao tiếp MCP của Orca.

    python run_all.py                       # tất cả suite
    python run_all.py discovery protocol    # chỉ một số suite
    python run_all.py --list                # liệt kê suite

Cấu hình đọc từ tests/mcp/.env (xem .env.example). Mã thoát khác 0 nếu có FAIL.
"""
from __future__ import annotations

import argparse
import importlib
import sys

from mcp_check_framework import build_context, run_suite, summarize

# Thứ tự: khám phá và chặn truy cập trước, rồi giao thức, công cụ, luồng task, WS, vòng đời token (chậm nhất).
SUITES = [
    ("discovery", "check_mcp_discovery"),
    ("auth-enforcement", "check_mcp_auth_enforcement"),
    ("protocol", "check_mcp_protocol"),
    ("tools", "check_mcp_tools"),
    ("task-worktree-flow", "check_mcp_task_worktree_flow"),
    ("ws-channels", "check_mcp_ws_channels"),
    ("token-lifecycle", "check_mcp_token_lifecycle"),
]


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("suites", nargs="*", help="tên suite (mặc định: tất cả)")
    ap.add_argument("--list", action="store_true", help="liệt kê suite rồi thoát")
    args = ap.parse_args()

    if args.list:
        for name, mod in SUITES:
            print(f"{name:20s} {mod}.py")
        return 0
    known = {n for n, _ in SUITES}
    unknown = [s for s in args.suites if s not in known]
    if unknown:
        print(f"suite không tồn tại: {unknown}. Dùng --list.", file=sys.stderr)
        return 2

    ctx = build_context()
    print(f"MCP: {ctx.cfg.mcp_url}   gateway: {ctx.cfg.base_url}   mcp_mounted={ctx.mcp_enabled}")
    if "unreachable" in ctx.state:
        ctx.suite = "preflight"
        ctx.check("gateway truy cập được", False, ctx.state["unreachable"])
        return summarize(ctx)
    if not ctx.mcp_enabled and ctx.cfg.expect_enabled:
        ctx.suite = "preflight"
        ctx.check("MCP được mount", False,
                    "/.well-known/oauth-protected-resource trả 404 — MCP_ENABLED=false? "
                    "(đặt ORCA_MCP_EXPECT_ENABLED=false nếu cố ý tắt)")
        return summarize(ctx)
    for name, mod in SUITES:
        if args.suites and name not in args.suites:
            continue
        run_suite(ctx, name, importlib.import_module(mod).run)
    return summarize(ctx)


if __name__ == "__main__":
    raise SystemExit(main())

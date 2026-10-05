#!/usr/bin/env python3
"""Liệt kê mọi project mà tài khoản của token MCP nhìn thấy, qua công cụ `project_list`.

    python3 mcp_list_projects.py            # bảng
    python3 mcp_list_projects.py --json     # JSON đầy đủ

Dùng ORCA_MCP_TOKEN trong tests/mcp/.env (tạo bằng get_mcp_token.py); không có thì tự tạo PAT
orca:read bằng tài khoản admin và thu hồi khi xong. Chỉ cần scope orca:read.
"""
from __future__ import annotations

import argparse
import json
import sys

from mcp_check_framework import build_context


def fetch_projects(client) -> tuple[list[dict], str | None]:
    """Gọi project_list qua mọi trang; trả (projects, lỗi)."""
    projects: list[dict] = []
    cursor = None
    for _ in range(100):  # chặn vòng lặp nếu server trả cursor lặp
        args = {"page_size": 200}
        if cursor:
            args["page_token"] = cursor
        reply = client.call_tool("project_list", args)
        if reply.status != 200 or reply.error is not None:
            return projects, f"HTTP {reply.status} {reply.text[:200]}"
        if reply.tool_is_error:
            return projects, reply.tool_text[:200]
        data = reply.structured if isinstance(reply.structured, dict) else {}
        projects += [p for p in data.get("items") or [] if isinstance(p, dict)]
        cursor = data.get("nextPageToken") or data.get("next_page_token") or data.get("nextCursor")
        if not cursor:
            return projects, None
    return projects, "quá 100 trang"


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--json", action="store_true", help="in JSON đầy đủ")
    args = ap.parse_args()

    ctx = build_context()
    if not ctx.mcp_enabled:
        print("MCP không phản hồi hoặc đang tắt (kiểm tra ORCA_API_BASE_URL / MCP_ENABLED).", file=sys.stderr)
        return 1
    client = ctx.session_client(["orca:read"])
    try:
        if client is None:
            print("Không có token: chạy get_mcp_token.py hoặc đặt ORCA_ADMIN_* trong .env.", file=sys.stderr)
            return 1
        projects, err = fetch_projects(client)
    finally:
        ctx.run_cleanups()  # đóng session, thu hồi PAT tạm (nếu có)
    if err:
        print(f"project_list lỗi: {err}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(projects, ensure_ascii=False, indent=2))
        return 0
    print(f"{len(projects)} project  (MCP {ctx.cfg.mcp_url})\n")
    for p in projects:
        print(f"{p.get('id', '?'):36s}  {str(p.get('name') or '?'):32s}  "
              f"jira={p.get('jiraProjectKey') or '-':8s}  {p.get('description') or ''}"[:160])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

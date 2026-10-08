#!/usr/bin/env python3
"""Liệt kê task của một project qua công cụ MCP `task_list`.

    python3 mcp_list_tasks.py --project aiops-v3          # theo tên (phải nằm trong project_list của token)
    python3 mcp_list_tasks.py --project <uuid>            # theo id (project mà token không thấy trong project_list vẫn thử được)
    python3 mcp_list_tasks.py --project <uuid> --json

Dùng ORCA_MCP_TOKEN trong tests/mcp/.env (xem get_mcp_token.py). Chỉ cần scope orca:read; quyền thực tế do server
quyết định theo thành viên/chế độ hiển thị của project.
"""
from __future__ import annotations

import argparse
import json
import re
import sys

from mcp_check_framework import build_context
from mcp_list_projects import fetch_projects

UUID_RE = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)


def fetch_tasks(client, project_id: str) -> tuple[list[dict], str | None]:
    tasks: list[dict] = []
    token = None
    for _ in range(100):
        args: dict = {"project_id": project_id, "page_size": 200}
        if token:
            args["page_token"] = token
        reply = client.call_tool("task_list", args)
        if reply.status != 200 or reply.error is not None:
            return tasks, f"HTTP {reply.status} {reply.text[:200]}"
        if reply.tool_is_error:
            return tasks, reply.tool_text[:300]
        data = reply.structured if isinstance(reply.structured, dict) else {}
        # task_list trả object {tasks, nextPageToken} (khác project_list: mảng được bọc thành {items}).
        tasks += [t for t in (data.get("tasks") or data.get("items") or []) if isinstance(t, dict)]
        token = data.get("nextPageToken") or data.get("next_page_token")
        if not token:
            return tasks, None
    return tasks, "quá 100 trang"


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--project", required=True, help="tên hoặc id (uuid) của project")
    ap.add_argument("--json", action="store_true", help="in JSON đầy đủ")
    args = ap.parse_args()

    ctx = build_context()
    if not ctx.mcp_enabled:
        print("MCP không phản hồi hoặc đang tắt.", file=sys.stderr)
        return 1
    client = ctx.session_client(["orca:read"])
    try:
        if client is None:
            print("Không có token: chạy get_mcp_token.py.", file=sys.stderr)
            return 1
        project_id = args.project
        if not UUID_RE.match(project_id):
            projects, err = fetch_projects(client)
            matches = [p for p in projects if str(p.get("name", "")).lower() == args.project.lower()]
            if err or not matches:
                print(f"Không tìm thấy project '{args.project}' trong project_list của token"
                      f"{' (' + err + ')' if err else ''}. Có thể token không phải thành viên: dùng --project <uuid>.",
                      file=sys.stderr)
                return 1
            project_id = str(matches[0]["id"])
        tasks, err = fetch_tasks(client, project_id)
    finally:
        ctx.run_cleanups()
    if err:
        print(f"task_list lỗi: {err}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(tasks, ensure_ascii=False, indent=2))
        return 0
    print(f"{len(tasks)} task  (project {project_id})\n")
    for t in tasks:
        print(f"#{t.get('taskNumber', '?'):<5} {str(t.get('status') or '?'):12s} {str(t.get('priority') or '-'):8s} "
              f"{str(t.get('id', '?')):36s} {t.get('title') or ''}"[:170])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

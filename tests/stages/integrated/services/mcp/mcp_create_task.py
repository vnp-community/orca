#!/usr/bin/env python3
"""Tạo task trong một project qua công cụ MCP `task_create` (cần scope orca:write).

    python3 mcp_create_task.py --project aiops-v3 --title "Sửa lỗi đăng nhập"
    python3 mcp_create_task.py --project <uuid> --title "Việc con" --parent-id <task-uuid>
    python3 mcp_create_task.py --project <uuid> --title "..." --labels bug,urgent
    python3 mcp_create_task.py --project <uuid> --title "..." --dry-run      # chỉ in, không tạo

Tham số mà MCP cho phép (tools/pack2_workspace.go):
    title      bắt buộc, tối đa 500 ký tự
    project_id tuỳ chọn, nhưng task_execute cần task thuộc một project có dev server
    parent_id  tuỳ chọn, id task cha (tạo subtask)
Mô tả, độ ưu tiên, người nhận việc, hạn... KHÔNG nhận được qua MCP; chỉ `labels` đặt được sau khi tạo (task_update).

Gateway chỉ công bố task_create khi bật MCP_TOOL_PACKS_ENABLED có pack 2 (mặc định chỉ pack 1).
Token: PAT orca:read+orca:write tự tạo bằng tài khoản admin trong .env và thu hồi khi xong.
"""
from __future__ import annotations

import argparse
import json
import re
import sys

from mcp_check_framework import build_context, find_value
from mcp_list_projects import fetch_projects

UUID_RE = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)
MAX_TITLE = 500  # Len(500) của tool task_create


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--project", required=True, help="tên hoặc id (uuid) của project")
    ap.add_argument("--title", required=True, help=f"tiêu đề task (1..{MAX_TITLE} ký tự)")
    ap.add_argument("--parent-id", default="", help="id task cha (tuỳ chọn)")
    ap.add_argument("--labels", default="", help="nhãn, phân cách bằng dấu phẩy (đặt bằng task_update sau khi tạo)")
    ap.add_argument("--dry-run", action="store_true", help="chỉ kiểm tra tham số và công cụ, không tạo task")
    ap.add_argument("--json", action="store_true", help="in task vừa tạo dạng JSON")
    args = ap.parse_args()

    title = args.title.strip()
    if not title or len(title) > MAX_TITLE:
        print(f"title phải có 1..{MAX_TITLE} ký tự (hiện {len(title)}).", file=sys.stderr)
        return 2
    if args.parent_id and not UUID_RE.match(args.parent_id):
        print("--parent-id phải là uuid của task cha.", file=sys.stderr)
        return 2
    labels = [x.strip() for x in args.labels.split(",") if x.strip()]

    ctx = build_context()
    if not ctx.mcp_enabled:
        print("MCP không phản hồi hoặc đang tắt.", file=sys.stderr)
        return 1
    client = ctx.session_client(["orca:read", "orca:write"])
    try:
        if client is None:
            print("Không có token: cần ORCA_ADMIN_* trong .env để tự cấp PAT có scope orca:write.", file=sys.stderr)
            return 1
        tools = {t.get("name") for t in client.list_tools()}
        if "task_create" not in tools:
            print("Gateway không công bố task_create: pack công cụ ghi chưa bật.\n"
                  "Đặt MCP_TOOL_PACKS_ENABLED=1,2 trong .env của server rồi khởi động lại api-gateway "
                  "(xem docs/guides/mcp/task-worktree-tools.md).", file=sys.stderr)
            return 1

        project_id = args.project
        if not UUID_RE.match(project_id):
            projects, err = fetch_projects(client)
            matches = [p for p in projects if str(p.get("name", "")).lower() == args.project.lower()]
            if err or not matches:
                print(f"Không tìm thấy project '{args.project}' trong project_list của token; dùng --project <uuid>.",
                      file=sys.stderr)
                return 1
            if len(matches) > 1:
                print(f"Có {len(matches)} project tên '{args.project}'; dùng --project <uuid>: "
                      + ", ".join(str(m["id"]) for m in matches), file=sys.stderr)
                return 1
            project_id = str(matches[0]["id"])

        payload = {"title": title, "project_id": project_id}
        if args.parent_id:
            payload["parent_id"] = args.parent_id
        if args.dry_run:
            print("[dry-run] sẽ gọi task_create với:", json.dumps(payload, ensure_ascii=False))
            if labels:
                print(f"[dry-run] rồi task_update labels={labels}")
            return 0

        reply = client.call_tool("task_create", payload)
        if reply.status != 200 or reply.error is not None or reply.tool_is_error:
            print(f"task_create lỗi: HTTP {reply.status} {reply.tool_text or reply.text[:300]}", file=sys.stderr)
            return 1
        task_id = find_value(reply.structured, "id")
        if not task_id:
            print(f"task_create không trả id: {reply.text[:300]}", file=sys.stderr)
            return 1
        if labels:
            upd = client.call_tool("task_update", {"id": str(task_id), "labels": labels})
            if upd.tool_is_error or upd.error is not None:
                print(f"Đã tạo task nhưng task_update(labels) lỗi: {upd.tool_text or upd.text[:200]}", file=sys.stderr)
        # Đọc lại để xác nhận task thật sự tồn tại (không tin vào phản hồi của lệnh tạo).
        got = client.call_tool("task_get", {"id": str(task_id)})
        task = got.structured if isinstance(got.structured, dict) else (reply.structured or {})
    finally:
        ctx.run_cleanups()

    if args.json:
        print(json.dumps(task, ensure_ascii=False, indent=2))
        return 0
    print(f"Đã tạo task #{task.get('taskNumber', '?')}  id={task.get('id')}\n"
          f"  project={task.get('projectId')}  status={task.get('status')}  title={task.get('title')}"
          + (f"\n  parent={task.get('parentId')}" if task.get("parentId") else "")
          + (f"\n  labels={task.get('labels')}" if task.get("labels") else ""))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

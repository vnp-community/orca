#!/usr/bin/env python3
"""Tạo task, kiểm tra tồn tại và theo dõi việc Orca thực thi — tất cả qua MCP server.

    python3 mcp_task_lifecycle.py create  [--title T]                 # tạo task trong project của .env và xác minh tồn tại
    python3 mcp_task_lifecycle.py status  --id <uuid> | --number N    # task có tồn tại không, đã thực thi ở Orca chưa
    python3 mcp_task_lifecycle.py execute --id <uuid> [--prompt P]    # task_execute rồi chờ kết quả (cần pack 3 + phê duyệt)
    python3 mcp_task_lifecycle.py run     [--title T] [--execute]     # tạo -> xác minh -> (thực thi) -> báo cáo

Project thử nghiệm và các tuỳ chọn đọc từ tests/mcp/.env (ORCA_MCP_TASK_PROJECT / _PROJECT_ID / _TITLE /
_EXECUTE / _PROMPT / _WAIT_S / _POLL_S); xem task_settings.py. Task được GIỮ LẠI để bạn xem trên giao diện,
thêm --cleanup để xoá sau khi chạy. Mã thoát 0 khi hoàn thành đúng kỳ vọng, 1 khi lỗi.
"""
from __future__ import annotations

import argparse
import sys
import time
import uuid

from task_execution_probe import RUNNING, TaskReport, execute_tool_available, find_task, inspect_task
from task_settings import TaskSettings, load_task_settings
from task_test_bed import TaskBed, task_id

from mcp_check_framework import Context, build_context
from mcp_http_client import McpClient

SCOPES_READ = ["orca:read"]
SCOPES_WRITE = ["orca:read", "orca:write"]
SCOPES_EXEC = ["orca:read", "orca:write", "orca:exec"]
FINISHED = ("review", "done")


def open_bed(ctx: Context, scopes: list[str]) -> TaskBed | None:
    """Mở phiên; task_create chỉ bắt buộc khi scope có ghi."""
    return TaskBed.open(ctx, scopes=scopes, auto_purge=False)


def create_and_verify(bed: TaskBed, title: str) -> tuple[str | None, TaskReport | None]:
    reply = bed.create(title)
    tid = task_id(reply)
    if not tid:
        print(f"Tạo task thất bại: HTTP {reply.status} {reply.tool_text or reply.text[:300]}", file=sys.stderr)
        return None, None
    print(f"Đã gọi task_create: id={tid}")
    rep = inspect_task(bed.client, bed.project_id, tid, variation=1)
    for line in rep.lines():
        print(line)
    return tid, rep


def wait_for_result(client: McpClient, project_id: str, tid: str, st: TaskSettings, started_status: str) -> TaskReport:
    """Kiểm tra định kỳ bằng task_list (đổi page_size mỗi lần để không dính bộ chặn lời gọi lặp)."""
    deadline = time.time() + st.wait_s
    n = 2
    last = ""
    while time.time() < deadline:
        t = find_task(client, project_id, task_id=tid, variation=n) or {}
        n += 1
        line = f"status={t.get('status')} actualHours={t.get('actualHours')}"
        if line != last:
            print(f"  [{time.strftime('%H:%M:%S')}] {line}")
            last = line
        finished = t.get("status") in FINISHED and (t.get("actualHours") or 0) > 0
        reverted = t.get("status") == started_status and (t.get("actualHours") or 0) == 0 and n > 4
        if finished or reverted:
            break
        time.sleep(st.poll_s)
    return inspect_task(client, project_id, tid, variation=n + 1)


def do_execute(bed: TaskBed, tid: str, st: TaskSettings, prompt: str) -> int:
    client = bed.client
    if not execute_tool_available(client):
        print("Gateway không công bố task_execute: pack exec chưa bật.\n"
              "Đặt MCP_TOOL_PACKS_ENABLED=1,2,3 trong .env của server rồi khởi động lại api-gateway; project cần có dev server kết nối.",
              file=sys.stderr)
        return 1
    before = inspect_task(client, bed.project_id, tid, variation=3)
    if not before.exists:
        print(f"Task {tid} không tồn tại trong project.", file=sys.stderr)
        return 1
    if before.running:
        print("Task đang chạy (in_progress); không gọi task_execute lần nữa.")
        return 0
    args: dict = {"task_id": tid, "request_id": str(uuid.uuid4())}
    if prompt:
        args["prompt"] = prompt
    print(f"Gọi task_execute cho {tid} …")
    reply = client.call_tool("task_execute", args)
    if reply.status != 200 or reply.error is not None or reply.tool_is_error:
        text = reply.tool_text or reply.text[:300]
        print(f"task_execute không chạy: {text}", file=sys.stderr)
        if "approv" in text.lower():
            print("Cần người duyệt trong Orca: Settings → MCP → Approvals.", file=sys.stderr)
        elif "NO_CONNECTION" in text:
            print("Project chưa có dev server kết nối.", file=sys.stderr)
        elif "ALREADY_IN_PROGRESS" in text:
            print("Task đang có một lượt chạy.", file=sys.stderr)
        return 1
    print(f"Orca đã nhận lệnh thực thi: {reply.tool_text[:160] or 'async'}")
    rep = wait_for_result(client, bed.project_id, tid, st, str(before.task.get("status")))
    print()
    for line in rep.lines():
        print(line)
    return 0 if rep.executed and not rep.running else 1


def main() -> int:
    st = load_task_settings()
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("create"); p.add_argument("--title", default=st.title)
    p = sub.add_parser("status"); p.add_argument("--id", default=""); p.add_argument("--number", type=int)
    p = sub.add_parser("execute"); p.add_argument("--id", required=True); p.add_argument("--prompt", default=st.prompt)
    p = sub.add_parser("run"); p.add_argument("--title", default=st.title); p.add_argument("--prompt", default=st.prompt)
    p.add_argument("--execute", action="store_true", default=st.execute, help="thực thi bằng task_execute (mặc định theo ORCA_MCP_TASK_EXECUTE)")
    for q in sub.choices.values():
        q.add_argument("--cleanup", action="store_true", help="xoá task do lệnh này tạo ra khi xong")
    args = ap.parse_args()

    ctx = build_context()
    if "unreachable" in ctx.state or not ctx.mcp_enabled:
        print(ctx.state.get("unreachable", "MCP không phản hồi hoặc đang tắt."), file=sys.stderr)
        return 1
    scopes = {"status": SCOPES_READ, "create": SCOPES_WRITE, "execute": SCOPES_EXEC,
              "run": SCOPES_EXEC if getattr(args, "execute", False) else SCOPES_WRITE}[args.cmd]
    bed = open_bed(ctx, scopes)
    code = 1
    created: list[str] = []
    try:
        if bed is None:
            return 1
        print(f"Project: {bed.project_name} ({bed.project_id})   MCP: {ctx.cfg.mcp_url}\n")
        if args.cmd == "create":
            tid, rep = create_and_verify(bed, args.title)
            if tid:
                created.append(tid)
            code = 0 if rep and rep.exists and rep.in_list else 1
        elif args.cmd == "status":
            tid = args.id
            if not tid:
                if args.number is None:
                    print("Cần --id hoặc --number.", file=sys.stderr)
                    return 2
                found = find_task(bed.client, bed.project_id, number=args.number)
                if not found:
                    print(f"Không có task #{args.number} trong project {bed.project_name}.")
                    return 1
                tid = str(found["id"])
            rep = inspect_task(bed.client, bed.project_id, tid)
            print("\n".join(rep.lines()))
            code = 0 if rep.exists else 1
        elif args.cmd == "execute":
            code = do_execute(bed, args.id, st, args.prompt)
        elif args.cmd == "run":
            tid, rep = create_and_verify(bed, args.title)
            if not tid:
                return 1
            created.append(tid)
            if not rep or not rep.exists:
                return 1
            if args.execute:
                print()
                code = do_execute(bed, tid, st, args.prompt)
            else:
                print("\n(Bỏ qua thực thi: dùng --execute hoặc ORCA_MCP_TASK_EXECUTE=true; cần pack 3 và dev server.)")
                code = 0 if not rep.executed else 1
    finally:
        if getattr(args, "cleanup", False) and created and bed is not None:
            bed.created = created
            bed.purge()
        elif created:
            print(f"\nTask được giữ lại: {', '.join(created)}  (thêm --cleanup để xoá)")
        ctx.run_cleanups()
    return code


if __name__ == "__main__":
    raise SystemExit(main())

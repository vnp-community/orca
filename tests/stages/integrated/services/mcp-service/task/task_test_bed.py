"""Phần dùng chung của bộ thử nghiệm tạo task qua MCP: project đích, tạo task có theo dõi, dọn dẹp.

Mọi task thử nghiệm mang tiền tố ORCA_TEST_PREFIX trong tiêu đề và bị xoá ở cuối bằng kênh /ws `task.delete`
(công cụ MCP task_delete là 'destructive' nên không dùng). Thiếu websocket-client thì không xoá được: bed in
danh sách id còn sót và ghi FAIL để người chạy biết phải dọn tay.
"""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))  # tests/mcp: framework + client dùng chung

import json
import os
import time
import uuid
from typing import Any

from mcp_check_framework import Context
from mcp_http_client import McpClient, RpcReply
from mcp_list_projects import fetch_projects

# mcp-service chặn lời gọi giống hệt lặp lại: >=5 trong 60s -> "slow down", >=20 trong 5 phút -> chặn vài phút
# (governance_ports.go: LoopSlowDown/LoopBlock). Bộ thử nghiệm tự giãn nhịp để không bị chặn nhầm.
_WINDOW_SHORT, _LIMIT_SHORT = 60.0, 4
_WINDOW_LONG, _LIMIT_LONG = 300.0, 15
_HISTORY: dict[str, list[float]] = {}
_original_call_tool = McpClient.call_tool


def _throttled_call_tool(self: McpClient, name: str, arguments: dict | None = None) -> RpcReply:
    key = name + json.dumps(arguments or {}, sort_keys=True, ensure_ascii=False)
    for attempt in range(8):
        now = time.monotonic()
        hist = [t for t in _HISTORY.get(key, []) if now - t < _WINDOW_LONG]
        wait = 0.0
        recent = [t for t in hist if now - t < _WINDOW_SHORT]
        if len(recent) >= _LIMIT_SHORT:
            wait = max(wait, recent[0] + _WINDOW_SHORT - now + 0.5)
        if len(hist) >= _LIMIT_LONG:
            wait = max(wait, hist[0] + _WINDOW_LONG - now + 0.5)
        if wait > 0:
            print(f"[info] giãn nhịp {wait:.0f}s trước khi gọi lại {name} (tránh bộ chặn lời gọi lặp)")
            time.sleep(wait)
        _HISTORY[key] = [t for t in hist if time.monotonic() - t < _WINDOW_LONG] + [time.monotonic()]
        reply = _original_call_tool(self, name, arguments)
        text = reply.tool_text
        if "Repeated identical call" not in text:
            return reply
        # Đã bị chặn từ các lần chạy trước (lịch sử trong tiến trình không biết): chờ rồi thử lại.
        pause = 70.0 if "blocked" in text else 25.0
        print(f"[info] server báo lặp lời gọi ({name}); chờ {pause:.0f}s rồi thử lại ({attempt + 1}/8)")
        time.sleep(pause)
    return reply


McpClient.call_tool = _throttled_call_tool  # type: ignore[method-assign]


class TaskBed:
    """Một phiên MCP có scope ghi + project đích + danh sách task đã tạo."""

    def __init__(self, ctx: Context, client: McpClient, project_id: str, project_name: str):
        self.ctx, self.client = ctx, client
        self.project_id, self.project_name = project_id, project_name
        self.created: list[str] = []  # thứ tự tạo; xoá theo thứ tự ngược (con trước cha)

    # ---- dựng bed -----------------------------------------------------------
    @staticmethod
    def open(ctx: Context, scopes: list[str] | None = None, auto_purge: bool = True) -> "TaskBed | None":
        """Mở phiên MCP + resolve project thử nghiệm theo cấu hình .env (task_settings).

        scopes mặc định read+write; auto_purge=False để CLI giữ lại task (tự gọi purge khi cần).
        """
        from task_settings import load_task_settings

        settings = load_task_settings()
        scopes = scopes or ["orca:read", "orca:write"]
        if not ctx.need_mcp():
            return None
        if "orca:write" in scopes and not ctx.cfg.allow_writes:
            ctx.skip("tạo task qua MCP", "ORCA_ALLOW_WRITES=false")
            return None
        client = ctx.session_client(scopes)
        if client is None:
            return None
        if "orca:write" in scopes and "task_create" not in {t.get("name") for t in client.list_tools()}:
            ctx.skip("tạo task qua MCP", "gateway chưa công bố task_create (MCP_TOOL_PACKS_ENABLED cần có pack 2)")
            return None
        projects, err = fetch_projects(client)
        wanted = settings.project_ref
        hits = [p for p in projects if str(p.get("name", "")).lower() == wanted.lower() or p.get("id") == wanted]
        if len(hits) == 1:
            bed = TaskBed(ctx, client, str(hits[0]["id"]), str(hits[0].get("name")))
        elif not hits and settings.project_id:
            # project không nằm trong project_list của token (không phải thành viên): dùng thẳng uuid trong .env
            bed = TaskBed(ctx, client, settings.project_id, "(không có trong project_list của token)")
        else:
            ctx.check(f"tìm project '{wanted}' trong project_list", False,
                      err or f"{len(hits)} kết quả (đặt ORCA_MCP_TASK_PROJECT_ID = uuid nếu trùng tên/không thấy)")
            return None
        if auto_purge:
            ctx.cleanup("xoá task thử nghiệm", bed.purge)
        return bed

    # ---- tạo / đọc ------------------------------------------------------------
    def title(self, tag: str) -> str:
        return self.ctx.name(tag)

    def create(self, title: str, *, project: str | None = "default", **extra: Any) -> RpcReply:
        """task_create và tự ghi nhận id nếu tạo được (kể cả khi test kỳ vọng nó bị từ chối)."""
        args: dict[str, Any] = {"title": title}
        if project == "default":
            args["project_id"] = self.project_id
        elif project:
            args["project_id"] = project
        args.update(extra)
        return self.track(self.client.call_tool("task_create", args))

    def track(self, reply: RpcReply) -> RpcReply:
        tid = task_id(reply)
        if tid and tid not in self.created:
            self.created.append(tid)
        return reply

    def get(self, tid: str) -> dict[str, Any]:
        r = self.client.call_tool("task_get", {"id": tid})
        return r.structured if isinstance(r.structured, dict) else {}

    def listed_ids(self) -> list[str]:
        out: list[str] = []
        token = None
        for _ in range(50):
            a: dict[str, Any] = {"project_id": self.project_id, "page_size": 200}
            if token:
                a["page_token"] = token
            s = self.client.call_tool("task_list", a).structured
            s = s if isinstance(s, dict) else {}
            out += [str(t.get("id")) for t in s.get("tasks") or [] if isinstance(t, dict)]
            token = s.get("nextPageToken")
            if not token:
                return out
        return out

    # ---- dọn dẹp ----------------------------------------------------------------
    def purge(self) -> None:
        ctx = self.ctx
        if not self.created:
            return
        admin = ctx.admin
        rpc = ctx.ws(admin) if admin is not None else None
        if rpc is None:
            ctx.check("dọn task thử nghiệm", False,
                      f"không có /ws để xoá; xoá tay các task: {', '.join(self.created)}")
            return
        failed: list[str] = []
        for tid in reversed(self.created):
            msg = rpc.invoke("task.delete", [{"id": tid}], timeout=ctx.cfg.timeout)
            if msg.get("type") == "error":
                failed.append(f"{tid} ({str(msg.get('message'))[:60]})")
        left = set(self.created) & set(self.listed_ids())
        ctx.check(f"dọn dẹp: đã xoá {len(self.created) - len(left)}/{len(self.created)} task thử nghiệm",
                  not failed and not left, f"không xoá được: {failed or sorted(left)}")
        self.created.clear()


def ctx_env(ctx: Context, key: str) -> str:
    """Đọc khoá tuỳ chọn từ tests/mcp/.env (McpConfig không khai báo mọi khoá)."""
    from mcp_env_config import HERE, parse_env_file
    return parse_env_file(HERE / ".env").get(key, "")


def task_id(reply: RpcReply) -> str | None:
    s = reply.structured
    if not (reply.status == 200 and reply.error is None and not reply.tool_is_error and isinstance(s, dict)):
        return None
    if isinstance(s.get("task"), dict):  # task_createFromSource trả {"created": bool, "task": {...}}
        s = s["task"]
    return str(s["id"]) if s.get("id") else None


def is_rejected(reply: RpcReply) -> bool:
    """Lời gọi bị từ chối có kiểm soát (lỗi JSON-RPC hoặc isError), không phải 5xx."""
    return reply.status < 500 and (reply.error is not None or reply.tool_is_error or reply.status in (400, 422))


def new_uuid() -> str:
    return str(uuid.uuid4())

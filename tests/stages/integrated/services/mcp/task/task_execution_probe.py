"""Kiểm tra qua MCP: task có tồn tại không, và đã được Orca thực thi chưa.

Chỉ dùng công cụ đọc của MCP server. Bằng chứng thực thi:
  - status = in_progress              → đang chạy (ExecuteTask đã nhận)
  - actualHours > 0                   → ExecuteTask đã ghi thời gian chạy khi kết thúc (task_update KHÔNG đặt trường này)
  - task_hasActiveExecutions(project) → project có task đang chạy
  - agentSession_listActive           → có phiên agent đang hoạt động
Chỉ riêng status = review/done KHÔNG đủ: task_update đặt tay được các trạng thái đó.
"""
from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any

from task_test_bed import task_id as reply_task_id  # noqa: F401  (nạp bộ giãn nhịp lời gọi lặp)

from mcp_http_client import McpClient

RUNNING = "in_progress"


@dataclass
class TaskReport:
    task_id: str
    exists: bool = False
    task: dict[str, Any] = field(default_factory=dict)
    in_list: bool = False
    project_active: bool | None = None
    agent_sessions: int | None = None
    evidence: list[str] = field(default_factory=list)
    executed: bool = False       # đã từng được thực thi
    running: bool = False        # đang thực thi
    verdict: str = ""

    def lines(self) -> list[str]:
        t = self.task
        out = [f"task {self.task_id}: {'TỒN TẠI' if self.exists else 'KHÔNG TỒN TẠI'}"
               + (f"  #{t.get('taskNumber')}  \"{t.get('title')}\"" if self.exists else "")]
        if self.exists:
            out.append(f"  trạng thái={t.get('status')}  actualHours={t.get('actualHours')}  "
                       f"labels={t.get('labels')}  project={t.get('projectId')}")
            out.append(f"  có trong task_list của project: {self.in_list}")
            out.append(f"  project đang có thực thi: {self.project_active}   phiên agent đang hoạt động: {self.agent_sessions}")
            out += [f"  bằng chứng: {e}" for e in self.evidence] or ["  bằng chứng thực thi: không có"]
        out.append(f"=> {self.verdict}")
        return out


def _structured(reply) -> dict[str, Any]:
    return reply.structured if isinstance(reply.structured, dict) else {}


def list_snapshot(client: McpClient, project_id: str, variation: int = 0) -> list[dict[str, Any]]:
    """task_list của project. `variation` đổi page_size để mỗi lần gọi có tham số khác nhau, tránh bộ chặn lặp."""
    out: list[dict[str, Any]] = []
    token = None
    for _ in range(50):
        args: dict[str, Any] = {"project_id": project_id, "page_size": 200 - (variation % 90)}
        if token:
            args["page_token"] = token
        s = _structured(client.call_tool("task_list", args))
        out += [t for t in s.get("tasks") or [] if isinstance(t, dict)]
        token = s.get("nextPageToken")
        if not token:
            break
    return out


def find_task(client: McpClient, project_id: str, *, task_id: str = "", number: int | None = None,
              variation: int = 0) -> dict[str, Any] | None:
    """Tìm task theo id hoặc taskNumber trong project (chỉ qua task_list/task_get)."""
    if task_id:
        got = _structured(client.call_tool("task_get", {"id": task_id}))
        if got.get("id") == task_id:
            return got
    for t in list_snapshot(client, project_id, variation):
        if (task_id and t.get("id") == task_id) or (number is not None and t.get("taskNumber") == number):
            return t
    return None


def inspect_task(client: McpClient, project_id: str, task_id: str, variation: int = 0,
                 with_project_signals: bool = True) -> TaskReport:
    rep = TaskReport(task_id=task_id)
    listed = {t.get("id"): t for t in list_snapshot(client, project_id, variation)}
    rep.in_list = task_id in listed
    got = _structured(client.call_tool("task_get", {"id": task_id}))
    rep.task = got if got.get("id") == task_id else listed.get(task_id, {})
    rep.exists = bool(rep.task)
    if not rep.exists:
        rep.verdict = "task không tồn tại"
        return rep

    if with_project_signals:
        act = _structured(client.call_tool("task_hasActiveExecutions", {"project_id": project_id}))
        rep.project_active = act.get("hasActiveExecutions")
        sess = _structured(client.call_tool("agentSession_listActive", {}))
        rep.agent_sessions = len(sess.get("agentSessions") or [])

    status = rep.task.get("status")
    hours = rep.task.get("actualHours") or 0
    if status == RUNNING:
        rep.running = rep.executed = True
        rep.evidence.append("status = in_progress (ExecuteTask đã nhận và đang chạy)")
        if rep.project_active:
            rep.evidence.append("task_hasActiveExecutions = true")
    if hours and hours > 0:
        rep.executed = True
        rep.evidence.append(f"actualHours = {hours} (do ExecuteTask ghi khi kết thúc)")
    if status in ("review", "done") and not (hours and hours > 0):
        rep.evidence.append(f"status = {status} nhưng actualHours = 0: có thể do đặt tay qua task_update, không chắc đã chạy")

    if rep.running:
        rep.verdict = "ĐANG thực thi ở Orca"
    elif rep.executed:
        rep.verdict = f"ĐÃ thực thi ở Orca (trạng thái hiện tại: {status})"
    else:
        rep.verdict = f"CHƯA thực thi ở Orca (trạng thái: {status}, actualHours: {hours})"
    return rep


def execute_tool_available(client: McpClient) -> bool:
    return "task_execute" in {t.get("name") for t in client.list_tools()}


def pretty(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, indent=2)

"""Cấu hình của bộ thử nghiệm task, đọc từ tests/mcp/.env (biến môi trường shell ghi đè).

    ORCA_MCP_TASK_PROJECT      tên project thử nghiệm (mặc định Vnp-asm)
    ORCA_MCP_TASK_PROJECT_ID   uuid project (nếu đặt thì ưu tiên hơn tên; dùng khi project không có trong project_list)
    ORCA_MCP_TASK_TITLE        tiêu đề task mặc định của CLI/suite lifecycle
    ORCA_MCP_TASK_EXECUTE      true: thực sự gọi task_execute (cần pack 3, dev server kết nối, người duyệt)
    ORCA_MCP_TASK_PROMPT       (tuỳ chọn) prompt gửi kèm task_execute
    ORCA_MCP_TASK_WAIT_S       thời gian tối đa chờ task chạy xong (giây)
    ORCA_MCP_TASK_POLL_S       chu kỳ kiểm tra trạng thái (giây)
    ORCA_MCP_TASK_KEEP         true: không xoá task sau khi chạy (để xem trên giao diện)
"""
from __future__ import annotations

import sys
from dataclasses import dataclass
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import os

from mcp_env_config import HERE, parse_env_file

DEFAULT_PROJECT = "Vnp-asm"


@dataclass(frozen=True)
class TaskSettings:
    project: str
    project_id: str
    title: str
    execute: bool
    prompt: str
    wait_s: float
    poll_s: float
    keep: bool

    @property
    def project_ref(self) -> str:
        """uuid nếu có, ngược lại tên project."""
        return self.project_id or self.project


def _truthy(v: str, default: bool = False) -> bool:
    return default if v == "" else v.strip().lower() in ("1", "true", "yes", "on")


def load_task_settings() -> TaskSettings:
    file_values = parse_env_file(HERE / ".env")

    def get(key: str, default: str = "") -> str:
        return os.environ.get(key) or file_values.get(key) or default

    return TaskSettings(
        project=get("ORCA_MCP_TASK_PROJECT", DEFAULT_PROJECT),
        project_id=get("ORCA_MCP_TASK_PROJECT_ID"),
        title=get("ORCA_MCP_TASK_TITLE", "Task thử nghiệm qua MCP"),
        execute=_truthy(get("ORCA_MCP_TASK_EXECUTE")),
        prompt=get("ORCA_MCP_TASK_PROMPT"),
        wait_s=float(get("ORCA_MCP_TASK_WAIT_S", "180")),
        poll_s=float(get("ORCA_MCP_TASK_POLL_S", "6")),
        keep=_truthy(get("ORCA_MCP_TASK_KEEP")),
    )

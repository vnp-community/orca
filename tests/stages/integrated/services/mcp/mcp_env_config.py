"""Nạp cấu hình kiểm tra MCP từ tests/mcp/.env (+ biến môi trường shell).

Thứ tự ưu tiên: biến môi trường shell > tests/mcp/.env > deploy/dev/.env (chỉ vài khóa trong
danh sách trắng, để dùng lại tài khoản bootstrap admin và URL công khai của môi trường dev).
"""
from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

HERE = Path(__file__).resolve().parent
DEPLOY_ENV = HERE.parents[1] / "deploy" / "dev" / ".env"
# Chỉ đọc các khóa này từ deploy/dev/.env: file đó còn chứa nhiều secret hạ tầng không liên quan.
DEPLOY_ENV_KEYS = {"BOOTSTRAP_ADMIN_EMAIL", "BOOTSTRAP_ADMIN_PASSWORD", "MCP_PUBLIC_BASE_URL", "PUBLIC_BASE_URL"}


def parse_env_file(path: Path, only: set[str] | None = None) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.is_file():
        return values
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        if line.startswith("export "):
            line = line[len("export "):]
        key, _, val = line.partition("=")
        key = key.strip()
        if only is not None and key not in only:
            continue
        val = val.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
            val = val[1:-1]
        elif " #" in val:
            val = val.split(" #", 1)[0].rstrip()
        values[key] = val
    return values


def _truthy(v: str, default: bool) -> bool:
    if v == "":
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


@dataclass(frozen=True)
class McpConfig:
    base_url: str          # api-gateway (REST /auth, /v1 và /ws)
    mcp_url: str           # endpoint MCP (streamable HTTP)
    ws_url: str
    admin_email: str
    admin_password: str
    user_email: str        # người dùng thường (tuỳ chọn) — kiểm tra phân quyền admin
    user_password: str
    mcp_token: str         # PAT dựng sẵn (tuỳ chọn) khi không có tài khoản đăng nhập
    allowed_origin: str    # Origin được phép theo MCP_ALLOWED_ORIGINS (tuỳ chọn)
    project_id: str        # project dùng cho luồng task/worktree (tuỳ chọn)
    expect_enabled: bool
    timeout: float
    verify_tls: bool
    allow_writes: bool
    probe_exec: bool
    revoke_wait_s: float
    prefix: str

    @property
    def has_admin_credentials(self) -> bool:
        return bool(self.admin_email and self.admin_password)

    @property
    def origin(self) -> str:
        """Origin của chính gateway — dùng làm Origin 'hợp lệ' khi không cấu hình riêng."""
        return self.allowed_origin or self.base_url


def load_config() -> McpConfig:
    file_values = parse_env_file(HERE / ".env")
    deploy_values = parse_env_file(DEPLOY_ENV, DEPLOY_ENV_KEYS)

    def get(key: str, default: str = "") -> str:
        return os.environ.get(key) or file_values.get(key) or deploy_values.get(key) or default

    base = (get("ORCA_API_BASE_URL") or get("MCP_PUBLIC_BASE_URL") or get("PUBLIC_BASE_URL")
            or "http://localhost:6768").rstrip("/")
    mcp_url = get("ORCA_MCP_URL") or base + "/mcp"
    ws = get("ORCA_WS_URL") or base.replace("https://", "wss://", 1).replace("http://", "ws://", 1) + "/ws"
    return McpConfig(
        base_url=base,
        mcp_url=mcp_url.rstrip("/"),
        ws_url=ws,
        admin_email=get("ORCA_ADMIN_EMAIL") or get("BOOTSTRAP_ADMIN_EMAIL"),
        admin_password=get("ORCA_ADMIN_PASSWORD") or get("BOOTSTRAP_ADMIN_PASSWORD"),
        user_email=get("ORCA_USER_EMAIL"),
        user_password=get("ORCA_USER_PASSWORD"),
        mcp_token=get("ORCA_MCP_TOKEN"),
        allowed_origin=get("ORCA_MCP_ALLOWED_ORIGIN"),
        project_id=get("ORCA_MCP_PROJECT_ID"),
        expect_enabled=_truthy(get("ORCA_MCP_EXPECT_ENABLED"), True),
        timeout=float(get("ORCA_HTTP_TIMEOUT", "20")),
        verify_tls=_truthy(get("ORCA_VERIFY_TLS"), True),
        allow_writes=_truthy(get("ORCA_ALLOW_WRITES"), True),
        probe_exec=_truthy(get("ORCA_MCP_PROBE_EXEC"), False),
        revoke_wait_s=float(get("ORCA_MCP_REVOKE_WAIT_S", "75")),
        prefix=get("ORCA_TEST_PREFIX", "mcptest"),
    )

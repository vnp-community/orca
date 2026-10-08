"""Nạp cấu hình kiểm tra từ tests/backend/.env (+ biến môi trường shell)."""
from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

HERE = Path(__file__).resolve().parent


def parse_env_file(path: Path) -> dict[str, str]:
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
        val = val.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
            val = val[1:-1]
        elif " #" in val:
            val = val.split(" #", 1)[0].rstrip()
        values[key.strip()] = val
    return values


def _truthy(v: str, default: bool) -> bool:
    if v == "":
        return default
    return v.strip().lower() in ("1", "true", "yes", "on")


@dataclass(frozen=True)
class Config:
    base_url: str
    admin_email: str
    admin_password: str
    user_email: str
    user_password: str
    timeout: float
    delay_s: float
    verify_tls: bool
    ws_url: str
    agent_secret: str
    allow_writes: bool
    prefix: str
    ws_probe_all: bool

    @property
    def has_admin_credentials(self) -> bool:
        return bool(self.admin_email and self.admin_password)


def load_config() -> Config:
    file_values = parse_env_file(HERE / ".env")

    def get(key: str, default: str = "") -> str:
        return os.environ.get(key, file_values.get(key, default))

    base = get("ORCA_API_BASE_URL", "http://localhost:6768").rstrip("/")
    ws = get("ORCA_WS_URL")
    if not ws:
        ws = base.replace("https://", "wss://", 1).replace("http://", "ws://", 1) + "/ws"
    return Config(
        base_url=base,
        admin_email=get("ORCA_ADMIN_EMAIL") or get("BOOTSTRAP_ADMIN_EMAIL"),
        admin_password=get("ORCA_ADMIN_PASSWORD") or get("BOOTSTRAP_ADMIN_PASSWORD"),
        user_email=get("ORCA_USER_EMAIL"),
        user_password=get("ORCA_USER_PASSWORD"),
        timeout=float(get("ORCA_HTTP_TIMEOUT", "20")),
        delay_s=float(get("ORCA_REQUEST_DELAY_MS", "0")) / 1000.0,
        verify_tls=_truthy(get("ORCA_VERIFY_TLS"), True),
        ws_url=ws,
        agent_secret=get("ORCA_AGENT_API_SECRET"),
        allow_writes=_truthy(get("ORCA_ALLOW_WRITES"), True),
        prefix=get("ORCA_TEST_PREFIX", "apitest"),
        ws_probe_all=_truthy(get("ORCA_WS_PROBE_ALL"), False),
    )

"""Run configuration. Admin credentials come from deploy/dev/.env (the local stack's own bootstrap admin) or ORCA_E2E_* overrides; they are never printed."""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[4]
STACK_ENV_FILE = REPO_ROOT / "deploy" / "dev" / ".env"


def _read_env_file(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.is_file():
        return values
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, val = line.partition("=")
        values[key.strip()] = val.strip().strip("'\"")
    return values


@dataclass
class StackConfig:
    gateway_url: str
    frontend_url: str
    admin_email: str
    admin_password: str = field(repr=False)
    dev_server_name: str
    repo_path: str  # a git repo on the dev server that suites may bind a project to
    run_prefix: str
    allow_agent_execution: bool  # task.execute / agent steps spend AI usage and run code on the dev server
    stack_env: dict[str, str] = field(default_factory=dict, repr=False)

    @property
    def ws_url(self) -> str:
        return self.gateway_url.replace("http://", "ws://").replace("https://", "wss://") + "/ws"


def load_config(run_id: str) -> StackConfig:
    stack = _read_env_file(STACK_ENV_FILE)

    def pick(name: str, stack_key: str = "", default: str = "") -> str:
        return os.environ.get(name) or (stack.get(stack_key) if stack_key else "") or default

    gw_port = stack.get("API_GATEWAY_PUBLIC_PORT", "8081")
    fe_port = stack.get("FRONTEND_HTTP_PORT", "8080")
    return StackConfig(
        gateway_url=pick("ORCA_E2E_GATEWAY_URL", default=f"http://localhost:{gw_port}").rstrip("/"),
        frontend_url=pick("ORCA_E2E_FRONTEND_URL", default=f"http://localhost:{fe_port}").rstrip("/"),
        admin_email=pick("ORCA_E2E_ADMIN_EMAIL", "BOOTSTRAP_ADMIN_EMAIL"),
        admin_password=pick("ORCA_E2E_ADMIN_PASSWORD", "BOOTSTRAP_ADMIN_PASSWORD"),
        dev_server_name=pick("ORCA_E2E_DEV_SERVER_NAME", default="dev-local-mac"),
        repo_path=pick("ORCA_E2E_REPO_PATH"),
        run_prefix=f"e2e-{run_id}",
        allow_agent_execution=os.environ.get("ORCA_E2E_EXECUTE_AGENT", "").lower() in ("1", "true", "yes"),
        stack_env=stack,
    )

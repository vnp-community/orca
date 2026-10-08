"""Infra fleet (/v1/infra/*, /v1/worktrees/{id}/agent/*, /agent, /api/agent-token) và
Orchestration (/v1/orchestration/*).

Spec: specs/backend-go/tdd/services/infra-fleet-service.md, orchestration-service.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import uuid

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "infra+orchestration"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return

    # ---------------- infra: read ----------------
    ctx.expect("GET /v1/infra/dev-servers", admin.get("/v1/infra/dev-servers"), 200)
    ctx.expect("GET /v1/infra/health", admin.get("/v1/infra/health"), 200)
    agent = f"/v1/worktrees/{ZERO_UUID}/agent"
    at = "/v1/worktrees/{worktreeId}/agent"
    ctx.expect("GET .../agent/status không có session -> 404", admin.get(agent + "/status", template=at + "/status"),
               404)
    ctx.expect("GET .../agent/snapshot không có session -> 404", admin.get(agent + "/snapshot",
               template=at + "/snapshot"), 404)
    ctx.expect("POST .../agent/wait không có session -> 404", admin.post(agent + "/wait", template=at + "/wait",
               json={"timeout_ms": 100}), 404)
    ctx.expect("POST .../agent/send không có session -> 404", admin.post(agent + "/send", template=at + "/send",
               json={"text": "hi"}), 404)

    # ---------------- agent proxy (public, auth bên trong infra-fleet) ----------------
    r = ctx.anon.request("GET", "/api/agent-token", auth=False)
    ctx.expect("GET /api/agent-token không bearer -> 401/403", r, 401, 403, 405)
    if ctx.cfg.agent_secret:
        r = ctx.anon.request("GET", "/api/agent-token", auth=False,
                             headers={"Authorization": f"Bearer {ctx.cfg.agent_secret}"})
        ctx.expect("GET /api/agent-token với ORCA_AGENT_API_SECRET", r, 200)
    else:
        ctx.skip("GET /api/agent-token với secret", "chưa cấu hình ORCA_AGENT_API_SECRET")
    ctx.expect("GET /agent không upgrade WebSocket -> 4xx", ctx.anon.request("GET", "/agent", auth=False), "4xx")

    if not ctx.need_writes("infra/orchestration writes"):
        return

    # ---------------- infra: write ----------------
    r = admin.post("/v1/infra/ssh-targets", json={"host": "127.0.0.1", "user": "apitest",
                                                  "vault_ssh_role": "apitest-role"})
    ctx.expect("POST /v1/infra/ssh-targets", r, "2xx", "handled")
    ssh_id = find_value(json_body(r), "id", "ssh_target_id", "sshTargetId") if r.status_code < 300 else None
    ctx.expect("POST /v1/infra/ssh-targets thiếu host -> 400", admin.post("/v1/infra/ssh-targets", json={}), 400)

    body = {"host": f"{ctx.cfg.prefix}-dev.example.test", "mode": "direct_websocket"}
    if ssh_id:
        body.update({"mode": "relay_ssh", "ssh_target_id": ssh_id})
    r = admin.post("/v1/infra/dev-servers", json=body)
    ctx.expect("POST /v1/infra/dev-servers", r, "2xx", "handled")
    ds_id = find_value(json_body(r), "id", "dev_server_id", "devServerId") if r.status_code < 300 else None
    ctx.expect("POST /v1/infra/dev-servers mode sai -> 400", admin.post(
               "/v1/infra/dev-servers", json={"host": "x.example.test", "mode": "teleport"}), 400)
    if ds_id:
        r = admin.get("/v1/infra/dev-servers")
        ctx.assert_true("dev-server vừa đăng ký có trong danh sách", ds_id in r.text, r.text[:200])
        r = admin.post("/v1/infra/connections", json={"dev_server_id": ds_id, "repo_path": "/tmp/apitest",
                                                      "worktree_id": ZERO_UUID})
        ctx.expect("POST /v1/infra/connections", r, "2xx", "handled")
        conn_id = find_value(json_body(r), "id", "connection_id", "connectionId") if r.status_code < 300 else None
        if conn_id:
            ctx.expect("POST /v1/infra/connections/resolve", admin.post(
                       "/v1/infra/connections/resolve", json={"connection_id": conn_id}), "2xx", "handled")
            ctx.expect("POST /v1/infra/workspaces/scan-ports", admin.post(
                       "/v1/infra/workspaces/scan-ports", json={"connection_id": conn_id,
                                                                "worktree_id": ZERO_UUID}), "handled", "no5xx")
            ctx.expect("POST /v1/infra/relay", admin.post(
                       "/v1/infra/relay", json={"connection_id": conn_id, "method": "ping", "params_json": "{}"}),
                       "handled", 502, 503, 504)  # dev server giả -> relay không tới được
        else:
            ctx.skip("connections resolve/scan-ports/relay", "tạo connection không thành công")
    else:
        ctx.skip("connections", "đăng ký dev-server không thành công")
    ctx.expect("POST /v1/infra/connections/resolve connection không tồn tại", admin.post(
               "/v1/infra/connections/resolve", json={"connection_id": ZERO_UUID}), "4xx")
    ctx.expect("POST /v1/infra/relay connection không tồn tại", admin.post(
               "/v1/infra/relay", json={"connection_id": ZERO_UUID, "method": "ping", "params_json": "{}"}), "4xx")
    ctx.expect("POST /v1/infra/workspaces/scan-ports connection không tồn tại", admin.post(
               "/v1/infra/workspaces/scan-ports", json={"connection_id": ZERO_UUID, "worktree_id": ZERO_UUID}),
               "4xx")

    # ---------------- orchestration ----------------
    r = admin.post("/v1/orchestration/dispatch-contexts", json={
        "handle": ctx.name("handle"), "coordinator_run_id": str(uuid.uuid4()),
        "orchestration_task_id": str(uuid.uuid4()), "worktree_id": ZERO_UUID})
    ctx.expect("POST /v1/orchestration/dispatch-contexts", r, "2xx", "handled")
    dc_id = find_value(json_body(r), "id", "dispatch_context_id", "dispatchContextId") if r.status_code < 300 else None
    ctx.expect("POST dispatch-contexts thiếu handle -> 400", admin.post(
               "/v1/orchestration/dispatch-contexts", json={}), 400)
    r = admin.post("/v1/orchestration/gates", json={
        "dispatch_context_id": dc_id or ZERO_UUID, "orchestration_task_id": str(uuid.uuid4()),
        "question": "Proceed?", "options": ["yes", "no"]})
    ctx.expect("POST /v1/orchestration/gates", r, "2xx", "handled")
    gate_id = find_value(json_body(r), "id", "gate_id", "gateId") if r.status_code < 300 else None
    if gate_id:
        ctx.expect("POST /v1/orchestration/gates/{id}/resolve", admin.post(
                   f"/v1/orchestration/gates/{gate_id}/resolve", template="/v1/orchestration/gates/{id}/resolve",
                   json={"outcome_json": '{"choice":"yes"}'}), "2xx")
    ctx.expect("POST gates/{id}/resolve không tồn tại -> 404", admin.post(
               f"/v1/orchestration/gates/{ZERO_UUID}/resolve", template="/v1/orchestration/gates/{id}/resolve",
               json={"outcome_json": "{}"}), 404, 400)
    ctx.expect("PUT /v1/orchestration/tasks/{id}/status task không tồn tại", admin.put(
               f"/v1/orchestration/tasks/{ZERO_UUID}/status", template="/v1/orchestration/tasks/{id}/status",
               json={"new_status": "in_progress"}), "4xx")


if __name__ == "__main__":
    run_single(SUITE, run)

"""Automation (/v1/automations/*) và Workflow (/v1/workflows/*).

Spec: specs/backend-go/tdd/services/automation-service.md, workflow-service.md.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import json
import uuid
from datetime import datetime, timezone

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "automation+workflow"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    ctx.expect("GET /v1/automations", admin.get("/v1/automations/", params={"page_size": 5}), 200)
    ctx.expect("GET /v1/workflows/templates", admin.get("/v1/workflows/templates", params={"page_size": 5}), 200)
    ctx.expect("GET /v1/workflows/templates/resolve", admin.get(
               "/v1/workflows/templates/resolve", params={"template_id": ZERO_UUID}), "handled")
    ctx.expect("GET /v1/workflows/executions/{id} không tồn tại -> 404", admin.get(
               f"/v1/workflows/executions/{ZERO_UUID}", template="/v1/workflows/executions/{id}"), 404)
    ctx.expect("GET /v1/workflows/{id}/active-executions", admin.get(
               f"/v1/workflows/{ZERO_UUID}/active-executions", template="/v1/workflows/{id}/active-executions"),
               "handled")
    if not ctx.need_writes("automation/workflow writes"):
        return

    # ---------------- workflow ----------------
    dag = {"steps": [{"id": "s1", "type": "notification", "config": {"message": "api test"}}]}
    r = admin.post("/v1/workflows/templates", json={"name": ctx.name("wf"), "dag_json": json.dumps(dag),
                                                    "scope": "personal"})
    ctx.expect("POST /v1/workflows/templates", r, "2xx")
    tid = find_value(json_body(r), "id", "template_id", "templateId")
    ctx.expect("POST /v1/workflows/templates dag hỏng -> 400", admin.post(
               "/v1/workflows/templates", json={"name": ctx.name("wf"), "dag_json": "{not json", "scope": "personal"}),
               400)
    ctx.expect("POST /v1/workflows/templates scope sai -> 400", admin.post(
               "/v1/workflows/templates", json={"name": ctx.name("wf"), "dag_json": json.dumps(dag),
                                                "scope": "galaxy"}), 400)
    if tid:
        ctx.expect("GET /v1/workflows/templates/resolve (template vừa tạo)", admin.get(
                   "/v1/workflows/templates/resolve", params={"template_id": tid}), 200)
        r = admin.post("/v1/workflows/executions", json={"template_id": tid, "request_id": str(uuid.uuid4())})
        ctx.expect("POST /v1/workflows/executions", r, "2xx", "handled")
        eid = find_value(json_body(r), "id", "execution_id", "executionId")
        if eid:
            e = f"/v1/workflows/executions/{eid}"
            et = "/v1/workflows/executions/{id}"
            ctx.expect("GET /v1/workflows/executions/{id}", admin.get(e, template=et), 200)
            ctx.expect("POST .../executions/{id}/pause", admin.post(e + "/pause", template=et + "/pause"), "handled")
            ctx.expect("POST .../executions/{id}/resume", admin.post(e + "/resume", template=et + "/resume"),
                       "handled")
            ctx.expect("POST .../executions/{id}/steps/adhoc", admin.post(
                       e + "/steps/adhoc", template=et + "/steps/adhoc",
                       json={"step_type": "notification", "step_config_json": '{"message":"adhoc"}',
                             "request_id": str(uuid.uuid4())}), "handled")
            ctx.expect("POST .../executions/{id}/cancel", admin.post(e + "/cancel", template=et + "/cancel"),
                       "handled")
        else:
            ctx.skip("pause/resume/adhoc/cancel", "execute không trả execution id")
        ctx.expect("GET /v1/workflows/{id}/active-executions (template thật)", admin.get(
                   f"/v1/workflows/{tid}/active-executions", template="/v1/workflows/{id}/active-executions"), 200)
    else:
        ctx.skip("vòng đời workflow", "tạo template không trả id")
    ctx.expect("POST /v1/workflows/executions template không tồn tại", admin.post(
               "/v1/workflows/executions", json={"template_id": ZERO_UUID, "request_id": str(uuid.uuid4())}),
               "4xx")

    # ---------------- automation ----------------
    dtstart = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    step = json.dumps({"message": "api test"})
    r = admin.post("/v1/automations/", json={
        "name": ctx.name("auto"), "rrule": "FREQ=DAILY;INTERVAL=1", "step_type": "notification",
        "step_config_json": step, "dtstart": dtstart, "timezone": "UTC"})
    ctx.expect("POST /v1/automations", r, "2xx")
    aid = find_value(json_body(r), "id", "automation_id", "automationId")
    ctx.expect("POST /v1/automations rrule sai -> 400", admin.post("/v1/automations/", json={
        "name": ctx.name("auto"), "rrule": "NOT-AN-RRULE", "step_type": "notification",
        "step_config_json": step, "dtstart": dtstart, "timezone": "UTC"}), 400)
    ctx.expect("POST /v1/automations thiếu name -> 400", admin.post("/v1/automations/", json={}), 400)
    if not aid:
        ctx.skip("vòng đời automation", "tạo automation không trả id")
        return
    ctx.cleanup(f"automation {aid}", lambda: admin.delete(f"/v1/automations/{aid}", template="/v1/automations/{id}"))
    a = f"/v1/automations/{aid}"
    at = "/v1/automations/{id}"
    r = admin.get("/v1/automations/", params={"page_size": 200})
    ctx.assert_true("automation có trong danh sách", aid in r.text, "không thấy trong 200 bản ghi đầu")
    ctx.expect("PATCH /v1/automations/{id}", admin.patch(a, template=at, json={"enabled": False,
               "name": ctx.name("auto-r")}), "2xx")
    ctx.expect("POST /v1/automations/{id}/run", admin.post(a + "/run", template=at + "/run",
               json={"request_id": str(uuid.uuid4())}), "handled")
    ctx.expect("GET /v1/automations/{id}/runs", admin.get(a + "/runs", template=at + "/runs",
               params={"page_size": 10}), 200)
    ctx.expect("POST /v1/automations/{id}/trigger", admin.post(a + "/trigger", template=at + "/trigger",
               json={"request_id": str(uuid.uuid4()), "payload_json": "{}"}), "handled")
    ctx.expect("DELETE /v1/automations/{id}", admin.delete(a, template=at), "2xx")
    ctx.expect("PATCH automation đã xoá -> 404", admin.patch(a, template=at, json={"enabled": True}), 404)


if __name__ == "__main__":
    run_single(SUITE, run)

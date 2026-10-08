"""Task: /v1/tasks/* (DAG, grants, comments, execution, subtree, progress).

Spec: specs/backend-go/tdd/services/task-service.md.
"""
from __future__ import annotations

import uuid

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "tasks"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    ctx.expect("GET /v1/tasks/{id} không tồn tại -> 404",
               admin.get(f"/v1/tasks/{ZERO_UUID}", template="/v1/tasks/{id}"), 404)
    if not ctx.need_writes("task writes"):
        return

    # Task thuộc một project -> tạo project riêng và dọn dẹp cuối suite.
    r = admin.post("/v1/projects/", json={"name": ctx.name("task-project"), "default_branch": "main",
                                          "visibility": "private"})
    project_id = find_value(json_body(r), "id", "project_id", "projectId")
    if project_id:
        ctx.cleanup(f"project {project_id}", lambda: admin.delete(f"/v1/projects/{project_id}",
                                                                 template="/v1/projects/{id}"))

    def create(title: str, parent: str | None = None):
        body = {"title": title}
        if project_id:
            body["project_id"] = project_id
        if parent:
            body["parent_id"] = parent
        return admin.post("/v1/tasks/", json=body)

    r = create(ctx.name("task-root"))
    ctx.expect("POST /v1/tasks", r, "2xx")
    root = find_value(json_body(r), "id", "task_id", "taskId")
    ctx.expect("POST /v1/tasks thiếu title -> 400", admin.post("/v1/tasks/", json={}), 400)
    if not root:
        ctx.skip("vòng đời task", "tạo task không trả id")
        return
    child = find_value(json_body(create(ctx.name("task-child"), root)), "id", "task_id", "taskId")
    other = find_value(json_body(create(ctx.name("task-other"))), "id", "task_id", "taskId")

    t = "/v1/tasks/{id}"
    ctx.expect("GET /v1/tasks/{id}", admin.get(f"/v1/tasks/{root}", template=t), 200)

    if other:
        ctx.expect("POST /v1/tasks/{id}/edges (depends_on)", admin.post(
                   f"/v1/tasks/{root}/edges", template=t + "/edges",
                   json={"to_task_id": other, "type": "depends_on"}), "2xx")
        ctx.expect("POST edges tạo chu trình -> bị từ chối", admin.post(
                   f"/v1/tasks/{other}/edges", template=t + "/edges",
                   json={"to_task_id": root, "type": "depends_on"}), 400, 409, 412)
    ctx.expect("POST edges type sai -> 400", admin.post(f"/v1/tasks/{root}/edges", template=t + "/edges",
               json={"to_task_id": other or ZERO_UUID, "type": "bogus"}), 400)

    uid = (admin.user or {}).get("id", ZERO_UUID)
    ctx.expect("POST /v1/tasks/{id}/grants", admin.post(f"/v1/tasks/{root}/grants", template=t + "/grants",
               json={"subject_id": uid, "level": "user", "apply_tree": True}), "2xx")
    ctx.expect("POST grants level sai -> 400", admin.post(f"/v1/tasks/{root}/grants", template=t + "/grants",
               json={"subject_id": uid, "level": "god"}), 400)
    r = admin.get(f"/v1/tasks/{root}/permission", template=t + "/permission")
    ctx.expect("GET /v1/tasks/{id}/permission", r, 200)
    ctx.assert_true("permission có effective_level", "effective_level" in (json_body(r) or {}), r.text[:200])

    ctx.expect("GET /v1/tasks/{id}/subtree", admin.get(f"/v1/tasks/{root}/subtree", template=t + "/subtree"), 200)
    ctx.expect("POST /v1/tasks/{id}/progress:recalculate", admin.post(
               f"/v1/tasks/{root}/progress:recalculate", template=t + "/progress:recalculate"), "2xx")
    r = admin.get(f"/v1/tasks/{root}/active-executions", template=t + "/active-executions")
    ctx.expect("GET /v1/tasks/{id}/active-executions", r, 200)
    ctx.assert_true("active-executions có has_active", "has_active" in (json_body(r) or {}), r.text[:200])

    ctx.expect("POST /v1/tasks/{id}/execute", admin.post(f"/v1/tasks/{root}/execute", template=t + "/execute",
               json={"request_id": str(uuid.uuid4())}), "handled")

    ctx.expect("POST /v1/tasks/{id}/comments", admin.post(f"/v1/tasks/{root}/comments", template=t + "/comments",
               json={"content": "api test comment"}), "2xx")
    r = admin.get(f"/v1/tasks/{root}/comments", template=t + "/comments", params={"page_size": 10})
    ctx.expect("GET /v1/tasks/{id}/comments", r, 200)
    ctx.assert_true("comment vừa tạo có trong danh sách", "api test comment" in r.text, r.text[:200])
    ctx.expect("POST comments rỗng -> 400", admin.post(f"/v1/tasks/{root}/comments", template=t + "/comments",
               json={"content": ""}), 400)
    ctx.expect("POST /v1/tasks/{id}/comments task không tồn tại -> 404", admin.post(
               f"/v1/tasks/{ZERO_UUID}/comments", template=t + "/comments", json={"content": "x"}), 404)


if __name__ == "__main__":
    run_single(SUITE, run)

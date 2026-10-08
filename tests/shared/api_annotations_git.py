"""Annotation (/v1/annotations/*) và Git gateway (/v1/git/*, POST /v1/worktrees).

Spec: specs/backend-go/tdd/services/annotation-service.md, git-gateway-service.md.
Git cần worktree thật trên host của git-gateway nên các lời gọi dùng id giả và chỉ
kiểm tra gateway ánh xạ lỗi đúng (4xx), không 5xx/501.
"""
from __future__ import annotations

import uuid

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "annotations+git"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return

    # ---------------- git ----------------
    wt = {"worktree_id": ZERO_UUID}
    ctx.expect("GET /v1/git/status worktree không tồn tại", admin.get("/v1/git/status", params=wt),
               "4xx")
    ctx.expect("GET /v1/git/status thiếu worktree_id -> 400", admin.get("/v1/git/status"), 400)
    ctx.expect("GET /v1/git/diff worktree không tồn tại", admin.get("/v1/git/diff", params=wt), "4xx")
    ctx.expect("POST /v1/git/commit worktree không tồn tại", admin.post(
               "/v1/git/commit", json={"worktree_id": ZERO_UUID, "message": "x", "paths": []}), "4xx")
    ctx.expect("POST /v1/git/commit thiếu message -> 400", admin.post(
               "/v1/git/commit", json={"worktree_id": ZERO_UUID}), 400)
    ctx.expect("POST /v1/git/push worktree không tồn tại", admin.post(
               "/v1/git/push", json={"worktree_id": ZERO_UUID, "remote": "origin", "branch": "main"}), "4xx")
    ctx.expect("POST /v1/git/pull worktree không tồn tại", admin.post("/v1/git/pull", json=wt), "4xx")
    ctx.expect("POST /v1/git/commit-message worktree không tồn tại", admin.post(
               "/v1/git/commit-message", json=wt), "4xx", "handled")
    ctx.expect("POST /v1/worktrees thiếu trường -> 400", admin.post("/v1/worktrees", json={}), 400)
    ctx.expect("POST /v1/worktrees project/repo không tồn tại", admin.post(
               "/v1/worktrees", json={"project_id": ZERO_UUID, "repo_id": ZERO_UUID, "branch": "api-test",
                                      "base_ref": "main", "idempotency_key": str(uuid.uuid4())}), "4xx")

    # ---------------- annotations ----------------
    ctx.expect("GET /v1/annotations", admin.get("/v1/annotations/", params={"worktree_id": ZERO_UUID}), "handled")
    if not ctx.need_writes("annotation writes"):
        return
    anchor = {"repo_id": ZERO_UUID, "worktree_id": ZERO_UUID, "file_path": "README.md", "line": 1,
              "end_line": 1, "side": 1, "ref": "HEAD"}
    r = admin.post("/v1/annotations/", json={"anchor": anchor, "content": "api test annotation",
                                             "request_id": str(uuid.uuid4()), "original_code": "x"})
    ctx.expect("POST /v1/annotations", r, "2xx", "handled")
    ctx.expect("POST /v1/annotations thiếu content -> 400",
               admin.post("/v1/annotations/", json={"anchor": anchor, "content": ""}), 400)
    aid = find_value(json_body(r), "id", "annotation_id", "annotationId") if r.status_code < 300 else None
    if not aid:
        ctx.skip("vòng đời annotation", "tạo annotation không thành công (cần repo/worktree thật)")
    else:
        t = "/v1/annotations/{id}"
        ctx.expect("PUT /v1/annotations/{id}", admin.put(f"/v1/annotations/{aid}", template=t,
                   json={"content": "updated", "resolved": False}), "2xx")
        ctx.expect("PATCH /v1/annotations/{id}", admin.patch(f"/v1/annotations/{aid}", template=t,
                   json={"content": "patched", "resolved": True}), "2xx")
        ctx.expect("POST /v1/annotations/mark-sent", admin.post("/v1/annotations/mark-sent",
                   json={"ids": [aid]}), "2xx")
        ctx.expect("DELETE /v1/annotations/{id}", admin.delete(f"/v1/annotations/{aid}", template=t,
                   params={"confirmed": "true"}), "2xx")
    ctx.expect("PUT /v1/annotations/{id} không tồn tại -> 404", admin.put(
               f"/v1/annotations/{ZERO_UUID}", template="/v1/annotations/{id}",
               json={"content": "x", "resolved": False}), 404)
    ctx.expect("POST /v1/annotations/mark-sent ids rỗng", admin.post("/v1/annotations/mark-sent",
               json={"ids": []}), "no5xx")
    ctx.expect("POST /v1/annotations/send-to-agent worktree không tồn tại", admin.post(
               "/v1/annotations/send-to-agent",
               json={"worktree_id": ZERO_UUID, "pty_id": "pty-none", "worktree_name": "x"}), "4xx", "handled")


if __name__ == "__main__":
    run_single(SUITE, run)

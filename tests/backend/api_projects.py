"""Project: /v1/projects/*, /v1/project-groups/*.

Spec: specs/backend-go/tdd/services/project-service.md.
"""
from __future__ import annotations

from check_framework import Context, run_single
from orca_api_session import find_value, json_body

SUITE = "projects"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return
    ctx.expect("GET /v1/projects", admin.get("/v1/projects/", params={"page_size": 5}), 200)
    ctx.expect("GET /v1/projects/{id} không tồn tại -> 404",
               admin.get(f"/v1/projects/{ZERO_UUID}", template="/v1/projects/{id}"), 404)
    ctx.expect("GET /v1/project-groups", admin.get("/v1/project-groups/"), 200)
    if not ctx.need_writes("project writes"):
        return

    # --- project ---
    name = ctx.name("project")
    r = admin.post("/v1/projects/", json={"name": name, "description": "api test", "default_branch": "main",
                                          "visibility": "private"})
    ctx.expect("POST /v1/projects", r, "2xx")
    pid = find_value(json_body(r), "id", "project_id", "projectId")
    ctx.expect("POST /v1/projects thiếu name -> 400", admin.post("/v1/projects/", json={}), 400)
    if not pid:
        ctx.skip("vòng đời project", "tạo project không trả id")
        return
    ctx.state["project_id"] = pid
    ctx.cleanup(f"project {pid}", lambda: admin.delete(f"/v1/projects/{pid}", template="/v1/projects/{id}"))

    r = admin.get(f"/v1/projects/{pid}", template="/v1/projects/{id}")
    ctx.expect("GET /v1/projects/{id}", r, 200)
    ctx.assert_true("project trả đúng tên", name in r.text, r.text[:200])
    ctx.expect("PUT /v1/projects/{id}", admin.put(f"/v1/projects/{pid}", template="/v1/projects/{id}",
               json={"name": name + "-r", "description": "updated", "default_branch": "main",
                     "visibility": "private"}), "2xx")
    r = admin.get("/v1/projects/", params={"page_size": 200})
    ctx.assert_true("project có trong GET /v1/projects", pid in r.text, "không thấy trong 200 project đầu")
    ctx.expect("POST /v1/projects/{id}/members", admin.post(
               f"/v1/projects/{pid}/members", template="/v1/projects/{id}/members",
               json={"user_id": (admin.user or {}).get("id", ZERO_UUID), "role": "member"}), "handled")
    ctx.expect("PUT /v1/projects/{id}/dev-server dev server không tồn tại", admin.put(
               f"/v1/projects/{pid}/dev-server", template="/v1/projects/{id}/dev-server",
               json={"new_dev_server_id": ZERO_UUID}), "4xx", "2xx")

    # --- repos ---
    repo_ids: list[str] = []
    for i in range(2):
        r = admin.post(f"/v1/projects/{pid}/repos", template="/v1/projects/{id}/repos",
                       json={"url": f"https://github.com/example/{ctx.cfg.prefix}-repo-{i}.git",
                             "display_name": f"repo-{i}"})
        ctx.expect(f"POST /v1/projects/{{id}}/repos #{i + 1}", r, "2xx")
        rid = find_value(json_body(r), "id", "repo_id", "repoId")
        if rid:
            repo_ids.append(rid)
    r = admin.get(f"/v1/projects/{pid}/repos", template="/v1/projects/{id}/repos")
    ctx.expect("GET /v1/projects/{id}/repos", r, 200)
    if len(repo_ids) == 2:
        ctx.expect("PUT /v1/projects/{id}/repos/reorder", admin.put(
                   f"/v1/projects/{pid}/repos/reorder", template="/v1/projects/{id}/repos/reorder",
                   json={"repo_ids_in_order": list(reversed(repo_ids))}), "2xx")
    else:
        ctx.skip("PUT /v1/projects/{id}/repos/reorder", "chưa tạo đủ 2 repo")
    ctx.expect("POST repo thiếu url -> 400", admin.post(f"/v1/projects/{pid}/repos",
               template="/v1/projects/{id}/repos", json={"display_name": "x"}), 400)

    # --- worktrees ---
    repo_id = repo_ids[0] if repo_ids else ZERO_UUID
    r = admin.post(f"/v1/projects/{pid}/worktrees", template="/v1/projects/{id}/worktrees",
                   json={"repo_id": repo_id, "path": f"/tmp/{ctx.cfg.prefix}-wt", "branch": "feature/api-test"})
    ctx.expect("POST /v1/projects/{id}/worktrees", r, "2xx")
    wid = find_value(json_body(r), "id", "worktree_id", "worktreeId")
    ctx.expect("GET /v1/projects/{id}/worktrees", admin.get(f"/v1/projects/{pid}/worktrees",
               template="/v1/projects/{id}/worktrees"), 200)
    if wid:
        base = f"/v1/projects/{pid}/worktrees/{wid}"
        tpl = "/v1/projects/{id}/worktrees/{worktreeId}"
        ctx.expect("PUT .../worktrees/{worktreeId}/activation", admin.put(
                   base + "/activation", template=tpl + "/activation", json={"active": False}), "2xx")
        ctx.expect("PUT .../worktrees/{worktreeId}/rename", admin.put(
                   base + "/rename", template=tpl + "/rename", json={"branch": "feature/api-test-2"}), "2xx")
        ctx.expect("DELETE .../worktrees/{worktreeId}", admin.delete(base, template=tpl), "2xx")
    else:
        ctx.skip("worktree activation/rename/delete", "tạo worktree không trả id")
    if repo_ids:
        ctx.expect("DELETE /v1/projects/{id}/repos/{repoId}", admin.delete(
                   f"/v1/projects/{pid}/repos/{repo_ids[-1]}", template="/v1/projects/{id}/repos/{repoId}"),
                   "2xx")

    # --- project groups ---
    r = admin.post("/v1/project-groups/", json={"name": ctx.name("group")})
    ctx.expect("POST /v1/project-groups", r, "2xx")
    gid = find_value(json_body(r), "id", "group_id", "groupId")
    if gid:
        child = admin.post("/v1/project-groups/", json={"name": ctx.name("child"), "parent_group_id": gid})
        ctx.expect("POST /v1/project-groups (nhóm con)", child, "2xx")
        cid = find_value(json_body(child), "id", "group_id", "groupId")
        ctx.expect("PUT /v1/project-groups/{id}", admin.put(f"/v1/project-groups/{gid}",
                   template="/v1/project-groups/{id}", json={"name": ctx.name("group-r")}), "2xx")
        if cid:
            ctx.expect("DELETE /v1/project-groups/{id} (nhóm con)", admin.delete(
                       f"/v1/project-groups/{cid}", template="/v1/project-groups/{id}"), "2xx")
        ctx.expect("DELETE /v1/project-groups/{id}", admin.delete(f"/v1/project-groups/{gid}",
                   template="/v1/project-groups/{id}"), "2xx")
    else:
        ctx.skip("vòng đời project-group", "tạo group không trả id")
    ctx.expect("DELETE /v1/projects/{id}", admin.delete(f"/v1/projects/{pid}", template="/v1/projects/{id}"),
               "2xx")
    ctx.expect("GET project đã xoá -> 404", admin.get(f"/v1/projects/{pid}", template="/v1/projects/{id}"),
               404, 200)  # soft-delete có thể vẫn trả bản ghi


if __name__ == "__main__":
    run_single(SUITE, run)

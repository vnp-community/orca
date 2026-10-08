"""
Script chuẩn bị dữ liệu kiểm thử (Test Preparation).
Căn cứ: TR-000-test-requirements.md - Section 4.2 Test Data.
Thực thi toàn bộ quá trình chuẩn bị dữ liệu trước khi chạy kiểm thử (seed database).
"""
from __future__ import annotations

import sys
import json
from check_framework import build_context, Context
from orca_api_session import find_value, json_body

def prepare_users(ctx: Context) -> None:
    print("--- Preparing Users ---")
    admin = ctx.need_admin()
    if not admin:
        print("Lỗi: Cần phiên đăng nhập admin.")
        return

    users = [
        {"email": "admin@test.com", "name": "Admin User", "role": "admin", "password": "Password123!"},
        {"email": "user@test.com", "name": "Standard User", "role": "user", "password": "Password123!"},
        {"email": "deactivated@test.com", "name": "Deactivated User", "role": "user", "password": "Password123!"}
    ]
    for u in users:
        r = admin.post("/v1/auth/users", json=u)
        if r.status_code in (200, 201):
            uid = find_value(json_body(r), "id", "userId", "user_id")
            print(f"  Created user: {u['email']} (ID: {uid})")
            if uid and u["email"] == "deactivated@test.com":
                dr = admin.delete(f"/admin/api/users/{uid}", template="/admin/api/users/{id}")
                if dr.status_code in (200, 201, 204):
                    print(f"    -> Deactivated {u['email']}")
        elif r.status_code == 409:
            print(f"  User {u['email']} already exists.")
        else:
            print(f"  Error creating user {u['email']}: {r.status_code} {r.text}")


def prepare_projects_and_worktrees(ctx: Context) -> None:
    print("\n--- Preparing Projects, Repos & Worktrees ---")
    admin = ctx.need_admin()
    if not admin:
        return
        
    projects = [
        {"name": "proj-backend", "description": "Backend services", "default_branch": "main", "visibility": "private"},
        {"name": "proj-frontend", "description": "Frontend app", "default_branch": "main", "visibility": "private"}
    ]
    
    for p in projects:
        r = admin.post("/v1/projects/", json=p)
        if r.status_code in (200, 201):
            pid = find_value(json_body(r), "id", "project_id", "projectId")
            print(f"  Created project: {p['name']} (ID: {pid})")
            
            # Create Git Repo test-repo
            repo_res = admin.post(f"/v1/projects/{pid}/repos", template="/v1/projects/{id}/repos", json={
                "url": "https://github.com/example/test-repo.git",
                "display_name": "test-repo"
            })
            if repo_res.status_code in (200, 201):
                repo_id = find_value(json_body(repo_res), "id", "repo_id", "repoId")
                print(f"    Created repo: test-repo (ID: {repo_id})")
                
                # Create Worktrees (clean, dirty, conflict simulated by names)
                for state in ["clean", "dirty", "conflict"]:
                    wt_res = admin.post(f"/v1/projects/{pid}/worktrees", template="/v1/projects/{id}/worktrees", json={
                        "repo_id": repo_id,
                        "path": f"/tmp/wt-{p['name']}-{state}",
                        "branch": f"feature/{state}-state"
                    })
                    if wt_res.status_code in (200, 201):
                        wt_id = find_value(json_body(wt_res), "id", "worktree_id", "worktreeId")
                        print(f"      Created worktree: {state} (ID: {wt_id})")
                    else:
                        print(f"      Lỗi tạo worktree {state}: {wt_res.text}")
            else:
                print(f"    Lỗi tạo repo cho {p['name']}: {repo_res.text}")
        elif r.status_code == 409:
            print(f"  Project {p['name']} already exists.")
        else:
            print(f"  Lỗi tạo project {p['name']}: {r.status_code} {r.text}")


def prepare_tasks(ctx: Context) -> None:
    print("\n--- Preparing Tasks (Epic -> Story -> Subtask) ---")
    admin = ctx.need_admin()
    if not admin:
        return
        
    pr_res = admin.post("/v1/projects/", json={"name": "task-project-seed", "visibility": "private"})
    if pr_res.status_code not in (200, 201):
        print("  Lỗi tạo project cho Tasks:", pr_res.text)
        return
        
    pid = find_value(json_body(pr_res), "id", "project_id", "projectId")
    print(f"  Created project for Tasks (ID: {pid})")
    
    epic_res = admin.post("/v1/tasks/", json={"title": "Epic: Platform Launch", "project_id": pid})
    if epic_res.status_code in (200, 201):
        epic_id = find_value(json_body(epic_res), "id", "task_id", "taskId")
        print(f"  Created Epic (ID: {epic_id})")
        
        for i in range(1, 3):
            story_res = admin.post("/v1/tasks/", json={"title": f"Story {i}", "parent_id": epic_id, "project_id": pid})
            if story_res.status_code in (200, 201):
                story_id = find_value(json_body(story_res), "id", "task_id", "taskId")
                print(f"    Created Story {i} (ID: {story_id})")
                
                sub_id_prev = None
                for j in range(1, 4):
                    sub_res = admin.post("/v1/tasks/", json={"title": f"Subtask {i}.{j}", "parent_id": story_id, "project_id": pid})
                    if sub_res.status_code in (200, 201):
                        sub_id = find_value(json_body(sub_res), "id", "task_id", "taskId")
                        print(f"      Created Subtask {i}.{j} (ID: {sub_id})")
                        
                        # Add some dependencies (Subtask i.1 blocks i.2)
                        if j == 2 and sub_id_prev:
                            admin.post(f"/v1/tasks/{sub_id}/edges", template="/v1/tasks/{id}/edges", json={
                                "to_task_id": sub_id_prev, "type": "depends_on"
                            })
                            print(f"        -> Subtask {i}.2 depends on Subtask {i}.1")
                        
                        sub_id_prev = sub_id
            else:
                print(f"    Lỗi tạo Story {i}: {story_res.text}")
    else:
        print(f"  Lỗi tạo Epic: {epic_res.text}")


def prepare_ai_providers(ctx: Context) -> None:
    print("\n--- Preparing AI Providers ---")
    print("  Giả lập credentials (có thể mở rộng gọi API /v1/ai-providers nếu endpoint tồn tại).")
    # TBD: Implementation for AI Providers seeding based on actual API definitions.


def prepare_workflow_templates(ctx: Context) -> None:
    print("\n--- Preparing Workflow Templates ---")
    admin = ctx.need_admin()
    if not admin:
        return
        
    dag = {"steps": [{"id": "step1", "type": "agent", "name": "Build"}]}
    templates = [
        {"name": "company-level template", "dag_json": json.dumps(dag), "scope": "company"},
        {"name": "team-level child", "dag_json": json.dumps(dag), "scope": "team"},
        {"name": "personal override", "dag_json": json.dumps(dag), "scope": "personal"}
    ]
    
    for t in templates:
        r = admin.post("/v1/workflows/templates", json=t)
        if r.status_code in (200, 201):
            tid = find_value(json_body(r), "id", "template_id", "templateId")
            print(f"  Created workflow template: {t['name']} (ID: {tid})")
        else:
            print(f"  Lỗi tạo workflow template {t['name']}: {r.text}")


def main() -> None:
    print("==================================================")
    print("Bắt đầu chuẩn bị môi trường kiểm thử (TR-000 Test Data)")
    print("==================================================")
    
    ctx = build_context(login=True)
    if not ctx.cfg.has_admin_credentials:
        print("Lỗi: Cần cấu hình ORCA_ADMIN_EMAIL và ORCA_ADMIN_PASSWORD trong tests/shared/.env")
        sys.exit(1)
        
    if not ctx.cfg.allow_writes:
        print("Lỗi: Cần cấu hình ORCA_ALLOW_WRITES=true trong tests/shared/.env")
        sys.exit(1)
        
    prepare_users(ctx)
    prepare_projects_and_worktrees(ctx)
    prepare_tasks(ctx)
    prepare_ai_providers(ctx)
    prepare_workflow_templates(ctx)
    
    print("\nHoàn tất quá trình chuẩn bị dữ liệu kiểm thử!")


if __name__ == "__main__":
    main()

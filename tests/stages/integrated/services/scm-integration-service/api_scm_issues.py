"""SCM (/v1/scm/*, webhook) và Issue tracking (/v1/issues/*).

Spec: specs/backend-go/tdd/services/scm-integration-service.md, issue-tracking-service.md.
Không có token GitHub/Jira thật nên các lời gọi gọi nhà cung cấp chỉ kiểm tra gateway xử lý
(không 5xx/501); các lỗi xác thực của provider (401/403 do auth-status) được chấp nhận ở bước đó.
"""
from __future__ import annotations
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))

import json
import uuid

from check_framework import Context, run_single

SUITE = "scm+issues"
ZERO_UUID = "00000000-0000-0000-0000-000000000000"


def run(ctx: Context) -> None:
    admin = ctx.need_admin()
    if admin is None:
        return

    # ---------------- SCM ----------------
    q = {"provider": "github", "repo": "octocat/Hello-World"}
    ctx.expect("GET /v1/scm/auth-status", admin.get("/v1/scm/auth-status", params={"provider": "github"}), 200)
    ctx.expect("GET /v1/scm/rate-limit", admin.get("/v1/scm/rate-limit", params={"provider": "github"}),
               200, "handled")
    ctx.expect("GET /v1/scm/issues", admin.get("/v1/scm/issues", params={**q, "state": "open"}),
               "no5xx", 401, 403)
    ctx.expect("GET /v1/scm/issues thiếu provider -> 400", admin.get("/v1/scm/issues"), 400)
    ctx.expect("GET /v1/scm/issues/{number}/comments", admin.get(
               "/v1/scm/issues/1/comments", template="/v1/scm/issues/{number}/comments", params=q),
               "no5xx", 401, 403)
    ctx.expect("GET /v1/scm/pull-requests", admin.get("/v1/scm/pull-requests", params=q), "no5xx", 401, 403)

    if not ctx.need_writes("scm writes"):
        return
    ctx.expect("POST /v1/scm/pull-requests thiếu title -> 400", admin.post(
               "/v1/scm/pull-requests", json={"provider": "github", "repo": q["repo"]}), 400)
    ctx.expect("POST /v1/scm/pull-requests (chưa kết nối provider)", admin.post(
               "/v1/scm/pull-requests", json={**{k: v for k, v in q.items()}, "title": "api test",
                                              "body": "x", "head_branch": "api-test", "base_branch": "main",
                                              "request_id": str(uuid.uuid4())}), "no5xx", 401, 403)
    ctx.expect("POST /v1/scm/oauth/start", admin.post(
               "/v1/scm/oauth/start", json={"provider": "github", "redirect_uri": "https://example.test/cb"}),
               "handled", 200)
    ctx.expect("POST /v1/scm/oauth/complete state sai", admin.post(
               "/v1/scm/oauth/complete", json={"provider": "github", "code": "bogus", "state": "bogus",
                                               "redirect_uri": "https://example.test/cb"}), "4xx")
    ctx.expect("POST /v1/scm/oauth/revoke", admin.post("/v1/scm/oauth/revoke", json={"provider": "github"}),
               "handled", 200)
    ctx.expect("POST /v1/scm/pull-requests/{prNumber}/reviews", admin.post(
               "/v1/scm/pull-requests/1/reviews", template="/v1/scm/pull-requests/{prNumber}/reviews",
               json={"repoId": ZERO_UUID, "provider": "github", "reviewType": "comment", "summary": "api test"}),
               "4xx", "handled")
    ctx.expect("POST reviews prNumber không phải số -> 400", admin.post(
               "/v1/scm/pull-requests/abc/reviews", template="/v1/scm/pull-requests/{prNumber}/reviews",
               json={"repoId": ZERO_UUID, "provider": "github", "reviewType": "comment"}), 400)

    # Webhook công khai: chữ ký sai phải bị từ chối, không crash.
    payload = json.dumps({"zen": "api test"})
    ctx.expect("POST /v1/scm/webhooks/github chữ ký sai", ctx.anon.post(
               "/v1/scm/webhooks/github", template="/v1/scm/webhooks/{provider}", data=payload,
               headers={"Content-Type": "application/json", "X-GitHub-Event": "ping",
                        "X-Hub-Signature-256": "sha256=" + "0" * 64}, auth=False), 400, 401, 403, 404)
    ctx.expect("POST /v1/scm/webhooks/{provider} provider lạ", ctx.anon.post(
               "/v1/scm/webhooks/unknown-scm", template="/v1/scm/webhooks/{provider}", data=payload,
               headers={"Content-Type": "application/json"}, auth=False), "4xx")

    # ---------------- Issue tracking ----------------
    ctx.expect("GET /v1/issues", admin.get("/v1/issues/", params={"provider": "jira", "project_key": "API"}),
               "no5xx", 401, 403)
    ctx.expect("GET /v1/issues provider sai -> 400", admin.get("/v1/issues/", params={"provider": "bogus"}),
               400)
    ctx.expect("POST /v1/issues (chưa kết nối provider)", admin.post(
               "/v1/issues/", json={"provider": "jira", "project_key": "API", "title": "api test",
                                    "description": "x"}), "no5xx", 401, 403)
    ctx.expect("POST /v1/issues thiếu title -> 400", admin.post(
               "/v1/issues/", json={"provider": "jira", "project_key": "API"}), 400)
    ctx.expect("POST /v1/issues/link issue/task không tồn tại", admin.post(
               "/v1/issues/link", json={"issue_id": ZERO_UUID, "task_id": ZERO_UUID}), "4xx")


if __name__ == "__main__":
    run_single(SUITE, run)

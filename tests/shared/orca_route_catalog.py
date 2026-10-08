"""Danh mục toàn bộ route HTTP của api-gateway (nguồn: internal/adapter/httpgateway/*_routes.go).

PUBLIC: không đi qua authMiddleware. Còn lại yêu cầu session cookie (hoặc bearer).
`verify_against_go_source()` phát hiện lệch giữa danh mục này và mã Go.
"""
from __future__ import annotations

import re
from pathlib import Path

Route = tuple[str, str]  # (METHOD, path-template)

PUBLIC: list[Route] = [
    ("POST", "/auth/local"), ("POST", "/auth/cli-token"), ("GET", "/auth/me"),
    ("POST", "/auth/logout"), ("POST", "/auth/refresh"), ("GET", "/auth/config"),
    ("GET", "/auth/sso/{provider}"), ("GET", "/auth/callback"),
    ("GET", "/api/trace-stream"),
    ("GET", "/api/vapid-public-key"), ("POST", "/api/push-subscribe"), ("POST", "/api/push-unsubscribe"),
    ("POST", "/v1/scm/webhooks/{provider}"),
    ("POST", "/v1/paired-devices/pairing-sessions/{token}/complete"),
]

_GROUPS: dict[str, list[str]] = {
    "": ["POST /v1/worktrees", "POST /v1/scm/pull-requests/{prNumber}/reviews"],
    "/admin/api": [
        "GET /stats", "GET /users", "POST /users", "PATCH /users/{id}", "DELETE /users/{id}",
        "GET /sessions", "DELETE /sessions/{sessionId}", "DELETE /users/{userId}/sessions",
        "GET /policies", "POST /policies", "PUT /policies/{id}", "DELETE /policies/{id}",
        "GET /audit", "GET /audit/export"],
    "/v1/auth": [
        "POST /users", "GET /users", "PUT /users/{id}/role", "POST /sessions/{id}/revoke", "GET /audit-log"],
    "/v1/auth/cli-tokens": ["POST /", "GET /", "DELETE /{id}"],
    "/v1/users/me/paired-devices": ["POST /pairing-sessions", "GET /", "DELETE /{deviceId}"],
    "/v1/ai-providers": [
        "POST /accounts", "GET /resolve", "POST /accounts/{id}/rotate-key", "GET /usage-today"],
    "/v1/usage": ["GET /daily", "POST /sessions", "GET /sessions"],
    "/v1/notifications": [
        "POST /subscribe", "GET /vapid-public-key", "GET /", "POST /{id}/read",
        "POST /read-all", "GET /unread-count", "GET /stream"],
    "/v1/orchestration": [
        "POST /dispatch-contexts", "POST /gates", "POST /gates/{id}/resolve", "PUT /tasks/{id}/status"],
    "/v1/git": [
        "GET /status", "GET /diff", "POST /commit", "POST /push", "POST /pull", "POST /commit-message"],
    "/v1/projects": [
        "POST /", "GET /", "GET /{id}", "PUT /{id}", "DELETE /{id}", "POST /{id}/members",
        "PUT /{id}/dev-server", "POST /{id}/repos", "GET /{id}/repos", "PUT /{id}/repos/reorder",
        "DELETE /{id}/repos/{repoId}", "POST /{id}/worktrees", "DELETE /{id}/worktrees/{worktreeId}",
        "GET /{id}/worktrees", "PUT /{id}/worktrees/{worktreeId}/activation",
        "PUT /{id}/worktrees/{worktreeId}/rename"],
    "/v1/project-groups": ["POST /", "GET /", "PUT /{id}", "DELETE /{id}"],
    "/v1/automations": [
        "POST /", "GET /", "PATCH /{id}", "DELETE /{id}", "POST /{id}/run", "GET /{id}/runs",
        "POST /{id}/trigger"],
    "/v1/scm": [
        "GET /issues", "GET /issues/{number}/comments", "POST /pull-requests", "GET /pull-requests",
        "GET /rate-limit", "GET /auth-status", "POST /oauth/start", "POST /oauth/complete",
        "POST /oauth/revoke"],
    "/v1/annotations": [
        "POST /", "GET /", "PUT /{id}", "PATCH /{id}", "DELETE /{id}", "POST /mark-sent",
        "POST /send-to-agent"],
    "/v1/tenants": [
        "POST /companies", "GET /validate", "POST /departments", "PUT /users/{id}/department",
        "GET /profile", "POST /teams", "POST /teams/{id}/members", "GET /teams/{id}/members",
        "POST /companies/{id}/email-domains", "GET /companies/{id}/email-domains",
        "DELETE /email-domains/{domain}"],
    "/v1/infra": [
        "POST /dev-servers", "GET /dev-servers", "POST /connections/resolve", "POST /connections",
        "POST /ssh-targets", "GET /health", "POST /workspaces/scan-ports", "POST /relay"],
    "/v1/worktrees/{worktreeId}/agent": ["GET /status", "POST /wait", "POST /send", "GET /snapshot"],
    "/v1/issues": ["GET /", "POST /", "POST /link"],
    "/v1/workflows": [
        "POST /templates", "GET /templates", "GET /templates/resolve", "POST /executions",
        "GET /executions/{id}", "POST /executions/{id}/pause", "POST /executions/{id}/resume",
        "POST /executions/{id}/cancel", "POST /executions/{id}/steps/adhoc",
        "GET /{id}/active-executions"],
    "/v1/tasks": [
        "POST /", "GET /{id}", "POST /{id}/edges", "POST /{id}/grants", "GET /{id}/permission",
        "POST /{id}/execute", "GET /{id}/active-executions", "GET /{id}/subtree",
        "POST /{id}/progress:recalculate", "POST /{id}/comments", "GET /{id}/comments"],
}


def _join(prefix: str, sub: str) -> str:
    if sub == "/":
        return prefix or "/"
    return prefix + sub


def authenticated_routes() -> list[Route]:
    out: list[Route] = []
    for prefix, items in _GROUPS.items():
        for item in items:
            method, _, sub = item.partition(" ")
            out.append((method, _join(prefix, sub)))
    return out


def all_routes() -> list[Route]:
    return PUBLIC + authenticated_routes()


def _norm(path: str) -> str:
    return re.sub(r"\{[^}]*\}", "{}", path).rstrip("/") or "/"


def normalize(route: Route) -> Route:
    return route[0], _norm(route[1])


_GATEWAY_DIR = (Path(__file__).resolve().parents[2] / "backend-go" / "services" / "api-gateway"
                / "internal" / "adapter" / "httpgateway")
_CALL = re.compile(r'\b(?:r|sub|mux|authed)(?:\.With\([^)]*\))?\.(Get|Post|Put|Patch|Delete)\("([^"]+)"')
_ROUTE = re.compile(r'\br\.Route\("([^"]+)"')


def discover_go_routes() -> set[Route] | None:
    """Phân tích tĩnh các *_routes.go; None nếu không có mã nguồn."""
    if not _GATEWAY_DIR.is_dir():
        return None
    found: set[Route] = set()
    for f in sorted(_GATEWAY_DIR.glob("*_routes.go")):
        if f.name.endswith("_test.go") or f.name == "stub_routes.go":
            continue
        prefix = ""
        for line in f.read_text(encoding="utf-8").splitlines():
            m = _ROUTE.search(line)
            if m:
                prefix = m.group(1)
                continue
            m = _CALL.search(line)
            if not m:
                continue
            method, path = m.group(1).upper(), m.group(2)
            if path.startswith("/v1/") or path.startswith("/auth/") or path.startswith("/api/") \
                    or path.startswith("/admin/"):
                found.add((method, _norm(path)))
            else:
                found.add((method, _norm(_join(prefix, path))))
    return found


def verify_against_go_source() -> tuple[set[Route], set[Route]] | None:
    """Trả về (thiếu_trong_danh_mục, thừa_trong_danh_mục) hoặc None."""
    go = discover_go_routes()
    if go is None:
        return None
    cat = {normalize(r) for r in all_routes()}
    # SSE/WS không nằm trong *_routes.go dạng sub.Get; bỏ khỏi so sánh hai chiều.
    ignore = {("GET", "/api/trace-stream"), ("GET", "/v1/notifications/stream")}
    return go - cat - ignore, cat - go - ignore


if __name__ == "__main__":
    res = verify_against_go_source()
    if res is None:
        print("Không tìm thấy mã nguồn Go — bỏ qua.")
    else:
        missing, extra = res
        print("Thiếu trong danh mục:", sorted(missing))
        print("Thừa trong danh mục:", sorted(extra))

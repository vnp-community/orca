"""PLT: the stack is up, serves the UI, authenticates, and the gateway reaches the request-service."""
from __future__ import annotations

import re
import shutil
import subprocess

from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked
from orca_session import OrcaSession, http_request
from ws_rpc_client import RpcError, WsRpcClient

CORE_CONTAINERS = [
    "orca-go-postgres", "orca-go-nats", "orca-go-vault", "orca-go-api-gateway", "orca-go-auth", "orca-go-tenant", "orca-go-project",
    "orca-go-task", "orca-go-workflow", "orca-go-request", "orca-go-infra-fleet", "orca-go-ai-provider", "orca-go-frontend",
]


def _docker_ps() -> dict[str, str]:
    if not shutil.which("docker"):
        raise Blocked("docker CLI not available")
    out = subprocess.run(["docker", "ps", "-a", "--filter", "name=orca-go", "--format", "{{.Names}}|{{.Status}}"],
                         capture_output=True, text=True, timeout=20)
    if out.returncode != 0:
        raise Blocked(f"docker ps failed: {out.stderr.strip()[:120]}")
    return dict(line.split("|", 1) for line in out.stdout.splitlines() if "|" in line)


def run(ctx: AcceptanceContext) -> None:
    rec, cfg = ctx.rec, ctx.cfg
    page: dict = {}

    def frontend_root():
        r = http_request("GET", cfg.frontend_url + "/")
        assert r.status == 200, f"GET / -> {r.status} (a 403 here means nginx mounts a stale dist directory; restart the frontend container)"
        assert "<title>" in r.text, "index page has no <title>"
        page["html"] = r.text
    rec.run("PLT-01", "Frontend serves the web app at /", frontend_root)

    def service_worker():
        r = http_request("GET", cfg.frontend_url + "/service-worker.js")
        assert r.status == 200, f"service-worker.js -> {r.status}"
    rec.run("PLT-02", "Service worker script is served", service_worker)

    def built_asset():
        m = re.search(r'src="\.?/?(assets/[^"]+\.js)"', page.get("html", ""))
        assert m, "index page references no assets/*.js bundle"
        r = http_request("GET", f"{cfg.frontend_url}/{m.group(1)}")
        assert r.status == 200 and len(r.body) > 1000, f"bundle {m.group(1)} -> {r.status}, {len(r.body)} bytes"
    rec.run("PLT-03", "A referenced JS bundle is reachable", built_asset)

    def login_ok():
        assert cfg.admin_email and cfg.admin_password, "no admin credentials (deploy/dev/.env BOOTSTRAP_ADMIN_* or ORCA_E2E_ADMIN_*)"
        r = ctx.session.login()
        assert r.status == 200, f"POST /auth/local -> {r.status}"
        assert ctx.session.user.get("role") == "admin", f"bootstrap user role is {ctx.session.user.get('role')!r}, expected admin"
    rec.run("PLT-04", "Admin can log in with local credentials", login_ok)

    def login_wrong_password():
        r = http_request("POST", cfg.gateway_url + "/auth/local", {"email": cfg.admin_email, "password": "wrong-" + cfg.run_prefix})
        assert r.status == 401, f"wrong password -> {r.status}, expected 401"
    rec.run("PLT-05", "Wrong password is rejected with 401", login_wrong_password)

    def me_endpoint():
        r = ctx.session.get_authed("/auth/me")
        assert r.status == 200, f"GET /auth/me -> {r.status}"
        assert r.json().get("email") == cfg.admin_email, "session belongs to another user"
    rec.run("PLT-06", "Session cookie identifies the logged-in user", me_endpoint)

    def ws_requires_auth():
        anon = WsRpcClient(cfg.ws_url, origin=cfg.frontend_url)
        try:
            anon.connect()
        except ConnectionError:
            return  # refused at the handshake: acceptable
        try:
            anon.call("request.flowStatus", timeout=8)
        except (RpcError, TimeoutError):
            return
        finally:
            anon.close()
        raise AssertionError("an anonymous WebSocket was served a request.* channel")
    rec.run("PLT-07", "Anonymous WebSocket cannot use request.* channels", ws_requires_auth)

    def containers_running():
        ps = _docker_ps()
        missing = [n for n in CORE_CONTAINERS if not ps.get(n, "").startswith("Up")]
        assert not missing, "core containers not running: " + ", ".join(missing)
    rec.run("PLT-08", "Core containers are running", containers_running)

    def no_crash_loop():
        ps = _docker_ps()
        bad = {n: s for n, s in ps.items() if ("Restarting" in s or s.startswith("Exited (1")) and n != "orca-go-vault-init"}
        assert not bad, f"crash-looping or failed containers: {bad}"
    rec.run("PLT-09", "No container is crash-looping", no_crash_loop)

    def gateway_reaches_request_service():
        try:
            reply = ctx.ws.call("request.flowStatus")
        except RpcError as exc:
            raise AssertionError(f"request.flowStatus failed: {exc.message[:200]} "
                                 "(REQUEST_FORBIDDEN here means the internal token pair or the OPA policy mount is broken)")
        assert "enabled" in reply, f"unexpected reply {reply!r}"
        ctx.state["request_flow_enabled"] = bool(reply["enabled"])
    rec.run("PLT-10", "Gateway reaches request-service (internal token + OPA policy)", gateway_reaches_request_service)

    def workflow_reachable():
        try:
            ctx.ws.call("workflow.template.list", {})
        except RpcError as exc:
            raise AssertionError(f"workflow.template.list failed: {exc.message[:200]}")
    rec.run("PLT-11", "Gateway reaches workflow-service", workflow_reachable)

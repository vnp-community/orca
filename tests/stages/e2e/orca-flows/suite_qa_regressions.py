"""QA: regression checks for the 2026-10-16 QA bugs (specs/frontend/bugs/qa-2026-10-16).

Each test names the bug it pins. A test is RED on a stack that still has the bug and GREEN once the fix is deployed, so QA
can run `python3 run_acceptance.py devserver qa` before and after a fix. Nothing here uses AI, and no secret is printed.

  QA-TERM-xx  BUG-QA-01  a terminal opens in the worktree's directory, never silently in $HOME
  QA-ENG-xx   BUG-QA-02  bug and task run on the native engine unless the project says otherwise
  QA-GH-xx    BUG-QA-03  the GitHub task source resolves a repo from the UI's `id:<repoId>` selector and explains a missing login
  QA-JIRA-xx  BUG-QA-04  a Jira Server/Data Center site accepts email + PAT (Bearer); QA-JIRA-LIVE-01 uses the .env site (opt-in)
"""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import request_flow_driver as drv
from acceptance_context import AcceptanceContext
from acceptance_recorder import Blocked, Skipped
from ws_rpc_client import RpcError

REPO_ROOT = Path(__file__).resolve().parents[4]


def _git(path: str, *args: str) -> None:
    subprocess.run(["git", "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", *args], cwd=path, check=True, capture_output=True)


def _repo_with_remote(prefix: str, remote_url: str) -> str:
    if not shutil.which("git"):
        raise Blocked("git not installed on the test runner")
    path = tempfile.mkdtemp(prefix=f"{prefix}-qa-repo-")
    _git(path, "init", "-q", "-b", "main")
    _git(path, "commit", "-q", "--allow-empty", "-m", "init")
    _git(path, "remote", "add", "origin", remote_url)
    return path


def _same_dir(a: str, b: str) -> bool:
    return os.path.realpath(a) == os.path.realpath(b)


def _repo_id_of(ws, project_id: str) -> str:
    repos = ws.call("repo.list", {}).get("repos", [])
    mine = [r for r in repos if r.get("projectId") == project_id]
    assert mine, f"project {project_id} has no repo in repo.list"
    return mine[0]["id"]


def _close_quietly(ws, pty_id: str) -> None:
    try:
        ws.call("terminal.close", {"ptyId": pty_id})
    except Exception:  # noqa: BLE001 - the session may already be gone
        pass


# ── a Jira Server/Data Center stand-in: accepts ONLY `Authorization: Bearer <PAT>` on /rest/api/2, has no /rest/api/3 ──────

class _FakeJiraServer:
    PAT = "qa-fake-pat-not-a-secret"

    def __init__(self) -> None:
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_a):  # quiet
                return

            def _send(self, code: int, body: object) -> None:
                data = json.dumps(body).encode()
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):  # noqa: N802
                path = self.path.split("?", 1)[0]
                if path.startswith("/rest/api/3/"):
                    return self._send(404, {"errorMessages": ["Cloud-only API version"], "errors": {}})
                if self.headers.get("Authorization") != f"Bearer {outer.PAT}":
                    outer.rejected += 1
                    return self._send(401, {"errorMessages": [], "errors": {}})
                if path == "/rest/api/2/serverInfo":
                    return self._send(200, {"deploymentType": "Server", "version": "9.12.0"})
                if path == "/rest/api/2/myself":
                    return self._send(200, {"name": "qa", "key": "qa", "accountId": "qa", "displayName": "QA Fake", "emailAddress": "qa@example.com"})
                if path == "/rest/api/2/search":
                    return self._send(200, {"startAt": 0, "maxResults": 50, "total": 1, "issues": [{
                        "key": "QA-1", "fields": {
                            "summary": "Fake Server issue", "status": {"name": "To Do", "statusCategory": {"key": "new"}},
                            "project": {"key": "QA", "name": "QA"}, "issuetype": {"name": "Task"}, "priority": {"name": "Medium"}, "labels": []}}]})
                return self._send(404, {"errorMessages": ["not found"], "errors": {}})

        self.rejected = 0
        self.httpd = ThreadingHTTPServer(("0.0.0.0", 0), Handler)
        self.port = self.httpd.server_address[1]
        threading.Thread(target=self.httpd.serve_forever, daemon=True).start()

    def url(self) -> str:
        # The gateway runs in a container; the stand-in listens on the host.
        return f"http://host.docker.internal:{self.port}"

    def stop(self) -> None:
        self.httpd.shutdown()


def _read_dotenv(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if path.is_file():
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            m = re.match(r"^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$", line)
            if m:
                values[m.group(1)] = m.group(2).strip().strip("'\"")
    return values


def run(ctx: AcceptanceContext) -> None:
    rec, ws, cfg = ctx.rec, ctx.ws, ctx.cfg
    dev_server_id = ctx.state.get("dev_server_id")
    project_id = ctx.state.get("project_id")
    has_ds = bool(dev_server_id and project_id and ctx.state.get("project_has_dev_server", True))

    # ───────────────────────── BUG-QA-01: terminal working directory ─────────────────────────
    def need_dev_server() -> None:
        if not has_ds:
            raise Blocked("needs the devserver suite (an approved, connected dev server and a project bound to it)")

    def create_terminal(**params) -> dict:
        reply = ws.call("terminal.create", {"connectionId": dev_server_id, "cols": 80, "rows": 24, **params})
        pty = reply["terminal"]["ptyId"]
        ctx.on_cleanup(f"close terminal {pty}", lambda p=pty: _close_quietly(ws, p))
        return reply["terminal"]

    def term_explicit_cwd():
        need_dev_server()
        repo = ctx.state["repo_path"]
        got = create_terminal(cwd=repo)
        assert _same_dir(got["cwd"], repo), f"terminal.create(cwd={repo!r}) opened in {got['cwd']!r}"
    rec.run("QA-TERM-01", "terminal.create with an explicit cwd opens exactly there (guard for the agent)", term_explicit_cwd)

    def term_from_worktree_selector():
        need_dev_server()
        repo = ctx.state["repo_path"]
        repo_id = _repo_id_of(ws, project_id)
        # What the web UI sends today: a worktree selector and no cwd (BUG-QA-01 step 4).
        got = create_terminal(worktree=f"id:{repo_id}::{repo}")
        assert _same_dir(got["cwd"], repo), (
            f"a terminal for worktree selector id:{repo_id}::{repo} opened in {got['cwd']!r} (home directory?). "
            "The gateway ignores `worktree`, and the web transport does not send `cwd` (BUG-QA-01)")
    rec.run("QA-TERM-02", "terminal.create from a worktree selector (no cwd) opens in that worktree", term_from_worktree_selector)

    def term_not_silent_on_bad_cwd():
        need_dev_server()
        bad = f"/nonexistent/{cfg.run_prefix}"
        try:
            got = create_terminal(cwd=bad)
        except RpcError as exc:
            assert exc.has_code("INFRA_TERMINAL_CWD_NOT_FOUND") or "cwd" in exc.message.lower(), f"unclear error for a missing cwd: {exc.message[:200]}"
            return
        assert got.get("cwdFellBack") is True, (
            f"cwd {bad!r} does not exist and the terminal opened in {got['cwd']!r} without saying it fell back "
            "(expected an error INFRA_TERMINAL_CWD_NOT_FOUND or cwdFellBack=true)")
    rec.run("QA-TERM-03", "A cwd that does not exist is reported, not silently replaced by $HOME", term_not_silent_on_bad_cwd)

    # ───────────────────────── BUG-QA-02: engine of bug and task ─────────────────────────
    def engine_of(typ: str, size: str) -> str:
        if not project_id:
            raise Blocked("no project (see DEV-04)")
        req = drv.create(ctx, f"qa engine {typ}", body="qa engine check")["request"]
        cur = drv.confirm(ctx, drv.settle_after_classify(ctx, req["id"]) and drv.get(ctx, req["id"]), typ, size)
        return drv.get(ctx, cur["id"]).get("engine", "")

    def engine_default(typ: str, size: str, want: str) -> None:
        got = engine_of(typ, size)
        assert got == want, (
            f"a {typ} Request in a project with no engine setting reports engine {got!r}, expected {want!r} "
            "(REQUEST_DEFAULT_ENGINE should only move change_request and refactor; BUG-QA-02)")
    rec.run("QA-ENG-01", "bug runs on the native engine under the deployment default", lambda: engine_default("bug", "M", "native"))
    rec.run("QA-ENG-02", "task runs on the native engine under the deployment default", lambda: engine_default("task", "S", "native"))

    def engine_explicit_openspec():
        if not project_id:
            raise Blocked("no project (see DEV-04)")
        cur = ws.call("request.engineSettings", {"projectId": project_id})
        before = {"engine": cur.get("engine") or "native", "solutionAgent": bool(cur.get("solutionAgent")), "executionMode": cur.get("executionMode") or "step_by_step"}
        stored = ws.call("request.engineSettingsSet", {"projectId": project_id, "engine": "openspec", "openspecMinVersion": cur.get("openspecMinVersion", ""),
                                                        "solutionAgent": False, "executionMode": before["executionMode"], "expectedVersion": cur.get("version", 0)})
        ctx.on_cleanup("restore project engine settings", lambda: ws.call("request.engineSettingsSet", {
            "projectId": project_id, "engine": before["engine"], "solutionAgent": before["solutionAgent"], "executionMode": before["executionMode"],
            "expectedVersion": stored.get("version", 0)}))
        bug, task = engine_of("bug", "M"), engine_of("task", "S")
        assert (bug, task) == ("openspec", "native"), f"with the project set to openspec: bug={bug!r} task={task!r}, expected openspec/native"
    rec.run("QA-ENG-03", "A project that explicitly chooses openspec moves bug (light profile) but never task", engine_explicit_openspec)

    # ───────────────────────── BUG-QA-03: GitHub task source ─────────────────────────
    gh: dict = {}

    def gh_project():
        need_dev_server()
        if gh:
            return
        path = _repo_with_remote(cfg.run_prefix, "git@github.com:qa-org/qa-repo.git")
        proj = ws.call("project.create", {"name": f"{cfg.run_prefix}-gh", "devServerId": dev_server_id, "repoPath": path, "defaultBranch": "main", "visibility": "private"})
        ctx.on_cleanup(f"delete project {proj['id']}", lambda: ws.call("project.delete", {"projectId": proj["id"]}))
        gh.update(project=proj["id"], repo_id=_repo_id_of(ws, proj["id"]))

    def list_items(repo: str, **extra) -> dict:
        return ws.call("github.listWorkItems", {"repo": repo, "limit": 5, "query": "is:issue is:open", "noCache": True, **extra})

    def gh_selector_resolves():
        gh_project()
        out = list_items(f"id:{gh['repo_id']}", repoId=gh["repo_id"])
        err = (out.get("errors") or {}).get("issues")
        assert not (err and "WORKTREE_REPO_NOT_FOUND" in err.get("message", "")), (
            f"github.listWorkItems with repo 'id:<repoId>' answered {err['message'][:160]!r}: the gateway passes the 'id:' prefix on to project-service (BUG-QA-03 layer 1)")
        src = (out.get("sources") or {}).get("issues")
        assert src and (src["owner"], src["repo"]) == ("qa-org", "qa-repo"), f"the origin remote was not resolved to qa-org/qa-repo: sources={out.get('sources')!r}"
    rec.run("QA-GH-01", "github.listWorkItems resolves owner/repo from the UI's 'id:<repoId>' selector", gh_selector_resolves)

    def gh_bare_id_still_works():
        gh_project()
        out = list_items(gh["repo_id"])
        src = (out.get("sources") or {}).get("issues")
        assert src and (src["owner"], src["repo"]) == ("qa-org", "qa-repo"), f"a bare repo id no longer resolves: errors={out.get('errors')!r}"
    rec.run("QA-GH-02", "A bare repoId (older clients, MCP) still resolves", gh_bare_id_still_works)

    def gh_missing_login_is_explained():
        gh_project()
        out = list_items(f"id:{gh['repo_id']}", repoId=gh["repo_id"])
        err = (out.get("errors") or {}).get("issues")
        try:
            ws.call("github.rateLimit", {})
            raise Skipped("a GitHub credential exists for this user, so the missing-login path cannot be checked")
        except RpcError as exc:
            if not exc.has_code("SCM_CREDENTIAL_NOT_FOUND"):
                raise Skipped(f"unexpected rateLimit answer: {exc.message[:120]}") from exc
        assert err and err.get("type") == "auth_required", (
            f"without a GitHub login the issue list reports {err!r}; expected type 'auth_required' with a message that tells the user to connect GitHub")
        assert "connect" in err.get("message", "").lower(), f"the message gives no next step: {err.get('message')!r}"
    rec.run("QA-GH-03", "No GitHub credential → 'auth_required' with a next step (not 'not_found'/'unknown')", gh_missing_login_is_explained)

    # ───────────────────────── BUG-QA-04: Jira Server/Data Center with email + PAT ─────────────────────────
    def jira_needs_free_slot() -> None:
        status = ws.call("jira.status", {})
        if status.get("connected"):
            raise Blocked("a Jira connection already exists for this user; disconnect it first so this test cannot overwrite it")

    fake: dict = {}

    def jira_fake_server() -> _FakeJiraServer:
        if "srv" not in fake:
            fake["srv"] = _FakeJiraServer()
            ctx.on_cleanup("stop the fake Jira server", fake["srv"].stop)
        return fake["srv"]

    def jira_connect_email_and_pat():
        jira_needs_free_slot()
        srv = jira_fake_server()
        out = ws.call("jira.connect", {"siteUrl": srv.url(), "email": "qa@example.com", "apiToken": srv.PAT})
        if out.get("ok"):
            ctx.on_cleanup("disconnect the fake Jira site", lambda: ws.call("jira.disconnect", {}))
        assert out.get("ok") is True, (
            f"jira.connect with email + PAT to a Server/Data Center site failed: {str(out.get('error'))[:200]}. "
            f"The adapter sent Basic (email:PAT) {srv.rejected} time(s); a Server/DC PAT only works as Bearer (BUG-QA-04)")
    rec.run("QA-JIRA-01", "jira.connect with email + PAT works against a Jira Server/Data Center site (Bearer)", jira_connect_email_and_pat)

    def jira_lists_issues_with_api_v2():
        status = ws.call("jira.status", {})
        if not status.get("connected"):
            raise Blocked("QA-JIRA-01 did not connect")
        issues = ws.call("jira.listIssues", {"limit": 5})
        assert [i.get("key") for i in issues] == ["QA-1"], f"issues from /rest/api/2/search: {issues!r}"
    rec.run("QA-JIRA-02", "After connecting, jira.listIssues reads /rest/api/2/search (not the Cloud-only v3)", jira_lists_issues_with_api_v2)

    def jira_wrong_token_is_a_clear_error():
        if ws.call("jira.status", {}).get("connected"):
            ws.call("jira.disconnect", {})
        srv = jira_fake_server()
        out = ws.call("jira.connect", {"siteUrl": srv.url(), "email": "qa@example.com", "apiToken": "wrong-token"})
        assert out.get("ok") is False, "jira.connect accepted a token the site rejects"
        assert re.search(r"401|reject|unauthor|denied|PAT", str(out.get("error")), re.I), f"unclear error: {str(out.get('error'))[:200]}"
        assert not ws.call("jira.status", {}).get("connected"), "a failed connect left a 'connected' row"
    rec.run("QA-JIRA-03", "A rejected token gives a clear error and no half-connected state", jira_wrong_token_is_a_clear_error)

    def jira_live():
        if os.environ.get("ORCA_E2E_JIRA_LIVE") != "1":
            raise Skipped("opt-in: set ORCA_E2E_JIRA_LIVE=1 to connect to the site in .env (JR_URL, JR_EMAIL, JR_PAT_TOKEN). The token is never printed")
        env = {**_read_dotenv(REPO_ROOT / ".env"), **{k: v for k, v in os.environ.items() if k.startswith(("JR_", "JIRA_"))}}
        site, email = env.get("JR_URL") or env.get("JIRA_SITE_URL"), env.get("JR_EMAIL") or env.get("JIRA_EMAIL")
        token = env.get("JR_PAT_TOKEN") or env.get("JIRA_PAT_TOKEN")
        if not (site and token):
            raise Blocked("JR_URL / JR_PAT_TOKEN are not set in .env")
        jira_needs_free_slot()
        out = ws.call("jira.connect", {"siteUrl": site, "email": email or "", "apiToken": token})
        if out.get("ok"):
            ctx.on_cleanup("disconnect the live Jira site", lambda: ws.call("jira.disconnect", {}))
        assert out.get("ok") is True, f"jira.connect to {site} failed: {str(out.get('error')).replace(token, '***')[:240]}"
        issues = ws.call("jira.listIssues", {"limit": 5})
        assert isinstance(issues, list), f"jira.listIssues did not return a list: {type(issues).__name__}"
        assert token not in json.dumps(issues), "the token leaked into an issue payload"
    rec.run("QA-JIRA-LIVE-01", "The Jira site in .env connects with email + PAT and lists issues (opt-in, real site)", jira_live)

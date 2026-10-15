#!/usr/bin/env python3
"""Runs the whole knowledge-hub Request flow from the `orca` command line on a throwaway repo, with the real Claude (T5.1).

  python3 run_cli_flow.py --real knowledge            # needs the local stack, an approved connected dev server, claude logged in
  python3 run_cli_flow.py --real knowledge --keep     # leave the temp repo and project for a look

It makes a small Go repo (knowledge_fixture/) with three short documents, builds its knowledge graph with `tdd-agent init
--no-prose` (no model call), registers a throwaway project on the dev server with the openspec engine and the agent-written
Solution switched on, then drives one change_request of size M from `request create` to `completed` and checks, step by step,
what the design says must be true: the Solution cites the graph, the authored change passes the structure check and covers
every acceptance criterion, each task ran its own check and exited 0, the change was archived on its branch, the handover is
listed and nothing was pushed. Size M is used on purpose: the built-in approval policy lets the reporter approve it, so the run
needs no policy change on a one-user stack.

It costs real model usage (about 15 to 25 minutes and a few dollars to some tens), so it never starts by itself: there is no CI
job for it. It touches only its own temp repo, project and Request, and takes the push block off again when it is done.

Exit codes: 0 every check passed, 1 a check failed (the report says which), 2 a precondition is missing.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from agent_push_block import PushBlockedAgent, agent_block_state, probe_push
from orca_session import OrcaSession
from orca_stack_config import REPO_ROOT, load_config

HERE = Path(__file__).resolve().parent
FIXTURE = HERE / "knowledge_fixture"
TOKEN_LIFE = 8 * 60  # the sign-in token lasts about ten minutes and cannot be renewed

REQUEST_BODY = """Add a health endpoint to the Notes service.

GET /healthz answers 200 with {"status":"ok"} while the process runs, and never reads the note store.
"""
CRITERIA = [
    "GIVEN the service is running WHEN a client calls GET /healthz THEN it receives 200 and the JSON body {\"status\":\"ok\"}",
    "GIVEN the note store holds notes WHEN a client calls GET /healthz THEN the answer does not depend on them and the store is not read",
    "GIVEN a client calls POST /healthz THEN it receives 405",
]


class Failure(Exception):
    pass


class Report:
    def __init__(self) -> None:
        self.failed: list[str] = []

    def check(self, name: str, ok: bool, detail: object = "") -> bool:
        print(f"[{'ok' if ok else 'FAIL'}] {name}" + ("" if ok else f"  {str(detail)[:400]}"), flush=True)
        if not ok:
            self.failed.append(name)
        return ok


class Cli:
    """The orca binary, signed in with the stack's admin account and signed in again before the token runs out."""

    def __init__(self, cfg, binary: Path) -> None:
        self.cfg, self.binary, self.signed_in_at = cfg, binary, 0.0
        self.env = {**os.environ, "ORCA_API_URL": cfg.gateway_url, "ORCA_EMAIL": cfg.admin_email, "ORCA_PASSWORD": cfg.admin_password}

    def login(self) -> None:
        done = subprocess.run([str(self.binary), "login"], env=self.env, capture_output=True, text=True, timeout=60)
        if done.returncode != 0:
            raise Failure("orca login failed: " + done.stderr.strip()[:200])
        self.signed_in_at = time.time()

    def run(self, *args: str, as_json: bool = True, check: bool = True) -> tuple[int, object]:
        if time.time() - self.signed_in_at > TOKEN_LIFE:
            self.login()
        done = subprocess.run([str(self.binary), *args, *(["--json"] if as_json else [])], env=self.env, capture_output=True, text=True, timeout=300)
        text = done.stdout.strip() or done.stderr.strip()
        if check and done.returncode != 0:
            raise Failure(f"orca {' '.join(args)} exited {done.returncode}: {text[:300]}")
        if not as_json:
            return done.returncode, text
        try:
            return done.returncode, json.loads(done.stdout)
        except json.JSONDecodeError:
            return done.returncode, {"_text": text}


def need(cond: bool, message: str) -> None:
    if not cond:
        print(f"precondition missing: {message}", file=sys.stderr)
        sys.exit(2)


def agent_work_dir() -> Path | None:
    from ai_stub_agent_switch import _pids_of_agent

    for pid in _pids_of_agent():
        out = subprocess.run(["ps", "eww", "-p", str(pid)], capture_output=True, text=True).stdout
        m = re.search(r"(?<=\s)AGENT_WORK_DIR=(\S+)", out)
        if m:
            return Path(m.group(1))
    return None


def git(repo: Path, *args: str) -> str:
    done = subprocess.run(["git", "-C", str(repo), "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", *args], capture_output=True, text=True, timeout=120)
    if done.returncode != 0:
        raise Failure(f"git {' '.join(args)} failed: {done.stderr.strip()[:200]}")
    return done.stdout


def build_repo(root: Path) -> Path:
    """A small Go repo with its documents and a knowledge graph built without any model call."""
    repo = root / "notes"
    (repo / "docs").mkdir(parents=True)
    for name in ("URD.md", "SRS.md", "SAD.md"):
        shutil.copy(FIXTURE / "docs" / name, repo / "docs" / name)
    for name in ("go.mod", "main.go", "main_test.go"):
        shutil.copy(FIXTURE / "src" / name, repo / name)
    git(repo, "init", "-q", "-b", "main")
    git(repo, "remote", "add", "origin", "git@github.com:example/notes.git")  # a real-looking remote, so the push block is what stops a push
    done = subprocess.run(["tdd-agent", "-C", str(repo), "init", "--no-prose", "--name", "Notes", "--tdd-id", "TDD-NOTES",
                           "--urd", str(repo / "docs/URD.md"), "--srs", str(repo / "docs/SRS.md"), "--sad", str(repo / "docs/SAD.md")], capture_output=True, text=True, timeout=300)
    if done.returncode != 0:
        raise Failure("tdd-agent init failed: " + (done.stderr or done.stdout)[-300:])
    done = subprocess.run(["tdd-agent", "-C", str(repo), "mcp", "enable", "--read-only"], capture_output=True, text=True, timeout=60)
    if done.returncode != 0:
        raise Failure("tdd-agent mcp enable failed: " + (done.stderr or done.stdout)[-300:])
    (repo / ".gitignore").write_text(".mcp.json\n.tdd-runs/\n")
    git(repo, "add", "-A")
    git(repo, "commit", "-qm", "notes service with its documents and knowledge graph")
    return repo


def tasks_md_shape(md: str) -> tuple[int, int]:
    """(tasks, tasks that have all four contract lines) in the plan region of a tasks.md."""
    region = md.split("<!-- orca:begin", 1)[-1].split("<!-- orca:end", 1)[0]
    blocks = re.split(r"(?m)^- \[[ x]\] T\d+\.\d+ ", region)[1:]
    full = sum(1 for b in blocks if all(k in b for k in ("Serves:", "Code:", "Verify:", "Done when:")))
    return len(blocks), full


def wait_for(cli: Cli, request_id: str, want: set[str], timeout: float, step: float = 20.0) -> str:
    end = time.time() + timeout
    status = ""
    while time.time() < end:
        _, doc = cli.run("request", "show", request_id)
        status = doc.get("request", {}).get("status", "") if isinstance(doc, dict) else ""
        if status in want or status in ("completed", "cancelled", "request_backlog"):
            return status
        time.sleep(step)
    return status


def generate_plan(cli: Cli, request_id: str, ws, timeout: float = 3900.0) -> dict:
    """plan generate waits for the run and commits the plan, but its token may run out first: the run goes on, so read it and commit it."""
    proc = subprocess.Popen([str(cli.binary), "plan", "generate", request_id, "--json"], env=cli.env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    end = time.time() + timeout
    proposal: dict = {}
    while time.time() < end:
        time.sleep(30)
        _, proposal = cli.run("plan", "proposal", request_id)
        if isinstance(proposal, dict) and proposal.get("status") in ("succeeded", "failed"):
            break
    try:
        out, _ = proc.communicate(timeout=180)
    except subprocess.TimeoutExpired:
        proc.kill()
        out = ""
    if proposal.get("status") == "succeeded" and '"planTaskId"' not in (out or ""):
        ws.call("request.generatePlan", {"id": request_id, "mode": "commit", "proposal": proposal["proposal"], "rawAiResponse": proposal.get("rawAiResponse", "")})
    return proposal


def run(args: argparse.Namespace) -> int:
    rep = Report()
    cfg = load_config("cli-knowledge")
    session = OrcaSession(cfg)
    need(session.login().status == 200, "the stack admin cannot sign in (check deploy/dev/.env)")
    ws = session.ws()
    server = next((s for s in ws.call("devServer.list", {}) if cfg.dev_server_name in (s.get("name"), s.get("hostname"))), None)
    need(bool(server) and server.get("status") == "connected", f"dev server {cfg.dev_server_name!r} is not connected")
    for binary in ("tdd-agent", "openspec", "claude", "git", "go"):
        need(shutil.which(binary) is not None, f"{binary} is not on PATH")
    work = agent_work_dir()
    need(work is not None, "no dev server agent process found")

    binary = Path(tempfile.mkdtemp(prefix="orca-cli-")) / "orca"
    built = subprocess.run(["go", "build", "-o", str(binary), "."], cwd=REPO_ROOT / "backend-go" / "cmd" / "orca-cli", capture_output=True, text=True, timeout=600)
    need(built.returncode == 0, "the orca CLI does not build: " + built.stderr[-200:])
    cli = Cli(cfg, binary)
    cli.login()

    root = Path(tempfile.mkdtemp(prefix="orca-e2e-knowledge-", dir=str(work / "tmp") if (work / "tmp").is_dir() else str(work)))
    agent = PushBlockedAgent(cfg)
    project_id = request_id = ""
    try:
        repo = build_repo(root)
        rep.check("the temp repo has a knowledge graph", json.loads(subprocess.run(["tdd-agent", "-C", str(repo), "status"], capture_output=True, text=True).stdout).get("knowledge", {}).get("elements", 0) > 0)
        agent.start()
        state = agent_block_state()
        rep.check("the agent runs with the push block", bool(state) and all(state.values()), state)
        rep.check("every push form fails locally", probe_push(str(repo)) == [])

        proj = ws.call("project.create", {"name": f"e2e-knowledge-{int(time.time())}", "description": "orca e2e (T5.1), throwaway", "devServerId": server["id"],
                                          "repoPath": str(repo), "defaultBranch": "main", "visibility": "private"})
        project_id = proj["id"]
        cur = ws.call("request.engineSettings", {"projectId": project_id})
        ws.call("request.engineSettingsSet", {"projectId": project_id, "engine": "openspec", "solutionAgent": True, "expectedVersion": cur.get("version", 0)})
        pre = ws.call("request.engineSettings", {"projectId": project_id, "check": True}).get("preflight") or {}
        rep.check("the readiness check is clean (no blocker, no gap)", pre.get("ok") and not pre.get("gaps"), pre)

        _, created = cli.run("request", "create", "--project", project_id, "--title", "Add a health endpoint", "--body", REQUEST_BODY)
        request_id = created["request"]["id"]
        cli.run("request", "classify", request_id, check=False)
        rep.check("classification ends at type confirmation", wait_for(cli, request_id, {"awaiting_type_confirmation"}, 300) == "awaiting_type_confirmation")
        cli.run("request", "confirm-type", request_id, "--type", "change_request", "--size", "M")
        _, clar = cli.run("clarification", "list", request_id)
        open_c = [c for c in clar.get("clarifications", []) if c.get("status") == "open"]
        if open_c:
            by_key = {q["questionKey"]: q["id"] for q in open_c[0]["questions"]}
            ans = {"type_fields.goal": "Let an operator check that the Notes service is running.", "type_fields.value": "A load balancer and humans can see liveness without touching notes.",
                   "type_fields.scope_in": "GET /healthz handler\nA test for it", "acceptance_criteria": "\n".join(CRITERIA)}
            cli.run("clarification", "answer", open_c[0]["id"], *[x for k, v in ans.items() if k in by_key for x in ("--answer", f"{by_key[k]}={v}")], as_json=False)
        rep.check("the analysis finishes and waits for approval", wait_for(cli, request_id, {"awaiting_analysis_approval"}, 1500) == "awaiting_analysis_approval")

        _, sol = cli.run("solution", "list", request_id)
        solutions = [s for s in sol.get("solutions", []) if s.get("status") == "proposed"]
        rep.check("a Solution is proposed", bool(solutions))
        refs = (solutions[0]["options"].get("design_references") or {}) if solutions else {}
        rep.check("the Solution cites the knowledge graph (available, a snapshot, elements read)", refs.get("available") is True and bool(refs.get("snapshot")) and len(refs.get("read") or []) >= 1, refs)
        runs = [r for r in sol.get("runs", []) if r.get("kind") == "solution" and r.get("status") == "succeeded"]
        rep.check("the run reports knowledge tool calls", bool(runs) and (runs[0].get("stats") or {}).get("knowledgeToolCalls", 0) >= 1, runs[:1])

        sid = solutions[0]["id"]
        blocking = [q for q in solutions[0]["options"].get("open_questions", []) if q.get("blocking")]
        if blocking:
            cli.run("clarification", "ask", request_id, "--solution", sid, as_json=False)
            _, clar = cli.run("clarification", "list", request_id)
            ask = [c for c in clar["clarifications"] if c.get("status") == "open"][0]
            cli.run("clarification", "answer", ask["id"], *[x for q in ask["questions"] for x in ("--answer", f"{q['id']}=Proceed with the recommended option; this is a throwaway run.")], as_json=False)
            rep.check("a blocking question is answered and the analysis runs again", wait_for(cli, request_id, {"awaiting_analysis_approval"}, 1500) == "awaiting_analysis_approval")
            _, sol = cli.run("solution", "list", request_id)
            sid = [s for s in sol["solutions"] if s.get("status") == "proposed"][0]["id"]
        _, chosen = cli.run("solution", "choose", request_id, sid, "--option", "opt-1", as_json=False)
        digest = re.search(r"approval digest ([0-9a-f]+)", str(chosen))
        _, apprs = cli.run("approval", "list", request_id)
        gate = next(a for a in apprs["approvals"] if a["subjectType"] == "solution" and a["status"] == "pending")
        cli.run("approval", "approve", gate["id"], "--digest", digest.group(1) if digest else gate["subjectDigest"], "--version", str(gate["version"]), "--comment", "e2e", as_json=False)

        proposal = generate_plan(cli, request_id, ws)
        rep.check("the plan run succeeds", proposal.get("status") == "succeeded", {k: proposal.get(k) for k in ("status", "errorCode", "errorMessage")})
        bundle = proposal.get("bundle") or {}
        codes = [c.get("code") for c in proposal.get("checks") or []]
        rep.check("the authored change passes the structure check", "STRUCTURE_OK" in codes, codes)
        paths = [f["path"] for f in bundle.get("files", [])]
        rep.check("the change has tasks.md, proposal.md and at least one spec", "tasks.md" in paths and "proposal.md" in paths and any(p.startswith("specs/") for p in paths), paths)
        _, tmd = cli.run("plan", "bundle", request_id, "--file", "tasks.md")
        total, full = tasks_md_shape(tmd.get("content", "") if isinstance(tmd, dict) else "")
        rep.check("every task has Serves, Code, Verify and Done when", total > 0 and total == full, f"{full} of {total}")
        specs = ""
        for p in paths:
            if p.startswith("specs/"):
                specs += cli.run("plan", "bundle", request_id, "--file", p)[1].get("content", "")
        rep.check("every acceptance criterion has a scenario named after it", all(f"AC-{n}" in specs for n in (1, 2, 3)), specs[:200])
        rep.check("the knowledge the author read is recorded", (proposal.get("knowledge") or {}).get("available") is True, proposal.get("knowledge"))

        _, apprs = cli.run("approval", "list", request_id)
        plan_gate = next(a for a in apprs["approvals"] if a["subjectType"] == "plan" and a["status"] == "pending")
        cli.run("approval", "approve", plan_gate["id"], "--digest", plan_gate["subjectDigest"], "--version", str(plan_gate["version"]), "--comment", "e2e: change read", as_json=False)

        _, tree = cli.run("plan", "tree", request_id)
        phases = [t for t in tree["tasks"] if t.get("task_type") == "phase"]
        order = list(phases)
        done_phases: set[str] = set()
        deadline = time.time() + 6 * 3600
        while len(done_phases) < len(phases) and time.time() < deadline:
            _, apprs = cli.run("approval", "list", request_id)
            pending = [a for a in apprs["approvals"] if a["subjectType"] == "phase" and a["status"] == "pending"]
            for a in pending:
                cli.run("approval", "approve", a["id"], "--digest", a["subjectDigest"], "--version", str(a["version"]), "--comment", "e2e", as_json=False)
                cli.run("plan", "start-phase", request_id, a["subjectId"], as_json=False, check=False)
            status = wait_for(cli, request_id, {"completed"}, 120, step=30)
            if status in ("completed", "cancelled", "request_backlog"):
                break
            _, tree = cli.run("plan", "tree", request_id)
            kids = {}
            for t in tree["tasks"]:
                kids.setdefault(t.get("parent_id"), []).append(t)
            done_phases = {p["id"] for p in order if kids.get(p["id"]) and all(k["status"] in ("done", "review") for k in kids[p["id"]])}
        final = wait_for(cli, request_id, {"completed"}, 600)
        rep.check("the Request reaches completed", final == "completed", final)

        _, tree = cli.run("plan", "tree", request_id)
        leaves = [t for t in tree["tasks"] if t.get("task_type") not in ("plan", "phase")]
        rep.check("every task is done", leaves and all(t["status"] in ("done", "review") for t in leaves), [t["status"] for t in leaves])
        green = 0
        for t in leaves:
            _, res = cli.run("plan", "result", t["id"])
            checks = res.get("checksRun") or [] if isinstance(res, dict) else []
            green += bool(checks) and all(c.get("exit") == 0 for c in checks)
        rep.check("every task ran its own check and it exited 0", green == len(leaves), f"{green} of {len(leaves)}")

        branch = None
        end = time.time() + 480
        while time.time() < end:  # the sync and archive sweep runs about every 30 seconds
            _, prop = cli.run("plan", "proposal", request_id)
            branch = (prop.get("bundle") or {}) if isinstance(prop, dict) else {}
            if branch.get("status") == "archived":
                break
            time.sleep(30)
        rep.check("the change was archived on its branch", branch.get("status") == "archived", branch)
        log = git(repo, "log", "--all", "--format=%s")
        rep.check("the branch history holds the tick and archive commits", "tick finished tasks" in log and "archive" in log, log[:300])
        rep.check("nothing was pushed (no remote-tracking branch exists)", "refs/remotes/" not in git(repo, "for-each-ref"), git(repo, "for-each-ref"))
        _, shown = cli.run("request", "show", request_id)
        rep.check("the handover is listed", bool((shown.get("handover") or {}).get("items")) or (shown.get("handover") or {}).get("nothingLeft") is not None, shown.get("handover"))
    except Failure as exc:
        rep.check("the flow ran to the end", False, exc)
    finally:
        if not args.keep:
            try:
                if request_id and cli.run("request", "show", request_id, check=False)[1].get("request", {}).get("status") not in ("completed", "cancelled"):
                    cli.run("request", "cancel", request_id, "--reason", "e2e cleanup", as_json=False, check=False)
                if request_id:
                    cli.run("request", "cleanup", request_id, as_json=False, check=False)
                if project_id:
                    ws.call("project.delete", {"projectId": project_id})
            except Exception as exc:  # noqa: BLE001
                print(f"cleanup problem: {exc}", file=sys.stderr)
            shutil.rmtree(root, ignore_errors=True)
        try:
            agent.restore()
        except Exception as exc:  # noqa: BLE001
            print(f"could not restore the agent: {exc}", file=sys.stderr)
        session.close()
        shutil.rmtree(binary.parent, ignore_errors=True)
    print(f"{len(rep.failed)} check(s) failed" if rep.failed else "all checks passed")
    return 1 if rep.failed else 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--real", choices=["knowledge"], required=True, help="the flow to run with the real Claude")
    ap.add_argument("--keep", action="store_true", help="leave the temp repo, project and Request in place")
    return run(ap.parse_args())


if __name__ == "__main__":
    sys.exit(main())

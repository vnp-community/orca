"""WFL: workflow templates run as DAGs, in dependency order, and invalid definitions never complete."""
from __future__ import annotations

import json
import shutil
import subprocess

from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ws_rpc_client import RpcError

TERMINAL = ("completed", "failed", "cancelled")


def _cond(step_id: str, expr: str, ctx_map: dict, *depends: str) -> dict:
    step = {"id": step_id, "type": "condition", "config": {"expression": expr, "context": ctx_map}}
    if depends:
        step["dependsOn"] = list(depends)
    return step


def _step_rows(execution_id: str) -> list[tuple[str, str, int, str]]:
    """(step_id, status, wave, output) straight from the workflow database; the WS API does not return steps."""
    if not shutil.which("docker"):
        raise Blocked("docker CLI not available to read step results")
    sql = f"select step_id,status,wave,output::text from workflow.step_executions where execution_id='{execution_id}' order by wave,step_id"
    out = subprocess.run(["docker", "exec", "orca-go-postgres", "psql", "-U", "orca", "-d", "workflow", "-At", "-F", "|", "-c", sql],
                         capture_output=True, text=True, timeout=20)
    if out.returncode != 0:
        raise Blocked(f"cannot query workflow DB: {out.stderr.strip()[:120]}")
    rows = []
    for line in out.stdout.splitlines():
        step_id, status, wave, output = line.split("|", 3)
        rows.append((step_id, status, int(wave), output))
    return rows


def run(ctx: AcceptanceContext) -> None:
    rec, cfg, ws = ctx.rec, ctx.cfg, ctx.ws

    def make_template(label: str, steps: list[dict]) -> str:
        tpl = ws.call("workflow.template.create", {"name": f"{cfg.run_prefix}-{label}", "dagJson": json.dumps({"steps": steps}), "scope": "personal"})
        assert tpl.get("id"), f"no template id in {tpl!r}"
        return tpl["id"]

    def execute(template_id: str, project_id: str = "") -> str:
        ex = ws.call("workflow.execute", {"templateId": template_id, "projectId": project_id, "requestId": f"{cfg.run_prefix}-{template_id[:8]}"})
        return ex["id"]

    def settle(execution_id: str, timeout: float = 30.0) -> str:
        def done():
            st = ws.call("workflow.getExecution", {"executionId": execution_id}).get("status", "")
            return st if st in TERMINAL else ""
        return wait_until(done, timeout=timeout, what=f"execution {execution_id[:8]} to finish")

    chain: dict = {}

    def linear_dag():
        tid = make_template("linear", [_cond("a", "x == '1'", {"x": "1"}), _cond("b", "y == '2'", {"y": "2"}, "a"), _cond("c", "z == '3'", {"z": "3"}, "b")])
        chain["execution"] = execute(tid)
        status = settle(chain["execution"])
        assert status == "completed", f"linear DAG finished as {status!r}"
    rec.run("WFL-01", "Linear 3-step DAG runs to completion", linear_dag)

    def linear_order():
        if "execution" not in chain:
            raise Blocked("WFL-01 produced no execution")
        rows = _step_rows(chain["execution"])
        assert [r[0] for r in rows] == ["a", "b", "c"], f"steps recorded: {[r[0] for r in rows]}"
        assert [r[2] for r in rows] == [0, 1, 2], f"waves {[r[2] for r in rows]}, expected strictly increasing 0,1,2"
        assert all(r[1] == "completed" for r in rows), f"step statuses {[r[1] for r in rows]}"
    rec.run("WFL-02", "Dependent steps run in successive waves, in order", linear_order)

    fan: dict = {}

    def fan_out():
        tid = make_template("fanout", [_cond("root", "k == 'v'", {"k": "v"}), _cond("left", "k == 'v'", {"k": "v"}, "root"),
                                       _cond("right", "k == 'v'", {"k": "v"}, "root"), _cond("join", "k == 'v'", {"k": "v"}, "left", "right")])
        fan["execution"] = execute(tid)
        assert settle(fan["execution"]) == "completed"
        rows = {r[0]: r[2] for r in _step_rows(fan["execution"])}
        assert rows == {"root": 0, "left": 1, "right": 1, "join": 2}, f"waves {rows}"
    rec.run("WFL-03", "Fan-out/fan-in: independent steps share a wave, the join waits for both", fan_out)

    def false_condition():
        tid = make_template("falsecond", [_cond("gate", "x == 'no'", {"x": "yes"})])
        eid = execute(tid)
        assert settle(eid) == "completed", "a false condition should not fail the execution"
        out = json.loads(_step_rows(eid)[0][3])
        assert out.get("result") is False, f"condition output {out!r}, expected result=false"
    rec.run("WFL-04", "A false condition is recorded as result=false (not an error)", false_condition)

    def malformed_condition():
        tid = make_template("badexpr", [_cond("bad", "this is ((not valid", {})])
        eid = execute(tid)
        status = settle(eid)
        assert status == "failed", f"an unparseable expression ended as {status!r}, expected failed"
    rec.run("WFL-05", "An unparseable condition fails the execution", malformed_condition)

    def must_not_complete(label: str, steps: list[dict]):
        try:
            tid = make_template(label, steps)
        except RpcError:
            return  # rejected at definition time: best outcome
        try:
            eid = execute(tid)
        except RpcError:
            return
        status = settle(eid)
        assert status != "completed", f"an invalid definition ({label}) completed successfully"

    rec.run("WFL-06", "Cyclic dependencies never complete", lambda: must_not_complete("cycle", [
        _cond("a", "k == 'v'", {"k": "v"}, "b"), _cond("b", "k == 'v'", {"k": "v"}, "a")]))
    rec.run("WFL-07", "Dependency on an unknown step never completes", lambda: must_not_complete("dangling", [
        _cond("a", "k == 'v'", {"k": "v"}, "ghost")]))
    rec.run("WFL-08", "Duplicate step ids never complete", lambda: must_not_complete("dup", [
        _cond("a", "k == 'v'", {"k": "v"}), _cond("a", "k == 'v'", {"k": "v"})]))

    def unknown_execution():
        try:
            ws.call("workflow.getExecution", {"executionId": "00000000-0000-0000-0000-000000000000"})
        except RpcError:
            return
        raise AssertionError("getExecution of an unknown id returned a result")
    rec.run("WFL-09", "Unknown execution id is an error", unknown_execution)

    def terminal_state_is_final():
        if "execution" not in chain:
            raise Blocked("WFL-01 produced no execution")
        for channel in ("workflow.pause", "workflow.resume", "workflow.cancel"):
            try:
                ws.call(channel, {"executionId": chain["execution"]})
            except RpcError:
                continue
            raise AssertionError(f"{channel} succeeded on an execution that already completed")
    rec.run("WFL-10", "A completed execution cannot be paused, resumed or cancelled", terminal_state_is_final)

    def listed():
        if "execution" not in chain:
            raise Blocked("WFL-01 produced no execution")
        listing = ws.call("workflow.listExecutions", {"projectId": ctx.state.get("project_id", "")})
        assert isinstance(listing, (list, dict)), f"unexpected listing {listing!r}"
    rec.run("WFL-11", "workflow.listExecutions answers for the project", listed)

    def shell_run(target_for):
        pid, ds = ctx.state.get("project_id"), ctx.state.get("dev_server_id")
        if not pid or not ds or not ctx.state.get("repo_path"):
            raise Blocked("needs a project bound to a connected dev server (DEV-04)")
        tid = make_template("shell", [{"id": "echo", "type": "shell", "config": {"script": "echo e2e-shell-ok", "target": target_for(pid, ds)}}])
        eid = execute(tid, pid)
        status = settle(eid, timeout=60)
        rows = _step_rows(eid)
        assert status == "completed", f"shell step finished as {status!r}: {rows[0][3][:200] if rows else 'no step rows'}"
        assert "e2e-shell-ok" in rows[0][3], f"step output lacks the echoed text: {rows[0][3][:200]}"
    rec.run("WFL-12", "Shell step with target 'server:<devServerId>' runs on the dev server agent", lambda: shell_run(lambda pid, ds: f"server:{ds}"))
    rec.run("WFL-13", "Shell step with target 'project:<projectId>' (documented in F36) runs on the project's dev server", lambda: shell_run(lambda pid, ds: f"project:{pid}"))

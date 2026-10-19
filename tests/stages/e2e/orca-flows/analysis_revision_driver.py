"""One Request under analysis revision (CR-REQ-041), driven through the public WebSocket channels like the UI does.

A Flow knows its analysis kind (solution, diagnosis, findings, answer) and the approval subject of its gate, and offers the waits and
reads the ARV cases share. The fake claude (--ai-stub) echoes a change request as "[revised, round N, asked: ...]" in the document,
which is how the cases see that the ask and the earlier versions reached the model."""
from __future__ import annotations

import json

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until

# request type -> (analysis kind, approval subject of its gate)
ANALYSES = {
    "change_request": ("solution", "solution"),
    "bug": ("diagnosis", "solution"),
    "spike": ("findings", "findings"),
    "question": ("answer", "answer"),
    "hotfix": ("diagnosis", ""),  # diagnosed like a bug, but its Diagnosis is not put to an approval
}
RUN_WAIT = 150  # the fake answers in seconds; a slow trigger adds six more
GATE = "awaiting_analysis_approval"


def echo(round_no: int, ask: str) -> str:
    """The marker the fake claude writes into a revised document (see fake_claude_cli/claude, revision_note)."""
    return f"[revised, round {round_no}, asked: {' '.join(ask.split())[:60]}]"


class Flow:
    def __init__(self, ctx: AcceptanceContext, request_id: str, typ: str, ws=None):
        self.ctx, self.id, self.typ = ctx, request_id, typ
        self.kind, self.subject = ANALYSES[typ]
        self.ws = ws or ctx.ws

    # ── reads ─────────────────────────────────────────────────────────────────────────────────────
    def listed(self) -> dict:
        return self.ctx.ws.call("solution.list", {"requestId": self.id})

    def versions(self) -> list[dict]:
        """Oldest first, as solution.list returns them; drafts of a running round are left out."""
        return [s for s in self.listed().get("solutions", []) if s["kind"] == self.kind and s["status"] != "draft"]

    def runs(self) -> list[dict]:
        return [r for r in self.listed().get("runs", []) if r["kind"] == self.kind]

    def status(self) -> str:
        return drv.get(self.ctx, self.id)["status"]

    def pending(self) -> list[dict]:
        listed = self.ctx.ws.call("approval.list", {"requestId": self.id, "status": "pending"}).get("approvals", [])
        return [a for a in listed if a["subjectType"] == self.subject]

    def activity(self, kind: str) -> list[dict]:
        return self.ctx.ws.call("request.activity", {"id": self.id, "kinds": [kind], "pageSize": 100}).get("entries", [])

    # ── waits ─────────────────────────────────────────────────────────────────────────────────────
    def wait_run(self, run_id: str, timeout: float = RUN_WAIT) -> dict:
        def done():
            run = next((r for r in self.listed().get("runs", []) if r["id"] == run_id), None)
            return run if run and run["status"] != "running" else None
        return wait_until(done, timeout=timeout, interval=1.0, what=f"the {self.kind} run {run_id} to finish")

    def wait_gate(self, timeout: float = 30.0) -> dict:
        return wait_until(lambda: (lambda r: r if r["status"] == GATE else None)(drv.get(self.ctx, self.id)), timeout=timeout, what="the Request to reach the analysis gate")

    # ── steps ─────────────────────────────────────────────────────────────────────────────────────
    def first_attempt(self) -> None:
        """The readiness answer starts the first run by itself (REQUEST_CLARIFICATION_AUTO_REGENERATE); only when no run shows up in time does a change_request ask for one."""
        if self.typ == "change_request":
            try:
                wait_until(lambda: self.runs() or None, timeout=20.0, interval=1.0, what=f"the automatic first {self.kind} run")
            except Exception:
                self.ctx.ws.call("solution.generate", {"requestId": self.id}, timeout=120)
        self.wait_gate(timeout=RUN_WAIT)
        assert len(self.versions()) == 1, f"expected one {self.kind} after the first attempt, got {len(self.versions())}"

    def revise(self, ask: str, key: str = "", ws=None) -> dict:
        args = {"requestId": self.id, "feedback": ask}
        if key:
            args["idempotencyKey"] = key
        return (ws or self.ws).call("solution.generate", args, timeout=60)

    def revise_and_wait(self, ask: str, key: str = "") -> dict:
        reply = self.revise(ask, key)
        run = self.wait_run(reply["runId"])
        assert run["status"] == "succeeded", f"the revision run ended {run['status']!r}: {run.get('errorCode')} {str(run.get('errorMessage', ''))[:160]}"
        self.wait_gate()
        return reply

    # ── checks ────────────────────────────────────────────────────────────────────────────────────
    def assert_one_pending_on_newest(self) -> None:
        versions, pending = self.versions(), self.pending()
        assert len(pending) == 1, f"{len(pending)} pending '{self.subject}' approvals after a revision, expected exactly one"
        assert pending[0]["subjectId"] == versions[-1]["id"], "the pending approval is not on the newest version"

    def assert_history(self, statuses: list[str], asks: list[str]) -> None:
        """Version statuses oldest first, and what each was asked for ('' for the first attempt)."""
        versions = self.versions()
        got = [(v["status"], v.get("revisionFeedback", "")) for v in versions]
        assert got == list(zip(statuses, asks)), f"versions (status, revisionFeedback) = {got!r}, expected {list(zip(statuses, asks))!r}"
        seqs = [v.get("seq") for v in versions]
        if all(seqs):
            assert seqs == sorted(seqs) and len(set(seqs)) == len(seqs), f"version numbers {seqs} are not 1, 2, 3 ... in order"

    def document_text(self, version: dict) -> str:
        return json.dumps(version.get("options") or {}, ensure_ascii=False)


def new_flow(ctx: AcceptanceContext, project_id: str, typ: str, title: str, size: str = "M") -> Flow:
    req = drv.create(ctx, title, project_id=project_id)["request"]
    cur = drv.to_first_stage(ctx, req, typ, size)
    assert cur["status"] == "analyzing", f"after the readiness answer the Request is {cur['status']!r}, expected analyzing"
    return Flow(ctx, req["id"], typ)

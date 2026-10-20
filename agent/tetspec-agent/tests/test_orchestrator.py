import pytest

from graphs import edge, el
from tdd_agent.config import load_config
from tdd_agent.errors import ChangeSetRejected
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.obs import Observer, bind
from tdd_agent.orchestrator import ApprovalRequest, FlowDef, GateDef, GateOutcome, Orchestrator
from tdd_agent.store.base import SourceDoc
from tdd_agent.store.memory import MemoryKnowledgeStore, MemoryOperationalStore

ACTORS = [{"id": "alice", "kind": "human", "roles": ["BA", "BA/owner"]}, {"id": "bob", "kind": "human", "roles": ["SA/Dev", "SA"]},
          {"id": "claude", "kind": "coding_agent", "roles": ["BA", "SA/Dev", "SA", "BA/owner"]}, {"id": "limited", "kind": "coding_agent", "roles": ["BA"]}]


class Counter:
    def __init__(self): self.n = {}
    def hit(self, k): self.n[k] = self.n.get(k, 0) + 1


def make_flow(counter, *, g4_blocks=False, g2_decision=None, g3_commit=None, g6_ready=lambda ctx: True):
    def mk(name, **kw):
        def run(ctx):
            counter.hit(name)
            return GateOutcome.ok({"gate": name, "k": counter.n[name]}, decision=kw.get("decision"))
        return run
    def g2(ctx):
        counter.hit("G2")
        return GateOutcome.ok({"gate": "G2"}, decision=g2_decision() if g2_decision else {"report": "r1"})
    def g3(ctx):
        counter.hit("G3")
        if g3_commit: g3_commit(ctx)
        return GateOutcome.ok({"gate": "G3"})
    def g4(ctx):
        counter.hit("G4")
        return GateOutcome.blocked("COVERAGE_GAP:CS-1") if g4_blocks else GateOutcome.ok({"gate": "G4"})
    def g6(ctx):
        counter.hit("G6")
        return GateOutcome.ok({"gate": "G6"}) if g6_ready(ctx) else GateOutcome.pending_evidence("TEST_RECEIPT_MISSING")
    return FlowDef("cr", (
        GateDef("G0", "intake", mk("G0"), source_roles=("SRS", "SAD")), GateDef("G1", "srs_ready", mk("G1", decision={"srs": "ok"}), approval="BA", source_roles=("SRS",), depends_on=()),
        GateDef("G2", "impact_approved", g2, approval="SA/Dev", source_roles=("SRS", "SAD")), GateDef("G3", "tdd_realized", g3),
        GateDef("G4", "spec_eligible", g4), GateDef("G5", "spec_authored", mk("G5", decision={"spec": 1}), approval="SA"),
        GateDef("G6", "verified", g6, evidence_gate=True), GateDef("G7", "accepted", mk("G7", decision={"accept": 1}), approval="BA/owner"),
    ))


def build(flow, **cfg):
    overrides = {"approvals": {"actors": ACTORS}}
    for k, v in cfg.items(): overrides.setdefault("llm", {}).setdefault("budget", {})[k] = v
    config = load_config(overrides=overrides, env={})
    ops = MemoryOperationalStore(); ob = Observer(ops); know = MemoryKnowledgeStore()
    gw = LlmGateway(config, ob, {"claude_code": FakeProvider({"*": {}})})
    return Orchestrator(config, know, ops, gw, ob, {"cr": flow}), know, ops


SOURCES = [SourceDoc.make("SRS-1", "1", "SRS", "srs v1"), SourceDoc.make("SAD-1", "1", "SAD", "sad v1"), SourceDoc.make("CR-DOC", "1", "CR", "cr")]


def approve(o, run_id, gate, actor, role, *, rationale="reviewed", read=True, **kw):
    if read:
        d = o.get_decision(run_id, gate, actor, source="test")
        h = d["decision_hash"]
    else:
        h = kw.pop("decision_hash", "x")
    return o.submit_approval(ApprovalRequest(run_id, gate, actor, role, kw.pop("decision_hash", h) if not read else h, rationale, **kw))


def test_full_workflow_with_mixed_human_and_agent_approvals():
    c = Counter(); o, know, ops = build(make_flow(c))
    run = o.start("cr", {"mode": "change-request"}, run_id="r1", sources=SOURCES)
    run = o.advance("r1")
    assert run.status == "awaiting_approval" and run.state == "intake" and run.result["pending_gate"] == "G1"
    assert approve(o, "r1", "G1", "alice", "BA").accepted
    run = o.get_run("r1"); assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G2" and run.state == "srs_ready"
    assert approve(o, "r1", "G2", "claude", "SA/Dev").accepted
    assert approve(o, "r1", "G5", "bob", "SA").accepted
    run = o.get_run("r1"); assert run.status == "awaiting_approval" and run.result["pending_gate"] == "G7"
    done = approve(o, "r1", "G7", "claude", "BA/owner")
    assert done.accepted and done.run_status == "completed" and o.get_run("r1").state == "accepted"
    grades = {a["gate"]: a["approval_grade"] for a in ops.list_approvals("r1")}
    assert grades == {"G1": "human", "G2": "agent", "G5": "human", "G7": "agent"}
    assert [t["to"] for t in ops.list_transitions("r1")] == ["intake", "srs_ready", "impact_approved", "tdd_realized", "spec_eligible", "spec_authored", "verified", "accepted"]
    assert all(c.n[g] == 1 for g in ("G0", "G1", "G2", "G3", "G4", "G5", "G7")), c.n
    spans = ops.list_spans(run_id="r1"); assert spans and {s["kind"] for s in spans} >= {"run", "gate"}


@pytest.mark.parametrize("who,role,rationale,read,code", [
    ("mallory", "BA", "ok", True, "ACTOR_NOT_ALLOWLISTED"), ("limited", "SA/Dev", "ok", True, "ROLE_NOT_PERMITTED"),
    ("alice", "SA/Dev", "ok", True, "ROLE_NOT_PERMITTED"), ("alice", "BA", "   ", True, "RATIONALE_REQUIRED"),
    ("alice", "BA", "ok", False, "REVIEW_RECEIPT_MISSING"),
])
def test_approval_rejections_leave_state_unchanged(who, role, rationale, read, code):
    c = Counter(); o, know, ops = build(make_flow(c))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r")
    if who == "mallory": o.approvals.record_receipt("r", "mallory", o.operational.get_task("r", "G1")["decision_hash"], "G1", "x")
    if read:
        r = approve(o, "r", "G1", who, role, rationale=rationale) if who != "mallory" else o.submit_approval(ApprovalRequest("r", "G1", who, role, o.operational.get_task("r", "G1")["decision_hash"], rationale))
    else:
        r = o.submit_approval(ApprovalRequest("r", "G1", who, role, o.operational.get_task("r", "G1")["decision_hash"], rationale))
    assert not r.accepted and r.code == code
    assert o.get_run("r").status == "awaiting_approval" and o.operational.get_task("r", "G1")["status"] == "awaiting_approval"
    assert ops.list_audit(kind="approval_rejected", run_id="r")[-1]["payload"]["code"] == code


def test_wrong_hash_wrong_kind_not_awaiting_and_blockers():
    c = Counter(); o, know, ops = build(make_flow(c))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r")
    o.get_decision("r", "G1", "alice")
    assert o.submit_approval(ApprovalRequest("r", "G1", "alice", "BA", "deadbeef", "ok")).code == "DECISION_HASH_MISMATCH"
    h = o.operational.get_task("r", "G1")["decision_hash"]
    assert o.submit_approval(ApprovalRequest("r", "G1", "alice", "BA", h, "ok", claimed_kind="coding_agent")).code == "ACTOR_KIND_MISMATCH"
    assert o.submit_approval(ApprovalRequest("r", "G2", "bob", "SA/Dev", h, "ok")).code == "NOT_AWAITING_APPROVAL"
    assert o.submit_approval(ApprovalRequest("r", "G3", "bob", "SA/Dev", h, "ok")).code == "GATE_NOT_APPROVABLE"
    # a blocker cannot be approved away
    t = dict(o.operational.get_task("r", "G1"), blockers=["SOURCE_STALE"]); o.operational.put_task("r", "G1", t)
    assert o.submit_approval(ApprovalRequest("r", "G1", "alice", "BA", h, "ok")).code == "BLOCKERS_PRESENT"


def test_pipeline_agent_context_can_never_approve():
    c = Counter(); o, know, ops = build(make_flow(c))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r")
    d = o.get_decision("r", "G1", "claude")
    with bind(agent="impact"):
        res = o.submit_approval(ApprovalRequest("r", "G1", "claude", "BA", d["decision_hash"], "agent tries to self-approve"))
    assert not res.accepted and res.code == "PIPELINE_AGENT_CANNOT_APPROVE" and o.get_run("r").status == "awaiting_approval"


def test_executors_have_no_approval_service_and_no_raw_source_registration():
    seen = {}
    def probe(ctx):
        seen["attrs"] = {a for a in dir(ctx) if "approv" in a.lower()}
        with pytest.raises(AttributeError):
            ctx.knowledge.register_source(SourceDoc.make("X", "1", "SRS", "t"))
        return GateOutcome.ok({})
    flow = FlowDef("cr", (GateDef("G0", "intake", probe),))
    o, *_ = build(flow); o.start("cr", {}, run_id="r"); assert o.advance("r").status == "completed" and seen["attrs"] == set()


def test_changed_decision_voids_old_hash():
    c = Counter(); box = {"v": "r1"}
    o, know, ops = build(make_flow(c, g2_decision=lambda: {"report": box["v"]}))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r"); approve(o, "r", "G1", "alice", "BA")
    old = o.get_decision("r", "G2", "claude")["decision_hash"]
    box["v"] = "r2-edited"
    o.update_sources("r", [SourceDoc.make("SRS-1", "2", "SRS", "srs v2")])
    o.advance("r")
    new = o.get_decision("r", "G1", "alice")
    assert o.operational.get_task("r", "G2")["status"] == "stale"
    res = o.submit_approval(ApprovalRequest("r", "G2", "claude", "SA/Dev", old, "late"))
    assert not res.accepted and res.code == "NOT_AWAITING_APPROVAL"


def test_resume_does_not_rerun_completed_gates_and_source_change_invalidates_dependents():
    c = Counter(); o, know, ops = build(make_flow(c))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r"); approve(o, "r", "G1", "alice", "BA"); approve(o, "r", "G2", "bob", "SA/Dev")
    assert (c.n["G0"], c.n["G1"], c.n["G2"], c.n["G3"]) == (1, 1, 1, 1)
    o.advance("r"); o.advance("r")
    assert (c.n["G0"], c.n["G1"], c.n["G2"], c.n["G3"], c.n["G4"]) == (1, 1, 1, 1, 1), "no gate re-ran on resume"
    # SAD-only change: G1 depends on SRS only, so G1 stays approved; G0 and G2 depend on SAD
    o.update_sources("r", [SourceDoc.make("SAD-1", "2", "SAD", "sad v2")]); o.advance("r")
    assert c.n["G0"] == 2 and c.n["G1"] == 1 and c.n["G2"] == 2
    assert o.operational.get_task("r", "G2")["status"] == "awaiting_approval" and o.get_run("r").status == "awaiting_approval"
    assert o.get_run("r").state == "srs_ready"
    audit = [a["payload"]["gate"] for a in ops.list_audit(kind="gate_stale", run_id="r")]
    assert "G2" in audit and "G3" in audit and "G4" in audit


def test_blocked_gate_is_recorded_and_metered_and_gates_do_not_skip():
    c = Counter(); o, know, ops = build(make_flow(c, g4_blocks=True))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r"); approve(o, "r", "G1", "alice", "BA"); approve(o, "r", "G2", "bob", "SA/Dev")
    run = o.get_run("r")
    assert run.status == "blocked" and run.result["blockers"] == ["COVERAGE_GAP:CS-1"] and run.result["blocked_gate"] == "G4" and run.state == "tdd_realized"
    assert c.n.get("G5", 0) == 0 and c.n.get("G7", 0) == 0
    assert {m["labels"]["code"] for m in ops.list_metrics("blocked_total")} == {"COVERAGE_GAP"}
    assert o.advance("r").status == "blocked" and c.n["G4"] == 2, "advance re-evaluates a blocked gate"


def test_sad_is_read_only_for_executors():
    def mutate(ctx):
        ctx.commit_changeset({"id": "cs", "base_version": "v1", "result_version": "v2", "ops": [
            {"seq": 1, "op": "modify", "target": "SAD-EL", "path": "name", "before": "SAD-EL", "after": "changed"}]})
    c = Counter(); o, know, ops = build(make_flow(c, g3_commit=mutate))
    know.commit_graph(None, {"version": "v1", "elements": [el("SAD-EL", "Decision", owner_doc="SAD")], "edges": []}, actor="seed", validate=False)
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r"); approve(o, "r", "G1", "alice", "BA"); approve(o, "r", "G2", "bob", "SA/Dev")
    run = o.get_run("r")
    assert run.status == "blocked" and run.result["blockers"] == ["SAD_READONLY", "SAD_CHANGED:SAD-EL"] and know.get_element("SAD-EL")["name"] == "SAD-EL"


def test_evidence_gate_waits_then_completes():
    c = Counter(); flag = {"ready": False}
    o, know, ops = build(make_flow(c, g6_ready=lambda ctx: flag["ready"]))
    o.start("cr", {}, run_id="r", sources=SOURCES); o.advance("r")
    for g, a, r in (("G1", "alice", "BA"), ("G2", "bob", "SA/Dev"), ("G5", "bob", "SA")): approve(o, "r", g, a, r)
    run = o.get_run("r"); assert run.status == "awaiting_evidence" and run.result["blockers"] == ["TEST_RECEIPT_MISSING"] and run.state == "spec_authored"
    flag["ready"] = True
    assert o.advance("r").status == "awaiting_approval" and o.get_run("r").state == "verified"


def test_executor_crash_marks_run_failed_and_reraises():
    def boom(ctx): raise RuntimeError("kaput")
    flow = FlowDef("cr", (GateDef("G0", "intake", boom),))
    o, know, ops = build(flow); o.start("cr", {}, run_id="r")
    with pytest.raises(RuntimeError):
        o.advance("r")
    assert o.get_run("r").status == "failed" and ops.get_task("r", "G0")["status"] == "failed"


def test_run_quota_blocks_start():
    c = Counter(); o, know, ops = build(make_flow(c), max_runs_per_day=1)
    o.start("cr", {}, run_id="a", sources=SOURCES)
    second = o.start("cr", {}, run_id="b", sources=SOURCES)
    assert second.status == "blocked" and second.result["blockers"] == ["BUDGET_EXCEEDED"]


def test_unknown_flow_and_run():
    c = Counter(); o, *_ = build(make_flow(c))
    from tdd_agent.errors import NotFound
    with pytest.raises(NotFound): o.start("nope", {})
    with pytest.raises(NotFound): o.advance("missing")


def test_update_sources_can_keep_the_before_revision_unpin_the_old_after_and_patch_params():
    c = Counter(); o, know, ops = build(make_flow(c))
    srs1, srs2, srs3 = (SourceDoc.make("SRS-1", r, "SRS", f"srs {r}") for r in ("1", "2", "3"))
    o.start("cr", {"params": {"srs": {"before": "1", "after": "2"}}}, run_id="r", sources=[srs1, srs2] + SOURCES[1:])
    run = o.update_sources("r", [srs3], keep_revisions=True, unpin=[("SRS", "SRS-1", "2")], params={"srs": {"after": "3"}})
    assert {(p["id"], p["revision"]) for p in run.request["pins"] if p["role"] == "SRS"} == {("SRS-1", "1"), ("SRS-1", "3")}
    assert run.request["params"]["srs"] == {"before": "1", "after": "3"}
    replaced = o.update_sources("r", [SourceDoc.make("SRS-1", "4", "SRS", "srs 4")])
    assert {(p["id"], p["revision"]) for p in replaced.request["pins"] if p["role"] == "SRS"} == {("SRS-1", "4")}, "default behaviour: the new revision replaces the old pins"


def test_an_interrupt_leaves_the_run_blocked_and_advance_continues_it_while_a_program_error_fails_it_and_retry_runs_it_again():
    c = Counter()
    mode = {"fail": KeyboardInterrupt}

    def g3_commit(ctx):
        if mode["fail"]:
            raise mode["fail"]()
    o, know, ops = build(make_flow(c, g3_commit=g3_commit))
    o.start("cr", {"mode": "change-request"}, run_id="r", sources=SOURCES)
    o.advance("r")                       # G1 and G2 are approved by the engine in auto mode only when no approvers are declared; here a person approves
    approve(o, "r", "G1", "alice", "BA")
    with pytest.raises(KeyboardInterrupt):
        approve(o, "r", "G2", "claude", "SA/Dev")
    run = o.get_run("r")
    assert run.status == "blocked" and run.result["blockers"] == ["INTERRUPTED"] and run.result["blocked_gate"] == "G3"
    mode["fail"] = None
    assert o.advance("r").status == "awaiting_approval", "the interrupted gate ran again and the run went on"
    mode["fail"] = ValueError
    o2, _, _ = build(make_flow(Counter(), g3_commit=lambda ctx: (_ for _ in ()).throw(ValueError("bug"))))
    o2.start("cr", {"mode": "change-request"}, run_id="r", sources=SOURCES)
    o2.advance("r"); approve(o2, "r", "G1", "alice", "BA")
    with pytest.raises(ValueError):
        approve(o2, "r", "G2", "claude", "SA/Dev")
    assert o2.get_run("r").status == "failed"
    assert o2.advance("r").status == "failed", "a program error is not retried by a plain advance"
    with pytest.raises(ValueError):
        o2.retry("r")
    assert ops.list_audit(kind="gate_interrupted") and o2.operational.list_audit(kind="run_retried")


def test_a_gate_goes_stale_only_when_the_configuration_it_reads_changes():
    c = Counter()
    o, know, ops = build(make_flow(c))
    o.start("cr", {"mode": "change-request"}, run_id="r", sources=SOURCES)
    o.advance("r")
    assert c.n["G0"] == 1
    o.config.data["approvals"]["actors"] = o.config.data["approvals"]["actors"] + [{"id": "carol", "kind": "human", "roles": ["BA"]}]
    o.config.data["project"]["name"] = "renamed"
    o.advance("r")
    assert c.n["G0"] == 1 and ops.get_task("r", "G0")["status"] != "stale", "who may approve and the project name are not inputs of a gate"
    o.config.data["llm"]["roles"] = dict(o.config.data["llm"].get("roles", {}), author="codex_cli")
    o.advance("r")
    assert c.n["G0"] == 2, "the model that does the work is an input"


def test_conflicts_are_listed_resolved_with_a_reason_and_only_those_in_the_change_zone_block_it():
    from tdd_agent.core import o5_path
    o5_path.o5_root()
    from spec_change import split_conflicts
    inside, outside = split_conflicts([{"type": "reviewed_element_differs", "id": "FR-1", "status": "unresolved"}, {"type": "entity_match", "ids": ["A", "B"], "status": "unresolved"},
                                       {"type": "source_version", "status": "unresolved"}, {"type": "entity_match", "ids": ["FR-1", "X"], "status": "resolved"}], {"FR-1", "FR-2"})
    assert [c.get("id") or c.get("ids") for c in inside] == ["FR-1", None] and [c["ids"] for c in outside] == [["A", "B"]], "far from the change: reported, not blocking; resolved: nothing"
    o, know, ops = build(make_flow(Counter()))
    conflict = {"type": "entity_match", "ids": ["A", "B"], "status": "unresolved", "reason": "maybe the same"}
    know.commit_graph(None, {"version": "v1", "elements": [el("A", "Decision"), el("B", "Decision")], "edges": [], "conflicts": [conflict]}, actor="seed", validate=False)
    (row,) = o.list_conflicts("unresolved")
    for who, role in (("mallory", "SA"), ("alice", "SA/Dev")):
        with pytest.raises(Exception) as e:
            o.resolve_conflicts([row["key"]], who, role, "same", "because")
        assert e.value.code == "RESOLVE_NOT_PERMITTED"
    with pytest.raises(Exception) as e:
        o.resolve_conflicts([row["key"]], "bob", "SA", "different things", "")
    assert e.value.code == "RATIONALE_REQUIRED"
    done = o.resolve_conflicts([row["key"]], "bob", "SA", "they are different", "A is the queue, B the topic")
    assert done["resolved"] == [row["key"]] and o.list_conflicts("unresolved") == []
    (settled,) = o.list_conflicts("resolved")
    assert settled["resolution"]["actor"] == "bob" and settled["resolution"]["rationale"] == "A is the queue, B the topic"
    assert ops.list_audit(kind="conflicts_resolved") and know.get_element("A")["name"] == "A"

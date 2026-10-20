"""T2.2: a gate starts and advances a child run through the ChildRuns hook."""
from tdd_agent.config import load_config
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.obs import Observer
from tdd_agent.orchestrator import FlowDef, GateDef, GateOutcome, Orchestrator
from tdd_agent.store.memory import MemoryKnowledgeStore, MemoryOperationalStore


def build(flows):
    config = load_config(env={})
    ops = MemoryOperationalStore(); ob = Observer(ops)
    gw = LlmGateway(config, ob, {"claude_code": FakeProvider({"*": {}})})
    return Orchestrator(config, MemoryKnowledgeStore(), ops, gw, ob, flows)


def test_gate_starts_and_advances_child_run():
    seen = {}

    def parent_gate(ctx):
        child = ctx.children.start("child", {"params": {"x": 1}})
        seen["status"] = ctx.children.advance(child)   # the child run's lock differs from the lock the parent holds
        return GateOutcome.ok({"child": child})

    def child_gate(ctx):
        seen["parent"] = ctx.run.parent_run_id
        return GateOutcome.ok({"x": ctx.request["params"]["x"]})

    o = build({"p": FlowDef("p", (GateDef("A", "a", parent_gate),)), "child": FlowDef("child", (GateDef("C", "c", child_gate),))})
    o.start("p", {}, run_id="r1")
    assert o.advance("r1").status == "completed"
    child = o.operational.get_task("r1", "A")["output"]["child"]
    run = o.get_run(child)
    assert run.flow == "child" and run.parent_run_id == "r1" and run.status == "completed"
    assert seen == {"status": "completed", "parent": "r1"}
    assert o.operational.get_task(child, "C")["output"] == {"x": 1}

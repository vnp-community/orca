"""Shared fixtures for the MCP tests: runtime builder, in-memory protocol session, event capture and a small flow."""
from __future__ import annotations

import logging
from contextlib import asynccontextmanager

import pytest
from mcp import Client

from graphs import load_workflow, register_sources
from tdd_agent.config import load_config
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.mcp import build_server
from tdd_agent.obs import context as obs_context
from tdd_agent.orchestrator import FlowDef, GateDef, GateOutcome
from tdd_agent.runtime import Runtime

ACTORS = [{"id": "alice", "kind": "human", "roles": ["BA", "BA/owner"]}, {"id": "bob", "kind": "human", "roles": ["SA/Dev", "SA"]},
          {"id": "claude", "kind": "coding_agent", "roles": ["BA", "SA/Dev", "SA", "BA/owner"]},
          {"id": "limited", "kind": "coding_agent", "roles": ["BA"]}]

READ_TOOLS = ["kg_get_element", "kg_elements", "kg_neighbors", "kg_resolve_scope", "kg_query", "kg_search", "kg_snapshots", "source_get", "impact_get",
              "readiness_get", "run_list", "run_get", "trace_get", "metrics_get", "testspec_get"]
WRITE_TOOLS = ["run_start", "run_resume", "run_dispatch", "run_cancel", "generate_start", "change_start", "testspec_start", "changeset_propose",
               "proposals_list"]


def make_runtime(tmp_path, *, flows=None, load: bool = True, **mcp) -> Runtime:
    config = load_config(overrides={"store": {"knowledge": "memory:", "operational": "memory:"}, "paths": {"work_dir": str(tmp_path)},
                                    "approvals": {"actors": ACTORS}, "mcp": mcp}, env={})
    runtime = Runtime.build(config, flows=flows or {}, providers={"claude_code": FakeProvider({"*": {}})}, configure_logging=False)
    if load:
        load_workflow(runtime.knowledge)
        register_sources(runtime.knowledge)
    return runtime


def cr_flow() -> FlowDef:
    """G1 (BA), G2 (SA/Dev) and G5 (SA) need approval; the other gates pass on their own."""
    def auto(name):
        return lambda ctx: GateOutcome.ok({"gate": name})

    def decide(name, decision):
        return lambda ctx: GateOutcome.ok({"gate": name}, decision=decision)

    return FlowDef("cr", (
        GateDef("G0", "intake", auto("G0")),
        GateDef("G1", "srs_ready", decide("G1", {"srs": "ready"}), approval="BA", depends_on=()),
        GateDef("G2", "impact_approved", decide("G2", {"report": "r1", "entries": [{"tdd_id": "TDD-A", "action": "update"}]}), approval="SA/Dev"),
        GateDef("G3", "tdd_realized", auto("G3")),
        GateDef("G4", "spec_eligible", auto("G4")),
        GateDef("G5", "spec_authored", decide("G5", {"spec": 1}), approval="SA"),
    ))


def trivial_flow() -> FlowDef:
    return FlowDef("cr", (GateDef("G0", "intake", lambda ctx: GateOutcome.ok({"gate": "G0"})),))


class Session:
    """Thin wrapper over the SDK client that returns the structured result of each call."""

    def __init__(self, client: Client):
        self.client = client

    async def __call__(self, tool: str, **arguments) -> tuple[bool, dict]:
        result = await self.client.call_tool(tool, arguments)
        assert result.structured_content is not None, result.content
        return result.is_error, result.structured_content

    async def ok(self, tool: str, **arguments) -> dict:
        is_error, body = await self(tool, **arguments)
        assert not is_error, body
        return body

    async def fail(self, tool: str, **arguments) -> dict:
        is_error, body = await self(tool, **arguments)
        assert is_error and "error" in body, body
        return body["error"]

    async def tool_names(self) -> list[str]:
        return [t.name for t in (await self.client.list_tools()).tools]


@asynccontextmanager
async def connect(runtime: Runtime, **build):
    async with Client(build_server(runtime, **build)) as client:
        yield Session(client)


class EventLog(logging.Handler):
    """Captures runtime log events with the correlation ids that the JSON formatter would add."""

    def __init__(self):
        super().__init__(logging.DEBUG)
        self.items: list[dict] = []

    def emit(self, record: logging.LogRecord) -> None:
        fields = dict(getattr(record, "fields", None) or {})
        self.items.append({"event": record.getMessage(), "level": record.levelname, **obs_context.snapshot(), **fields})

    def named(self, name: str) -> list[dict]:
        return [i for i in self.items if i["event"] == name]


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture
def events():
    logger = logging.getLogger("tdd_agent")
    handler, previous = EventLog(), logger.level
    logger.addHandler(handler)
    logger.setLevel(logging.DEBUG)
    yield handler
    logger.removeHandler(handler)
    logger.setLevel(previous)

"""Shared builders for agent tests: a runtime on memory stores with scripted fake LLM answers."""
from __future__ import annotations

import re
from pathlib import Path

from tdd_agent.config import load_config
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.obs import Observer
from tdd_agent.orchestrator import FlowDef, Orchestrator
from tdd_agent.store.base import SourceDoc
from tdd_agent.store.memory import MemoryKnowledgeStore, MemoryOperationalStore

DATA = Path(__file__).parent / "data"
ACTORS = [{"id": "alice", "kind": "human", "roles": ["BA", "BA/owner"]}, {"id": "bob", "kind": "human", "roles": ["SA/Dev", "SA"]},
          {"id": "claude", "kind": "coding_agent", "roles": ["BA", "SA/Dev", "SA", "BA/owner"]}]


def doc(name: str, role: str, revision: str = "1", doc_id: str | None = None, text: str | None = None) -> SourceDoc:
    return SourceDoc.make(doc_id or name.upper(), revision, role, text if text is not None else (DATA / f"{name}.md").read_text(encoding="utf-8"))


def make(flows: dict[str, FlowDef], handler=None, tmp_path: Path | None = None, overrides: dict | None = None):
    ov = {"approvals": {"actors": ACTORS, "mode": "manual"}}  # most tests exercise the person-in-the-loop path; auto mode has its own tests
    if tmp_path is not None:
        ov["paths"] = {"work_dir": str(tmp_path / "work")}
    for k, v in (overrides or {}).items():
        ov.setdefault(k, {}).update(v)
    config = load_config(overrides=ov, env={})
    ops, know = MemoryOperationalStore(), MemoryKnowledgeStore()
    ob = Observer(ops)
    provider = FakeProvider(handler or {"*": {}})
    gw = LlmGateway(config, ob, {"claude_code": provider})
    return Orchestrator(config, know, ops, gw, ob, flows), know, ops, provider


def prompt_blocks(prompt: str) -> list[tuple[str, str]]:
    """``line:N | (heading) text`` pairs found in an extraction prompt."""
    return re.findall(r"^(line:\d+) \| (?:\([^)]*\)\s*)?(.*)$", prompt, re.M)


def props_items(props: dict) -> list[dict]:
    """The properties of a fact as the model lists them for a strict output schema: key/value pairs, the value as JSON text."""
    import json
    return [{"key": k, "value": json.dumps(v)} for k, v in props.items()]

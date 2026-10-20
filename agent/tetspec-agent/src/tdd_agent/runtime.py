"""Composition root: wires config, stores, observability, LLM gateway and orchestrator.

CLI and MCP server both go through ``Runtime`` so they share one set of stores,
one observer and one approval policy.
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from .config import Config, load_config
from .llm.gateway import LlmGateway
from .obs.observer import Observer
from .orchestrator import FlowDef, Orchestrator
from .retrieval import ClosureService
from .retrieval.queries import QueryService
from .store import open_knowledge_store, open_operational_store
from .store.base import KnowledgeStore, OperationalStore


@dataclass
class Runtime:
    config: Config
    knowledge: KnowledgeStore
    operational: OperationalStore
    observer: Observer
    gateway: LlmGateway
    orchestrator: Orchestrator

    @classmethod
    def build(cls, config: Config | None = None, *, flows: dict[str, FlowDef] | None = None, providers: dict | None = None,
              configure_logging: bool = True) -> "Runtime":
        config = config or load_config()
        config.work_dir.mkdir(parents=True, exist_ok=True)
        knowledge = open_knowledge_store(config.store_url("knowledge"))
        operational = open_operational_store(config.store_url("operational"))
        observer = Observer.from_config(config, operational, configure=configure_logging)
        gateway = LlmGateway(config, observer, providers)
        if flows is None:
            from .agents.flows import default_flows
            flows = default_flows()
        orchestrator = Orchestrator(config, knowledge, operational, gateway, observer, flows)
        return cls(config, knowledge, operational, observer, gateway, orchestrator)

    def closure(self) -> ClosureService:
        return ClosureService(self.knowledge)

    def queries(self) -> QueryService:
        return QueryService(self.knowledge)

    def close(self) -> None:
        self.knowledge.close()
        self.operational.close()

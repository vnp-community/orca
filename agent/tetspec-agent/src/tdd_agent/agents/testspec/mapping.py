"""Mapping table design KG -> tdd-kg/v1 (data in ``mapping.yaml``, SOLUTION sections 6.2, 6.3)."""
from __future__ import annotations

import hashlib
from functools import lru_cache
from pathlib import Path

import yaml

from ...errors import ConfigError

MAPPING_FILE = Path(__file__).with_name("mapping.yaml")
FUNCTIONS = ("ms", "int", "bool", "not", "num", "unit", "list", "str")


@lru_cache(maxsize=4)
def load_mapping(path: str | None = None) -> dict:
    p = Path(path) if path else MAPPING_FILE
    raw = p.read_bytes()
    data = yaml.safe_load(raw)
    _check(data)
    data["sha256"] = hashlib.sha256(raw).hexdigest()
    return data


def fingerprint(mapping: dict | None = None) -> dict:
    """Version + hash of the table; part of the input hash of GT and TS1."""
    m = mapping or load_mapping()
    return {"version": m["version"], "sha256": m["sha256"]}


def tsagent_ontology():
    from tsagent.ingest.tdd_ontology import core_ontology
    return core_ontology()


def _check(data: dict) -> None:
    onto = tsagent_ontology()
    if not isinstance(data, dict) or not isinstance(data.get("version"), int):
        raise ConfigError("mapping.yaml: missing version (integer)", code="TESTSPEC_MAPPING_INVALID")
    problems = []
    for cls, rules in (data.get("classes") or {}).items():
        for i, r in enumerate(rules):
            for to in [r.get("to")] + [d.get("to") for d in r.get("derive", [])]:
                if to not in onto.classes:
                    problems.append(f"classes.{cls}[{i}]: no such tsagent class: {to}")
            for spec in list((r.get("attrs") or {}).values()) + [v for d in r.get("derive", []) for v in (d.get("attrs") or {}).values()]:
                if isinstance(spec, dict) and spec.get("fn") and spec["fn"] not in FUNCTIONS:
                    problems.append(f"classes.{cls}[{i}]: unsupported function: {spec['fn']}")
            for d in r.get("derive", []):
                if d["link"]["rel"] not in onto.relations:
                    problems.append(f"classes.{cls}[{i}].derive: no such tsagent relation: {d['link']['rel']}")
    for rel, cands in (data.get("relations") or {}).items():
        for c in cands:
            if c["to"] not in onto.relations:
                problems.append(f"relations.{rel}: no such tsagent relation: {c['to']}")
    for cls in data.get("extended_passthrough") or []:
        if cls in (data.get("classes") or {}):
            problems.append(f"extended_passthrough.{cls}: already has a rule in classes")
    if problems:
        raise ConfigError("invalid mapping.yaml: " + "; ".join(problems), code="TESTSPEC_MAPPING_INVALID")

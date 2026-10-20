"""Profile-aware expansion: obligations/gaps implied by what the closure contains.

All checks are deterministic.  A missing contract becomes a gap with a stable
code; nothing here ever supplies a value on the design's behalf.
"""
from __future__ import annotations

import re
from typing import Callable

from ..core import o5_path

Gap = dict

WRITE_METHODS = ("POST", "PUT", "PATCH", "DELETE")
PROFILE_VERSION = "1"


def _field(el: dict, name: str):
    props = el.get("props") or {}
    return el.get(name) if el.get(name) is not None else props.get(name)


def _empty(value) -> bool:
    return value is None or value == "" or value == [] or value == {}


def conditional_contract_gaps(elements: list[dict], edges: list[dict]) -> list[Gap]:
    o5 = o5_path.load()
    gaps = []
    for el in elements:
        for name in o5.ontology5.conditional_required(el):
            if _empty(_field(el, name)):
                gaps.append({"type": "missing_contract", "code": "MISSING_CONDITIONAL_FIELD", "element": el["id"], "class": el.get("class"), "field": name})
    return gaps


def integration_gaps(elements: list[dict], edges: list[dict]) -> list[Gap]:
    by_id = {e["id"]: e for e in elements}
    governed = {e["src"] for e in edges if e.get("rel") == "governed_by" and by_id.get(e["dst"], {}).get("class") == "FailurePolicy"}
    external_iface = {i["id"] for i in elements if i.get("class") == "Interface" and _field(i, "external")}
    uses_external = {e["src"] for e in edges if e.get("rel") in ("uses", "exposes", "consumes", "publishes") and e["dst"] in external_iface}
    gaps = []
    for el in elements:
        if el.get("class") == "Interface" and el["id"] in external_iface and el["id"] not in governed:
            # a contract with a system we do not control needs its timeout/retry/fallback/DLQ policy even before an operation uses it
            gaps.append({"type": "missing_contract", "code": "MISSING_FAILURE_POLICY", "element": el["id"], "class": "Interface"})
            continue
        if el.get("class") != "Operation":
            continue
        if (_field(el, "integration") or el["id"] in uses_external) and el["id"] not in governed:
            gaps.append({"type": "missing_contract", "code": "MISSING_FAILURE_POLICY", "element": el["id"], "class": "Operation"})
    return gaps


def migration_gaps(elements: list[dict], edges: list[dict]) -> list[Gap]:
    staged = {e["src"] for e in edges if e.get("rel") == "has_migration_stage"}
    gaps = []
    for el in elements:
        if el.get("class") != "Migration":
            continue
        if _empty(_field(el, "rollback")):
            gaps.append({"type": "missing_contract", "code": "MISSING_ROLLBACK", "element": el["id"], "class": "Migration"})
        if _empty(_field(el, "backfill")):
            gaps.append({"type": "missing_contract", "code": "MISSING_BACKFILL", "element": el["id"], "class": "Migration"})
        if _empty(_field(el, "stages")) and el["id"] not in staged:
            gaps.append({"type": "missing_contract", "code": "MISSING_MIGRATION_STAGES", "element": el["id"], "class": "Migration"})
    return gaps


def recovery_gaps(elements: list[dict], edges: list[dict]) -> list[Gap]:
    has_objective = any(e.get("class") == "RecoveryObjective" for e in elements)
    return [{"type": "missing_contract", "code": "MISSING_RECOVERY_OBJECTIVE", "element": el["id"], "class": el["class"]}
            for el in elements if el.get("class") == "HighAvailabilityTopology" and not has_objective]


_THROUGHPUT = re.compile(r"\d[\d.,]*\s*(?:tps|qps|rps|req/s|requests?/s|tx/s|giao dịch/giây)(?![A-Za-z0-9])", re.I)
_AVAILABILITY = re.compile(r"\d{2}[.,]\d+\s*%|active-active|active/passive|(?<![A-Za-z0-9])HA(?![A-Za-z0-9])|failover|high availability|khả dụng|availability", re.I)


def _quality_text(el: dict) -> str:
    p = el.get("props") or {}
    quote = next((str(x.get("quote") or "") for x in el.get("provenance") or [] if x.get("quote")), "")
    return " ".join([str(el.get("name") or ""), str(p.get("target") or ""), str(p.get("definition") or ""), quote])


def workload_gaps(elements: list[dict], edges: list[dict]) -> list[Gap]:
    """A throughput or availability target stated only as text is not a design: it needs a typed load profile / recovery objective."""
    has_load = any(e.get("class") in ("LoadProfile", "CapacityModel") for e in elements)
    has_recovery = any(e.get("class") in ("RecoveryObjective", "HighAvailabilityTopology") for e in elements)
    gaps = []
    for el in elements:
        if el.get("class") != "QualityTarget":
            continue
        text = _quality_text(el)
        if not has_load and _THROUGHPUT.search(text):
            gaps.append({"type": "missing_contract", "code": "MISSING_LOAD_PROFILE", "element": el["id"], "class": "QualityTarget"})
        if not has_recovery and _AVAILABILITY.search(text):
            gaps.append({"type": "missing_contract", "code": "MISSING_RECOVERY_OBJECTIVE", "element": el["id"], "class": "QualityTarget"})
    return gaps


CHECKS: dict[str, Callable[[list[dict], list[dict]], list[Gap]]] = {
    "conditional-contracts": conditional_contract_gaps,
    "integration": integration_gaps,
    "migration": migration_gaps,
    "operations": recovery_gaps,
    "workload": workload_gaps,
}

PROFILES: dict[str, tuple[str, ...]] = {
    "tdd-generation": ("conditional-contracts", "integration", "migration", "operations", "workload"),
    "spec-change-generation": ("conditional-contracts", "integration", "migration", "workload"),
    "prompt-tdd-edit": ("conditional-contracts",),
    "none": (),
}


def gaps_for(profile: str, elements: list[dict], edges: list[dict]) -> list[Gap]:
    if profile not in PROFILES:
        raise KeyError(profile)
    out: list[Gap] = []
    for name in PROFILES[profile]:
        out.extend(CHECKS[name](elements, edges))
    return sorted(out, key=lambda g: (g["code"], g["element"], g.get("field", "")))

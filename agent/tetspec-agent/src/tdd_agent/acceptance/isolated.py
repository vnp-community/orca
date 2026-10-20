"""Acceptance of each agent in isolation (phase 10).

Runs only after the TDD-generation acceptance report exists.  Each agent runs alone with the real provider; the
other agents are replaced by frozen outputs taken from that report's runs.  Every case is repeated ``k`` times.
"""
from __future__ import annotations

import shutil
import sqlite3
from collections import Counter
from pathlib import Path

from ..config import Config
from ..errors import TddAgentError
from ..runtime import Runtime
from ..store.base import SourceDoc, digest
from . import oracles
from .common import Timer, fresh_runtime, provider_facts, repo_root, usage_summary, write_json

PREREQUISITE = "acceptance-report-tdd.json"


def require_tdd_acceptance(out: Path) -> dict:
    path = out / PREREQUISITE
    if not path.is_file():
        raise TddAgentError(f"isolated agent acceptance needs the TDD-generation acceptance report ({PREREQUISITE}) in {out}", code="ACCEPTANCE_PREREQUISITE_MISSING")
    import json
    return json.loads(path.read_text(encoding="utf-8"))


def seed_runtime(base: Config, src_folder: Path, out: Path, name: str, partition: bool = False) -> tuple[Runtime, dict[str, list[str]]]:
    """A fresh runtime seeded from the knowledge graph and sources of a finished generation run (nothing else is carried over).

    ``partition=True`` splits the single generated TDD into one TDD per requirement group so that impact analysis has several
    documents to choose between (the frozen upstream input of the impact agent).
    """
    import re
    from ..agents.common import render_tdd
    from ..store import open_knowledge_store
    rt = fresh_runtime(base, out, name)
    src = open_knowledge_store(f"sqlite:{src_folder}/knowledge.db")
    try:
        graph = src.materialize()
        docs = src.list_sources()
    finally:
        src.close()
    groups: dict[str, set[str]] = {}
    if partition:
        adj: dict[str, set[str]] = {}
        for e in graph["edges"]:
            adj.setdefault(e["src"], set()).add(e["dst"]); adj.setdefault(e["dst"], set()).add(e["src"])
        for el in graph["elements"]:
            m = re.match(r"^FR-([A-Z]+)-\d+$", el["id"])
            if m:
                g = groups.setdefault(f"TDD-{m.group(1)}", set())
                g.add(el["id"]); g |= adj.get(el["id"], set())
        for el in graph["elements"]:
            el["tdds"] = sorted(t for t, ids in groups.items() if el["id"] in ids)
    rt.knowledge.commit_graph(None, graph, actor="acceptance-fixture", validate=False, meta={"seeded_from": str(src_folder.name)})
    if partition:
        committed = rt.knowledge.materialize()
        for d in docs:
            if d.role != "TDD":
                rt.knowledge.register_source(d)
        for tdd in sorted(groups):
            rt.knowledge.register_source(SourceDoc.make(tdd, "1", "TDD", render_tdd(committed, tdd), {"fixture": "requirement-group partition"}))
    else:
        for d in docs:
            rt.knowledge.register_source(d)
    return rt, {k: sorted(v) for k, v in groups.items()}


def _stability(results: list[bool]) -> str:
    return "stable_pass" if all(results) else ("stable_fail" if not any(results) else "unstable")


def _impact_case(base: Config, out: Path, src: Path, case: dict, rep: int, approved: bool) -> dict:
    rt, _ = seed_runtime(base, src, out, f"isolated-impact-{case['id']}-r{rep}", partition=True)
    try:
        sad = rt.knowledge.list_sources("SAD")[-1]
        delta = {"ref": {"document_id": "SRS", "role": "SRS", "revision": "2", "content_hash": "frozen", "section_id": "delta"},
                 "cr_ref": {"document_id": "CR-ISO", "role": "CR", "revision": "1", "content_hash": "frozen", "section_id": "document"},
                 "changes": [{"rationale": "frozen delta", "section_ref": {"document_id": "SRS", "role": "SRS", "revision": "2", "content_hash": "frozen", "section_id": c["id"]}, **c} for c in case["delta"]["changes"]],
                 "conflicts": []}
        with Timer() as t:
            rt.orchestrator.start("impact-agent", {"mode": "change-request", "params": {"srs_delta": delta}}, run_id="impact", sources=[sad])
            run = rt.orchestrator.advance("impact")
        task = rt.operational.get_task("impact", "G2") or {}
        out_ = task.get("output", {})
        entries = {e["tdd_id"]: e for e in out_.get("report", {}).get("entries", [])}
        exp = case["expect"]
        checks = {"status": run.status == exp["status"]}
        for tdd, action in exp.get("actions", {}).items():
            checks[f"{tdd}={action}"] = entries.get(tdd, {}).get("action") == action
        for tdd in exp.get("never_no_change", []):
            checks[f"dangerous:{tdd}"] = entries.get(tdd, {}).get("action") != "no_change"
        for b in exp.get("blockers", []):
            checks[f"blocker:{b}"] = b in run.result.get("blockers", [])
        if exp.get("every_no_change_has_rule_or_verified_reason"):
            checks["no_change_reasons"] = all(e.get("rationale") and e.get("srs_refs") and e.get("tdd_refs") for e in entries.values() if e["action"] == "no_change")
        verdicts = Counter(out_.get("scan", {}).values())
        return {"case": case["id"], "repetition": rep, "passed": all(checks.values()) if approved else None, "graded": "pass" if all(checks.values()) else "fail" if approved else "not_graded",
                "checks": checks, "run_status": run.status, "blockers": run.result.get("blockers", []), "actions": {k: v["action"] for k, v in entries.items()},
                "scan_verdicts": dict(verdicts), "judgements": sum(len(v) for v in out_.get("judgements", {}).values()), "seconds": t.seconds, "usage": usage_summary(rt.operational, "impact")}
    finally:
        rt.close()


def _edit_graph() -> dict:
    prov = [{"kind": "extracted", "source_id": "SRS", "source_version": "1", "source_hash": "h", "block": "line:1", "quote": "q", "quote_hash": "qh"}]
    def op(i, name, **props):
        return {"id": i, "class": "Operation", "name": name, "origin": "upstream", "maturity": "sketch", "owner_doc": "srs", "tdds": ["TDD-X"], "review_status": "approved",
                "version_introduced": "1", "props": props, "provenance": prov}
    return {"version": "v1", "elements": [op("API-I-01", "Submit SBOM", timeout_seconds=30), op("API-P-01", "SSO login", timeout_seconds=10)], "edges": []}


def _prompt_case(base: Config, out: Path, src: Path, case: dict, rep: int, approved: bool) -> dict:
    name = f"isolated-edit-{case['id']}-r{rep}"
    if case["kb"] == "pbom":
        rt, _ = seed_runtime(base, src, out, name)
    else:
        rt = fresh_runtime(base, out, name)
        rt.knowledge.commit_graph(None, _edit_graph(), actor="acceptance-fixture")
        rt.knowledge.register_source(SourceDoc.make("TDD-X", "1", "TDD", "# Technical Design Document\n"))
    try:
        head = rt.knowledge.head()
        before = digest(rt.knowledge.materialize(head.id)["elements"])
        params = {"prompt": case["prompt"], "target": case["target"], "authorization": case["auth"]}
        if case.get("base_snapshot"):
            params["base_snapshot"] = case["base_snapshot"]
        with Timer() as t:
            rt.orchestrator.start("prompt-edit", {"params": params}, run_id="edit")
            run = rt.orchestrator.advance("edit")
        task = rt.operational.get_task("edit", "P1") or {}
        out_ = task.get("output", {})
        outcome = out_.get("status") or ("blocked" if run.status == "blocked" else run.status)
        mutated = digest(rt.knowledge.materialize()["elements"]) != before
        exp = case["expect"]
        checks = {"mutation": mutated == exp["mutates"], "outcome": outcome in exp["outcome_in"]}
        if exp.get("blockers_any"):
            checks["blocker"] = bool(set(exp["blockers_any"]) & set(run.result.get("blockers", []))) or outcome == "clarification_required"
        if exp.get("property"):
            pr = exp["property"]
            checks["value"] = (rt.knowledge.get_element(pr["id"]) or {}).get("props", {}).get(pr["path"]) == pr["value"]
        return {"case": case["id"], "repetition": rep, "passed": all(checks.values()) if approved else None, "graded": "pass" if all(checks.values()) else "fail" if approved else "not_graded",
                "checks": checks, "outcome": outcome, "blockers": run.result.get("blockers", []), "classification": (out_.get("classification") or {}).get("kind"), "mutated": mutated,
                "seconds": t.seconds, "usage": usage_summary(rt.operational, "edit")}
    finally:
        rt.close()


def _changespec_case(base: Config, out: Path, spec: dict, rep: int, approved: bool, *, finish: bool = False, name: str | None = None) -> dict:
    """G4 (table) and G5 (authoring + critic + strict validation) with frozen upstream approvals on the synthetic payment fixture."""
    from ..orchestrator import ApprovalRequest
    from .generation import sources_for
    folder = name or f"isolated-changespec-r{rep}"
    rt = fresh_runtime(base, out, folder)
    repo = out / folder / "repo"
    (repo / "openspec" / "specs").mkdir(parents=True); (repo / "openspec" / "changes").mkdir()
    (repo / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    actor = "coding-agent"
    try:
        urd, srs1, sad = sources_for("second")
        srs2 = SourceDoc.make("SRS", "2", "SRS", srs1.text.replace("| FR-PAY-03 |", "| FR-PAY-04 | The system SHALL send a payment receipt after capture. | | AC-4 |\n| FR-PAY-03 |", 1)
                              .replace("## 5. Interfaces", "## 4b. Components\n| ID | Name | Satisfies |\n|---|---|---|\n| SVC-2 | Receipt service | FR-PAY-04 |\n\n## 5. Interfaces", 1))
        srs1 = SourceDoc.make("SRS", "1", "SRS", srs1.text)
        rt.orchestrator.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": "TDD-PAY", "prose": False}}, run_id="base", sources=[urd, srs1, sad])
        rt.orchestrator.advance("base")
        params = {"srs": {"id": "SRS", "before": "1", "after": "2"}, "cr": {"id": "CR-PAY-4", "revision": "1", "title": "Payment receipts", "acceptance": ["AC-4"]}, "change_name": "payment-receipts",
                  "repo_root": str(repo), "tdd_for_new": {"FR-PAY-04": "TDD-PAY", "SVC-2": "TDD-PAY"}, "capability": {"purpose": "Send receipts", "scope": "Receipt sending", "owner": "payments"},
                  "code_targets": {"FR-PAY-04": "src/receipt.py::send_receipt", "SVC-2": "src/receipt_service.py::ReceiptService"},
                  "verifications": {"FR-PAY-04": "GIVEN a captured payment WHEN capture completes THEN a receipt is sent", "SVC-2": "GIVEN the receipt service WHEN a receipt is requested THEN it is delivered"}}
        rt.orchestrator.start("change-request", {"mode": "change-request", "params": params}, run_id="cs", sources=[srs1, srs2, sad])
        with Timer() as t:
            rt.orchestrator.advance("cs")
            for gate, role in (("G1", "BA"), ("G2", "SA/Dev")):
                d = rt.orchestrator.get_decision("cs", gate, actor, source="acceptance")
                rt.orchestrator.submit_approval(ApprovalRequest("cs", gate, actor, role, d["decision_hash"], "isolated acceptance: frozen upstream decision"))
            pending = [e["id"] for e in rt.orchestrator.elements_needing_review()]
            if pending:
                rt.orchestrator.review_elements(pending, "approved", actor, "SA", "isolated acceptance: facts of the synthetic fixture match its sources")
            run = rt.orchestrator.advance("cs")
        g4, g5 = rt.operational.get_task("cs", "G4") or {}, rt.operational.get_task("cs", "G5") or {}
        exp = spec["expect"]
        table = (g4.get("output") or {}).get("change_spec", {})
        checks = {"table": table.get("status") == exp["table_status"] and len(table.get("items", [])) == exp["rows"],
                  "published_to_staging": run.status == "awaiting_approval" and (run.result or {}).get("pending_gate") == "G5", "no_blockers": not (run.result or {}).get("blockers")}
        manifest = ((g5.get("decision") or {}).get("trace_manifest")) or {}
        checks["critic"] = (manifest.get("semantic_review") or {}).get("outcome") == exp["critic"]
        checks["strict_validation"] = bool((manifest.get("cli_validation") or {}).get("ok"))
        final = {}
        if finish:
            final = {"g5_approved": None, "published": False, "openspec_validate_strict": None, "after_publication": None}
            if run.status == "awaiting_approval" and run.result.get("pending_gate") == "G5":
                d = rt.orchestrator.get_decision("cs", "G5", actor, source="acceptance")
                res = rt.orchestrator.submit_approval(ApprovalRequest("cs", "G5", actor, "SA", d["decision_hash"], "e2e acceptance: staged change reviewed (agent grade)"))
                final["g5_approved"] = res.accepted
                target = repo / "openspec" / "changes" / "payment-receipts"
                final["published"] = target.is_dir()
                if final["published"]:
                    import subprocess
                    v = subprocess.run(["openspec", "validate", "payment-receipts", "--strict", "--no-interactive"], cwd=repo, capture_output=True, text=True)
                    final["openspec_validate_strict"] = {"returncode": v.returncode, "stdout": v.stdout[-200:]}
                after = rt.orchestrator.get_run("cs")
                final["after_publication"] = {"status": after.status, "state": after.state, "blockers": (after.result or {}).get("blockers")}
                final["approvals"] = [{k: a[k] for k in ("gate", "actor_id", "approval_grade")} for a in rt.operational.list_approvals("cs")]
                final["provider"] = rt.gateway.describe(); final["config_hash"] = rt.config.fingerprint()
        return {"case": "changespec-payment-receipts", **({"final": final} if finish else {}), "repetition": rep, "passed": all(checks.values()) if approved else None, "graded": "pass" if all(checks.values()) else "fail" if approved else "not_graded",
                "checks": checks, "run_status": run.status, "blockers": (run.result or {}).get("blockers", []), "failing_rules": (g5.get("output") or {}).get("failing_rules"),
                "seconds": t.seconds, "usage": usage_summary(rt.operational, "cs"), "synthetic": True}
    finally:
        rt.close()


def run_isolated(base: Config, out: Path, agents: list[str], repetitions: int) -> dict:
    prereq = require_tdd_acceptance(out)
    approval = oracles.approval_for(out, "isolated-agents")
    oracle = oracles.load("isolated-agents")
    approved = approval is not None
    pbom_src = out / "generation-pbom-r1"
    frozen = {"tdd_acceptance_report": PREREQUISITE, "tdd_acceptance_hash": digest(prereq), "pbom_kg_head": None}
    if pbom_src.is_dir():
        from ..store import open_knowledge_store
        k = open_knowledge_store(f"sqlite:{pbom_src}/knowledge.db")
        frozen["pbom_kg_head"] = k.head().hash if k.head() else None
        k.close()
    previous = out / "isolated-agents.json"
    import json
    report: dict = {"task": "10.4", "repetitions": repetitions, "oracle_approval": approval, "frozen_inputs": frozen, "agents": {}}
    if previous.is_file():  # agents run earlier stay in the report unless they are run again
        report["agents"] = json.loads(previous.read_text(encoding="utf-8")).get("agents", {})
    if "generation" in agents:
        from .generation import run_one, INPUTS
        gen = {}
        for name in ("pbom", "prose", "second"):
            ap = oracles.approval_for(out, INPUTS[name]["oracle"])
            runs = [run_one(base, out, name, 100 + r, ap) for r in range(1, repetitions + 1)]
            gen[name] = {"runs": [{k: v for k, v in r.items() if k in ("repetition", "graded", "run_status", "seconds", "usage", "readiness", "kg")} for r in runs],
                         "stability": _stability([r.get("graded") == "pass" for r in runs]), "failed_checks": sorted({c["id"] for r in runs for c in r.get("checks", []) if not c["passed"]})}
        report["agents"]["generation"] = gen
    if "impact" in agents:
        cases = {}
        for case in oracle["impact"]:
            runs = [_impact_case(base, out, pbom_src, case, r, approved) for r in range(1, repetitions + 1)]
            cases[case["id"]] = {"runs": runs, "stability": _stability([r["graded"] == "pass" for r in runs]),
                                 "dangerous_errors": sum(1 for r in runs for k, v in r["checks"].items() if k.startswith("dangerous:") and not v)}
        report["agents"]["impact"] = cases
    if "promptedit" in agents:
        cases = {}
        for case in oracle["promptedit"]:
            runs = [_prompt_case(base, out, pbom_src, case, r, approved) for r in range(1, repetitions + 1)]
            cases[case["id"]] = {"runs": runs, "stability": _stability([r["graded"] == "pass" for r in runs]), "unexpected_mutations": sum(1 for r in runs if r["checks"].get("mutation") is False)}
        report["agents"]["promptedit"] = cases
    if "changespec" in agents:
        runs = [_changespec_case(base, out, oracle["changespec"], r, approved) for r in range(1, repetitions + 1)]
        report["agents"]["changespec"] = {"runs": runs, "stability": _stability([r["graded"] == "pass" for r in runs])}
    write_json(out / "isolated-agents.json", report)
    return report

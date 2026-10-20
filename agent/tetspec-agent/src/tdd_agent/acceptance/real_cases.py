"""Phase 11: the ten use cases of the design (section 9) on the real PBOM documents with the real provider.

Inputs are the PBOM URD/SRS/SAD, either untouched (case 1) or with one or two rows added/edited the way a change request would
(each edit is listed in the oracle ``input`` field and in the result).  The baseline TDD/KG is what the generation agent produced
from those documents with the real provider.  Nothing is scripted: no ``FakeProvider``, no synthetic fixture.

Expectations live in ``oracles/real-cases.json`` (written from the design document before any run, approved by an allowlisted
actor).  A case ``fail``s only on its *safety* checks; its *design* checks decide ``pass`` vs ``partial`` (or ``blocked_by_data``
when the only unmet design checks depend on information PBOM does not contain).  Results are never adjusted after a run.
"""
from __future__ import annotations

import json
import re
import shutil
import subprocess
from pathlib import Path
from typing import Any

from ..config import Config
from ..core import o5_path
from ..errors import TddAgentError
from ..orchestrator import ApprovalRequest
from ..retrieval.queries import QueryService
from ..store.base import SourceDoc, digest
from . import oracles
from .common import Timer, fresh_runtime, pbom_sources, repo_root, usage_summary, write_json
from .isolated import seed_runtime

ORACLE = "real-cases"
ACTOR = "coding-agent"
TDD_ID = "TDD-PBOM"


# ------------------------------------------------------------------ grading
def _get(obs: Any, path: str) -> Any:
    node = obs
    for part in path.split("."):
        if isinstance(node, dict) and part in node:
            node = node[part]
        else:
            return None
    return node


def _text(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, default=str).lower()


def evaluate_check(check: dict, obs: dict) -> dict:
    kind, got = check["kind"], _get(obs, check["path"])
    if kind == "eq":
        ok = got == check["value"]
    elif kind == "in":
        ok = got in check["values"]
    elif kind == "not_in":
        ok = got not in check["values"]
    elif kind == "set_eq":
        ok = isinstance(got, list) and set(got) == set(check["values"])
    elif kind == "subset":
        ok = isinstance(got, list) and set(got) <= set(check["values"])
    elif kind == "contains_any":
        ok = isinstance(got, list) and bool(set(got) & set(check["values"]))
    elif kind == "truthy":
        ok = bool(got)
    elif kind == "falsy":
        ok = not got
    elif kind == "ge":
        ok = isinstance(got, (int, float)) and got >= check["n"]
    else:
        raise TddAgentError(f"unknown real-case check kind {kind}", code="ORACLE_CHECK_UNKNOWN")
    return {"id": check["id"], "level": check["level"], "passed": bool(ok), "observed": got if not isinstance(got, (dict, list)) or len(_text(got)) < 400 else "<large>", "data_limited": bool(check.get("data_limited"))}


def grade(case: dict, obs: dict) -> dict:
    checks = [evaluate_check(c, obs) for c in case["checks"]]
    safety_bad = [c for c in checks if c["level"] == "safety" and not c["passed"]]
    design_bad = [c for c in checks if c["level"] == "design" and not c["passed"]]
    if safety_bad:
        verdict = "fail"
    elif not design_bad:
        verdict = "pass"
    elif all(c["data_limited"] for c in design_bad) and _get(obs, "g5.status") == "blocked":
        verdict = "blocked_by_data"
    else:
        verdict = "partial"
    return {"verdict": verdict, "checks": checks, "safety_failed": [c["id"] for c in safety_bad], "design_unmet": [c["id"] for c in design_bad]}


# ------------------------------------------------------------------ inputs (PBOM rows)
def srs_variant(srs: SourceDoc, revision: str, *, replace: list[tuple[str, str]] = (), insert_after: tuple[str, str] | None = None) -> SourceDoc:
    text = srs.text
    for old, new in replace:
        if text.count(old) < 1:
            raise TddAgentError(f"PBOM SRS no longer contains the text this case edits: {old[:60]!r}", code="CASE_INPUT_DRIFT")
        text = text.replace(old, new)
    if insert_after:
        marker, row = insert_after
        lines = text.split("\n")
        idx = next((i for i, line in enumerate(lines) if line.startswith(marker)), None)
        if idx is None:
            raise TddAgentError(f"PBOM SRS has no line starting with {marker!r}", code="CASE_INPUT_DRIFT")
        lines.insert(idx + 1, row)
        text = "\n".join(lines)
    return SourceDoc.make("SRS", revision, "SRS", text)


def _in_rows(text: str, ids: tuple[str, ...], old: str, new: str) -> list[tuple[str, str]]:
    """(old_row, new_row) pairs for the rows starting with the given ids that contain ``old``."""
    pairs = []
    for line in text.split("\n"):
        if any(line.startswith(f"| {i} |") for i in ids) and old in line:
            pairs.append((line, line.replace(old, new)))
    if len(pairs) != len(ids):
        raise TddAgentError(f"expected rows {ids} to contain {old!r}", code="CASE_INPUT_DRIFT")
    return pairs


FR_AST_20 = ("| FR-AST-20 | Hệ thống phải lưu chứng từ (evidence) của Entitlement thuê bao tối thiểu 5 năm sau ngày hết hạn; chỉ SEC và PROC được xem | M | UC-34 | BRL-46 | ENT-35, ENT-43 | API-S-25..33 | "
             "\\[T\\] Chứng từ không bị xoá trong 5 năm sau `valid_until`; người dùng khác SEC, PROC nhận 403 |")
EIF_20 = "| EIF-20 | ServiceNow REST API. Ra: tạo ticket khi có Finding mức Critical | HTTPS `/api/now/table/incident`, JSON | Token ở kho bí mật (`credential_ref`) | Finding id, severity, project, liên kết | — | FR-VUL-04, API-G-40 |"
NFR_PE_09 = "| NFR-PE-09 | Gate evaluate khi tải đỉnh | Evaluate chịu 500 TPS đỉnh, p99 ≤ 300 ms; khả dụng 99,95% theo mô hình active-active | A | Must | UC-05, UC-31; API-C-01 |"
FR_EVT_01 = ("| FR-EVT-01 | Hệ thống phải phát sự kiện nghiệp vụ qua NATS JetStream thay cho transactional outbox của ADR-08 | M | UC-05 | BRL-01 | ENT-17 | API-I-01 | "
             "\\[T\\] Sự kiện được phát qua NATS JetStream, không qua bảng `event_outbox` |")


def cr_params(change: str, cr_id: str, title: str, acceptance: list[str], repo: Path, targets: dict, verifications: dict, **extra) -> dict:
    return {"srs": {"id": "SRS", "before": "1.0", "after": extra.pop("after", "2.0")}, "cr": {"id": cr_id, "revision": "1", "title": title, "acceptance": acceptance}, "change_name": change,
            "repo_root": str(repo), "capability": {"purpose": title, "scope": title, "owner": "pbom-team"}, "code_targets": targets, "verifications": verifications,
            "tdd_for_new": extra.pop("tdd_for_new", {}), **extra}


# ------------------------------------------------------------------ observations
def _tasks(rt, run_id: str) -> dict:
    return {g: (rt.operational.get_task(run_id, g) or {}) for g in ("G0", "G1", "G2", "G3", "G4", "G5")}


def _queries(rt, scope_ids: list[str]) -> dict:
    out: dict = {}
    svc = QueryService(rt.knowledge)
    for name in ("Q2", "Q4", "Q5", "Q9"):
        try:
            res = svc.run(name).to_dict()
            rows = res.get("rows", [])
            hits = [r for r in rows if any(i in _text(r) for i in [s.lower() for s in scope_ids])] if scope_ids else []
            out[name.lower()] = {"count": len(rows), "scope_hits": len(hits), "sample": hits[:5] or rows[:3]}
        except Exception as exc:  # a query the registry cannot answer is itself a result
            out[name.lower()] = {"error": f"{type(exc).__name__}: {exc}"[:200]}
    return out


def observe_change(rt, run_id: str, scope_ids: list[str], published: bool, head_before: str | None) -> dict:
    run = rt.orchestrator.get_run(run_id)
    t = _tasks(rt, run_id)
    out1, out2, out3, out4, out5 = (t[g].get("output") or {} for g in ("G1", "G2", "G3", "G4", "G5"))
    changes = (out1.get("srs_delta") or {}).get("changes", [])
    entries = (out2.get("report") or {}).get("entries", [])
    no_change = [e for e in entries if e["action"] == "no_change"]
    rows = (out4.get("change_spec") or {}).get("items", [])
    row_ids = sorted({r["srs_refs"][0]["section_id"] for r in rows if r.get("srs_refs")})
    traced = bool(rows) and all(r.get("srs_refs") and r.get("tdd_refs") and r.get("cr_acceptance_refs") for r in rows)
    sad = out2.get("sad_cr_proposal") or {}
    failing = out5.get("failing_rules") or {}
    lines = [f"{rule}:{item}" for rule, items in failing.items() for item in (items if isinstance(items, list) else [items])]
    queries = _queries(rt, scope_ids)
    g5_status = (t["G5"].get("status") if t["G5"] else None)
    atomic = [line for rule, items in failing.items() for item in (items if isinstance(items, list) else [items]) for line in [f"{rule}:{item}"] if not str(item).startswith(("QG-", "TS-"))]
    scope_l = [x.lower() for x in scope_ids]
    q_rows = [_text(g) for g in rt.knowledge.materialize().get("gaps", []) if str(g.get("target", "")).lower() in scope_l] + [_text(r) for q in ("q2", "q4", "q5") for r in (queries.get(q, {}).get("sample") or [])]

    def keyword_hits(words):
        pattern = re.compile(r"(?<![A-Za-z0-9])(?:" + "|".join(re.escape(w) for w in words) + r")(?![A-Za-z0-9])", re.I)  # identifiers such as MISSING_FAILURE_POLICY count
        return [line for line in atomic + q_rows if any(s.lower() in line.lower() for s in scope_ids) and pattern.search(line.replace("_", " "))]
    head = rt.knowledge.head()
    rationale_text = _text([e.get("rationale") for e in entries]) + _text([e.get("evidence") for e in entries])
    return {
        "run": {"status": run.status, "state": run.state, "blockers": list((run.result or {}).get("blockers", [])), "blocked_gate": (run.result or {}).get("blocked_gate"), "pending_gate": (run.result or {}).get("pending_gate")},
        "gates": {g: (v.get("status") if v else None) for g, v in t.items()},
        "delta": {"ids": sorted(c["id"] for c in changes), "kinds": {c["id"]: c["kind"] for c in changes}, "parameter_only": {c["id"]: bool(c.get("parameter_only")) for c in changes}},
        "impact": {"status": t["G2"].get("status"), "actions": {e["tdd_id"]: e["action"] for e in entries}, "reasons": {e["tdd_id"]: e.get("rationale") for e in entries},
                   "no_change_proven": all(e.get("rationale") and e.get("srs_refs") and e.get("tdd_refs") for e in no_change),
                   "sad_cr": bool(sad), "sad_cr_mentions_adr08": any(c.get("sad_element") == "ADR-08" for c in sad.get("conflicts", [])),
                   "capacity_obligation": any(w in rationale_text for w in ("capacity", "outside the range", "out of range")), "judgements": sum(len(v) for v in (out2.get("judgements") or {}).values())},
        "g3": {"actions": {m["tdd_id"]: m["action"] for m in (out3.get("tdd_set") or {}).get("mappings", [])}},
        "g4": {"reached": bool(t["G4"]), "status": (out4.get("change_spec") or {}).get("status"), "row_ids": row_ids, "rows_count": len(rows), "rows_traced": traced,
               "rows_traced_or_none": traced or not rows, "rows": [{"id": r["id"], "action": r.get("action"), "code_target": r.get("code_target")} for r in rows]},
        "g5": {"status": g5_status, "blockers": sorted(run.result.get("blockers", [])) if g5_status == "blocked" else [], "rules": sorted(failing), "lines_sample": lines[:12],
               "data_migration_surfaced": bool(keyword_hits(("data schema", "migration", "backfill", "rollback", "physical", "expand"))),
               "failure_policy_surfaced": bool(keyword_hits(("failure policy", "failure_policy", "retry", "timeout", "circuit", "dlq", "idempot", "fallback"))),
               "load_ha_surfaced": bool(keyword_hits(("rto", "rpo", "recovery objective", "failover", "replica", "capacity model", "load profile", "peak", "payload", "growth", "availability", "topology")))},
        "published": published, "queries": queries,
        "kg": {"head_before": head_before, "head_after": head.id if head else None},
    }


def drive_change(rt, run_id: str, flow_params: dict, sources: list[SourceDoc], expect_delta: set[str] | None, scope_ids: list[str], *, approve_g5: bool = True) -> tuple[dict, dict]:
    """Start the change-request flow and approve G1/G2 as the coding agent when the G1 delta is exactly what the case expects."""
    o = rt.orchestrator
    head_before = rt.knowledge.head().id
    notes: list[str] = []
    published = False
    o.start("change-request", {"mode": "change-request", "params": flow_params}, run_id=run_id, sources=sources)
    for _ in range(8):
        run = o.advance(run_id)
        if run.status != "awaiting_approval":
            break
        gate = run.result["pending_gate"]
        if gate == "G1" and expect_delta is not None:
            ids = {c["id"] for c in (o.get_decision(run_id, "G1", ACTOR, source="acceptance")["decision"].get("srs_delta") or {}).get("changes", [])}
            if ids != expect_delta:
                notes.append(f"G1 approval withheld: delta {sorted(ids)} != expected {sorted(expect_delta)}")
                break
        if gate in ("G1", "G2") or (gate == "G5" and approve_g5):
            role = {"G1": "BA", "G2": "SA/Dev", "G5": "SA"}[gate]
            d = o.get_decision(run_id, gate, ACTOR, source="acceptance")
            res = o.submit_approval(ApprovalRequest(run_id, gate, ACTOR, role, d["decision_hash"], f"real-case acceptance: {gate} decision matches the case input (agent grade)"))
            notes.append(f"{gate} approved={res.accepted}")
            if gate == "G5":
                published = Path(flow_params["repo_root"], "openspec", "changes", flow_params["change_name"]).is_dir()
                break
            continue
        break
    obs = observe_change(rt, run_id, scope_ids, published, head_before)
    return obs, {"notes": notes}


def _repo(folder: Path) -> Path:
    repo = folder / "repo"
    (repo / "openspec" / "specs").mkdir(parents=True, exist_ok=True)
    (repo / "openspec" / "changes").mkdir(parents=True, exist_ok=True)
    (repo / "openspec" / "config.yaml").write_text("schema: spec-driven\n")
    return repo


# ------------------------------------------------------------------ generation observation
SRS_ID = re.compile(r"^\| ((?:N?FR)-[A-Z]+-\d+) \|", re.M)


def observe_generation(rt, run, docs: list[SourceDoc], output: dict) -> dict:
    o5 = o5_path.load()
    graph = rt.knowledge.materialize()
    srs = next(d for d in docs if d.role == "SRS")
    expected = set(SRS_ID.findall(srs.text))
    present = {e["id"] for e in graph["elements"]}
    src = [{"id": d.id, "version": d.revision, "content": d.text} for d in docs]
    bad_ev = [e["id"] for e in graph["elements"] for ev in e.get("provenance", []) if ev.get("kind") == "extracted" and o5.verify_evidence(ev, src)]
    invented = [e["id"] for e in graph["elements"] if e.get("origin") in ("upstream", "external") and not e.get("provenance")]
    questions = output.get("questions", [])
    asked = {q["gap"] for q in questions}
    blockers = output.get("blockers", [])
    corpus = _text([[p.get("quote") for p in e.get("provenance", [])] for e in graph["elements"]]) + _text(graph.get("gaps", [])) + _text(questions)
    pins = rt.operational.get_run(run.run_id).request.get("pins", [])
    return {"gen": {"status": output.get("status"), "blockers": blockers, "evidence_ok": not bad_ev, "invented_ok": not invented, "roundtrip_ok": bool((output.get("roundtrip") or {}).get("ok")),
                    "blockers_have_questions": {f"NEED-{b}" for b in blockers} <= asked, "source_roles": sorted({p["role"] for p in pins}),
                    "requirement_coverage": round(len(expected & present) / max(1, len(expected)), 4), "requirements_expected": len(expected),
                    "state_machines": sum(1 for e in graph["elements"] if e.get("class") == "StateMachine"), "quality_targets": sum(1 for e in graph["elements"] if e.get("class") == "QualityTarget"),
                    "trace_fr_ast_13": any(e["src"] == "FR-AST-13" and e["dst"] == "BRL-48" and e["rel"] == "traces_to" for e in graph["edges"]),
                    "api_gaps": sum(1 for g in graph.get("gaps", []) if g.get("need") == "definition" and str(g.get("target", "")).startswith("API-")),
                    "gaps": len(graph.get("gaps", [])), "elements": len(graph["elements"]), "questions": len(questions),
                    "surfaced": {"migration": bool(re.search(r"expand|migrat|backfill", corpus)), "load_ha": bool(re.search(r"\brpo\b|\brto\b|khả dụng|failover|replica", corpus))}}}


# ------------------------------------------------------------------ cases
def case_1_baseline(base: Config, out: Path) -> tuple[dict, Path]:
    rt = fresh_runtime(base, out, "real-base")
    try:
        docs = pbom_sources(repo_root())
        with Timer() as t:
            rt.orchestrator.start("tdd-generation", {"mode": "initial", "params": {"tdd_id": TDD_ID, "prose": True, "prose_limit": 40}}, run_id="base", sources=docs)
            run = rt.orchestrator.advance("base")
        output = (rt.operational.get_task("base", "G3") or {}).get("output", {})
        obs = observe_generation(rt, run, docs, output)
        obs["run"] = {"status": run.status, "blockers": run.result.get("blockers", [])}
        obs["seconds"], obs["usage"] = t.seconds, usage_summary(rt.operational, "base")
        return obs, out / "real-base"
    finally:
        rt.close()


def _edit_case(rt, run_id: str, prompt: str, target: list[str], scope: list[str] | None = None) -> dict:
    before = digest(rt.knowledge.materialize()["elements"])
    head = rt.knowledge.head().id
    params = {"prompt": prompt, "target": target, "authorization": {"scope": scope or ["*"], "operations": ["modify"]}}
    rt.orchestrator.start("prompt-edit", {"params": params}, run_id=run_id)
    run = rt.orchestrator.advance(run_id)
    out = (rt.operational.get_task(run_id, "P1") or {}).get("output", {})
    outcome = out.get("status") or ("blocked" if run.status == "blocked" else run.status)
    mutated = digest(rt.knowledge.materialize()["elements"]) != before or rt.knowledge.head().id != head
    return {"edit": {"outcome": outcome, "reason": out.get("reason"), "blockers": list((run.result or {}).get("blockers", [])), "mutated": mutated, "semantic_changed": mutated,
                     "classification": (out.get("classification") or {}).get("kind"), "question": out.get("question"), "candidates": out.get("candidates"),
                     "overlay": bool(out.get("overlay"))}}


def run_case(case_id: str, base: Config, out: Path, base_folder: Path, docs: list[SourceDoc]) -> dict:
    srs1 = next(d for d in docs if d.role == "SRS")
    sad = next(d for d in docs if d.role == "SAD")
    name = f"real-{case_id}"
    rt, _ = seed_runtime(base, base_folder, out, name)
    folder = out / name
    repo = _repo(folder)
    extra: dict = {}
    try:
        with Timer() as t:
            if case_id == "case-2":
                srs2 = srs_variant(srs1, "2.0", insert_after=("| FR-AST-19 |", FR_AST_20))
                params = cr_params("entitlement-evidence-retention", "CR-0001", "Lưu chứng từ Entitlement 5 năm", ["AC-EVID-5Y"], repo, {"FR-AST-20": "internal/entitlement/evidence_retention.go::Retain"},
                                   {"FR-AST-20": "GIVEN một Entitlement thuê bao đã hết hạn 5 năm trừ một ngày WHEN job dọn dẹp chạy THEN chứng từ vẫn còn và chỉ SEC, PROC xem được"}, tdd_for_new={"FR-AST-20": TDD_ID})
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"FR-AST-20"}, ["FR-AST-20"])
            elif case_id in ("case-4a", "case-4b"):
                new_val = "15.000" if case_id == "case-4a" else "40.000"
                pairs = []
                for ids in (("NFR-PE-01", "FR-INV-02"),):
                    for line in srs1.text.split("\n"):
                        if any(line.startswith(f"| {i} |") for i in ids) and "20.000" in line:
                            pairs.append((line, line.replace("20.000", new_val)))
                if len(pairs) != 2:
                    raise TddAgentError(f"expected NFR-PE-01 and FR-INV-02 to carry the 20.000 limit, found {len(pairs)}", code="CASE_INPUT_DRIFT")
                srs2 = SourceDoc.make("SRS", "2.0", "SRS", srs1.text.replace(pairs[0][0], pairs[0][1]).replace(pairs[1][0], pairs[1][1]))
                params = cr_params("sbom-max-components", "CR-LIMIT", f"limits.sbom_max_components = {new_val}", ["AC-LIMIT"], repo, {"NFR-PE-01": "config/limits.yaml::limits.sbom_max_components", "FR-INV-02": "config/limits.yaml::limits.sbom_max_components"},
                                   {i: f"GIVEN một SBOM có {new_val} component WHEN nộp THEN không trả 413 và vượt giá trị này trả 413" for i in ("NFR-PE-01", "FR-INV-02")})
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"NFR-PE-01", "FR-INV-02"}, ["NFR-PE-01", "FR-INV-02"])
            elif case_id == "case-5":
                pairs = _in_rows(srs1.text, ("FR-AST-12",), "dự án)", "dự án), mã trung tâm chi phí `cost_center` bắt buộc cho mọi Entitlement (điền cho dữ liệu hiện có)")
                srs2 = srs_variant(srs1, "2.0", replace=pairs)
                params = cr_params("entitlement-cost-center", "CR-SCHEMA", "Thêm cost_center cho Entitlement", ["AC-COSTCENTER"], repo, {"FR-AST-12": "internal/entitlement/store.go::Entitlement"},
                                   {"FR-AST-12": "GIVEN một Entitlement không có cost_center WHEN lưu THEN bị từ chối"})
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"FR-AST-12"}, ["FR-AST-12", "ENT-35"])
            elif case_id in ("case-6", "case-6b"):
                srs2 = srs_variant(srs1, "2.0", insert_after=("| EIF-19 |", EIF_20))
                params = cr_params("servicenow-integration", "CR-INTEG", "Tích hợp ServiceNow", ["AC-SNOW"], repo, {"EIF-20": "internal/integration/servicenow.go::CreateIncident"},
                                   {"EIF-20": "GIVEN một Finding Critical WHEN được tạo THEN một ticket ServiceNow được tạo"},
                                   **({"id_class_map": {"EIF": "Interface"}, "tdd_for_new": {"EIF-20": TDD_ID}} if case_id == "case-6b" else {}))
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"EIF-20"}, ["EIF-20", "ServiceNow"])
            elif case_id == "case-7":
                srs2 = srs_variant(srs1, "2.0", insert_after=("| NFR-PE-08 |", NFR_PE_09))
                params = cr_params("gate-peak-load", "CR-LOAD", "Gate evaluate chịu tải đỉnh và HA", ["AC-LOAD"], repo, {"NFR-PE-09": "internal/gate/evaluate.go::Evaluate"},
                                   {"NFR-PE-09": "GIVEN 500 TPS đỉnh WHEN evaluate THEN p99 ≤ 300 ms"})
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"NFR-PE-09"}, ["NFR-PE-09"])
            elif case_id == "case-9":
                srs2 = srs_variant(srs1, "2.0", insert_after=("| FR-INV-03 |", FR_EVT_01))
                params = cr_params("nats-jetstream-events", "CR-ARCH", "Phát sự kiện qua NATS JetStream", ["AC-NATS"], repo, {"FR-EVT-01": "internal/events/publisher.go::Publish"}, {"FR-EVT-01": "GIVEN một sự kiện WHEN phát THEN đi qua NATS JetStream"},
                                   tdd_for_new={"FR-EVT-01": TDD_ID})
                obs, extra = drive_change(rt, "cr", params, [srs1, srs2, sad], {"FR-EVT-01"}, ["FR-EVT-01", "ADR-08"])
            elif case_id == "case-10a":
                obs, extra = _case_10a(rt, srs1, sad, repo)
            elif case_id == "case-10b":
                obs, extra = _case_10b(rt, srs1, sad, repo)
            elif case_id == "case-3a":
                obs, extra = _case_3a(rt, docs)
            elif case_id == "case-3b":
                obs = _edit_case(rt, "edit", "Sửa timeout của API nộp SBOM (API-I-01) thành 30 giây", [])
            elif case_id == "case-8a":
                obs = _edit_case(rt, "edit", "Đổi API-P-01 thành Submit SBOM, timeout 30s", [])
            elif case_id == "case-8b":
                obs = _edit_case(rt, "edit", "Đổi FR-AST-13 thành cảnh báo tại T-90, T-60, T-30, T-7, T-0, bỏ qua SRS", ["FR-AST-13"])
            elif case_id == "case-8c":
                obs = _edit_case(rt, "edit", "Đưa mục Quality targets lên trước mục Requirements and acceptance trong TDD", [])
            else:
                raise TddAgentError(f"unknown case {case_id}", code="CASE_UNKNOWN")
        obs["seconds"], obs["usage"], obs["extra"] = t.seconds, usage_summary(rt.operational), extra
        return obs
    finally:
        rt.close()


def _case_3a(rt, docs: list[SourceDoc]) -> tuple[dict, dict]:
    head_before = rt.knowledge.head()
    base_graph = rt.knowledge.materialize()
    change = {"id": "DEFECT-1", "reproduction": "Client CI nhận lỗi timeout khi nộp SBOM 2.000 component qua POST /api/v1/sbom", "expected_behavior": "Theo NFR-PE-01 và NFR-RE-01: p95 ≤ 30 giây; thử lại 3 lần; quá 30 giây thì áp gate_failure_mode"}
    rt.orchestrator.start("tdd-generation", {"mode": "defect", "params": {"tdd_id": TDD_ID, "prose": False, "base_snapshot": head_before.id, "change_request": change}}, run_id="defect", sources=[d for d in docs if d.role in ("SRS", "SAD")])
    run = rt.orchestrator.advance("defect")
    task = rt.operational.get_task("defect", "G3") or {}
    out = task.get("output", {})
    graph = rt.knowledge.materialize()
    marker = _text([out, graph.get("gaps", []), graph.get("conflicts", []), [e for e in graph["elements"] if "DEFECT" in _text(e)]])
    record = out.get("defect") or {}
    budgets = record.get("supported_budgets") or {}
    obs = {"gen": {"status": out.get("status"), "blockers": out.get("blockers"), "invented_ok": not [e["id"] for e in graph["elements"] if e.get("origin") in ("upstream", "external") and not e.get("provenance")],
                   "base_respected": run.status in ("completed", "blocked"), "defect_recorded": "defect-1" in marker,
                   "timeout_semantics": bool(budgets.get("deadline_s")) and bool(budgets.get("retries")), "run_status": run.status, "run_blockers": run.result.get("blockers", []),
                   "kg_changed": rt.knowledge.head().id != head_before.id, "elements_before": len(base_graph["elements"]), "elements_after": len(graph["elements"])}}
    return obs, {"notes": ["defect mode run on the case-1 baseline"]}


def _case_10a(rt, srs1: SourceDoc, sad: SourceDoc, repo: Path) -> tuple[dict, dict]:
    pairs_a = _in_rows(srs1.text, ("FR-INV-02",), "`evaluation_id`", "`evaluation_id` và `etag`")
    pairs_b = _in_rows(srs1.text, ("FR-INV-02",), "`evaluation_id`", "`evaluation_id` và `received_at`")
    srs_a = srs_variant(srs1, "2.0a", replace=pairs_a)
    srs_b = srs_variant(srs1, "2.0b", replace=pairs_b)
    pa = cr_params("sbom-response-etag", "CR-A", "Thêm etag vào phản hồi POST /sbom", ["AC-A"], repo, {"FR-INV-02": "internal/api/sbom.go::Submit"}, {"FR-INV-02": "GIVEN một SBOM hợp lệ WHEN nộp THEN phản hồi có etag"}, after="2.0a")
    pb = cr_params("sbom-response-received-at", "CR-B", "Thêm received_at vào phản hồi POST /sbom", ["AC-B"], repo, {"FR-INV-02": "internal/api/sbom.go::Submit"}, {"FR-INV-02": "GIVEN một SBOM hợp lệ WHEN nộp THEN phản hồi có received_at"}, after="2.0b")
    obs_a, notes_a = drive_change(rt, "cr-a", pa, [srs1, srs_a, sad], {"FR-INV-02"}, ["FR-INV-02"], approve_g5=False)
    before_items = digest([e for e in rt.knowledge.materialize()["elements"] if str(e["id"]).startswith("CR-A")])
    run_a_before = (rt.operational.get_run("cr-a").status, rt.operational.get_run("cr-a").state)
    obs_b, notes_b = drive_change(rt, "cr-b", pb, [srs1, srs_b, sad], {"FR-INV-02"}, ["FR-INV-02"], approve_g5=False)
    after_items = digest([e for e in rt.knowledge.materialize()["elements"] if str(e["id"]).startswith("CR-A")])
    run_a_after = (rt.operational.get_run("cr-a").status, rt.operational.get_run("cr-a").state)
    q9 = QueryService(rt.knowledge).run("Q9").to_dict()
    b_text = _text(obs_b["run"]) + _text(obs_b["impact"]["reasons"]) + _text(obs_b["g5"])
    detected = bool(q9.get("rows")) or any(w in b_text for w in ("cr-a", "stale", "conflict", "concurrent"))
    obs = {**obs_b, "cr_a": obs_a, "cr10": {"cr_a_items_unchanged": before_items == after_items, "cr_a_run_unchanged": run_a_before == run_a_after, "conflict_detected": detected,
                                              "q9_rows": len(q9.get("rows", [])), "cr_b_status": obs_b["run"]["status"], "cr_b_blockers": obs_b["run"]["blockers"]}}
    return obs, {"notes": notes_a["notes"] + notes_b["notes"]}


def _case_10b(rt, srs1: SourceDoc, sad: SourceDoc, repo: Path) -> tuple[dict, dict]:
    pairs = _in_rows(srs1.text, ("FR-AST-13",), "T-7, T-0", "T-14, T-7, T-0")
    pairs21 = _in_rows(srs1.text, ("FR-AST-13",), "T-7, T-0", "T-45, T-7, T-0")
    srs2, srs21 = srs_variant(srs1, "2.0", replace=pairs), srs_variant(srs1, "2.1", replace=pairs21)
    params = cr_params("reminder-milestone", "CR-REM", "Thêm mốc nhắc", ["AC-REM"], repo, {"FR-AST-13": "internal/entitlement/reminder.go::Milestones"}, {"FR-AST-13": "GIVEN một Entitlement WHEN tới mốc mới THEN gửi một lần"})
    obs, notes = drive_change(rt, "cr-a", params, [srs1, srs2, sad], {"FR-AST-13"}, ["FR-AST-13"], approve_g5=False)
    before = {g: v for g, v in obs["gates"].items()}
    approvals_before = len(rt.operational.list_approvals("cr-a"))
    rt.orchestrator.update_sources("cr-a", [srs21], keep_revisions=True, unpin=[("SRS", "SRS", "2.0")], params={"srs": {"after": "2.1"}})
    run = rt.orchestrator.advance("cr-a")
    tasks = _tasks(rt, "cr-a")
    audit = [a["payload"].get("gate") for a in rt.operational.list_audit(kind="gate_stale", run_id="cr-a")]
    delta_after = sorted(c["id"] for c in ((tasks["G1"].get("output") or {}).get("srs_delta") or {}).get("changes", []))
    new_delta_text = _text((tasks["G1"].get("output") or {}).get("srs_delta"))
    obs["cr10"] = {"stale_detected": bool(audit), "stale_gates": audit, "rebased": "t-45" in new_delta_text and "2.1" in new_delta_text, "status_after": run.status, "state_after": run.state,
                   "gates_before": before, "gates_after": {g: v.get("status") for g, v in tasks.items()}, "approvals_before": approvals_before, "approvals_after": len(rt.operational.list_approvals("cr-a")),
                   "delta_after": delta_after}
    return obs, notes


# ------------------------------------------------------------------ runner
def run_real_cases(base: Config, out: Path, only: list[str] | None = None) -> dict:
    approval = oracles.approval_for(out, ORACLE)
    if approval is None:
        raise TddAgentError("the real-cases oracle has no approval from an allowlisted actor; approve it before running", code="ORACLE_NOT_APPROVED")
    describe = base.get("llm.default")
    if describe in ("fake", "", None):
        raise TddAgentError("real cases need a real provider (llm.default must be claude_code or codex_cli)", code="REAL_PROVIDER_REQUIRED")
    oracle = oracles.load(ORACLE)
    docs = pbom_sources(repo_root())
    results_path = out / "real-cases.json"
    report = json.loads(results_path.read_text()) if results_path.is_file() else {"task": "11", "oracle_approval": {k: approval[k] for k in ("actor", "kind", "role", "grade", "oracle_hash")}, "results": {}}
    base_folder = out / "real-base"
    if not (only and base_folder.is_dir() and "case-1" not in only):
        obs, base_folder = case_1_baseline(base, out)
        case1 = next(c for c in oracle["cases"] if c["id"] == "case-1")
        report["results"]["case-1"] = {"case": "case-1", "title": case1["title"], "observed": obs, **grade(case1, obs)}
        write_json(results_path, report)
    for case in oracle["cases"]:
        cid = case["id"]
        if cid == "case-1" or (only and cid not in only):
            continue
        try:
            obs = run_case(cid, base, out, base_folder, docs)
            result = {"case": cid, "title": case["title"], "observed": obs, **grade(case, obs)}
        except Exception as exc:  # a case that cannot run is a result, never silently skipped
            result = {"case": cid, "title": case["title"], "verdict": "fail", "error": f"{type(exc).__name__}: {exc}"[:500], "checks": [], "safety_failed": ["case-did-not-run"], "design_unmet": []}
        report["results"][cid] = result
        write_json(results_path, report)
    return report

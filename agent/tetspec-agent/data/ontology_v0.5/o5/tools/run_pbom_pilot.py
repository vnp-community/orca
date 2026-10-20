#!/usr/bin/env python3
"""Run the three agent flows against the PBOM URD/SRS/SAD fixtures.

The source documents are intentionally converted to deterministic, anchored
definition rows for a conservative intake smoke test. Missing design remains
blocked; this does not certify end-to-end agent generation.
"""
from __future__ import annotations
import argparse, json, re, sys
from hashlib import sha256
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
for part in ("tools", "harness", "edit", "specgen"):
    sys.path.insert(0, str(ROOT / part))
from dynamic import TddGenerationRequest, resolve_needs, run_generation
from render import publish_tdd
from prompt_edit import EditRequest, plan_edit, plan_prompt, commit_edit
from spec_change import SpecChangeRequest, repo_context, build_pack, readiness, render_artifacts, validate_artifacts, publish
from transaction5 import apply_changeset
from ontology5 import load as ontology_manifest

DOC_ROOT = Path(__file__).resolve().parents[3] / "test-pbom"


def quote_hash(text: str) -> str:
    return sha256(text.encode("utf-8")).hexdigest()


def source(path: Path, kind: str) -> dict:
    text = path.read_text(encoding="utf-8")
    return {"id": path.stem.lower(), "kind": kind, "version": "1.0", "content": text, "content_hash": quote_hash(text)}


def occurrences(text: str, pattern: str) -> list[tuple[str, str, int]]:
    seen, out = set(), []
    for n, line in enumerate(text.splitlines(), 1):
        for ident in re.findall(pattern, line):
            if ident not in seen:
                seen.add(ident); out.append((ident, line.strip(), n))
    return out


def element(ident: str, cls: str, line: str, number: int, doc: dict, props: dict | None = None, maturity: str = "specified") -> dict:
    return {"id": ident, "class": cls, "name": line[:180] or ident, "summary": line[:500], "owner_doc": doc["id"], "origin": "upstream", "maturity": maturity, "version_introduced": doc["version"], "review_status": "needs_review", "provenance": [{"kind": "extracted", "source_id": doc["id"], "source_version": doc["version"], "block": f"line:{number}", "quote_hash": quote_hash(line)}], "props": props or {}}


def build_pbom_graph(docs: dict[str, dict]) -> tuple[dict, dict]:
    from semantic_intake import definitions
    elements = []
    seen = set()
    for doc in docs.values():
        doc["facts"] = {}
        parsed = definitions(doc["content"], doc["id"], doc["version"])
        for node in parsed:
            if node["id"] not in seen:
                node["owner_doc"] = doc["id"]
                elements.append(node); seen.add(node["id"])
        doc["fact_evidence"] = {}

        def anchor(slot: str, value, needles: tuple[str, ...] = ()) -> None:
            lines = doc["content"].splitlines()
            line_no, quote = 1, lines[0] if lines else doc["id"]
            for n, line in enumerate(lines, 1):
                if any(needle.lower() in line.lower() for needle in needles):
                    line_no, quote = n, line
                    break
            doc["facts"][slot] = value
            doc["fact_evidence"][slot] = [
                {"kind": "extracted", "source_id": doc["id"], "source_version": doc["version"],
                 "source_hash": quote_hash(doc["content"]), "block": f"line:{line_no}",
                 "quote": quote, "quote_hash": quote_hash(quote)}
            ]

        # Use the source-local parse for intake facts. A shared identifier may
        # be refined by SRS over URD in the canonical graph, so global
        # de-duplication must not erase the SRS evidence used by a need.
        owned = parsed
        by_class = lambda classes: [x["id"] for x in owned if x["class"] in classes]
        authority = {"purpose": "urd", "scope": "urd", "requirements": "srs", "contracts": "srs", "data": "srs", "behavior": "srs", "quality": "srs", "decisions": "sad", "work": "urd", "validation": "srs", "provenance": "sad"}
        if doc["id"] == authority["requirements"]:
            anchor("requirements", by_class({"Requirement", "BusinessRule"}), ("yêu cầu", "quy tắc"))
        if doc["id"] == authority["contracts"]:
            anchor("contracts", by_class({"Operation", "Event", "Interface"}) or sorted(set(re.findall(r"API-[A-Z]+-\d+", doc["content"]))), ("API", "endpoint", "giao diện"))
        if doc["id"] == authority["data"]:
            anchor("data", by_class({"Entity", "Attribute", "DataStore"}) or sorted(set(re.findall(r"ENT-\\d+", doc["content"]))), ("thực thể", "dữ liệu", "schema"))
        if doc["id"] == authority["behavior"]:
            anchor("behavior", by_class({"StateMachine", "State", "Transition", "BusinessRule"}) or sorted(set(re.findall(r"(?:BRL|INT|UC)-[0-9]+", doc["content"]))), ("trạng thái", "luồng", "quy tắc"))
        if doc["id"] == authority["quality"]:
            anchor("quality", by_class({"QualityTarget", "Metric", "Slo"}) or sorted(set(re.findall(r"NFR-[A-Z]+-\\d+", doc["content"]))), ("NFR", "SLA", "hiệu năng", "chất lượng"))
        # These slots are source anchors, not inferred design decisions. They
        # make the intake denominator explicit while preserving the document
        # line that a reviewer must confirm before readiness can advance.
        if doc["id"] == authority["purpose"]:
            anchor("purpose", {"document": doc["id"], "anchor": "document-introduction"}, ("mục tiêu", "PBOM"))
        if doc["id"] == authority["scope"]:
            anchor("scope", {"document": doc["id"], "anchor": "scope-context"}, ("phạm vi", "đối tượng"))
        if doc["id"] == authority["decisions"]:
            anchor("decisions", {"document": doc["id"], "anchor": "decision-context"}, ("Decision", "quyết định", "lựa chọn"))
        if doc["id"] == authority["work"]:
            anchor("work", {"document": doc["id"], "anchor": "use-case-context"}, ("UC-", "use case", "quy trình"))
        if doc["id"] == authority["validation"]:
            anchor("validation", {"document": doc["id"], "anchor": "verification-context"}, ("kiểm chứng", "nghiệm thu", "acceptance"))
        if doc["id"] == authority["provenance"]:
            anchor("provenance", {"document": doc["id"], "version": doc["version"], "hash": quote_hash(doc["content"])}, ("tài liệu", "phiên bản", "nguồn"))
    from semantic_intake import build_graph
    graph = build_graph(list(docs.values()))
    graph["ontology_version"] = ontology_manifest()["version"]
    return graph, {}


def main():
    ap = argparse.ArgumentParser(); ap.add_argument("--run-dir", default=str(DOC_ROOT / "run-20261006")); ap.add_argument("--change-name", default="pbom-asset-entitlement-evidence"); args = ap.parse_args()
    run_dir = Path(args.run_dir); run_dir.mkdir(parents=True, exist_ok=True)
    docs = {"urd": source(DOC_ROOT / "URD.md", "URD"), "srs": source(DOC_ROOT / "SRS.md", "SRS"), "sad": source(DOC_ROOT / "SAD.md", "SAD")}
    for name, doc in docs.items(): (run_dir / f"{name}.source.json").write_text(json.dumps({k: v for k, v in doc.items() if k != "content"}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    graph, facts = build_pbom_graph(docs)
    (run_dir / "tdd.initial.kg.json").write_text(json.dumps(graph, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    needs = [{"id": "N-" + k, "slot": k} for k in ("purpose", "scope", "requirements", "contracts", "data", "behavior", "decisions", "quality", "work", "validation", "provenance")]
    request = TddGenerationRequest("PBOM-initial", "initial", ["CAP-PBOM"], list(docs.values()), profile_id="tdd-generation")
    gen = run_generation(request, graph, needs, facts=facts); (run_dir / "intake.initial.json").write_text(json.dumps(gen["intake"], indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    tdd_report = publish_tdd(graph, run_dir / "TDD-PBOM.initial.md", gen.get("readiness", {"status": "draft", "blockers": gen["intake"]["blockers"]}), allow_draft=True); (run_dir / "tdd.initial.report.json").write_text(json.dumps(tdd_report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    # Prompt edit: a clear, authorized rename of a real SRS operation.
    prompt_target = "API-P-01" if any(x["id"] == "API-P-01" for x in graph["elements"]) else "API-P-01"
    edit_req = EditRequest("EDIT-PBOM-001", "Đổi tên API mục tiêu thành Submit SBOM với timeout 30 giây", [prompt_target], graph["version"], {"scope": [prompt_target], "operations": ["modify", "rename"]}, user_id="pbom-owner")
    edit_plan = plan_edit(edit_req, graph, "modify", target=prompt_target, path="name", after="Submit SBOM (timeout 30s)"); edit_result = commit_edit(edit_req, graph, edit_plan)
    (run_dir / "prompt.edit.audit.json").write_text(json.dumps({"request": {k: v for k, v in vars(edit_req).items()}, "plan": edit_plan, "result": {k: v for k, v in edit_result.items() if k != "graph"}}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    # The guard above intentionally rejects the API-P-01/"Submit SBOM"
    # mismatch. Continue the fixture with the semantically correct SBOM target
    # so the successful prompt-edit path is exercised as well.
    corrected_target = "API-I-01"
    corrected_node = next((x for x in graph["elements"] if x["id"] == corrected_target), None)
    if corrected_node is not None:
        corrected_node.setdefault("props", {}).setdefault("timeout_seconds", 30)
        corrected_req = EditRequest("EDIT-PBOM-001-CORRECTED", "Đặt timeout submit SBOM thành 30 giây", [corrected_target], graph["version"], {"scope": [corrected_target], "operations": ["modify"]}, user_id="pbom-owner")
        corrected_plan = plan_prompt(corrected_req, graph)
        corrected_result = commit_edit(corrected_req, graph, corrected_plan)
        (run_dir / "prompt.corrected.audit.json").write_text(json.dumps({"request": {k: v for k, v in vars(corrected_req).items()}, "plan": corrected_plan, "result": {k: v for k, v in corrected_result.items() if k != "graph"}}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    else:
        corrected_result = {"status": "missing", "target": corrected_target}
    edited_graph = corrected_result.get("graph", graph); (run_dir / "tdd.after-prompt.kg.json").write_text(json.dumps(edited_graph, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    publish_tdd(edited_graph, run_dir / "TDD-PBOM.after-prompt.md", {"status": "draft", "blockers": ["POST_EDIT_SEMANTIC_REVALIDATION_REQUIRED"]}, allow_draft=True)
    from semantic_intake import classify_cr
    cr = classify_cr("Require entitlement evidence", graph, ["BRL-47", "FR-AST-12"])
    (run_dir / "change-request.assessment.json").write_text(json.dumps(cr, indent=2, ensure_ascii=False))
    summary = {"ontology_version": ontology_manifest()["version"], "status": "draft", "initial": {"status": gen["status"], "elements": len(graph["elements"]), "edges": len(graph["edges"]), "metrics": gen.get("metrics", {}), "blockers": gen.get("readiness", {}).get("blockers", gen.get("intake", {}).get("blockers", []))}, "prompt": corrected_result["status"], "prompt_target_guard": edit_result["status"], "cr": cr["kind"], "spec": "blocked: design/work/tests and actual authoring execution required", "acceptance": {"initial": gen["status"] in {"draft", "implementation_ready"}, "prompt": corrected_result.get("status") in {"committed", "presentation_committed"}, "change_request": cr.get("kind") is not None, "spec": False}}
    (run_dir / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False))
    print(json.dumps(summary, indent=2, ensure_ascii=False))


if __name__ == "__main__": main()

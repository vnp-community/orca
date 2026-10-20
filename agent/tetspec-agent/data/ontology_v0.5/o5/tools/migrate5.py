#!/usr/bin/env python3
"""Migrate v0.4 graph envelopes to v0.5 without mutating the source."""
from __future__ import annotations
import argparse, json
from pathlib import Path
from envelope5 import normalize_element, validate_graph
from transaction5 import snapshot_hash
from ontology5 import load

def migrate(source: dict, target_version: str | None = None) -> tuple[dict, dict]:
    """Normalize a snapshot for the runtime or an explicitly named release target."""
    target = target_version or load().get("version", "0.5.0")
    allowed = {load().get("version", "0.6.0"), load().get("canonical_release", {}).get("version"), *load().get("base", {}).get("compatibility_versions", [])}
    if target not in allowed:
        raise ValueError(f"unsupported migration target: {target}")
    out = {"version": target, "elements": [], "edges": list(source.get("edges", [])), "migration": {"from": source.get("version"), "source_version": source.get("version"), "target_version": target}}
    for raw in source.get("elements", []):
        x = normalize_element(raw)
        had_evidence = bool(x["provenance"])
        x["provenance"] = [p if "kind" in p else {"kind": "extracted", "source_id": p.get("doc") or p.get("block") or "v0.4", "source_version": source.get("version", "0.4.0"), **{k: p[k] for k in ("block", "quote_hash") if k in p}} for p in x["provenance"]]
        if not had_evidence:
            x["provenance"] = [{"kind": "derived", "source_id": "migration-v0.4", "source_version": source.get("version", "0.4.0"), "inputs": [x["id"]]}]
            x["gaps"] = list(x.get("gaps") or []) + [{"slot": "source_ref", "status": "unknown", "reason": "v0.4 snapshot has no retained source evidence", "owner": "ontology-migration"}]
        x.setdefault("version_introduced", source.get("version", "0.4.0"))
        x.setdefault("origin", "own"); x.setdefault("maturity", "specified")
        if x.get("review_status") in {"approved", "baselined", "implementation_ready"}:
            x.setdefault("migration", {})["legacy_review_status"] = x.get("review_status")
            x["review_status"] = "needs_review"
            x.setdefault("gaps", []).append({"slot": "legacy_acceptance", "status": "unknown", "reason": "legacy readiness cannot be reused as v0.5 acceptance", "owner": "ontology-migration"})
        if x.get("class") in ("Requirement", "Scenario", "WorkItem"):
            x.setdefault("gaps", [])
        out["elements"].append(x)
    validation = validate_graph(out)
    return out, {**validation, **loss_report(source, out), "target_version": target}


def loss_report(source: dict, migrated: dict) -> dict:
    """Report IDs/edges/provenance that were lost or became unresolved."""
    old_ids = {x.get("id") for x in source.get("elements", [])}
    new_ids = {x.get("id") for x in migrated.get("elements", [])}
    old_edges = {tuple((e.get("src"), e.get("rel"), e.get("dst"))) for e in source.get("edges", [])}
    new_edges = {tuple((e.get("src"), e.get("rel"), e.get("dst"))) for e in migrated.get("edges", [])}
    return {"lost_ids": sorted(old_ids - new_ids), "lost_edges": sorted(old_edges - new_edges), "unprovenanced": sorted(x.get("id") for x in migrated.get("elements", []) if not x.get("provenance")), "legacy_ready_reset": sorted(x.get("id") for x in migrated.get("elements", []) if x.get("migration", {}).get("legacy_review_status")), "source_snapshot": snapshot_hash(source), "migrated_snapshot": snapshot_hash(migrated), "status": "loss" if old_ids - new_ids or old_edges - new_edges else "lossless_ids_edges"}


def normalize_legacy_obligations(source: dict) -> list[dict]:
    """Turn legacy gaps/assumptions into explicit open obligations."""
    obligations = []
    for gap in source.get("gaps", []) or []:
        ident = str(gap.get("id") or gap.get("target") or gap.get("slot") or "legacy-gap")
        obligations.append({"id": "OBL-LEGACY-" + ident, "question": gap.get("reason", gap.get("slot", "Resolve legacy design gap")), "srs_refs": gap.get("srs_refs", []), "cr_acceptance_refs": gap.get("cr_acceptance_refs", []), "applicability": "unknown", "status": "open", "profile_id": gap.get("profile_id", "legacy-migration"), "provenance": [{"kind": "derived", "source_id": "migration-v0.4", "source_version": source.get("version", "0.4.0"), "inputs": [ident], "derivation": "legacy gap normalized"}]})
    for element in source.get("elements", []):
        for gap in element.get("gaps", []) or []:
            ident = str(gap.get("slot", element.get("id", "legacy-gap")))
            obligations.append({"id": "OBL-LEGACY-" + str(element.get("id")) + "-" + ident, "question": gap.get("reason", ident), "srs_refs": gap.get("srs_refs", []), "cr_acceptance_refs": gap.get("cr_acceptance_refs", []), "applicability": "unknown", "status": "open", "profile_id": gap.get("profile_id", "legacy-migration"), "provenance": [{"kind": "derived", "source_id": "migration-v0.4", "source_version": source.get("version", "0.4.0"), "inputs": [str(element.get("id"))], "derivation": "legacy element gap normalized"}]})
    return obligations


def dry_run(source: dict, target_version: str | None = None) -> tuple[dict, dict]:
    migrated, validation = migrate(source, target_version=target_version)
    report = {**validation, **loss_report(source, migrated), "dry_run": True, "writes": False}
    return migrated, report


def rollback_exact(original: dict, backup: dict, expected_source_hash: str | None = None) -> tuple[dict, dict]:
    """Restore an immutable backup only when the current snapshot matches."""
    current_hash = snapshot_hash(original)
    if expected_source_hash and current_hash != expected_source_hash:
        return original, {"ok": False, "code": "ROLLBACK_VERSION_CONFLICT", "expected": expected_source_hash, "actual": current_hash}
    restored = json.loads(json.dumps(backup, ensure_ascii=False))
    return restored, {"ok": True, "rolled_back": True, "snapshot": snapshot_hash(restored)}

def main():
    ap = argparse.ArgumentParser(); ap.add_argument("source"); ap.add_argument("output"); ap.add_argument("--report", required=True); ap.add_argument("--backup"); ap.add_argument("--target-version"); ap.add_argument("--dry-run", action="store_true"); ap.add_argument("--rollback", action="store_true")
    args = ap.parse_args(); source = json.loads(Path(args.source).read_text(encoding="utf-8"))
    if args.rollback:
        restored = dict(source); restored.setdefault("migration", {})["rollback_of"] = load().get("version", "0.6.0"); report = {"version": restored.get("version", "0.4.0"), "ok": True, "rolled_back": True, "element_count": len(restored.get("elements", [])), "edge_count": len(restored.get("edges", []))}; migrated = restored
    else:
        migrated, report = migrate(source, target_version=args.target_version)
    if args.backup and not args.dry_run and not args.rollback: Path(args.backup).write_text(json.dumps(source, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    if not args.dry_run: Path(args.output).write_text(json.dumps(migrated, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    Path(args.report).write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))

if __name__ == "__main__": main()

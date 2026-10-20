"""Report of the ten real cases (task 11.8), built from ``real-cases.json`` plus the kept earlier attempts."""
from __future__ import annotations

import json
from pathlib import Path

from . import oracles
from .common import git_head, repo_root

DEFECTS = [
    ("prompt-edit edited an unrelated element", "Cases 3b and 8a (first run, safety fail): the prompt named API-I-01 / API-P-01, which are not elements of the PBOM KG; the agent matched the word `timeout` to the ErrorCode `ERR-timeout` and changed it (a rename and a new property).",
     "An id named in the prompt that does not exist is `TARGET_UNRESOLVED` (with the gap that explains it); names of ErrorCode/State/Transition/Attribute never select a target by words; a name read from a source document is locked like a sourced value."),
    ("defect mode was a plain regeneration", "Case 3a: mode `defect` made no LLM call, recorded nothing about the defect and left the KG as it was.",
     "`defect_record`: affected facts (named ids first, then lexical), budgets read from their quotes (deadline, retries, failure mode), expected values no source supports become `srs_clarification` gaps and questions, a test scenario from the user's own words; an unchanged graph no longer creates a snapshot."),
    ("cap changes were not recognised as parameter-only", "Cases 4a/4b: `20.000 component` has no unit the delta understood, so the change was an ordinary modification and `parameter_only` stayed false.",
     "Numbers are compared as numbers; capacity caps (`tối đa`, `≤`, `vượt` + count/size unit) are extracted. Lowering a cap is `no_change` with the proof (inside the envelope the design was made for); raising it is `update` with a capacity/quality obligation; latency targets are never treated as caps."),
    ("shared-contract conflicts only saw Interface elements", "Case 10a: two open CRs on the same PBOM requirement raised no conflict because the API contract is not an element there.",
     "Q9 also covers Operation, Event and Requirement; the second CR is blocked with `SHARED_CONTRACT_CONFLICT:<id>:<other CR>`."),
    ("rows with an unmapped id prefix vanished from the delta", "Case 6: PBOM's `EIF-xx` rows have no class by default, so a new external interface gave `SRS_DELTA_EMPTY` without saying why.",
     "`SRS_DELTA_UNMAPPED_IDS` with the gap per unmapped id; the project's `id_class_map` is honoured by the impact flow."),
    ("an external interface without a failure policy was invisible", "Case 6b: Q4 and the profile checks only looked at Operations, so a new external Interface (EIF-20) with no timeout/retry/fallback/DLQ policy produced no gap.",
     "Interfaces listed under an external-interfaces section carry `external`; an external Interface without a governing FailurePolicy is a `MISSING_FAILURE_POLICY` gap; the change-request realization applies the same profile checks to the elements the change brings in."),
    ("a project id_class_map replaced the defaults", "Cases 6/6b: giving `{EIF: Interface}` made every other prefix unmapped.", "`id_class_map` extends the default map."),
    ("a run over fewer documents dropped the other documents' evidence", "Found while reading case 3a's KG: the defect run pinned only SRS and SAD, and every element it re-extracted lost its URD provenance and `upstream_definition` (the change-request realization had the same effect for every changed requirement).",
     "Merging keeps the provenance of sources the new extraction did not read, and an unchanged graph commits nothing."),
    ("throughput and availability targets stated as text raised no gap", "Case 7: `500 TPS đỉnh` and `99,95% active-active` produced only a generic `quality measurement missing`.",
     "Quality targets with a throughput or availability statement and no LoadProfile / RecoveryObjective in the KG get `MISSING_LOAD_PROFILE` / `MISSING_RECOVERY_OBJECTIVE` gaps (design case 7)."),
    ("a document-level presentation edit was impossible", "Case 8c: 'move section X before section Y' names no element, and a reorder carrying a position was classified as a semantic edit.",
     "A presentation-only intent with no property path is a presentation edit; when it names no element it applies to the (unique or named) TDD document and the semantic hash stays unchanged."),
    ("a run could not move its `after` SRS revision", "Case 10b: `update_sources` replaced the before and after SRS pins (same role and id).", "`update_sources(keep_revisions, unpin, params)`."),
    ("lexical candidates ignored the quoted definition", "Used by the defect record and by search.", "The lexical text now includes `definition` and the first provenance quote."),
    ("rendered TDD hid gaps", "Found when the PBOM TDD was first opened: `Gaps: 0` although the KG has 549 gaps.", "Renderer shows gaps on non-element targets (`known_ids`); regression test."),
]


CAVEATS = {
    "case-1": "Provider calls vary run to run (prose facts extracted by the model: 6-16 calls, 992-1016 elements); the oracle checks are all rule-level so they hold across runs. Readiness stays blocked: purpose/work/validation are not in the documents.",
    "case-2": "(The new row was inserted after FR-AST-19, the last FR-AST row, not after FR-AST-13 as the oracle text says; the id FR-AST-20 is free.) Blocked at G5 only by the draft PBOM KG (see below); the TDD is `update` as expected, and every change-spec row cites SRS section, TDD section and the CR acceptance. There is a single TDD in the inventory, so 'other TDDs stay no_change' is not measured here (the partitioned-inventory runs of phase 10 measure it).",
    "case-3a": "The defect run records the governing facts and budgets (deadline/retries/fail mode read from NFR-PE-01/NFR-RE-01) and finds nothing unsupported, so no SRS clarification is raised and no TDD revision is written; it makes no model call.",
    "case-3b": "The prompt names API-I-01, which PBOM defines nowhere (Appendix B missing); the agent says so instead of editing something else.",
    "case-4a": "`no_change` is justified by the cap being lowered (inside the envelope the baseline statement was made for); no capacity measurement exists, and PBOM has no typed range/capacity model for it.",
    "case-4b": "`update` with a capacity/quality obligation in the rationale; the obligation is text on the impact entry, not yet a typed DesignObligation in the KG.",
    "case-5": "Surfaced through the existing gate (`QG-10: ENT-35: data schema missing`). The runtime has no expand/contract, backfill or rollback obligation for a plain column addition, so those are not reported separately.",
    "case-6": "Default configuration: PBOM's `EIF` prefix has no class, so the new interface is reported as an unmapped id and the flow stops at G1 (`SRS_DELTA_UNMAPPED_IDS`); the project must map the prefix (case 6b).",
    "case-6b": "With the project's `id_class_map` the new external interface is an Interface with a `MISSING_FAILURE_POLICY` gap and G5 rules `QG-08`/`TS-08` name it.",
    "case-7": "Gaps `MISSING_LOAD_PROFILE` and `MISSING_RECOVERY_OBJECTIVE` on the new NFR; the impact entry is `add` of a new TDD (the requirement has no trace path to existing design).",
    "case-8c": "The overlay is recorded in the task output (document id, unchanged semantic hash); there is no persisted presentation layer yet.",
    "case-9": "Blocked at G2 with `SAD_CHANGE_REQUIRED`; no TDD set, change spec or publication exists. The rule that detects it compares the new text with ADR-08's listed alternatives.",
    "case-10a": "Both CRs stop at G5 (draft KG), so no OpenSpec change was published; the conflict is caught at G4 of the second CR.",
    "case-10b": "After the SRS 'after' revision changes, G0-G5 become stale, the G1 approval is void and the delta is re-derived against revision 2.1.",
}


def _line(cid: str, o: dict) -> str:
    if "gen" in o and cid == "case-1":
        g = o["gen"]
        return f"generation: {g['status']}, {g['elements']} elements, {g['gaps']} gaps, {g['questions']} questions, requirement coverage {g['requirement_coverage']}, {g['state_machines']} state machines"
    if "gen" in o:
        g = o["gen"]
        return f"defect record: recorded={g['defect_recorded']}, budgets in record={g['timeout_semantics']}, KG changed={g['kg_changed']}"
    if "edit" in o:
        e = o["edit"]
        return f"{e['outcome']}{' / ' + e['reason'] if e.get('reason') else ''}; blockers {e['blockers']}; KG mutated={e['mutated']}"
    parts = [f"delta {o['delta']['ids']}", f"impact {o['impact']['actions']}"]
    if o["run"]["blockers"] and o["g4"]["status"] is None:
        parts.append(f"stopped at {o['run'].get('blocked_gate') or o['run']['state']}: {o['run']['blockers'][:3]}")
    else:
        parts.append(f"G4 {o['g4']['status']} ({o['g4']['rows_count']} rows), G5 {o['g5']['status']}")
    if "cr10" in o:
        parts.append(f"cr10 {json.dumps(o['cr10'], ensure_ascii=False)[:260]}")
    return "; ".join(parts)


def build_real_cases_report(out: Path) -> Path:
    data = json.loads((out / "real-cases.json").read_text(encoding="utf-8"))
    oracle = oracles.load("real-cases")
    by_id = {c["id"]: c for c in oracle["cases"]}
    results = data["results"]
    lines = ["# Real cases on PBOM - build-agentic-tdd-runtime phase 11", "",
             f"Repository commit: `{git_head(repo_root())}` (working tree may contain uncommitted changes).", "",
             "All cases ran the real provider (Claude Code, `claude-sonnet-5-5`, effort `medium`) through the runtime flows on the real PBOM URD/SRS/SAD. "
             "No `FakeProvider` and no synthetic fixture. Inputs are PBOM rows, untouched (case 1) or edited/added the way a change request would (see *input*). "
             "The baseline TDD/KG is what the generation agent produced from the PBOM documents in this same run.", "",
             f"Expectations: `oracles/real-cases.json` (written from design section 9 before the runs; approval grade **{data['oracle_approval']['grade']}** by `{data['oracle_approval']['actor']}`, hash `{data['oracle_approval']['oracle_hash'][:12]}`). "
             "`fail` = a safety check failed; `partial` = safe but a design expectation was not met; `blocked_by_data` = safe and the only unmet design check depends on information PBOM does not contain; `pass` = everything met.", "",
             "| Case | Verdict | Safety failed | Design unmet | Calls | USD | Seconds |", "|---|---|---|---|---|---|---|"]
    total_cost = total_calls = 0
    for cid, r in results.items():
        o = r.get("observed") or {}
        u = o.get("usage") or {}
        total_cost += u.get("cost_usd", 0); total_calls += u.get("calls", 0)
        lines.append(f"| {cid} {r['title']} | **{r['verdict']}** | {', '.join(r.get('safety_failed') or []) or '-'} | {', '.join(r.get('design_unmet') or []) or '-'} | {u.get('calls', '-')} | {u.get('cost_usd', '-')} | {o.get('seconds', '-')} |")
    counts = {v: sum(1 for r in results.values() if r["verdict"] == v) for v in ("pass", "partial", "blocked_by_data", "fail")}
    lines += ["", f"Totals: {counts}; {total_calls} provider calls, about ${total_cost:.2f} in this run.", "", "## Case by case", ""]
    for cid, r in results.items():
        c = by_id[cid]
        if CAVEATS.get(cid):
            pass
        lines += [f"### {cid} - {r['title']}  ({r['verdict']})", "", f"- design: {c['design_ref']}", f"- input: {c['input']}", f"- expected (design): {c['expected_from_design']}",
                  f"- observed: {_line(cid, r.get('observed') or {})}" if r.get("observed") else f"- error: {r.get('error')}"]
        for ch in r.get("checks", []):
            lines.append(f"  - [{'x' if ch['passed'] else ' '}] {ch['level']}: `{ch['id']}` (observed {ch['observed']!r})")
        if CAVEATS.get(cid):
            lines.append(f"- reading the result: {CAVEATS[cid]}")
        g5 = (r.get("observed") or {}).get("g5") or {}
        if g5.get("status") == "blocked":
            lines.append(f"  - G5 blocked by whole-closure rules of the draft PBOM KG: {g5.get('rules')} (these block every PBOM change spec: unreviewed facts, no acceptance/work links, no data schemas)")
        lines.append("")
    lines += ["## Why PBOM changes stop at G5", "",
              "Every case that reached G5 was blocked by `QG-03` (facts need review), `QG-06/07` (no acceptance or work/test link for the requirements the closure reaches), `QG-10` (no data schema / measurement) and `TS-*` rules. "
              "These come from the PBOM KG being `draft` (appendices A/B/C missing, 549 gaps), so no PBOM change spec can be published until the SA reviews the facts and the missing design is supplied; the runtime refuses instead of authoring from an incomplete KG. "
              "Nothing was bulk-reviewed to get past it.", "", "## Defects the real cases exposed (all fixed with regression tests, none by editing expectations)", ""]
    for title, found, fix in DEFECTS:
        lines += [f"- **{title}.** {found} Fix: {fix}"]
    history = []
    for n in (1, 2, 3):
        path = out / f"real-attempt-{n}" / "real-cases.json"
        if path.is_file():
            prev = json.loads(path.read_text(encoding="utf-8"))["results"]
            history.append((n, {k: v["verdict"] for k, v in prev.items()}))
    if history:
        lines += ["", "## Earlier runs (kept)", "", "| Case | " + " | ".join(f"run {n}" for n, _ in history) + " | final |", "|---|" + "---|" * (len(history) + 1)]
        for cid, r in results.items():
            lines.append(f"| {cid} | " + " | ".join(h.get(cid, '-') for _, h in history) + f" | {r['verdict']} |")
        lines += ["", "Run 1 also contained input mistakes of mine (new ids that already exist in the real SRS: FR-AST-14, EIF-19; an expected delta that included OI-12, which is not a KG element) an over-generous keyword observation (it matched substrings inside aggregated rule lines, which is why case 7 read `pass` in runs 1 and 2 before it was made strict; the strict version found case 7 `partial` until the load/availability rule existed) and, in run 3, a variable shadowed in my observation code (it made 4a's `config-row-exists` fail); the oracle was revised before runs 2 and 3 (`revision_note` in the oracle) without changing any expected outcome."]
    lines += ["", "## Limits", "",
              "- Approval of the expectations is `agent` grade (coding agent); no human approved them.",
              "- Case inputs are edited PBOM rows (listed per case), not real change requests raised by PBOM stakeholders.",
              "- The change-spec authoring step (G5 author/critic) was not reached on PBOM; its real-provider result is the synthetic payment fixture in the main report.",
              "- Case 8c (document-level presentation reorder) is unsupported: a presentation overlay needs a target element.",
              "- Case 10b/10a use two CRs on one runtime; no OpenSpec change was published (G5 blocked), so overwriting of a published change was not exercised.",
              "- D5 L2 (independent semantic review) is not claimed."]
    path = out / "real-cases-report.md"
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return path

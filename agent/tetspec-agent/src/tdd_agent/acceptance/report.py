"""Assemble the acceptance report (task 9.6 and 10.5) from the JSON results of the individual runs."""
from __future__ import annotations

import json
from pathlib import Path

from .common import git_head, repo_root


def _load(path: Path):
    return json.loads(path.read_text(encoding="utf-8")) if path.is_file() else None


def _gen_rows(tdd: dict) -> list[str]:
    rows = ["| Input | Kind | Result | Run status | Readiness | KG elements | Gaps | Questions | LLM calls | Seconds | Cost USD |", "|---|---|---|---|---|---|---|---|---|---|---|"]
    for name, runs in tdd["inputs"].items():
        for r in runs:
            kg = r.get("kg") or {}
            rows.append(f"| {name} r{r['repetition']} | {'synthetic' if r.get('synthetic') else 'real document'} | {r['graded']} | {r['run_status']} | {(r.get('readiness') or {}).get('status', '-')} | {kg.get('elements', '-')} | "
                        f"{r.get('gaps', '-')} | {r.get('questions', '-')} | {r['usage']['calls']} | {r['seconds']} | {r['usage']['cost_usd']} |")
    return rows


def build_report(out: Path) -> Path:
    existing, tdd, cases = _load(out / "existing-tools.json"), _load(out / "acceptance-report-tdd.json"), _load(out / "case-table.json")
    approvals, isolated = _load(out / "oracle-approvals.json") or {}, _load(out / "isolated-agents.json")
    lines = ["# Acceptance report - build-agentic-tdd-runtime", "", f"Repository commit: `{git_head(repo_root())}` (working tree may contain uncommitted changes).", ""]
    lines += ["## 9.2 Existing tools", ""]
    if existing:
        lines += ["| Tool | Status | Seconds | Summary |", "|---|---|---|---|"]
        for k, v in existing["tools"].items():
            lines.append(f"| {k} | {v['status']} | {v['seconds']} | `{json.dumps(v.get('summary'), ensure_ascii=False)[:160]}` |")
    else:
        lines.append("not run")
    lines += ["", "## 9.3 TDD generation (real provider)", ""]
    if tdd:
        some = next((r for runs in tdd["inputs"].values() for r in runs), None)
        if some:
            g = some["provider"]["gateway"]
            lines += [f"Provider/models: `{json.dumps(g, ensure_ascii=False)}`; config fingerprint `{some['provider']['config_hash']}`.", ""]
        lines += _gen_rows(tdd) + [""]
        for name, runs in tdd["inputs"].items():
            for r in runs:
                bad = [c for c in r.get("checks", []) if not c["passed"]]
                if bad:
                    lines.append(f"- {name} r{r['repetition']} failed checks: " + "; ".join(f"`{c['id']}` {c['detail']}" for c in bad))
        if tdd.get("comparison_with_existing_tools"):
            lines += ["", "Comparison with the o5 pilot: `" + json.dumps(tdd["comparison_with_existing_tools"], ensure_ascii=False)[:900] + "`"]
    else:
        lines.append("not run")
    lines += ["", "## 9.4 Oracle approvals", ""]
    for name, rec in approvals.items():
        lines.append(f"- `{name}` approved by `{rec['actor']}` ({rec['kind']}, role {rec['role']}), grade **{rec['grade']}**, hash `{rec['oracle_hash'][:12]}`: {rec['rationale']}")
    if not approvals:
        lines.append("none")
    lines += ["", "## 9.5 Ten-case table", ""]
    if cases:
        lines += (out / "case-table.md").read_text(encoding="utf-8").splitlines() if (out / "case-table.md").is_file() else []
        lines += ["", f"Summary: {cases['summary']}; full acceptance claim: **{cases['full_acceptance_claim']}** (cases 3 and 7 carry documented limits; those tests use scripted `fixture`-grade model answers and are not real-provider evidence)."]
    else:
        lines.append("not run")
    lines += ["", "## 10 Isolated agents", ""]
    if isolated:
        lines += [f"Repetitions per case: {isolated['repetitions']}; frozen inputs: `{json.dumps(isolated['frozen_inputs'])}`.", ""]
        for agent, body in isolated["agents"].items():
            lines.append(f"### {agent}")
            if agent in ("impact", "promptedit"):
                lines += ["| Case | Stability | Passed | Dangerous errors / unexpected mutations | Checks failing |", "|---|---|---|---|---|"]
                for cid, c in body.items():
                    failing = sorted({k for r in c["runs"] for k, v in r["checks"].items() if not v})
                    lines.append(f"| {cid} | {c['stability']} | {sum(r['graded']=='pass' for r in c['runs'])}/{len(c['runs'])} | {c.get('dangerous_errors', c.get('unexpected_mutations'))} | {', '.join(failing) or '-'} |")
            elif agent == "changespec":
                lines.append(f"final batch: stability {body['stability']}; runs: " + ", ".join(f"{r['graded']} ({r['seconds']}s, ${r['usage']['cost_usd']})" for r in body["runs"]))
                extra = [json.loads(p.read_text()) for p in sorted((out / "first-attempt").glob("e2e-second.attempt*.json"))]
                if extra:
                    pooled = [r["graded"] for r in body["runs"]] + [r["graded"] for r in extra]
                    verdict = "stable_pass" if all(g == "pass" for g in pooled) else "unstable" if "pass" in pooled else "stable_fail"
                    lines.append(f"pooled with the independent e2e runs on the same code ({', '.join(r['graded'] for r in extra)}): {pooled.count('pass')}/{len(pooled)} pass => **{verdict}**; an unstable case is not counted as pass.")
            else:
                for name, c in body.items():
                    lines.append(f"- {name}: {c['stability']}; failed checks {c['failed_checks']}")
        if "generation" not in isolated["agents"] and tdd:
            lines.append("### generation\nCovered by section 9.3: three repetitions per input with frozen input documents (the generation agent has no upstream agent).")
            lines.append("")
    else:
        lines.append("not run (requires the TDD-generation acceptance report)")
    e2e, e2e2 = _load(out / "e2e-pbom.json"), _load(out / "e2e-second.json")
    lines += ["", "## 7.6 End-to-end change-request runs (real provider)", ""]
    if e2e:
        lines.append(f"- PBOM (SRS 1.0 -> 2.0, FR-AST-13 gains T-14): final **{e2e['final_status']}** at {e2e['final_state']} after {e2e['seconds']}s, {e2e['usage']['calls']} LLM calls, ${e2e['usage']['cost_usd']}; gates {e2e['gates']}; blockers {e2e['final_blockers']}. Expected: the PBOM KG is not implementation-ready (appendices A/B/C missing, no acceptance/work links, no data schemas), so the flow refuses to publish a change spec; nothing was bulk-reviewed to get past it.")
    if e2e2:
        f = e2e2["final"]
        attempts = sorted((out / "first-attempt").glob("e2e-second.attempt*.json"))
        lines.append(f"- Synthetic payment fixture, G0-G5: published={f['published']}, `openspec validate --strict` returncode {(f['openspec_validate_strict'] or {}).get('returncode')}, afterwards {f['after_publication']} (G6 waits for real test receipts; none are recorded because no code was implemented). Attempts: " + ", ".join(json.loads(p.read_text())["graded"] for p in attempts) + f"; last run {e2e2['seconds']}s, ${e2e2['usage']['cost_usd']}; config hash `{f.get('config_hash')}`; provider `{json.dumps(f.get('provider', {}).get('providers'), ensure_ascii=False)}`.")
    lines += ["- G6/G7 mechanics (receipts, `accepted(agent)`) are covered by `tests/test_agents_change_flow.py` with scripted answers (fixture grade).", ""]
    total = 0.0
    if tdd:
        total += sum(r["usage"]["cost_usd"] for runs in tdd["inputs"].values() for r in runs)
    if isolated:
        for agent, body in isolated["agents"].items():
            if agent in ("impact", "promptedit"):
                total += sum(r["usage"]["cost_usd"] for c in body.values() for r in c["runs"])
            elif agent == "changespec":
                total += sum(r["usage"]["cost_usd"] for r in body["runs"])
    total += sum(x["usage"]["cost_usd"] for x in (e2e, e2e2) if x)
    lines += [f"Reported provider cost of the final runs (earlier iterations not included): about ${total:.2f}. Wall-clock times are per run in the tables above.", ""]
    real = _load(out / "real-cases.json")
    if real:
        res = real["results"]
        counts = {v: sum(1 for r in res.values() if r["verdict"] == v) for v in ("pass", "partial", "blocked_by_data", "fail")}
        lines += ["", "## 11 Ten real cases on PBOM (real provider, no fixtures)", "", f"{counts} - details, checks per case, defects found and fixed: `evidence/real-cases/real-cases-report.md`.", ""]
        lines += ["| Case | Verdict |", "|---|---|"] + [f"| {cid} {r['title']} | {r['verdict']} |" for cid, r in res.items()] + [""]
    lines += ["", "## Limits of this evidence", "",
              "- Prompt-edit case P2 (presentation-only reorder): the real model's intent was classified as ambiguous, so the agent asked for clarification (the safe outcome the oracle allows); the presentation-commit path itself is exercised only by unit tests.",
              "- Impact reports on PBOM mark most TDDs `update` for a single changed requirement: residue TDDs cannot be proven independent by rules and the model may only add coverage evidence it cites, so over-updating is the safe direction; no `no_change` was ever wrong (dangerous error count 0).",
              "- The change-spec authoring instructions were corrected over six iterations against the synthetic payment fixture (`earlier-attempts/isolated-agents.attempt*.json`); with the earlier code the pooled result was 4/5 (unstable). Phase 9/10 were then re-run on the final code (after the phase 11 fixes): the numbers in this report are from that final code.",
              "- The PBOM change-request e2e stops at G5 by design (PBOM is not implementation-ready); a published change was produced only on the synthetic payment fixture.",
              "- Rendered TDD defect found after the runs: gaps whose target is not an element were not shown (PBOM TDD said 0 gaps, the KG has 549); fixed in the renderer and the PBOM TDD re-rendered into `evidence/generated-tdd/TDD-PBOM.r1.md`. Earlier run folders still hold the pre-fix files.",
              "- Generation oracle `second-generation` was corrected once after run 1 (see 9.4); the other two run-1 failures were runtime defects fixed in code.",
              "- Second input document and prose input are synthetic fixtures unless the table above says `real document`.",
              "- Oracle approvals are `agent` grade when granted by a coding agent; a human approval is needed for `human` grade.",
              "- D5 L2 semantic quality (independent reviewer) is **not** claimed; task 6.3 of `harden-tdd-agent-semantic-quality` stays open.",
              "- Cases using scripted model answers are `fixture` grade and never counted as real-provider evidence.",
              "- Only the Claude Code provider was run for real; the Codex CLI adapter is covered by contract tests with a fake executable."]
    path = out / "acceptance-report.md"
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return path

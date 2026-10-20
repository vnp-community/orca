"""Authoring runner for ``o5``'s ``publish`` backed by skills and the LLM gateway.

``specgen.spec_change.publish`` expects an object with ``author`` and ``review``.
This runner provides both through the ``author-spec-prose`` and ``critique-spec``
skills, so the provider (and the author/critic split) comes from configuration
instead of a hard-wired CLI.
"""
from __future__ import annotations

import json
from pathlib import Path

from ..core import o5_path
from ..orchestrator.context import GateContext
from .common import skills


PROMPT_BUDGET_CHARS = 420_000  # about 100k tokens for the pack; the model's window is the hard limit, a bigger prompt fails with "Prompt is too long"
SCOPED_KEYS = ("graph", "design", "versions", "scenarios", "work", "tests", "provenance")


def scoped_pack(pack: dict, budget: int = PROMPT_BUDGET_CHARS) -> tuple[dict, dict]:
    """The pack as the author sees it, and the record of what was left out.

    The knowledge-graph closure of a change can be the whole graph (PBOM: 5 MB).  The author is given the change itself (canonical
    change-spec rows, impact, workflow context) plus the facts those rows point at and their neighbours, nearest first, within a character
    budget.  What is left out is recorded (``truncated``), never silently dropped; assertions can only cite facts that were shown.
    """
    def size(x) -> int:
        return len(json.dumps(x, ensure_ascii=False, default=str))

    full = size(pack)
    if full <= budget:
        return pack, {"scoped": False, "chars": full}
    elems = {e["id"]: e for e in pack["graph"]["elements"]}
    seeds: set[str] = set(pack.get("impact", {}).get("changed", [])) | {d["id"] for d in pack.get("deltas", [])}
    for item in (pack.get("canonical_change_spec") or {}).get("items", []):
        for ref in list(item.get("srs_refs", [])) + list(item.get("tdd_refs", [])):
            seeds.add(ref.get("section_id"))
    for d in pack.get("deltas", []):
        seeds |= {v for k, v in (d.get("props") or {}).items() if k in ("before_requirement_id", "after_requirement_id", "requirement_id", "capability_id")}
    seeds &= set(elems)
    neighbours: dict[str, set[str]] = {i: set() for i in elems}
    for ed in pack["graph"].get("edges", []):
        if ed["src"] in elems and ed["dst"] in elems:
            neighbours[ed["src"]].add(ed["dst"]); neighbours[ed["dst"]].add(ed["src"])
    fixed = {k: v for k, v in pack.items() if k not in SCOPED_KEYS}
    fixed["request"] = {k: v for k, v in pack.get("request", {}).items() if k != "workflow_context"}  # already present at the top level
    room = budget - size(fixed)
    chosen: list[str] = []
    seen = set(seeds)
    layer = sorted(seeds)
    truncated = False
    while layer and not truncated:
        for eid in layer:
            cost = 3 * size(elems[eid])  # it appears in graph.elements, in design (or versions) and in its provenance
            if cost > room:
                truncated = True
                break
            room -= cost
            chosen.append(eid)
        nxt = sorted({n for i in layer for n in neighbours[i]} - seen)
        seen |= set(nxt)
        layer = nxt
    truncated = truncated or bool(layer)
    keep = set(chosen)
    graph = dict(pack["graph"])
    graph["elements"] = [elems[i] for i in sorted(keep)]
    graph["edges"] = [e for e in pack["graph"].get("edges", []) if e["src"] in keep and e["dst"] in keep]
    graph["gaps"] = [g for g in pack["graph"].get("gaps", []) if g.get("target") in keep or g.get("referenced_by") in keep]
    scoped = dict(fixed)
    scoped["graph"] = graph
    for key in ("design", "versions", "scenarios", "work", "tests"):
        scoped[key] = [x for x in pack.get(key, []) if x["id"] in keep]
    scoped["provenance"] = {k: v for k, v in pack.get("provenance", {}).items() if k in keep}
    record = {"scoped": True, "chars_before": full, "chars_after": size(scoped), "seeds": len(seeds), "included": len(keep), "of": len(elems), "truncated": truncated}
    scoped["pack_scope"] = {k: record[k] for k in ("included", "of", "truncated")} | {"note": "only the facts the change points at and their nearest neighbours are shown; cite only facts that appear in graph.elements"}
    return scoped, record


# what the independent reviewer rejects most often; telling the author up front saves rounds
AUTHORING_RULES = ("A MODIFIED requirement restates the whole requirement: keep every scenario of the baseline (the requirement's block in pack.repo.contents under openspec/specs/) word for word and add the new scenarios after them; "
                   "never drop a baseline scenario or its acceptance. "
                   "Write a claim of coverage only as far as the test row behind it goes: a test that checks one case does not verify 'every case'. "
                   "Write a number, a duration or a limit only when it appears in the pack facts you cite.")


class ProviderAuthoringRunner:
    def __init__(self, ctx: GateContext, *, language: str = "", feedback: list | None = None):
        self.ctx = ctx
        self.language = language
        self.feedback = list(feedback or [])  # what the reviewer or the validation found in the previous draft: the author is shown it and writes again
        self.hub = skills(ctx)
        self.o5 = o5_path.load()

    def _base_record(self, workflow: str, payload: dict, role: str) -> dict:
        provider = self.ctx.gateway.provider_for(role)
        return {"workflow": workflow, "runner": provider.name, "model": provider.effective_model() or "provider default",
                "config_hash": self.ctx.config.fingerprint(), "configuration": self.ctx.gateway.describe(), "input_hash": self.o5.fingerprint(payload),
                "attempts": [], "status": "blocked"}

    @staticmethod
    def _save(output: Path, record: dict) -> None:
        output.mkdir(parents=True, exist_ok=True)
        (output / "execution.json").write_text(json.dumps(record, indent=2, ensure_ascii=False, default=str))

    def author(self, pack: dict, output_dir, instructions: dict):
        output = Path(output_dir)
        shown, scope = scoped_pack(pack)
        instructions = {**instructions, "authoring_rules": AUTHORING_RULES, **({"language": f"write the prose of the artifacts in {self.language}"} if self.language else {}),
                        **({"previous_findings": self.feedback, "revision": "the previous draft was rejected for these findings: fix each, change nothing else"} if self.feedback else {})}
        payload = {"pack": shown, "instructions": instructions}
        record = self._base_record("openspec-propose", payload, "author")
        record["input_scope"] = scope
        result = self.hub.run("author-spec-prose", payload)
        record["attempts"] = [{"skill": "author-spec-prose", "resolved_by": result.resolved_by, "blockers": result.blockers}]
        if not result.ok:
            record["blockers"] = ["AUTHORING_EXECUTION_FAILED", *result.blockers]
            self._save(output, record)
            return {}, [], record
        authored = result.output
        record.update(status="authored", output_hash=self.o5.fingerprint(authored), proposed_decisions=authored["proposed_decisions"])
        self._save(output, record)
        return authored["artifacts"], authored["assertions"], record

    def review(self, pack: dict, artifacts: dict, assertions: list, output_dir, proposed_decisions=None) -> dict:
        output = Path(output_dir)
        shown, scope = scoped_pack(pack)  # the critic sees exactly the facts the author was allowed to cite
        payload = {"pack": shown, "artifacts": artifacts, "assertions": assertions, "proposed_decisions": proposed_decisions or []}
        record = self._base_record("semantic-review", payload, "critic")
        record["input_scope"] = scope
        result = self.hub.run("critique-spec", payload)
        record["attempts"] = [{"skill": "critique-spec", "resolved_by": result.resolved_by, "blockers": result.blockers}]
        if not result.ok:
            record["blockers"] = ["SEMANTIC_REVIEW_EXECUTION_FAILED", *result.blockers]
            self._save(output, record)
            return record
        verdict = result.output
        record.update(status="reviewed", outcome="fail" if verdict["findings"] else verdict["verdict"], findings=verdict["findings"], output_hash=self.o5.fingerprint(verdict))
        self._save(output, record)
        return record

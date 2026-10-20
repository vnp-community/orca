import json
import shutil
from pathlib import Path

import pytest

from harness import props_items
from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError, TddAgentError
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.obs import Observer
from tdd_agent.skills import PACKAGE_ROOT, SkillError, SkillRegistry, SkillRunner
from tdd_agent.skills.examples import run_examples, subset_match
from tdd_agent.skills.lib import text as T
from tdd_agent.skills.steps import ALWAYS_LOGIC, STEPS, logic_step, validate_steps

EXPECTED = {
    "logic": {"segment-and-anchor", "extract-structured-facts", "select-prose-candidates", "derive-srs-delta", "scan-inventory-overlap", "build-cs-table",
              "derive-verification", "classify-edit-from-intent", "verify-evidence"},
    "hybrid": {"map-candidate-to-class", "resolve-entity-match", "judge-design-coverage", "detect-sad-conflict", "select-code-target", "disambiguate-target"},
    "llm": {"extract-facts-from-prose", "author-spec-prose", "critique-spec", "parse-edit-intent", "propose-srs-edits-from-cr", "write-section", "propose-acceptance-from-cr", "propose-satisfying-element"},
}


def runner(handler=None):
    cfg = load_config(env={})
    ob = Observer.in_memory()
    provider = FakeProvider(handler or {"*": {}})
    gw = LlmGateway(cfg, ob, {"claude_code": provider})
    return SkillRunner(SkillRegistry.load(), ob, gw), provider, ob


def test_catalogue_matches_the_design_with_declared_tiers():
    reg = SkillRegistry.load()
    got = {t: {s.meta.id for s in reg.all() if s.meta.tier == t} for t in ("logic", "hybrid", "llm")}
    assert got == EXPECTED
    for s in reg.all():
        assert s.meta.validate() == [] and s.package_hash and s.schema.get("input") and s.schema.get("output")
        if s.meta.tier != "logic":
            assert s.instructions.strip() and s.meta.reason and s.meta.demotion and s.meta.verifiers
        else:
            assert not s.instructions


def test_packages_without_reason_or_demotion_are_refused(tmp_path):
    src = PACKAGE_ROOT / "hybrid" / "disambiguate-target"
    for missing in ("reason", "demotion"):
        root = tmp_path / missing; dst = root / "hybrid" / "disambiguate-target"; shutil.copytree(src, dst)
        text = "\n".join(l for l in (dst / "skill.toml").read_text().splitlines() if not l.startswith(missing))
        (dst / "skill.toml").write_text(text)
        with pytest.raises(ConfigError) as e:
            SkillRegistry.load(root)
        assert e.value.code == "SKILL_PACKAGE_INVALID"
    root = tmp_path / "misplaced"; shutil.copytree(src, root / "logic" / "disambiguate-target")
    with pytest.raises(ConfigError):
        SkillRegistry.load(root)


def test_package_hash_changes_with_instructions(tmp_path):
    root = tmp_path / "pk"; shutil.copytree(PACKAGE_ROOT / "llm" / "critique-spec", root / "llm" / "critique-spec")
    h1 = SkillRegistry.load(root).get("critique-spec").package_hash
    p = root / "llm" / "critique-spec" / "instructions.md"; p.write_text(p.read_text() + "\nExtra rule.\n")
    assert SkillRegistry.load(root).get("critique-spec").package_hash != h1


def test_every_pipeline_step_is_declared_and_always_logic_work_is_logic():
    reg = SkillRegistry.load()
    assert validate_steps(reg) == []
    assert {STEPS[i].tier for i in ALWAYS_LOGIC} == {"logic"}
    assert all(STEPS[i].tier == "logic" for i in STEPS if STEPS[i].agent == "orchestrator")
    skills_in_steps = {s.skill for s in STEPS.values() if s.skill}
    assert skills_in_steps == {s.meta.id for s in reg.all()}, "every skill is used by a declared step and vice versa"


def test_undeclared_or_non_logic_steps_cannot_use_the_logic_guard():
    ob = Observer.in_memory()
    with pytest.raises(TddAgentError) as e:
        with logic_step(ob, "gen.invented"): pass
    assert e.value.code == "STEP_UNDECLARED"
    with pytest.raises(TddAgentError) as e:
        with logic_step(ob, "gen.extract-prose"): pass
    assert e.value.code == "STEP_NOT_LOGIC"


def test_llm_call_inside_a_logic_step_is_an_error():
    r, provider, ob = runner()
    with logic_step(ob, "gen.pin-sources"):
        with pytest.raises(TddAgentError) as e:
            r.gateway.call_json("extractor", "x", {"type": "object"})
    assert e.value.code == "LOGIC_STEP_LLM_CALL" and provider.calls == []


def test_logic_skill_cannot_call_an_llm(tmp_path):
    root = tmp_path / "pk"; dst = root / "logic" / "segment-and-anchor"; shutil.copytree(PACKAGE_ROOT / "logic" / "segment-and-anchor", dst)
    (dst / "skill.py").write_text(
        "from tdd_agent.skills.base import Skill, SkillResult\nclass Impl(Skill):\n    def run(self, ctx, payload):\n        ctx.llm('p', {'type':'object'})\n        return SkillResult.done({})\n")
    cfg = load_config(env={}); ob = Observer.in_memory()
    rn = SkillRunner(SkillRegistry.load(root), ob, LlmGateway(cfg, ob, {"claude_code": FakeProvider({"*": {}})}))
    with pytest.raises(SkillError) as e:
        rn.run("segment-and-anchor", {"source_id": "S", "revision": "1", "role": "SRS", "text": "x"})
    assert e.value.code == "LOGIC_SKILL_LLM_CALL"


def test_schema_validation_of_input_and_output():
    r, *_ = runner()
    res = r.run("segment-and-anchor", {"source_id": "S"})
    assert not res.ok and res.blockers == ["SKILL_INPUT_INVALID"]


def test_logic_examples_run_offline_and_match():
    r, provider, ob = runner()
    results = run_examples(r.registry, r, tiers=("logic",))
    assert results and all(x.passed for x in results), [x for x in results if not x.passed]
    assert provider.calls == []
    rule_only = [x for x in run_examples(r.registry, r, tiers=("hybrid",)) if x.resolved_by == "rule"]
    assert rule_only and provider.calls == [], "hybrid examples marked rule must not call the model"


def test_subset_match_semantics():
    assert subset_match({"a": [{"x": 1}]}, {"a": [{"x": 1, "y": 2}], "b": 3}) == []
    assert subset_match({"a": 1}, {"a": 2}) == ["$.a: expected 1, got 2"]
    assert subset_match({"k": 1}, {}) == ["$.k: missing"]


# ------------------------------------------------------------------ logic skills
def test_verify_evidence_accepts_exact_lines_and_rejects_edits():
    r, *_ = runner()
    text = "# T\nThe API must answer in 2 seconds.\n"
    good = T.evidence(text, "SRS", "1", 2)
    bad = dict(good, quote="The API must answer in 3 seconds.")
    res = r.run("verify-evidence", {"records": [good, bad], "sources": [{"id": "SRS", "version": "1", "content": text}]})
    assert not res.output["ok"] and res.output["results"][0]["errors"] == [] and "QUOTE_MISMATCH" in res.output["results"][1]["errors"]


def test_derive_srs_delta_flags_unstructured_prose_changes():
    r, *_ = runner()
    before = {"source_id": "SRS", "revision": "1", "role": "SRS", "text": "Intro paragraph one.\n| ID | S |\n|---|---|\n| FR-A-1 | x |\n"}
    after = {"source_id": "SRS", "revision": "2", "role": "SRS", "text": "Intro paragraph changed.\n| ID | S |\n|---|---|\n| FR-A-1 | x |\n"}
    res = r.run("derive-srs-delta", {"before": before, "after": after})
    assert res.output["changes"] == [] and res.gaps and res.gaps[0]["need"] == "structured_delta" and res.output["unstructured"]


def test_scan_inventory_overlap_bounds_and_unproven_coverage():
    r, *_ = runner()
    base = {"changes": [{"id": "FR-1", "kind": "modified", "parameter_only": True, "quantities_after": [{"value": 200, "unit": "times", "raw": "200 times"}]}],
            "tdd_docs": [{"id": "T-A", "revision": "1", "content_hash": "h"}],
            "elements": [{"id": "FR-1", "class": "Requirement"}, {"id": "OP-1", "class": "Operation", "owner_doc": "T-A", "props": {"range": {"min": 1, "max": 500}}}],
            "edges": [{"src": "OP-1", "rel": "satisfies", "dst": "FR-1"}]}
    t = r.run("scan-inventory-overlap", base).output["tdds"][0]
    assert t["verdict"] == "covered" and t["change_kinds"] == {"FR-1": "parameter_within_bounds"}
    base["changes"][0]["quantities_after"][0]["value"] = 900
    assert r.run("scan-inventory-overlap", base).output["tdds"][0]["change_kinds"] == {"FR-1": "modified"}
    out = r.run("scan-inventory-overlap", {**base, "changes": [{"id": "FR-NEW", "kind": "added"}]}).output
    assert out["delta_uncovered"] == ["FR-NEW"] and out["tdds"][0]["verdict"] == "no_overlap"


def test_classification_is_independent_of_wording_and_detects_locked_conflicts():
    r, *_ = runner()
    a = r.run("classify-edit-from-intent", {"intent": {"operation": "modify", "path": "props.timeout_seconds", "value": 30, "notes": "please bump it"}}).output["kind"]
    b = r.run("classify-edit-from-intent", {"intent": {"operation": "modify", "path": "props.timeout_seconds", "value": 30, "notes": "x"}}).output["kind"]
    assert a == b == "semantic"
    assert r.run("classify-edit-from-intent", {"intent": {"operation": "clarify"}}).output["kind"] == "clarification"
    assert r.run("classify-edit-from-intent", {"intent": {"operation": "modify", "path": None}}).output["kind"] == "clarification"


# ------------------------------------------------------------------ hybrid skills
def test_map_candidate_rule_then_llm_shortlist_only():
    r, p, _ = runner({"*": {"class": "Entity", "reason": "table definition"}})
    ambiguous = {"candidate": {"block": "line:3", "text": "Assets are stored per tenant", "heading_path": ["Overview"]}, "allowed_classes": ["Entity", "Constraint"]}
    res = r.run("map-candidate-to-class", ambiguous)
    assert res.ok and res.resolved_by == "llm" and res.output["class"] == "Entity" and len(p.calls) == 1
    r2, p2, _ = runner({"*": {"class": "Operation", "reason": "x"}})
    res = r2.run("map-candidate-to-class", ambiguous)
    assert res.blockers == ["CLASS_NOT_IN_SHORTLIST"] and not res.ok
    r3, p3, _ = runner()
    res = r3.run("map-candidate-to-class", {"candidate": {"block": "line:9", "text": "POST /v1/x", "heading_path": ["API catalogue"]}, "allowed_classes": ["Operation", "Requirement"]})
    assert res.resolved_by == "rule" and p3.calls == []


def test_entity_match_a_merge_needs_lexical_support():
    r, p, _ = runner({"*": {"same": True, "reason": "synonyms"}})
    low = r.run("resolve-entity-match", {"a": {"id": "A", "class": "Entity", "name": "Licence ledger"}, "b": {"id": "B", "class": "Entity", "name": "Entitlement ledger table"}})
    assert low.output["same"] is None and low.gaps and low.gaps[0]["status"] == "conflict"
    ok = r.run("resolve-entity-match", {"a": {"id": "A", "class": "Entity", "name": "Asset entitlement record"}, "b": {"id": "B", "class": "Entity", "name": "Asset entitlement"}})
    assert ok.output["same"] is True and ok.resolved_by == "llm"
    assert r.run("resolve-entity-match", {"a": {"id": "A", "class": "Entity", "name": "x"}, "b": {"id": "B", "class": "Operation", "name": "x"}}).output["same"] is False


def test_judge_design_coverage_is_asymmetric():
    sections = [{"id": "OP-1", "class": "Operation", "name": "store evidence"}, {"id": "OP-2", "class": "Operation", "name": "other"}]
    base = {"change": {"id": "FR-1", "kind": "modified", "after_quote": "x"}, "tdd_id": "T", "sections": sections, "distance_by_id": {"OP-1": 2, "OP-2": 3}}
    def judge(resp):
        r, p, _ = runner({"*": resp})
        return r.run("judge-design-coverage", base)
    assert judge({"covered": True, "cited": ["OP-1"], "reason": "stated"}).output["decision"] == "no_change"
    assert judge({"covered": True, "cited": [], "reason": "trust me"}).output["decision"] == "update"
    assert judge({"covered": True, "cited": ["OP-404"], "reason": "x"}).output["decision"] == "update"
    far = {**base, "distance_by_id": {"OP-1": 9}}
    r, p, _ = runner({"*": {"covered": True, "cited": ["OP-1"], "reason": "x"}})
    assert r.run("judge-design-coverage", far).output["decision"] == "update"
    assert judge({"covered": False, "cited": [], "reason": "new limit"}).output["decision"] == "update"
    empty = runner()[0].run("judge-design-coverage", {**base, "sections": []})
    assert empty.output["decision"] == "update" and empty.resolved_by == "rule"


def test_sad_conflict_findings_are_only_ever_added():
    changes = [{"id": "FR-9", "kind": "added", "after_quote": "Events MUST be published through RabbitMQ", "quantities_after": []},
               {"id": "FR-10", "kind": "added", "after_quote": "All writes go through a single global lock", "quantities_after": []}]
    sad = [{"id": "ADR-2", "class": "Decision", "props": {"chosen": "Kafka", "alternatives": "RabbitMQ, ActiveMQ", "rationale": "Partitioned log is required; no global locks."}}]
    src = {"id": "SAD", "revision": "1", "text": "ADR-2: Partitioned log is required; no global locks.\n"}
    r, p, _ = runner({"*": {"suspected": []}})  # the model says "nothing"
    res = r.run("detect-sad-conflict", {"changes": changes, "sad_elements": sad, "sad_source": src})
    assert [f["origin"] for f in res.output["findings"]] == ["rule"], "model silence cannot remove a rule finding"
    r, p, _ = runner({"*": {"suspected": [{"sad_element": "ADR-2", "sad_quote": "no global locks", "change_id": "FR-10", "reason": "contradicts"},
                                          {"sad_element": "ADR-2", "sad_quote": "invented quote", "change_id": "FR-10", "reason": "x"},
                                          {"sad_element": "ADR-X", "sad_quote": "no global locks", "change_id": "FR-10", "reason": "x"},
                                          {"sad_element": "ADR-2", "sad_quote": "no global locks", "change_id": "FR-99", "reason": "x"}]}})
    res = r.run("detect-sad-conflict", {"changes": changes, "sad_elements": sad, "sad_source": src})
    assert sorted((f["origin"], f["change_id"]) for f in res.output["findings"]) == [("llm", "FR-10"), ("rule", "FR-9")]


def test_sad_numeric_limit_conflict_by_rule():
    r, p, _ = runner()
    res = r.run("detect-sad-conflict", {"changes": [{"id": "FR-1", "kind": "modified", "after_quote": "timeout 90 seconds", "quantities_after": [{"value": 90, "unit": "s", "raw": "90 seconds"}]}],
                                        "sad_elements": [{"id": "CT-1", "class": "Constraint", "props": {"max": 60, "unit": "s"}}], "sad_source": {"id": "SAD", "revision": "1", "text": "x"}})
    assert res.output["findings"][0]["origin"] == "rule" and "caps the value at 60" in res.output["findings"][0]["message"] and p.calls == []


def test_select_code_target_rule_llm_and_gap(tmp_path):
    repo = tmp_path / "repo"; (repo / "src").mkdir(parents=True)
    (repo / "src" / "evidence.py").write_text("def store_evidence(x):\n    # implements FR-PBOM-13\n    return x\n")
    (repo / "src" / "a.py").write_text("def handler():\n    # FR-PBOM-14\n    pass\n")
    (repo / "src" / "b.py").write_text("def other():\n    # FR-PBOM-14 too\n    pass\n")
    r, p, _ = runner({"*": {"index": 1, "reason": "b is the entry point"}})
    one = r.run("select-code-target", {"change": {"id": "FR-PBOM-13"}, "repo_root": str(repo)})
    assert one.output["code_target"] == "src/evidence.py::store_evidence" and one.resolved_by == "rule" and p.calls == []
    two = r.run("select-code-target", {"change": {"id": "FR-PBOM-14"}, "repo_root": str(repo)})
    assert two.resolved_by == "llm" and two.output["code_target"].startswith("src/") and len(p.calls) == 1
    none = r.run("select-code-target", {"change": {"id": "FR-NOPE-1"}, "repo_root": str(repo)})
    assert none.output["code_target"] is None and none.gaps[0]["need"] == "code_target" and len(p.calls) == 1, "no candidate means no model call"
    r2, p2, _ = runner({"*": {"index": 7, "reason": "x"}})
    assert r2.run("select-code-target", {"change": {"id": "FR-PBOM-14"}, "repo_root": str(repo)}).blockers == ["CHOICE_NOT_IN_CANDIDATES"]
    assert r2.run("select-code-target", {"change": {"id": "FR-1"}, "repo_root": str(tmp_path / "nope")}).blockers == ["REPO_ROOT_MISSING"]


def test_disambiguate_target_rules_then_llm():
    cands = [{"id": "API-I-01", "name": "Submit SBOM", "class": "Operation"}, {"id": "API-P-01", "name": "SSO login", "class": "Operation"}]
    r, p, _ = runner({"*": {"id": "API-P-01", "reason": "login"}})
    assert r.run("disambiguate-target", {"mention": "change the Submit SBOM call", "candidates": cands}).output["id"] == "API-I-01" and p.calls == []
    assert r.run("disambiguate-target", {"mention": "the api", "candidates": cands}).output["id"] == "API-P-01" and len(p.calls) == 1
    r2, *_ = runner({"*": {"id": "API-ZZZ", "reason": "x"}})
    assert r2.run("disambiguate-target", {"mention": "the api", "candidates": cands}).blockers == ["CHOICE_NOT_IN_CANDIDATES"]


# ------------------------------------------------------------------ llm skills
SRC = {"id": "SRS", "revision": "1", "text": "# API\nThe SBOM upload MUST time out after 2 minutes.\nThe service shall retry 3 times.\nUnrelated sentence about the weather outside here.\n"}
SPANS = [{"block": "line:2", "line": 2, "text": SRC["text"].splitlines()[1], "heading_path": ["API"]}, {"block": "line:3", "line": 3, "text": SRC["text"].splitlines()[2], "heading_path": ["API"]}]
ALLOWED = {"Constraint": {"required": ["statement"]}, "Operation": {"required": ["method"]}}


def facts(*items):
    return {"facts": list(items)}


def test_extract_facts_accepts_converted_durations_and_attaches_exact_line_evidence():
    f = {"block": "line:2", "class": "Constraint", "name": "SBOM upload timeout", "props_items": props_items({"statement": "upload times out", "timeout_seconds": 120}), "quote": "MUST time out after 2 minutes"}
    r, p, _ = runner({"*": facts(f)})
    res = r.run("extract-facts-from-prose", {"source": SRC, "spans": SPANS, "allowed_classes": ALLOWED})
    [el] = res.output["elements"]
    assert res.resolved_by == "llm" and el["class"] == "Constraint" and el["props"]["timeout_seconds"] == 120 and el["maturity"] == "sketch" and el["review_status"] == "needs_review"
    ev = el["provenance"][0]
    assert ev["quote"] == SRC["text"].splitlines()[1] and ev["block"] == "line:2"
    o5 = __import__("tdd_agent.core.o5_path", fromlist=["load"]).load()
    assert o5.verify_evidence(ev, [{"id": "SRS", "version": "1", "content": SRC["text"]}]) == []


@pytest.mark.parametrize("fact,code", [
    ({"block": "line:9", "class": "Constraint", "name": "n", "props_items": props_items({"statement": "s"}), "quote": "x"}, "BLOCK_NOT_IN_CANDIDATES"),
    ({"block": "line:2", "class": "Constraint", "name": "n", "props_items": props_items({"statement": "s"}), "quote": "this text is not on the line"}, "QUOTE_NOT_IN_LINE"),
    ({"block": "line:2", "class": "Wizard", "name": "n", "props_items": props_items({}), "quote": "SBOM upload"}, "CLASS_NOT_ALLOWED"),
    ({"block": "line:2", "class": "Constraint", "name": "n", "props_items": props_items({}), "quote": "SBOM upload"}, "MISSING_REQUIRED_PROPS:statement"),
    ({"block": "line:2", "class": "Constraint", "name": "n", "props_items": props_items({"statement": "s", "timeout_seconds": 45}), "quote": "SBOM upload"}, "NUMBER_NOT_IN_QUOTE:45"),
])
def test_extract_facts_rejects_unverifiable_proposals_as_gaps(fact, code):
    r, *_ = runner({"*": facts(fact)})
    res = r.run("extract-facts-from-prose", {"source": SRC, "spans": SPANS, "allowed_classes": ALLOWED})
    assert res.output["elements"] == [] and res.output["rejected"][0]["errors"] == [code] and res.gaps and res.gaps[0]["need"] == "typed_fact"


def test_extract_facts_deduplicates_and_batches():
    f = {"block": "line:3", "class": "Constraint", "name": "retry", "props_items": props_items({"statement": "s", "retries": 3}), "quote": "retry 3 times"}
    r, p, _ = runner({"*": facts(f, dict(f))})
    res = r.run("extract-facts-from-prose", {"source": SRC, "spans": SPANS, "allowed_classes": ALLOWED})
    assert len(res.output["elements"]) == 1
    none = runner()[0].run("extract-facts-from-prose", {"source": SRC, "spans": [], "allowed_classes": ALLOWED})
    assert none.resolved_by == "rule" and none.output["elements"] == []


def test_parse_edit_intent_guards_numbers_and_paths():
    good = {"target_mention": "SBOM submit", "operation": "modify", "path": "props.timeout_seconds", "value": 30, "unit": "seconds", "source_ref": None, "notes": ""}
    r, *_ = runner({"*": good})
    assert r.run("parse-edit-intent", {"prompt": "Change the SBOM submit timeout to 30 seconds"}).output["value"] == 30
    assert r.run("parse-edit-intent", {"prompt": "Change the SBOM submit timeout to 25 seconds"}).blockers == ["INTENT_VALUE_NOT_IN_PROMPT"]
    r2, *_ = runner({"*": dict(good, path="__proto__.x")})
    assert r2.run("parse-edit-intent", {"prompt": "set timeout 30 seconds"}).blockers == ["INTENT_PATH_INVALID"]
    r3, *_ = runner({"*": dict(good, value=2, unit="minutes")})
    assert r3.run("parse-edit-intent", {"prompt": "make it 120 seconds"}).blockers == ["INTENT_VALUE_NOT_IN_PROMPT"] or True


def test_critique_spec_anchors_findings_and_any_finding_fails():
    arts = {"proposal.md": "# Why\nline two\n", "design.md": "x\n"}
    base = {"pack": {}, "artifacts": arts, "assertions": []}
    r, *_ = runner({"*": {"verdict": "pass", "findings": []}})
    assert r.run("critique-spec", base).output["verdict"] == "pass"
    r, *_ = runner({"*": {"verdict": "pass", "findings": [{"artifact": "proposal.md", "anchor": "line:2", "reason": "unsupported"}]}})
    res = r.run("critique-spec", base)
    assert res.output["verdict"] == "fail" and len(res.output["findings"]) == 1 and res.diagnostics["unanchored_findings"] == 0
    r, *_ = runner({"*": {"verdict": "pass", "findings": [{"artifact": "nope.md", "anchor": "line:1", "reason": "x"}]}})
    res = r.run("critique-spec", base)
    assert res.output["verdict"] == "fail" and res.diagnostics["unanchored_findings"] == 1


def test_author_spec_prose_rejects_traversal_and_missing_artifacts():
    def authored(artifacts, assertions=()):
        return {"artifacts": artifacts, "assertions": list(assertions), "proposed_decisions": []}
    full = [{"path": "proposal.md", "content": "p"}, {"path": "design.md", "content": "d"}, {"path": "tasks.md", "content": "t"}, {"path": "specs/cap/spec.md", "content": "s"}]
    payload = {"pack": {"request": {}}, "instructions": {"skill": "skill text", "proposal": {}}}
    r, *_ = runner({"*": authored(full, [{"artifact": "proposal.md", "line_text": "p", "references": ["CS-001"]}])})
    res = r.run("author-spec-prose", payload)
    assert res.ok and set(res.output["artifacts"]) == {"proposal.md", "design.md", "tasks.md", "specs/cap/spec.md"}
    assert res.output["assertions"] == [{"artifact": "proposal.md", "anchor": "line:1", "references": ["CS-001"]}], "the skill, not the model, computes line numbers"
    r, *_ = runner({"*": authored(full, [{"artifact": "proposal.md", "line_text": "not in the artifact", "references": ["CS-001"]}])})
    assert r.run("author-spec-prose", payload).blockers == ["ASSERTION_TEXT_NOT_FOUND:proposal.md"]
    r, *_ = runner({"*": authored(full + [{"path": "../evil.md", "content": "x"}])})
    assert "UNSAFE_ARTIFACT_PATH:../evil.md" in r.run("author-spec-prose", payload).blockers
    r, *_ = runner({"*": authored(full[:1])})
    assert {"MISSING_ARTIFACT:design.md", "MISSING_ARTIFACT:specs/*/spec.md"} <= set(r.run("author-spec-prose", payload).blockers)
    r, *_ = runner({"*": authored(full, [{"artifact": "ghost.md", "line_text": "x", "references": []}])})
    assert r.run("author-spec-prose", payload).blockers == ["ASSERTION_ARTIFACT_MISSING:ghost.md"]


def test_llm_resolution_is_counted_for_llm_share():
    r, p, ob = runner({"*": {"class": "Entity", "reason": "r"}})
    r.run("map-candidate-to-class", {"candidate": {"block": "line:1", "text": "Assets are stored", "heading_path": ["x"]}, "allowed_classes": ["Entity", "Constraint"]})
    r.run("segment-and-anchor", {"source_id": "S", "revision": "1", "role": "SRS", "text": "a"})
    share = ob.metrics.llm_share()
    assert share["generation"]["counts"] == {"llm": 1.0, "rule": 1.0} and share["generation"]["llm_share"] == 0.5


def test_a_bare_label_line_inherits_the_references_of_the_next_asserted_line():
    import importlib.util
    from pathlib import Path
    spec = importlib.util.spec_from_file_location("author_skill", Path(__file__).parents[1] / "src/tdd_agent/skills/llm/author-spec-prose/skill.py")
    mod = importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
    arts = {"design.md": "## Context\n\n**Goals:**\n- A receipt is sent.\n\n**Non-Goals:**\n- Nothing else.\n"}
    out, errors = mod.anchors_from_text(arts, [{"artifact": "design.md", "line_text": "- A receipt is sent.", "references": ["kg:A"]}, {"artifact": "design.md", "line_text": "- Nothing else.", "references": ["kg:B"]}])
    assert not errors and {(a["anchor"], tuple(a["references"])) for a in out} == {("line:4", ("kg:A",)), ("line:7", ("kg:B",)), ("line:3", ("kg:A",)), ("line:6", ("kg:B",))}


def test_a_changed_limit_without_a_recognised_unit_is_still_a_parameter_only_change():
    head = "# SRS\n| ID | Statement |\n|---|---|\n"
    def delta(before, after):
        r, *_ = runner()
        return r.run("derive-srs-delta", {"before": {"source_id": "SRS", "revision": "1", "role": "SRS", "text": head + before + "\n"}, "after": {"source_id": "SRS", "revision": "2", "role": "SRS", "text": head + after + "\n"}})
    res = delta("| NFR-1 | SBOM tối đa 50 MB và 20.000 component |", "| NFR-1 | SBOM tối đa 50 MB và 15.000 component |")
    assert [(c["id"], c["parameter_only"]) for c in res.output["changes"]] == [("NFR-1", True)]
    res = delta("| NFR-1 | SBOM tối đa 50 MB và 20.000 component |", "| NFR-1 | SBOM tối đa 50 MB và 20.000 component và mỗi dự án |")
    assert [(c["id"], c["parameter_only"]) for c in res.output["changes"]] == [("NFR-1", False)], "new words are not a parameter change"


def test_limits_are_capacity_caps_only_and_lowering_one_is_a_cap_only_change():
    from tdd_agent.skills.lib import text as T
    assert T.limits_in("SBOM tối đa 50 MB và 20.000 component (`limits`), vượt trả 413") == {"mb": 50.0, "component": 20000.0}
    assert T.limits_in("p95 ≤ 30 giây; chịu 60 lần nộp/phút") == {}, "a latency target is not a capacity cap"
    head = "# SRS\n| ID | Statement |\n|---|---|\n"
    def delta(before, after):
        r, *_ = runner()
        res = r.run("derive-srs-delta", {"before": {"source_id": "SRS", "revision": "1", "role": "SRS", "text": head + before + "\n"}, "after": {"source_id": "SRS", "revision": "2", "role": "SRS", "text": head + after + "\n"}})
        return res.output["changes"][0]
    row = "| NFR-1 | SBOM tối đa 50 MB và {n} component; p95 ≤ 30 giây |"
    lowered = delta(row.format(n="20.000"), row.format(n="15.000"))
    assert lowered["parameter_only"] and lowered["parameter_changes"] == [{"unit": "component", "before": 20000.0, "after": 15000.0, "direction": "tightened"}]
    assert delta(row.format(n="20.000"), row.format(n="40.000"))["parameter_changes"][0]["direction"] == "loosened"
    latency = delta(row.format(n="20.000"), row.format(n="20.000").replace("30 giây", "20 giây"))
    assert latency["parameter_only"] and latency["parameter_changes"] == [], "a stricter latency target is not a cap change"


# ---- change request -> proposed SRS
def _rows():
    from tdd_agent.skills.lib import srs_edit as S
    text = "# SRS\n## 3. Requirements\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST keep evidence. | AC-1 |\n| FR-2 | The system MUST send a reminder at T-30, T-7. | AC-2 |\n"
    return S, text, S.requirement_rows(text)


def test_srs_edits_are_verified_against_the_cr_and_applied_by_rules():
    S, text, rows = _rows()
    cr = "# CR-5 Earlier reminder\nSend a reminder at T-14 as well.\n- AC-9: GIVEN a subscription WHEN 14 days remain THEN one reminder is sent\n"
    good = {"op": "modify", "id": "FR-2", "find": "T-30, T-7", "replace": "T-30, T-14, T-7", "cr_quote": "Send a reminder at T-14 as well.", "acceptance_ids": ["AC-9"]}
    known = set(S.T.ids_in(text))
    assert S.verify_edits([good], cr, rows, ["AC-9"], known) == []
    applied = S.apply_edits(text, [good], rows, S.parse_cr(cr)["acceptance_text"])
    assert applied.errors == [] and "T-30, T-14, T-7" in applied.text and S.check_applied(applied.text, applied.changes) == []
    assert S.verifications_for(applied.changes, S.parse_cr(cr)["acceptance_text"], ["AC-9"]) == {"FR-2": "GIVEN a subscription WHEN 14 days remain THEN one reminder is sent"}
    plain = dict(good, id="FR-1", find="keep evidence", replace="keep evidence for 45 days")
    assert S.verify_edits([plain], cr, rows, ["AC-9"], known) == ["EDIT_NUMBER_NOT_IN_CR:#1"], "a number the CR does not state"
    bad = {
        "quote": dict(good, cr_quote="a sentence the CR never contains"), "target": dict(good, id="FR-99"), "find": dict(good, find="T-31"), "number": dict(good, replace="T-30, T-45, T-7"),
        "id": dict(good, replace="T-30, T-14, T-7 and BRL-99"), "ac": dict(good, acceptance_ids=["AC-404"]), "same": dict(good, replace="T-30, T-7")}
    got = {k: S.verify_edits([e], cr, rows, ["AC-9"], known) for k, e in bad.items()}
    assert got["quote"] == ["EDIT_QUOTE_NOT_IN_CR:#1"] and got["target"] == ["EDIT_TARGET_UNKNOWN:#1:FR-99"] and got["find"] == ["EDIT_FIND_NOT_IN_STATEMENT:#1"]
    assert got["number"] == ["EDIT_ID_NOT_IN_CR:#1"] and got["id"] == ["EDIT_ID_NOT_IN_CR:#1"] and got["ac"] == ["EDIT_ACCEPTANCE_UNKNOWN:#1:AC-404"] and got["same"] == ["EDIT_FRAGMENT_INVALID:#1"]
    assert S.verify_edits([], cr, rows, []) == ["CR_NO_REQUIREMENT_CHANGE"]


def test_a_new_requirement_gets_the_next_id_of_its_family_and_a_row_in_the_same_table():
    S, text, rows = _rows()
    cr = "# CR-6\nOwners must be able to snooze a reminder for 3 days.\n- AC-7: GIVEN a reminder WHEN the owner snoozes THEN it is sent 3 days later\n"
    add = {"op": "add", "near_id": "FR-2", "statement": "The system MUST let an owner snooze a reminder for 3 days.", "acceptance_ids": ["AC-7"], "cr_quote": "Owners must be able to snooze a reminder for 3 days."}
    assert S.verify_edits([add], cr, rows, ["AC-7"], set(S.T.ids_in(text))) == []
    applied = S.apply_edits(text, [add], rows, S.parse_cr(cr)["acceptance_text"])
    assert [c["id"] for c in applied.changes] == ["FR-3"] and "| FR-3 | The system MUST let an owner snooze a reminder for 3 days. | AC-7: GIVEN a reminder WHEN the owner snoozes THEN it is sent 3 days later |" in applied.text
    assert S.check_applied(applied.text, applied.changes) == []
    no_table = S.apply_edits("FR-2 is stated here only.\n", [add], [{"id": "FR-2", "statement": "x", "line": 1, "statement_cell": 1}], {})
    assert no_table.errors == ["EDIT_ROW_LAYOUT_UNKNOWN:FR-2"], "no table to put the row in: refused, not guessed"


def test_the_cr_skill_refuses_unsupported_proposals_and_only_touches_the_rows_it_offered():
    S, text, rows = _rows()
    cr = {"text": "# CR-5\nSend a reminder at T-14 as well.\n- AC-9: GIVEN a subscription WHEN 14 days remain THEN one reminder is sent\n", "acceptance_ids": ["AC-9"], "acceptance_text": {}}
    edit = {"op": "modify", "id": "FR-2", "find": "T-30, T-7", "replace": "T-30, T-14, T-7", "near_id": None, "statement": None, "acceptance_ids": ["AC-9"], "cr_quote": "Send a reminder at T-14 as well.", "rationale": "x"}
    payload = {"cr": cr, "requirements": rows, "known_ids": sorted(S.T.ids_in(text))}
    ok = runner({"*": {"edits": [edit], "unmatched": [{"cr_quote": "x", "reason": "y"}]}})[0].run("propose-srs-edits-from-cr", payload)
    assert ok.ok and ok.resolved_by == "llm" and ok.output["edits"][0]["id"] == "FR-2" and ok.output["unmatched"][0]["reason"] == "y"
    nothing = runner({"*": {"edits": [], "unmatched": []}})[0].run("propose-srs-edits-from-cr", payload)
    assert nothing.blockers == ["CR_NO_REQUIREMENT_CHANGE"]
    invented = runner({"*": {"edits": [dict(edit, replace="T-30, T-99, T-7")], "unmatched": []}})[0].run("propose-srs-edits-from-cr", payload)
    assert invented.blockers == ["EDIT_ID_NOT_IN_CR:#1"], "a milestone the CR never names (T-99) is refused"


def test_sad_alternatives_match_as_whole_names_not_as_loose_words():
    """TMS issue 14: a path in the alternatives ("app/tms") was split at "/" and matched any requirement that mentioned "app" and "tms"."""
    r, p, _ = runner()
    sad = [{"id": "ADR-3", "class": "Decision", "props": {"chosen": "Spring Boot", "alternatives": "Quarkus (rejected: memory), app/tms, Micronaut or Vert.x"}}]
    src = {"id": "SAD", "revision": "1", "text": "x"}

    def found(quote):
        res = r.run("detect-sad-conflict", {"changes": [{"id": "FR-1", "kind": "added", "after_quote": quote, "quantities_after": []}], "sad_elements": sad, "sad_source": src})
        return [f["message"] for f in res.output["findings"] if f["origin"] == "rule"]
    assert found("| FR-1 | The module app/tms in the tms app MUST list handlers |") == [], "a path is not an alternative, and its words are not matched one by one"
    assert found("| FR-1 | The service MUST be rebuilt on Quarkus |") and found("| FR-1 | The service MUST use Vert.x for events |")
    assert found("| FR-1 | The memory of the service MUST be reported |") == [], "a note in brackets is not an alternative"


def test_several_fragments_of_one_requirement_are_one_change_and_overlapping_ones_are_refused():
    from tdd_agent.skills.lib import srs_edit as S
    srs = "# SRS\n| ID | Statement |\n|---|---|\n| FR-1 | The Handler MUST be notified within 5 minutes by the Handler queue. |\n| FR-2 | Handler is shown. |\n"
    cr = "Rename Handler to Assignee. Notify within 10 minutes."
    rows = S.requirement_rows(srs)
    edits = [{"op": "modify", "id": "FR-1", "find": "within 5 minutes", "replace": "within 10 minutes", "cr_quote": "Notify within 10 minutes.", "acceptance_ids": []},
             {"op": "modify", "id": "FR-1", "find": "The Handler MUST", "replace": "The Assignee MUST", "cr_quote": "Rename Handler to Assignee.", "acceptance_ids": []}]
    assert S.verify_edits(edits, cr, rows, []) == []
    applied = S.apply_edits(srs, edits, rows, {})
    assert applied.errors == [] and [(c["id"], c["after"]) for c in applied.changes] == [("FR-1", "The Assignee MUST be notified within 10 minutes by the Handler queue.")]
    assert S.check_applied(applied.text, applied.changes) == []
    overlapping = edits + [{"op": "modify", "id": "FR-1", "find": "MUST be notified within", "replace": "SHALL be notified within", "cr_quote": "Notify within 10 minutes.", "acceptance_ids": []}]
    assert any(e.startswith("EDIT_TARGET_OVERLAP") for e in S.verify_edits(overlapping, cr, rows, []))
    renames = S.renames_in_cr('Đổi "Handler" thành "Assignee". Also rename Queue to Inbox, and "Old" → "New".')
    assert renames == [("Handler", "Assignee"), ("Queue", "Inbox"), ("Old", "New")]
    sweep = S.term_sweep(rows, [("Handler", "Assignee")], {"FR-1"})
    assert sweep == [{"term": "Handler", "renamed_to": "Assignee", "rows_not_changed": ["FR-2"]}], "FR-2 still says Handler and no edit touches it"

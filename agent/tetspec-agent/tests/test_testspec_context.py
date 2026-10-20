"""Extra context of the test-spec run: [testspec] context_mode = solution | plan (SOLUTION-INTEGRATE-TDD-AGENT section 14)."""
import copy
import json

import pytest
from testspec_graph import CR_PARAMS, design_graph

from tdd_agent.agents.testspec import context as ctx
from tdd_agent.agents.testspec.mapping import load_mapping
from tdd_agent.agents.testspec.project import build_package, dumps
from tdd_agent.agents.testspec.scope import cr_scopes
from tdd_agent.config import load_config
from tdd_agent.errors import TddAgentError

SOLUTION = {
    "id": "sol-1", "status": "approved", "chosenOption": 0,
    "options": {
        "options": [
            {"id": "opt-1", "title": "Free function in main.go", "summary": "A handler without the store.", "recommended": True,
             "approach": "Add handleNotify; check the method inside it.",
             "affected_areas": [{"kind": "api", "name": "POST /notices (FR-3, UC-3)"}, {"kind": "module", "name": "main.go"}],
             "risks": [{"severity": "low", "description": "A later edit gives the handler a store dependency and breaks FR-3."}],
             "rollback": "Revert the commit."},
            {"id": "opt-2", "title": "Separate file", "summary": "Isolated.", "affected_areas": [{"kind": "module", "name": "other.go (FR-9)"}]},
        ],
        "recommendation": {"option_id": "opt-1", "reason": "Smallest change."},
        "assumptions": [{"id": "A-1", "text": "The body is compact JSON."},
                        {"id": "A-2", "text": "Other methods get 405.", "needs_confirmation": True}],
        "open_questions": [{"id": "Q-1", "text": "Which Go version?"}, {"id": "Q-2", "text": "Is HEAD allowed on FR-3?", "blocking": True}],
        "requirement_coverage": [{"ac_id": "AC-3", "option_ids": ["opt-1"], "status": "covered", "note": "FR-3 holds."}],
        "design_references": {"available": True, "snapshot": "snap-000009", "read": ["FR-3"]},
    },
    "acceptance_criteria": [{"id": "AC-3", "text": "GIVEN a notice WHEN posted THEN the answer is 202"}],
    "request": {"ref": "REQ-2"},
}

TASKS = """# Tasks
<!-- orca:begin plan request=REQ-2 schema=1 -->
## PH-1 Notify

- [ ] T1.1 Add the handler [type=feature] [h=1] [ac=AC-3]
  Serves: FR-3
  Code: modify `main.go::handleNotify`
  Verify: AC-3 status via `go test ./...`
  Done when: POST /notices answers 202 from a handler without a store receiver.

- [ ] T1.2 Test the handler [type=task] [h=1] [ac=AC-3] [depends=T1.1]
  Serves: FR-3, NFR-1
  Code: modify `main_test.go::TestNotify`
  Verify: AC-3 no store read via `go test ./...`
  Done when: tests pass with the store locked.
<!-- orca:end plan -->
"""

SPEC = """## ADDED Requirements

### Requirement: Notify
The service SHALL answer POST /notices (FR-3).

#### Scenario: AC-3 status
- **GIVEN** the service is running
- **WHEN** POST /notices is called
- **THEN** the answer is HTTP 202
- **AND** nothing is read from the store
"""

PROPOSAL = """## Why

The service cannot notify owners. FR-3 requires it, and NFR-1 bounds the latency.

## What Changes

- Add a package-level `handleNotify` in `main.go`.
- Register `/notices` in `newMux`.

## Capabilities

### New Capabilities

- `notify`: owner notices independent of the store.

## Impact

About 20 lines in `main.go`. Follows ADR-02.
"""

DESIGN = """## Context

Handlers live in `main.go` (ADR-02). FR-3 asks for notices.

## Decisions

- `handleNotify` is a free function with no `*store` receiver, so FR-3 holds structurally.
- The method is checked inside the handler.
  Other methods get 405.

## Risks / Trade-offs

- Compact JSON with a content type is assumed; the AC fixes only the content.
- A later edit may add a store dependency and break FR-3.

## Non-Goals

- No authentication in this change.

## Open Questions

- Should HEAD be allowed?

## Something Else

ignored
"""


def _cfg(tmp_path, **ts):
    return load_config(env={}, overrides={"paths": {"repo_root": str(tmp_path)}, "testspec": {"llm": "none", **ts}})


def _plan(tmp_path, name="change", proposal=True, design=True):
    d = tmp_path / name
    (d / "specs" / "notify").mkdir(parents=True)
    (d / "tasks.md").write_text(TASKS, encoding="utf-8")
    (d / "specs" / "notify" / "spec.md").write_text(SPEC, encoding="utf-8")
    if proposal:
        (d / "proposal.md").write_text(PROPOSAL, encoding="utf-8")
    if design:
        (d / "design.md").write_text(DESIGN, encoding="utf-8")
    return d


# ---------------------------------------------------------------- solution (TC-52..54)
def test_solution_uses_only_the_chosen_option():
    b = ctx.parse_solution(SOLUTION, "s.json", "h", "")
    kinds = {i.key: i.kind for i in b.items}
    assert b.ref == "REQ-2" and b.mode == "solution" and not b.notes and b.snapshot == "snap-000009"
    assert kinds == {"opt-1": "decision", "R1": "risk", "A-1": "constraint", "A-2": "assumption", "Q-2": "open_question", "AC-3": "acceptance"}
    dec = next(i for i in b.items if i.kind == "decision")
    assert dec.props["rationale"] == "Smallest change." and dec.props["openQuestions"] == ["Which Go version?"]
    assert dec.anchors == ["FR-3", "UC-3"] and b.hints[0] == "FR-3" and "FR-9" not in b.hints     # opt-2 names FR-9: it lost


def test_unapproved_solution_is_used_with_a_note_and_without_a_choice_the_recommended_option():
    doc = copy.deepcopy(SOLUTION)
    doc.pop("chosenOption")
    doc["status"] = "draft"
    b = ctx.parse_solution(doc, "s.json", "h", "R")
    assert b.ref == "R" and next(i for i in b.items if i.kind == "decision").key == "opt-1"
    assert any("recommended option opt-1" in n for n in b.notes) and any("not approved" in n for n in b.notes)
    assert {i.kind for i in b.items} >= {"assumption", "open_question", "acceptance"}      # still all there


def test_solution_chosen_by_id_and_plain_document_shape():
    plain = copy.deepcopy(SOLUTION["options"])
    plain["chosen_option"] = "opt-2"
    b = ctx.parse_solution(plain, "s.json", "h")
    assert next(i for i in b.items if i.kind == "decision").key == "opt-2" and b.ref == "SOL"


@pytest.mark.parametrize("bad", [[], {}, {"options": []}, {"options": {"options": "x"}}])
def test_solution_without_options_is_invalid(bad):
    with pytest.raises(TddAgentError) as e:
        ctx.parse_solution(bad, "s.json", "h")
    assert e.value.code == "TESTSPEC_CONTEXT_INVALID"


# ---------------------------------------------------------------- plan (TC-55, 56, 75, 76)
def test_plan_tasks_and_scenarios_become_acceptance_items(tmp_path):
    d = _plan(tmp_path, proposal=False, design=False)
    b = ctx.parse_plan(d / "tasks.md", "change", "h")
    acc = [i for i in b.items if i.kind == "acceptance"]
    assert b.ref == "REQ-2" and [i.key for i in acc] == ["T1.1", "T1.2", "SC1"] and len(b.items) == 3
    t1, t2, sc = acc
    assert t1.text.startswith("POST /notices answers 202") and t1.anchors == ["FR-3"] and t2.anchors == ["FR-3", "NFR-1"]
    assert t1.props["codeTargets"] == ["modify main.go::handleNotify"] and t1.props["verifyCommand"] == "go test ./..."
    assert t2.props["dependsOn"] == ["T1.1"] and t1.props["phase"] == "PH-1 Notify" and t1.props["acceptanceIds"] == ["AC-3"]
    assert sc.text == "GIVEN the service is running WHEN POST /notices is called THEN the answer is HTTP 202 AND nothing is read from the store"
    assert sc.anchors == ["AC-3", "FR-3"] and sc.props["file"] == "specs/notify/spec.md"
    assert any("proposal.md not found" in n for n in b.notes) and any("design.md not found" in n for n in b.notes)


def test_plan_reads_proposal_and_design(tmp_path):
    b = ctx.parse_plan(_plan(tmp_path) / "tasks.md", "change", "h")
    by = {(i.kind, i.key): i for i in b.items}
    prop = by[("decision", "PROPOSAL")]
    assert "handleNotify" in prop.text and prop.props["why"].startswith("The service cannot notify")
    assert prop.props["capabilities"] == ["`notify`: owner notices independent of the store."] and prop.anchors == ["FR-3", "NFR-1", "ADR-02"]
    d1, d2 = by[("decision", "D1")], by[("decision", "D2")]
    assert "no `*store` receiver" in d1.text and d2.text.endswith("Other methods get 405.")          # a continuation line joins its item
    assert d1.props["context"].startswith("Handlers live in `main.go`") and d1.props["openQuestions"] == ["Should HEAD be allowed?"]
    assert by[("assumption", "R1")].text.startswith("Compact JSON") and by[("risk", "R2")].props == {"impact": "Medium"}
    assert by[("constraint", "N1")].props["constraintType"] == "non-goal"
    assert not any(i.kind == "open_question" for i in b.items)                                        # design questions never become gaps
    assert b.notes == ["design.md: section 'Something Else' is not read"]


def test_plan_task_without_done_when_is_reported_not_invented(tmp_path):
    d = tmp_path / "c"
    d.mkdir()
    (d / "tasks.md").write_text(TASKS.replace("  Done when: tests pass with the store locked.\n", ""), encoding="utf-8")
    (d / "proposal.md").write_text(PROPOSAL, encoding="utf-8")
    b = ctx.parse_plan(d / "tasks.md", "c", "h")
    assert [i.key for i in b.items if i.kind == "acceptance"] == ["T1.1"] and any("T1.2 has no Done when" in n for n in b.notes)


def test_plan_without_anything_to_read_is_invalid(tmp_path):
    (tmp_path / "tasks.md").write_text("# nothing\n", encoding="utf-8")
    with pytest.raises(TddAgentError) as e:
        ctx.parse_plan(tmp_path / "tasks.md", "t", "h")
    assert e.value.code == "TESTSPEC_CONTEXT_INVALID"


# ---------------------------------------------------------------- loading and hashing (TC-57, 64)
def test_mode_none_reads_nothing(tmp_path):
    cfg = _cfg(tmp_path)
    assert ctx.load(cfg) is None and ctx.fingerprint(cfg) == {"mode": "none"}


def test_load_solution_and_plan_by_config_and_hash_follows_every_file(tmp_path):
    (tmp_path / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    cfg = _cfg(tmp_path, context_mode="solution", context_path="sol.json")
    b = ctx.load(cfg)
    assert b.mode == "solution" and b.path == "sol.json" and len(b.sha256) == 64 and ctx.fingerprint(cfg)["sha256"] == b.sha256
    d = _plan(tmp_path)
    cfg = _cfg(tmp_path, context_mode="plan", context_path="change")
    p = ctx.load(cfg)
    assert p.mode == "plan" and len([i for i in p.items if i.kind == "acceptance"]) == 3 and p.path == "change/tasks.md"
    for name in ("tasks.md", "proposal.md", "design.md", "specs/notify/spec.md"):
        before = ctx.fingerprint(cfg)["sha256"]
        f = d / name
        f.write_text(f.read_text(encoding="utf-8") + "\nmore\n", encoding="utf-8")
        assert ctx.fingerprint(cfg)["sha256"] != before, name        # any file of the change runs the test specs again


def test_a_run_may_override_the_path_but_not_the_mode(tmp_path):
    (tmp_path / "a.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    cfg = _cfg(tmp_path, context_mode="solution", context_path="missing.json")
    assert ctx.load(cfg, {"context_path": "a.json"}).path == "a.json"
    assert ctx.load(cfg, {"context_ref": "R9", "context_path": "a.json"}).ref == "R9"
    assert ctx.context_params(cfg, {"context_mode": "plan"})[0] == "solution"


@pytest.mark.parametrize("mode,path,code", [("solution", "", "TESTSPEC_CONTEXT_MISSING"), ("solution", "nope.json", "TESTSPEC_CONTEXT_MISSING"),
                                            ("plan", "nope", "TESTSPEC_CONTEXT_MISSING")])
def test_missing_document_stops_with_a_code(tmp_path, mode, path, code):
    with pytest.raises(TddAgentError) as e:
        ctx.load(_cfg(tmp_path, context_mode=mode, context_path=path))
    assert e.value.code == code
    assert ctx.fingerprint(_cfg(tmp_path, context_mode=mode, context_path=path)).get("error") == code


def test_a_path_that_leaves_the_repository_through_a_link_is_refused(tmp_path):
    outside = tmp_path / "outside"
    outside.mkdir()
    (outside / "sol.json").write_text(json.dumps(SOLUTION), encoding="utf-8")
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / "link").symlink_to(outside)
    with pytest.raises(TddAgentError) as e:
        ctx.load(_cfg(repo, context_mode="solution", context_path="link/sol.json"))
    assert e.value.code == "TESTSPEC_CONTEXT_PATH"


def test_not_json_is_invalid(tmp_path):
    (tmp_path / "s.json").write_text("{nope", encoding="utf-8")
    with pytest.raises(TddAgentError) as e:
        ctx.load(_cfg(tmp_path, context_mode="solution", context_path="s.json"))
    assert e.value.code == "TESTSPEC_CONTEXT_INVALID"


# ---------------------------------------------------------------- documents sent over MCP (TC-73)
def _wcfg(tmp_path):
    repo = tmp_path / "repo"
    repo.mkdir(exist_ok=True)
    return load_config(env={}, overrides={"paths": {"repo_root": str(repo), "work_dir": str(repo / ".tdd-agent")},
                                          "testspec": {"llm": "none", "context_mode": "plan"}})


PLAN_FILES = {"tasks.md": TASKS, "proposal.md": PROPOSAL, "design.md": DESIGN, "specs/notify/spec.md": SPEC}


def test_write_inline_plan_and_solution(tmp_path):
    cfg = _wcfg(tmp_path)
    got = ctx.write_inline(cfg, "plan", "REQ-2", {"files": PLAN_FILES})
    assert got == {"path": ".tdd-agent/context/plan-REQ-2", "files": sorted(PLAN_FILES)}
    root = tmp_path / "repo" / ".tdd-agent" / "context" / "plan-REQ-2"
    assert (root / "specs" / "notify" / "spec.md").read_text(encoding="utf-8") == SPEC
    got = ctx.write_inline(cfg, "solution", "REQ-2", {"document": SOLUTION})
    assert got == {"path": ".tdd-agent/context/solution-REQ-2", "files": ["solution.json"]}
    assert json.loads((tmp_path / "repo" / got["path"] / "solution.json").read_text(encoding="utf-8"))["id"] == "sol-1"


def test_write_inline_replaces_the_folder_of_the_same_ref_and_leaves_no_temp(tmp_path):
    cfg = _wcfg(tmp_path)
    ctx.write_inline(cfg, "plan", "R1", {"files": PLAN_FILES})
    ctx.write_inline(cfg, "plan", "R1", {"files": {"tasks.md": TASKS}})
    base = tmp_path / "repo" / ".tdd-agent" / "context"
    assert sorted(p.name for p in base.iterdir()) == ["plan-R1"]
    assert sorted(str(p.relative_to(base / "plan-R1")) for p in (base / "plan-R1").rglob("*") if p.is_file()) == ["tasks.md"]


@pytest.mark.parametrize("files,code", [
    ({"tasks.md": TASKS, "../x.md": "x"}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": TASKS, "/etc/passwd": "x"}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": TASKS, "specs/a/b/spec.md": "x"}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": TASKS, "run.sh": "x"}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": TASKS, "solution.json": "{}"}, "TESTSPEC_CONTEXT_FILE"),
    ({"proposal.md": PROPOSAL}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": 5}, "TESTSPEC_CONTEXT_FILE"),
    ({"tasks.md": "x" * (ctx.MAX_FILE_BYTES + 1)}, "TESTSPEC_CONTEXT_TOO_LARGE"),
    ({"tasks.md": TASKS, **{f"specs/s{i}/spec.md": "x" for i in range(ctx.MAX_FILES)}}, "TESTSPEC_CONTEXT_TOO_LARGE"),
    ({"tasks.md": "x" * 200_000, **{f"specs/s{i}/spec.md": "x" * 200_000 for i in range(12)}}, "TESTSPEC_CONTEXT_TOO_LARGE"),
    ({"tasks.md": "# nothing"}, "TESTSPEC_CONTEXT_INVALID"),
])
def test_write_inline_refuses_before_writing_anything(tmp_path, files, code):
    cfg = _wcfg(tmp_path)
    ctx.write_inline(cfg, "plan", "KEEP", {"files": PLAN_FILES})
    base = tmp_path / "repo" / ".tdd-agent" / "context"
    before = sorted(str(p.relative_to(base)) for p in base.rglob("*"))
    with pytest.raises(TddAgentError) as e:
        ctx.write_inline(cfg, "plan", "KEEP", {"files": files})
    assert e.value.code == code
    assert sorted(str(p.relative_to(base)) for p in base.rglob("*")) == before                 # the old document is untouched, no temp left
    assert (base / "plan-KEEP" / "design.md").read_text(encoding="utf-8") == DESIGN


@pytest.mark.parametrize("kind,payload", [("plan", {"document": {}}), ("plan", {"files": PLAN_FILES, "document": {}}), ("solution", {"files": PLAN_FILES}),
                                          ("solution", {"document": []}), ("other", {"document": {}}), ("plan", {})])
def test_write_inline_needs_the_right_payload(tmp_path, kind, payload):
    with pytest.raises(TddAgentError) as e:
        ctx.write_inline(_wcfg(tmp_path), kind, "R", payload)
    assert e.value.code == "TESTSPEC_CONTEXT_FILE"


@pytest.mark.parametrize("ref", ["", "a b", "../x", "x" * 65, ".hidden", None, 5])
def test_ref_is_checked(tmp_path, ref):
    with pytest.raises(TddAgentError) as e:
        ctx.write_inline(_wcfg(tmp_path), "plan", ref, {"files": PLAN_FILES})
    assert e.value.code == "TESTSPEC_CONTEXT_FILE"


def test_an_invalid_solution_sent_inline_does_not_replace_the_good_one(tmp_path):
    cfg = _wcfg(tmp_path)
    ctx.write_inline(cfg, "solution", "R", {"document": SOLUTION})
    with pytest.raises(TddAgentError) as e:
        ctx.write_inline(cfg, "solution", "R", {"document": {"options": []}})
    assert e.value.code == "TESTSPEC_CONTEXT_INVALID"
    assert json.loads((tmp_path / "repo/.tdd-agent/context/solution-R/solution.json").read_text(encoding="utf-8"))["id"] == "sol-1"


# ---------------------------------------------------------------- projection (TC-58..63)
def _build(bundle):
    (s,) = cr_scopes(design_graph(), CR_PARAMS, load_mapping())
    return build_package(design_graph(), s, cr="CR-1", generated_at="2026-10-13T00:00:00Z", source_hash="sha256:abc", context=bundle)


def _ingest(pkg, tmp_path):
    from tsagent.config import Config
    from tsagent.ingest import ingest
    p = tmp_path / "pkg.tdd-kg.json"
    p.write_text(dumps(pkg), encoding="utf-8")
    c = Config()
    c.source_roots = (str(tmp_path),)
    return ingest([str(p)], pkg["feature"], c)


def test_solution_nodes_sit_next_to_the_requirement(tmp_path):
    pkg, rep = _build(ctx.parse_solution(SOLUTION, "s.json", "h"))
    nodes = {n["code"]: n for n in pkg["nodes"]}
    edges = {tuple(e) for e in pkg["edges"]}
    assert not [d for d in _ingest(pkg, tmp_path)[1] if d.severity == "error"]
    assert nodes["ADR-REQ-2-opt-1"]["class"] == "Decision" and nodes["ADR-REQ-2-opt-1"]["testScope"] == "context"
    assert nodes["ADR-REQ-2-opt-1"]["props"]["rationale"] == "Smallest change."
    assert ("FR-3", "satisfiedBy", "ADR-REQ-2-opt-1") in edges
    assert nodes["CON-REQ-2-A-1"]["class"] == "Constraint" and ("ADR-REQ-2-opt-1", "constrainedBy", "CON-REQ-2-A-1") in edges
    assert nodes["ASM-REQ-2-A-2"]["props"]["confirmStatus"] == "unconfirmed" and ("FR-3", "assumes", "ASM-REQ-2-A-2") in edges
    assert ("FR-3", "blockedBy", "OQ-REQ-2-Q-2") in edges and ("FR-3", "hasRisk", "RISK-REQ-2-R1") in edges
    ac = nodes["AC-REQ-2-AC-3"]
    assert ac["class"] == "AcceptanceCriterion" and ac["props"]["derivation"] == "solution" and "testScope" not in ac
    assert ("FR-3", "hasAcceptance", "AC-REQ-2-AC-3") in edges
    c = rep["context_doc"]
    assert c["mode"] == "solution" and c["anchors"] == ["FR-3"] and c["unanchored"] == [] and c["nodes"] == 6
    assert c["by_kind"] == {"decision": 1, "risk": 1, "constraint": 1, "assumption": 1, "open_question": 1, "acceptance": 1}


def _gaps(pkg, tmp_path):
    """Codes of the TestGap entities the seed rules make for a package."""
    from tsagent.analyze.seed_core import seed
    return [x.code for x in seed(_ingest(pkg, tmp_path)[0], "b-1").g.of("TestGap")]


def test_unconfirmed_assumption_and_blocking_question_become_test_gaps(tmp_path):
    pkg, _ = _build(ctx.parse_solution(SOLUTION, "s.json", "h"))
    codes = " ".join(_gaps(pkg, tmp_path))
    assert "ASM-REQ-2-A-2" in codes and "OQ-REQ-2-Q-2" in codes
    assert "ASM-REQ-2-A-1" not in codes and "CON-REQ-2-A-1" not in codes     # a confirmed assumption is a fact, not a gap
    assert "Which Go version" not in codes                                    # a question that does not block is only a note


def test_plan_nodes_and_gaps(tmp_path):
    pkg, rep = _build(ctx.parse_plan(_plan(tmp_path) / "tasks.md", "change", "h"))
    nodes = {n["code"]: n for n in pkg["nodes"]}
    edges = {tuple(e) for e in pkg["edges"]}
    assert not [d for d in _ingest(pkg, tmp_path)[1] if d.severity == "error"]
    assert {"AC-REQ-2-T1-1", "AC-REQ-2-T1-2", "AC-REQ-2-SC1", "ADR-REQ-2-PROPOSAL", "ADR-REQ-2-D1", "ADR-REQ-2-D2", "ASM-REQ-2-R1", "RISK-REQ-2-R1",
            "CON-REQ-2-N1"} <= set(nodes)
    assert nodes["AC-REQ-2-T1-1"]["props"]["derivation"] == "plan" and nodes["AC-REQ-2-T1-1"]["props"]["verifyCommand"] == "go test ./..."
    assert ("ADR-REQ-2-PROPOSAL", "constrainedBy", "CON-REQ-2-N1") in edges                 # non-goals hang off the first decision
    assert ("FR-3", "satisfiedBy", "ADR-REQ-2-PROPOSAL") in edges and ("FR-3", "assumes", "ASM-REQ-2-R1") in edges
    assert rep["context_doc"]["by_kind"]["acceptance"] == 3 and rep["context_doc"]["notes"] == ["design.md: section 'Something Else' is not read"]
    assert any("ASM-REQ-2-R1" in g for g in _gaps(pkg, tmp_path))                          # "assumed" in a design trade-off is an unconfirmed assumption


def test_a_plan_acceptance_closes_the_missing_criterion_gap(tmp_path):
    from tdd_agent.agents.testspec.scope import tdd_scope
    d = tmp_path / "c"
    d.mkdir()
    (d / "tasks.md").write_text(TASKS.replace("FR-3, NFR-1", "FR-9").replace("FR-3", "FR-9"), encoding="utf-8")
    scope = tdd_scope(design_graph(), "TDD-B", load_mapping())               # FR-9 alone, with no acceptance criterion in the KG
    kw = {"generated_at": "x", "source_hash": "sha256:a"}
    base, _ = build_package(design_graph(), scope, **kw)
    pkg, rep = build_package(design_graph(), scope, context=ctx.parse_plan(d / "tasks.md", "c", "h"), **kw)
    assert any("FR-9" in g and "no-ac" in g for g in _gaps(base, tmp_path))
    assert not any("FR-9" in g and "no-ac" in g for g in _gaps(pkg, tmp_path))             # the plan names FR-9: it has criteria now
    assert rep["context_doc"]["anchors"] == ["FR-9"]


def test_an_item_with_no_anchor_is_listed_never_guessed():
    b = ctx.parse_solution(SOLUTION, "s.json", "h")
    b.hints = []
    for it in b.items:
        it.anchors = []
    (s,) = cr_scopes(design_graph(), CR_PARAMS, load_mapping())
    assert len([x for x in s.targets if x in ("FR-3", "NFR-1")]) == 2                     # two requirements in scope and none named
    _, rep = build_package(design_graph(), s, cr="CR-1", generated_at="x", source_hash="sha256:a", context=b)
    c = rep["context_doc"]
    assert c["nodes"] == 0 and c["anchors"] == [] and len(c["unanchored"]) + 1 == len(b.items)
    assert any("constraint A-1 has no Decision" in n for n in c["notes"])                  # a constraint hangs off a decision, and there is none


def test_package_without_context_is_unchanged_and_with_context_is_deterministic():
    a, ra = _build(None)
    assert "context_doc" not in ra and "REQ-2" not in dumps(a)
    assert dumps(_build(None)[0]) == dumps(a)
    b1 = dumps(_build(ctx.parse_solution(SOLUTION, "s.json", "h"))[0])
    assert b1 == dumps(_build(ctx.parse_solution(SOLUTION, "s.json", "h"))[0]) and b1 != dumps(a)

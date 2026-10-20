"""T3.1-T3.4: mapping table, code normalisation, scope, tdd-kg/v1 package (TC-03 I1-I8 part, TC-08, TC-12, TC-13, TC-14)."""
import copy
from pathlib import Path

import pytest
import yaml
from testspec_graph import CR_PARAMS, design_graph

from tdd_agent.agents.testspec.codes import CODE_RE, norm_code, norm_feature
from tdd_agent.agents.testspec.mapping import fingerprint, load_mapping
from tdd_agent.agents.testspec.project import apply_fn, build_package, dumps
from tdd_agent.agents.testspec.scope import cr_scopes, tdd_scope
from tdd_agent.errors import TddAgentError

V04 = Path(__file__).resolve().parents[1] / "data" / "ontology_v0.4" / "o4" / "classes.yaml"


# ---------------------------------------------------------------- T3.1
def test_every_v04_class_is_mapped_or_listed_unmapped():
    m = load_mapping()
    with open(V04, encoding="utf-8") as fh:
        classes = set(yaml.safe_load(fh)["classes"])
    handled = set(m["classes"]) | set(m["unmapped"])
    assert classes - handled == set(), "lớp v0.4 chưa có luật ánh xạ hay chưa ghi là không chiếu"
    assert not set(m["classes"]) & set(m["unmapped"])


def test_mapping_fingerprint_is_stable():
    fp = fingerprint()
    assert fp["version"] == 1 and len(fp["sha256"]) == 64 and fingerprint() == fp


# ---------------------------------------------------------------- T3.2
@pytest.mark.parametrize("raw", ["Attribute:route.id", "RP-01@T01", "Transition:a.Active->Expired", "Mã lỗi đặc biệt", "#x/y z", "đ"])
def test_norm_code_valid_stable_unique(raw):
    c = norm_code(raw)
    assert CODE_RE.match(c) and norm_code(raw) == c and c != raw and c.rsplit("-h", 1)[1] and len(c.rsplit("-h", 1)[1]) == 6


def test_norm_code_keeps_valid_and_distinguishes():
    assert norm_code("FR-3") == "FR-3" and norm_code("API_P.39") == "API_P.39"
    assert norm_code("a:b") != norm_code("a/b")
    assert len(norm_feature("x" * 80)) <= 64 and norm_feature("TDD-A") == "TDD-A"


@pytest.mark.parametrize("fn,v,out", [("ms", "5 giây", 5000), ("ms", "250 ms", 250), ("ms", "2 phút", 120000), ("ms", 30, 30),
                                       ("num", "≤ 300 ms", 300), ("unit", "≤ 300 ms", "ms"), ("int", "409", 409), ("not", False, True),
                                       ("bool", "có", True), ("list", "a -> b -> c", ["a", "b", "c"]), ("ms", "khoảng", None)])
def test_value_functions(fn, v, out):
    assert apply_fn(fn, v) == out


# ---------------------------------------------------------------- T3.3
def test_cr_scope_targets_context_and_refs():
    (s,) = cr_scopes(design_graph(), CR_PARAMS, load_mapping())
    assert s.tdd == "TDD-A"
    assert s.targets == ["API-1", "FR-3", "NFR-1", "SVC-N-01"]
    assert {"IF-1", "MSG-1", "E-1", "OI-1", "SM-1"} <= set(s.context) and "SEC-1" not in s.context
    assert s.unresolved_refs == ["TDD-A#GONE-1"] and s.unmapped.get("Section")
    ac = [{"id": "AC-3", "text": CR_PARAMS["acceptance"][0]["text"]}]
    assert s.acceptance == {"FR-3": ac, "NFR-1": ac}   # the CR criterion attaches to every requirement of the change-spec item


def test_tdd_scope_and_empty_scope():
    s = tdd_scope(design_graph(), "TDD-B", load_mapping())
    assert s.targets == ["FR-9"] and s.acceptance == {}
    with pytest.raises(TddAgentError) as e:
        tdd_scope(design_graph(), "TDD-Z", load_mapping())
    assert e.value.code == "TESTSPEC_EMPTY_SCOPE"
    with pytest.raises(TddAgentError) as e:
        cr_scopes(design_graph(), {"cs_elements": ["nope"]}, load_mapping())
    assert e.value.code == "TESTSPEC_EMPTY_SCOPE"


# ---------------------------------------------------------------- T3.4
def _pkg():
    (s,) = cr_scopes(design_graph(), CR_PARAMS, load_mapping())
    return build_package(design_graph(), s, cr="CR-1", generated_at="2026-10-13T00:00:00Z", source_hash="sha256:abc")


def _ingest(pkg, tmp_path):
    from tsagent.config import Config
    from tsagent.ingest import ingest
    p = tmp_path / "pkg.tdd-kg.json"
    p.write_text(dumps(pkg), encoding="utf-8")
    cfg = Config()
    cfg.source_roots = (str(tmp_path),)
    return ingest([str(p)], pkg["feature"], cfg)


def test_package_passes_tsagent_input_checks(tmp_path):
    pkg, rep = _pkg()
    _src, diags = _ingest(pkg, tmp_path)
    errors = [d for d in diags if d.severity == "error"]
    assert not errors, errors
    cls = {n["code"]: n["class"] for n in pkg["nodes"]}
    assert cls["FR-3"] == "FunctionalRequirement" and cls["API-1"] == "Endpoint" and cls["IF-1"] == "Integration"
    assert cls["NFR-1"] == "NonFunctionalRequirement" and cls["E-1"] == "Response" and cls["AC-3"] == "AcceptanceCriterion"
    assert cls[norm_code("Attribute:notice.owner")] == "Field" and cls[norm_code("RP-1@T01") + "-TO"] == "TimeoutPolicy"
    edges = {tuple(e) for e in pkg["edges"]}
    assert ("FR-3", "satisfiedBy", "SVC-N-01") in edges and ("FR-3", "hasAcceptance", "AC-3") in edges
    assert ("IF-1", "hasEndpoint", "API-1") in edges and ("API-1", "hasField", norm_code("Attribute:notice.owner")) in edges
    assert ("IF-1", "hasTimeoutPolicy", norm_code("RP-1@T01") + "-TO") in edges and ("FR-3", "blockedBy", "OI-1") in edges
    nodes = {n["code"]: n for n in pkg["nodes"]}
    assert nodes["API-1"]["props"]["path"] == "/notices" and nodes["E-1"]["props"]["statusCode"] == 409
    assert nodes[norm_code("RP-1@T01") + "-TO"]["props"]["totalMs"] == 5000
    assert nodes["AC-3"]["props"]["statement"].startswith("GIVEN") and nodes["IF-1"]["testScope"] == "context"
    assert "testScope" not in nodes["FR-3"] and nodes["FR-3"]["props"]["tddAgent"] == {"id": "FR-3", "class": "Requirement"}
    assert rep["unmapped"].get("Section") and rep["unmapped_relations"].get("logs_with") == 1
    assert rep["code_map"][norm_code("Attribute:notice.owner")] == "Attribute:notice.owner"
    assert pkg["package"] == {"kind": "slice", "cr": "CR-1", "generatedAt": "2026-10-13T00:00:00Z", "sourceHash": "sha256:abc"}


def test_missing_values_become_open_questions_not_values():
    pkg, rep = _pkg()
    nodes = {n["code"]: n for n in pkg["nodes"]}
    edges = {tuple(e) for e in pkg["edges"]}
    # NFR-1 has no target (THIẾU in the KG): no targetValue, an open question blocks the element
    assert "targetValue" not in nodes["NFR-1"]["props"]
    assert ("NFR-1", "blockedBy", "NFR-1-Q-target") in edges and nodes["NFR-1-Q-target"]["class"] == "OpenQuestion"
    assert "THIẾU target" in nodes["NFR-1-Q-target"]["props"]["statement"]
    # Endpoint, Integration... missing tsagent-required attributes are only asked for in-scope elements
    assert not any(c.startswith("IF-1-Q-") for c in nodes)
    assert rep["missing"] >= 1


def test_package_is_deterministic():
    a, _ = _pkg()
    b, _ = _pkg()
    assert dumps(a) == dumps(b)
    g = design_graph()
    g["elements"] = list(reversed(g["elements"])); g["edges"] = list(reversed(g["edges"]))
    (s,) = cr_scopes(g, CR_PARAMS, load_mapping())
    c, _ = build_package(g, s, cr="CR-1", generated_at="2026-10-13T00:00:00Z", source_hash="sha256:abc")
    assert dumps(c) == dumps(a)


def test_acceptance_without_text_is_gap():
    params = copy.deepcopy(CR_PARAMS); params["acceptance"] = []
    (s,) = cr_scopes(design_graph(), params, load_mapping())
    pkg, _ = build_package(design_graph(), s)
    nodes = {n["code"]: n for n in pkg["nodes"]}
    assert "statement" not in nodes["AC-3"]["props"] and nodes["AC-3-Q-statement"]["class"] == "OpenQuestion"

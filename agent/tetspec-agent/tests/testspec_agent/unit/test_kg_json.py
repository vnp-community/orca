"""T3.1: tdd-kg/v1 loader and rules I1-I8 (TST-01, 02, 03)."""
import copy
import json

import pytest

from tsagent.ingest.kg_json import load_kg_json

pytestmark = pytest.mark.unit


def rules(diags, severity="error"):
    return sorted({d.rule for d in diags if d.severity == severity})


def test_minipay_slice_and_full_load(minipay_pkg, minipay_full):
    for obj in (minipay_pkg, minipay_full):
        pkg, diags = load_kg_json(obj)
        assert pkg is not None, [str(d) for d in diags if d.severity == "error"]
        assert len(pkg.nodes) == len(obj["nodes"]) and len(pkg.edges) == len(obj["edges"])
        assert "Integration" in pkg.ontology.classes and pkg.meta["primaryDoc"]
    pkg, _ = load_kg_json(minipay_pkg)
    assert not pkg.get("MOD-Account").in_scope  # testScope=context nodes are kept as context


def test_file_path_and_broken_json(tmp_path, minipay_pkg):
    p = tmp_path / "x.json"
    p.write_text(json.dumps(minipay_pkg), encoding="utf-8")
    pkg, _ = load_kg_json(str(p))
    assert pkg is not None and pkg.meta["docs"] == ["x.json"]
    p.write_text("{oops", encoding="utf-8")
    pkg, diags = load_kg_json(str(p))
    assert pkg is None and diags[0].rule == "I0" and diags[0].loc == "1:2"


def mutate(obj, fn):
    o = copy.deepcopy(obj)
    fn(o)
    return o


@pytest.mark.parametrize("change,rule,loc_prefix", [
    (lambda o: o["edges"].append(["FR-046", "allocatedTo", "MOD-NOPE", {}]), "I1", "/edges/"),
    (lambda o: o["edges"].append(["FR-046", "hasErrorMapping", "MOD-Scheduler", {}]), "I2", "/edges/"),
    (lambda o: o["nodes"][[n["code"] for n in o["nodes"]].index("FR-046")]["props"].pop("text"), "I3", "/nodes/"),
    (lambda o: o["nodes"].append(dict(o["nodes"][0])), "I4", "/nodes/"),
    (lambda o: o.setdefault("supersede", []).append({"old": "FR-046", "new": "MOD-Scheduler"}), "I5", "/supersede/"),
    (lambda o: [n.__setitem__("primary", True) for n in o["nodes"] if n["class"] == "Document"], "I6", "/nodes"),
])
def test_each_input_rule_has_position(minipay_pkg, change, rule, loc_prefix):
    pkg, diags = load_kg_json(mutate(minipay_pkg, change))
    errs = [d for d in diags if d.severity == "error"]
    assert pkg is None and rule in rules(diags), [str(d) for d in diags]
    assert all(d.loc.startswith("/") for d in errs) and any(d.loc.startswith(loc_prefix) for d in errs if d.rule == rule)


def test_i7_string_steps_is_warning(minipay_pkg):
    def f(o):
        n = next(n for n in o["nodes"] if n["class"] == "Scenario")
        n["props"]["steps"] = "a -> b -> c"
    pkg, diags = load_kg_json(mutate(minipay_pkg, f))
    assert pkg is not None and "I7" in rules(diags, "warning")


@pytest.mark.parametrize("change", [lambda o: o.__setitem__("format", "tdd-kg/v9"),
                                    lambda o: o.__setitem__("ontologyVersion", "tdd-core-7")])
def test_i8_unsupported_version(minipay_pkg, change):
    pkg, diags = load_kg_json(mutate(minipay_pkg, change))
    assert pkg is None and rules(diags) == ["I8"]


def test_schema_errors_have_pointer(minipay_pkg):
    pkg, diags = load_kg_json(mutate(minipay_pkg, lambda o: o["nodes"][3].__setitem__("code", "bad code!")))
    assert pkg is None and diags[0].rule == "SCHEMA" and diags[0].loc == "/nodes/3/code"


def test_unknown_class_and_relation_are_warnings(minipay_pkg):
    def f(o):
        o["nodes"].append({"code": "X-1", "class": "Gizmo", "props": {}})
        o["edges"].append(["X-1", "wobbles", "FR-046", {}])
    pkg, diags = load_kg_json(mutate(minipay_pkg, f))
    assert pkg is not None and rules(diags, "warning") == ["I2"]

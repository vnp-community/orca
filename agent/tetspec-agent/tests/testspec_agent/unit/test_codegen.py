"""T1.4: generated files match tso.yaml; the DDL runs; the LLM schemas are valid."""
import json
import sqlite3

import jsonschema
import pytest

from tsagent.ontology import codegen, load_ontology, ontology

pytestmark = pytest.mark.unit


def test_generated_files_match_yaml():
    stale = codegen.stale()
    assert not stale, f"generated files differ from tso.yaml, run `make codegen`: {stale}"


def test_generated_files_carry_do_not_edit_header():
    for path, text in codegen.generate().items():
        assert "GENERATED" in text and ontology().yaml_hash in text, path


def test_ddl_and_views_run_on_empty_sqlite():
    c = sqlite3.connect(":memory:")
    c.execute("PRAGMA foreign_keys=ON")
    c.executescript(codegen.gen_schema_sql(ontology()))
    c.executescript(codegen.gen_views_sql(ontology()))
    assert c.execute("PRAGMA foreign_key_check").fetchall() == []
    tables = {r[0] for r in c.execute("select name from sqlite_master where type='table'")}
    for cls in ontology().classes.values():
        assert cls.table in tables
    assert {"tso_node", "tso_spec_version", "tso_lifecycle", "src_node", "run_step", "stage_item"} <= tables
    for (v,) in c.execute("select name from sqlite_master where type='view'").fetchall():
        c.execute(f"select * from {v}").fetchall()


def test_enum_and_required_constraints_are_enforced():
    c = sqlite3.connect(":memory:")
    c.execute("PRAGMA foreign_keys=ON")
    c.executescript(codegen.gen_schema_sql(ontology()))
    c.execute("insert into tso_node(feature, cls, code) values ('F','RiskRating','RR-1')")
    base = "insert into tso_risk_rating(node_id,node_version,content_hash,run_id,derivation,confidence,created_at,likelihood,impact,priority,rationale)"
    c.execute(base + " values (1,1,'h','r','rule',0.5,'t','High','High','P0','x')")
    with pytest.raises(sqlite3.IntegrityError):
        c.execute(base + " values (1,2,'h','r','rule',0.5,'t','Huge','High','P0','x')")
    with pytest.raises(sqlite3.IntegrityError):
        c.execute(base + " values (1,3,'h','r','guess',0.5,'t','High','High','P0','x')")


def test_llm_schemas_are_valid_and_use_yaml_enums():
    schemas = codegen.gen_llm_schemas(ontology())
    for s in schemas.values():
        jsonschema.Draft202012Validator.check_schema(s)
    kinds = schemas["analysis"]["properties"]["proposedConditions"]["items"]["properties"]["conditionKind"]["enum"]
    assert kinds == ontology().enums["conditionKind"]
    ok = {"schemaVersion": 2, "subject": "INT-01", "conditions": [{"seedCode": "TCND-1", "statement": "s", "expectedEffect": "e",
                                                                   "basedOn": ["INT-01"], "evidence": [], "confidence": 0.8}]}
    jsonschema.validate(ok, schemas["analysis"])
    with pytest.raises(jsonschema.ValidationError):
        jsonschema.validate({**ok, "extra": 1}, schemas["analysis"])


def test_changing_yaml_changes_outputs():
    text = open(load_ontology.__globals__["YAML_PATH"], encoding="utf-8").read()
    o2 = load_ontology(path=None, text=text.replace("llmTextMaxLength: 600", "llmTextMaxLength: 500"))
    s = codegen.gen_llm_schemas(o2)["analysis"]
    assert s["properties"]["conditions"]["items"]["properties"]["statement"]["maxLength"] == 500
    assert codegen.gen_schema_sql(o2) != codegen.gen_schema_sql(ontology())  # the yaml hash in the header changes


def test_shape_rules_list_plus_relations():
    rules = json.loads(codegen.generate()[codegen.OUT_SHAPES])["rules"]
    assert {"class": "TestCase", "relation": "covers", "to": "CoverageItem", "min": 1} in rules
    assert any(r.get("field") == "values" and r["class"] == "Partition" for r in rules)

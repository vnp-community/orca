"""Code generation from tso.yaml (MECH-13): DDL + triggers, read-only views, JSON Schemas of LLM outputs, shape rules.

Generated files carry a "do not edit" header with the tso.yaml hash; a golden test catches a forgotten `make codegen`.
"""
import json
import os

from . import Ontology, fk_column, join_table, load_ontology, snake

PKG = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT_SCHEMA_SQL = os.path.join(PKG, "store", "schema.sql")
OUT_VIEWS_SQL = os.path.join(PKG, "gateway", "views.sql")
OUT_JSON_DIR = os.path.join(PKG, "schemas", "v2")
OUT_SHAPES = os.path.join(PKG, "validate", "shape_rules.json")
SQL_TYPE = {"text": "TEXT", "int": "INTEGER", "real": "REAL", "bool": "INTEGER", "json": "TEXT"}
FROZEN = "('Baselined','Superseded','Retired')"


def _header_sql(o):
    return f"-- GENERATED from tsagent/ontology/tso.yaml (version {o.version}, sha256 {o.yaml_hash}).\n" \
           "-- Do not edit by hand: edit tso.yaml, then run `make codegen`.\n"


# ---------------------------------------------------------------------------------------------- DDL
def _field_sql(f, enums):
    col = f'"{f.column}"'  # always quoted: field names such as values, trigger, key are SQL keywords
    parts = [f"{col} {SQL_TYPE['text'] if f.enum else SQL_TYPE[f.type]}"]
    if f.required:
        parts.append("NOT NULL")
    checks = []
    if f.enum:
        vals = ", ".join("'" + v + "'" for v in enums[f.enum])
        checks.append(f"{col} IN ({vals})")
    if f.type == "bool":
        checks.append(f"{col} IN (0, 1)")
    if f.type == "json":
        checks.append(f"json_valid({col})")
        if f.card == "+":
            checks.append(f"json_array_length({col}) >= 1")
    if checks:
        cond = " AND ".join(checks)
        parts.append(f"CHECK ({cond})" if f.required else f"CHECK ({col} IS NULL OR ({cond}))")
    return " ".join(parts)


def _class_ddl(o, c):
    t = c.table
    cols = [
        "id INTEGER PRIMARY KEY",
        "node_id INTEGER NOT NULL REFERENCES tso_node(id)",
        "node_version INTEGER NOT NULL CHECK (node_version >= 1)",
        "content_hash TEXT NOT NULL",
        f"supersedes_id INTEGER REFERENCES {t}(id)",
        "run_id TEXT NOT NULL",
        "derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human'))",
        "confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1)",
        "model TEXT",
        "prompt_hash TEXT",
        "created_at TEXT NOT NULL",
    ]
    cols += [_field_sql(f, o.enums) for f in c.fields.values()]
    for r in c.relations.values():
        if not r.many:
            cols.append(f"{fk_column(r.name)} INTEGER{' NOT NULL' if r.required else ''} REFERENCES tso_node(id)")
    cols.append("UNIQUE(node_id, node_version)")
    out = [f"CREATE TABLE {t} (\n  " + ",\n  ".join(cols) + "\n);",
           f"CREATE INDEX ix_{t}_node ON {t}(node_id);"]
    for r in c.relations.values():
        if not r.many:
            out.append(f"CREATE INDEX ix_{t}_{snake(r.name)} ON {t}({fk_column(r.name)});")
    # immutability (MECH-03): baselined rows are never updated or deleted - enforced by the DB, even for direct SQL
    out.append(f"CREATE TRIGGER trg_{t}_immutable_upd BEFORE UPDATE ON {t}\n"
               f"WHEN (SELECT status FROM tso_lifecycle WHERE tbl = '{t}' AND row_id = OLD.id) IN {FROZEN}\n"
               f"BEGIN SELECT RAISE(ABORT, 'immutable: {t} is baselined and cannot be updated'); END;")
    out.append(f"CREATE TRIGGER trg_{t}_immutable_del BEFORE DELETE ON {t}\n"
               f"WHEN (SELECT status FROM tso_lifecycle WHERE tbl = '{t}' AND row_id = OLD.id) IN {FROZEN}\n"
               f"BEGIN SELECT RAISE(ABORT, 'immutable: {t} is baselined and cannot be deleted'); END;")
    for r in c.relations.values():
        if r.many:
            j = join_table(c.name, r.name)
            out.append(f"CREATE TABLE {j} (\n  row_id INTEGER NOT NULL REFERENCES {t}(id) ON DELETE CASCADE,\n"
                       f"  node_id INTEGER NOT NULL REFERENCES tso_node(id),\n  PRIMARY KEY(row_id, node_id)\n);")
            out.append(f"CREATE INDEX ix_{j}_node ON {j}(node_id);")
            for ev, ref in (("INSERT", "NEW"), ("DELETE", "OLD")):
                out.append(f"CREATE TRIGGER trg_{j}_immutable_{ev.lower()[:3]} BEFORE {ev} ON {j}\n"
                           f"WHEN (SELECT status FROM tso_lifecycle WHERE tbl = '{t}' AND row_id = {ref}.row_id) IN {FROZEN}\n"
                           f"BEGIN SELECT RAISE(ABORT, 'immutable: {t} is baselined, relation {r.name} cannot change'); END;")
    return out


CORE_DDL = """
CREATE TABLE tso_node (
  id INTEGER PRIMARY KEY,
  feature TEXT NOT NULL,
  cls TEXT NOT NULL,
  code TEXT NOT NULL,
  UNIQUE(feature, code)
);
CREATE TRIGGER trg_tso_node_immutable BEFORE UPDATE ON tso_node
BEGIN SELECT RAISE(ABORT, 'immutable: node identity cannot change'); END;

CREATE TABLE tso_spec_version (
  id INTEGER PRIMARY KEY,
  feature TEXT NOT NULL,
  version INTEGER NOT NULL CHECK (version >= 1),
  status TEXT NOT NULL CHECK (status IN ('Draft', 'Baselined', 'Superseded')),
  base_baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  changed_by TEXT NOT NULL DEFAULT '[]',
  graph_hash TEXT,
  created_at TEXT NOT NULL,
  baselined_at TEXT,
  baselined_by TEXT,
  UNIQUE(feature, version)
);
-- V8: at most one Draft version per feature
CREATE UNIQUE INDEX ux_tso_spec_version_one_draft ON tso_spec_version(feature) WHERE status = 'Draft';
CREATE TRIGGER trg_tso_spec_version_frozen BEFORE UPDATE ON tso_spec_version
WHEN OLD.status = 'Superseded'
  OR (OLD.status = 'Baselined' AND (NEW.status NOT IN ('Baselined', 'Superseded') OR NEW.graph_hash IS NOT OLD.graph_hash
      OR NEW.base_baseline_id IS NOT OLD.base_baseline_id OR NEW.changed_by IS NOT OLD.changed_by))
BEGIN SELECT RAISE(ABORT, 'immutable: version is baselined'); END;
CREATE TRIGGER trg_tso_spec_version_nodel BEFORE DELETE ON tso_spec_version
WHEN OLD.status <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: a baselined version cannot be deleted'); END;

CREATE TABLE tso_spec_version_item (
  spec_version_id INTEGER NOT NULL REFERENCES tso_spec_version(id),
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  tbl TEXT NOT NULL,
  row_id INTEGER NOT NULL,
  PRIMARY KEY(spec_version_id, node_id)
);
CREATE INDEX ix_tso_spec_version_item_row ON tso_spec_version_item(tbl, row_id);
CREATE TRIGGER trg_tso_spec_version_item_ins BEFORE INSERT ON tso_spec_version_item
WHEN (SELECT status FROM tso_spec_version WHERE id = NEW.spec_version_id) <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: version is baselined'); END;
CREATE TRIGGER trg_tso_spec_version_item_del BEFORE DELETE ON tso_spec_version_item
WHEN (SELECT status FROM tso_spec_version WHERE id = OLD.spec_version_id) <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: version is baselined'); END;
CREATE TRIGGER trg_tso_spec_version_item_upd BEFORE UPDATE ON tso_spec_version_item
BEGIN SELECT RAISE(ABORT, 'immutable: version items cannot be updated'); END;

CREATE TABLE tso_regression (
  spec_version_id INTEGER NOT NULL REFERENCES tso_spec_version(id),
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(spec_version_id, node_id)
);
CREATE TRIGGER trg_tso_regression_ins BEFORE INSERT ON tso_regression
WHEN (SELECT status FROM tso_spec_version WHERE id = NEW.spec_version_id) <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: version is baselined'); END;
CREATE TRIGGER trg_tso_regression_del BEFORE DELETE ON tso_regression
WHEN (SELECT status FROM tso_spec_version WHERE id = OLD.spec_version_id) <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: version is baselined'); END;

CREATE TABLE tso_lifecycle (
  tbl TEXT NOT NULL,
  row_id INTEGER NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('Draft', 'InReview', 'Baselined', 'Superseded', 'Retired')),
  review_status TEXT NOT NULL CHECK (review_status IN ('proposed', 'approved', 'rejected', 'needsInfo')),
  reviewer TEXT,
  reviewed_at TEXT,
  note TEXT,
  retired_by TEXT,
  PRIMARY KEY(tbl, row_id)
);
CREATE TRIGGER trg_tso_lifecycle_transition BEFORE UPDATE ON tso_lifecycle
WHEN OLD.status IN ('Superseded', 'Retired')
  OR (OLD.status = 'Baselined' AND (NEW.status NOT IN ('Baselined', 'Superseded', 'Retired')
      OR NEW.review_status IS NOT OLD.review_status OR NEW.reviewer IS NOT OLD.reviewer OR NEW.note IS NOT OLD.note
      OR (NEW.status = 'Retired' AND NEW.retired_by IS NULL)))
  OR (OLD.status = 'Draft' AND NEW.status NOT IN ('Draft', 'InReview', 'Baselined', 'Retired'))
  OR (OLD.status = 'InReview' AND NEW.status NOT IN ('Draft', 'InReview', 'Baselined'))
BEGIN SELECT RAISE(ABORT, 'immutable: state transition not allowed'); END;
CREATE TRIGGER trg_tso_lifecycle_nodel BEFORE DELETE ON tso_lifecycle
WHEN OLD.status <> 'Draft'
BEGIN SELECT RAISE(ABORT, 'immutable: the lifecycle of a baselined node cannot be deleted'); END;

CREATE TABLE tso_based_on (
  tbl TEXT NOT NULL,
  row_id INTEGER NOT NULL,
  src_node_id INTEGER NOT NULL REFERENCES src_node(id),
  PRIMARY KEY(tbl, row_id, src_node_id)
);
CREATE TABLE tso_evidence (
  id INTEGER PRIMARY KEY,
  tbl TEXT NOT NULL,
  row_id INTEGER NOT NULL,
  src_node_id INTEGER NOT NULL REFERENCES src_node(id),
  quote TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ix_tso_evidence_row ON tso_evidence(tbl, row_id);
"""


def _prov_triggers():
    out = []
    for t in ("tso_based_on", "tso_evidence"):
        for ev, ref in (("INSERT", "NEW"), ("DELETE", "OLD"), ("UPDATE", "OLD")):
            out.append(f"CREATE TRIGGER trg_{t}_immutable_{ev.lower()[:3]} BEFORE {ev} ON {t}\n"
                       f"WHEN (SELECT status FROM tso_lifecycle WHERE tbl = {ref}.tbl AND row_id = {ref}.row_id) IN {FROZEN}\n"
                       f"BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;")
    return out


def _table_ddl(name, spec, insert_only=False):
    cols = [f"{k} {v}" for k, v in spec["columns"].items()] + list(spec.get("constraints", []))
    out = [f"CREATE TABLE {name} (\n  " + ",\n  ".join(cols) + "\n);"]
    for i, idx in enumerate(spec.get("indexes", [])):
        out.append(f"CREATE INDEX ix_{name}_{i} ON {name}({idx});")
    if insert_only:
        # R17 on the tsagent side: a loaded source graph is immutable
        out.append(f"CREATE TRIGGER trg_{name}_no_update BEFORE UPDATE ON {name}\n"
                   f"BEGIN SELECT RAISE(ABORT, 'immutable: {name} is append-only'); END;")
        out.append(f"CREATE TRIGGER trg_{name}_no_delete BEFORE DELETE ON {name}\n"
                   f"BEGIN SELECT RAISE(ABORT, 'immutable: {name} is append-only'); END;")
    return out


def gen_schema_sql(o: Ontology):
    parts = [_header_sql(o), "PRAGMA foreign_keys = ON;", "", "-- ===== sources (immutable) ====="]
    for name, spec in o.source.items():
        parts += _table_ddl(name, spec, insert_only=True)
    parts += ["", "-- ===== operations ====="]
    for name, spec in o.ops.items():
        parts += _table_ddl(name, spec)
    parts += ["", "-- ===== test KG core =====", CORE_DDL.strip()]
    parts += _prov_triggers()
    for c in o.classes.values():
        parts += ["", f"-- ----- {c.name} (layer {c.layer}): {c.doc}"]
        parts += _class_ddl(o, c)
    return "\n".join(parts) + "\n"


# ---------------------------------------------------------------------------------------------- view
def gen_views_sql(o: Ontology):
    out = [_header_sql(o),
           "-- Read-only views for MCP sql_select (MECH-14). Each view takes the latest version of each feature.",
           "CREATE VIEW IF NOT EXISTS v_spec_current AS\n"
           "SELECT sv.* FROM tso_spec_version sv\n"
           "WHERE sv.version = (SELECT MAX(version) FROM tso_spec_version x WHERE x.feature = sv.feature);",
           "CREATE VIEW IF NOT EXISTS v_source_node AS\n"
           "SELECT b.feature, n.baseline_id, n.code, n.class, n.base, n.props, n.test_scope\n"
           "FROM src_node n JOIN src_baseline b ON b.id = n.baseline_id\n"
           "WHERE b.ordinal = (SELECT MAX(ordinal) FROM src_baseline x WHERE x.feature = b.feature);"]
    for c in o.classes.values():
        t = c.table
        cols = ", ".join(f't."{f.column}"' for f in c.fields.values())
        fks = "".join(f", (SELECT code FROM tso_node WHERE id = t.{fk_column(r.name)}) AS {snake(r.name)}"
                      for r in c.relations.values() if not r.many)
        many = "".join(f", (SELECT json_group_array(nn.code) FROM {join_table(c.name, r.name)} j JOIN tso_node nn ON nn.id = j.node_id "
                       f"WHERE j.row_id = t.id) AS {snake(r.name)}" for r in c.relations.values() if r.many)
        out.append(
            f"CREATE VIEW IF NOT EXISTS v_{snake(c.name)} AS\n"
            f"SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, "
            f"lc.status, lc.review_status, {cols}{fks}{many}\n"
            f"FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = '{t}'\n"
            f"JOIN {t} t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id\n"
            f"LEFT JOIN tso_lifecycle lc ON lc.tbl = '{t}' AND lc.row_id = t.id;")
    out.append("CREATE VIEW IF NOT EXISTS v_gap AS SELECT * FROM v_test_gap;")
    out.append(
        "CREATE VIEW IF NOT EXISTS v_trace AS\n"
        "SELECT r.feature, r.code AS requirement, r.level, pc.ordinal, c.code AS condition, c.condition_kind,\n"
        "  b.tdd_code AS source, ci.code AS coverage_item, tc.code AS test_case\n"
        "FROM v_test_requirement r\n"
        "JOIN v_pass_criterion pc ON pc.feature = r.feature AND pc.requirement = r.code\n"
        "JOIN v_test_condition c ON c.feature = r.feature AND c.code = pc.condition\n"
        "LEFT JOIN v_test_basis_item b ON b.feature = r.feature AND b.code = c.subject\n"
        "LEFT JOIN v_coverage_item ci ON ci.feature = r.feature AND ci.realizes = c.code\n"
        "LEFT JOIN v_test_case tc ON tc.feature = r.feature AND EXISTS (SELECT 1 FROM json_each(tc.covers) WHERE value = ci.code);")
    return "\n".join(out) + "\n"


# ---------------------------------------------------------------------------------------------- JSON Schema LLM
def _text(o):
    return {"type": "string", "minLength": 1, "maxLength": o.llm_text_max}


def _opt_text(o):
    return {"type": ["string", "null"], "maxLength": o.llm_text_max}


def _evidence(o):
    return {"type": "array", "items": {"type": "object", "required": ["sourceCode"], "additionalProperties": False,
                                        "properties": {"sourceCode": {"type": "string", "minLength": 1},
                                                       "quote": {"type": "string", "maxLength": o.llm_text_max}}}}


def _codes():
    return {"type": "array", "items": {"type": "string", "minLength": 1}}


def _enum(o, name):
    return {"type": "string", "enum": list(o.enums[name])}


def _obj(required, props):
    return {"type": "object", "required": required, "additionalProperties": False, "properties": props}


def gen_llm_schemas(o: Ontology):
    common = {"basedOn": _codes(), "evidence": _evidence(o), "confidence": {"type": "number", "minimum": 0, "maximum": 1}}
    cond_fill = _obj(["seedCode", "statement", "expectedEffect", "basedOn", "evidence", "confidence"], {
        "seedCode": {"type": "string", "minLength": 1},
        "statement": _text(o), "expectedEffect": _text(o), "preState": _opt_text(o), "trigger": _opt_text(o),
        "oracle": _obj([], {"expected": _text(o), "tolerance": _opt_text(o), "measurement": _opt_text(o)}),
        "faultModel": _obj([], {"expectedHandling": _text(o)}),
        "partition": _obj([], {"values": {"type": "array", "minItems": 1}}),
        "path": _obj([], {"steps": {"type": "array", "minItems": 1, "items": _text(o)}}),
        **common})
    proposed = _obj(["conditionKind", "statement", "expectedEffect", "oracle", "basedOn", "evidence", "confidence"], {
        "conditionKind": _enum(o, "conditionKind"), "statement": _text(o), "expectedEffect": _text(o),
        "preState": _opt_text(o), "trigger": _opt_text(o),
        "oracle": _obj(["oracleKind", "expected"], {"oracleKind": _enum(o, "oracleKind"), "expected": _text(o),
                                                     "tolerance": _opt_text(o), "measurement": _opt_text(o),
                                                     "sourceCode": {"type": ["string", "null"]}}),
        **common})
    gap = _obj(["missing", "askedQuestion"], {"missing": _text(o), "askedQuestion": _text(o), "blocks": _codes()})
    analysis = {
        "$schema": "https://json-schema.org/draft/2020-12/schema", "title": "analysis",
        "description": "LLM output of the analysis step (layer A): fill seed nodes, propose conditions, report gaps. Natural-language values are Vietnamese.",
        **_obj(["schemaVersion", "subject", "conditions"], {
            "schemaVersion": {"const": 2}, "subject": {"type": "string", "minLength": 1},
            "obligations": {"type": "array", "items": _obj(["seedCode", "question"], {"seedCode": {"type": "string"}, "question": _text(o)})},
            "conditions": {"type": "array", "items": cond_fill},
            "proposedConditions": {"type": "array", "items": proposed},
            "gaps": {"type": "array", "items": gap}})}
    seed = {"type": "string", "minLength": 1}
    pass_crit = _obj(["conditionCode", "text"], {"conditionCode": seed, "text": _text(o)})
    cov = _obj(["seedCode"], {"seedCode": seed, "description": _opt_text(o), "value": {}})
    req = _obj(["seedCode", "statement", "scopeNote", "passCriteria"], {
        "seedCode": seed, "statement": _text(o), "scopeNote": _text(o), "rationale": _opt_text(o),
        "passCriteria": {"type": "array", "minItems": 1, "items": pass_crit},
        "coverage": {"type": "array", "items": cov}})
    design = {
        "$schema": "https://json-schema.org/draft/2020-12/schema", "title": "design",
        "description": "LLM output of the design step (layer B): TR wording, criteria, coverage item values. Natural-language values are Vietnamese.",
        **_obj(["schemaVersion", "requirements"], {"schemaVersion": {"const": 2},
                                                   "requirements": {"type": "array", "items": req}})}
    step = _obj(["stepKind", "action"], {"stepKind": _enum(o, "stepKind"), "action": _text(o), "input": {},
                                         "targetCode": {"type": ["string", "null"]}})
    tc = _obj(["seedCode", "objective", "steps"], {
        "seedCode": seed, "objective": _text(o),
        "preconditions": {"type": "array", "items": _text(o)},
        "steps": {"type": "array", "minItems": 1, "items": step},
        "assertions": {"type": "array", "items": _obj(["seedCode", "description"], {"seedCode": seed, "description": _text(o)})},
        "data": {"type": "array", "items": _obj(["seedCode", "example"], {"seedCode": seed, "example": _text(o)})},
        "fixtures": {"type": "array", "items": _obj(["seedCode", "setup", "teardown"],
                                                     {"seedCode": seed, "setup": _text(o), "teardown": _text(o)})},
        "faultInjections": {"type": "array", "items": _obj(["seedCode", "mechanism", "trigger"],
                                                            {"seedCode": seed, "mechanism": _text(o), "trigger": _text(o)})}})
    spec = {
        "$schema": "https://json-schema.org/draft/2020-12/schema", "title": "specification",
        "description": "LLM output of the specification step (layer C): steps, data, fixtures, fault injections; figures come from the oracles. Natural-language values are Vietnamese.",
        **_obj(["schemaVersion", "testcases"], {"schemaVersion": {"const": 2},
                                                "testcases": {"type": "array", "items": tc}})}
    for s in (analysis, design, spec):
        s["$comment"] = f"GENERATED from tso.yaml sha256 {o.yaml_hash}; do not edit by hand"
    return {"analysis": analysis, "design": design, "specification": spec}


# ---------------------------------------------------------------------------------------------- shape rules
def gen_shape_rules(o: Ontology):
    rules = []
    for c in o.classes.values():
        for r in c.relations.values():
            if r.card == "+":
                rules.append({"class": c.name, "relation": r.name, "to": r.to, "min": 1})
        for f in c.fields.values():
            if f.card == "+":
                rules.append({"class": c.name, "field": f.name, "min": 1})
    return {"$comment": f"GENERATED from tso.yaml sha256 {o.yaml_hash}; do not edit by hand", "rules": rules}


def generate(o=None):
    """Return {path: content} of every generated file."""
    o = o or load_ontology()
    files = {OUT_SCHEMA_SQL: gen_schema_sql(o), OUT_VIEWS_SQL: gen_views_sql(o),
             OUT_SHAPES: json.dumps(gen_shape_rules(o), ensure_ascii=False, indent=1) + "\n"}
    for name, schema in gen_llm_schemas(o).items():
        files[os.path.join(OUT_JSON_DIR, f"{name}.schema.json")] = json.dumps(schema, ensure_ascii=False, indent=1) + "\n"
    return files


def write_all(o=None):
    files = generate(o)
    for path, text in files.items():
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(text)
    return sorted(files)


def stale(o=None):
    """Generated files that differ from the current tso.yaml."""
    out = []
    for path, text in generate(o).items():
        try:
            cur = open(path, encoding="utf-8").read()
        except OSError:
            cur = None
        if cur != text:
            out.append(path)
    return out

-- GENERATED from tsagent/ontology/tso.yaml (version 1, sha256 827d5259050344fed165347904c5b9d357c7b16d77ccb6c2ff9977cb6cce9982).
-- Do not edit by hand: edit tso.yaml, then run `make codegen`.

PRAGMA foreign_keys = ON;

-- ===== sources (immutable) =====
CREATE TABLE src_baseline (
  id TEXT PRIMARY KEY,
  feature TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  format TEXT NOT NULL,
  ontology_version TEXT NOT NULL,
  primary_doc TEXT,
  content_hash TEXT NOT NULL,
  docs TEXT NOT NULL DEFAULT '[]',
  ingested_at TEXT NOT NULL,
  UNIQUE(feature, ordinal)
);
CREATE TRIGGER trg_src_baseline_no_update BEFORE UPDATE ON src_baseline
BEGIN SELECT RAISE(ABORT, 'immutable: src_baseline is append-only'); END;
CREATE TRIGGER trg_src_baseline_no_delete BEFORE DELETE ON src_baseline
BEGIN SELECT RAISE(ABORT, 'immutable: src_baseline is append-only'); END;
CREATE TABLE src_onto_class (
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  name TEXT NOT NULL,
  parent TEXT,
  required TEXT NOT NULL DEFAULT '[]',
  attrs TEXT NOT NULL DEFAULT '[]',
  doc TEXT,
  PRIMARY KEY(baseline_id, name)
);
CREATE TRIGGER trg_src_onto_class_no_update BEFORE UPDATE ON src_onto_class
BEGIN SELECT RAISE(ABORT, 'immutable: src_onto_class is append-only'); END;
CREATE TRIGGER trg_src_onto_class_no_delete BEFORE DELETE ON src_onto_class
BEGIN SELECT RAISE(ABORT, 'immutable: src_onto_class is append-only'); END;
CREATE TABLE src_onto_relation (
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  name TEXT NOT NULL,
  domain TEXT NOT NULL DEFAULT '[]',
  range TEXT NOT NULL DEFAULT '[]',
  doc TEXT,
  PRIMARY KEY(baseline_id, name)
);
CREATE TRIGGER trg_src_onto_relation_no_update BEFORE UPDATE ON src_onto_relation
BEGIN SELECT RAISE(ABORT, 'immutable: src_onto_relation is append-only'); END;
CREATE TRIGGER trg_src_onto_relation_no_delete BEFORE DELETE ON src_onto_relation
BEGIN SELECT RAISE(ABORT, 'immutable: src_onto_relation is append-only'); END;
CREATE TABLE src_node (
  id INTEGER PRIMARY KEY,
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  code TEXT NOT NULL,
  class TEXT NOT NULL,
  base TEXT NOT NULL DEFAULT '{}',
  props TEXT NOT NULL DEFAULT '{}',
  test_scope TEXT,
  origin TEXT,
  content_hash TEXT NOT NULL,
  supersedes_node_id INTEGER REFERENCES src_node(id),
  UNIQUE(baseline_id, code)
);
CREATE INDEX ix_src_node_0 ON src_node(class);
CREATE TRIGGER trg_src_node_no_update BEFORE UPDATE ON src_node
BEGIN SELECT RAISE(ABORT, 'immutable: src_node is append-only'); END;
CREATE TRIGGER trg_src_node_no_delete BEFORE DELETE ON src_node
BEGIN SELECT RAISE(ABORT, 'immutable: src_node is append-only'); END;
CREATE TABLE src_edge (
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  src_id INTEGER NOT NULL REFERENCES src_node(id),
  rel TEXT NOT NULL,
  dst_id INTEGER NOT NULL REFERENCES src_node(id),
  props TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY(baseline_id, src_id, rel, dst_id)
);
CREATE INDEX ix_src_edge_0 ON src_edge(dst_id, rel);
CREATE TRIGGER trg_src_edge_no_update BEFORE UPDATE ON src_edge
BEGIN SELECT RAISE(ABORT, 'immutable: src_edge is append-only'); END;
CREATE TRIGGER trg_src_edge_no_delete BEFORE DELETE ON src_edge
BEGIN SELECT RAISE(ABORT, 'immutable: src_edge is append-only'); END;
CREATE TABLE src_supersede (
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  old_code TEXT NOT NULL,
  new_code TEXT NOT NULL,
  PRIMARY KEY(baseline_id, old_code)
);
CREATE TRIGGER trg_src_supersede_no_update BEFORE UPDATE ON src_supersede
BEGIN SELECT RAISE(ABORT, 'immutable: src_supersede is append-only'); END;
CREATE TRIGGER trg_src_supersede_no_delete BEFORE DELETE ON src_supersede
BEGIN SELECT RAISE(ABORT, 'immutable: src_supersede is append-only'); END;
CREATE TABLE src_derived_node (
  id INTEGER PRIMARY KEY,
  baseline_id TEXT NOT NULL REFERENCES src_baseline(id),
  code TEXT NOT NULL,
  class TEXT NOT NULL,
  props TEXT NOT NULL DEFAULT '{}',
  edges TEXT NOT NULL DEFAULT '[]',
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(baseline_id, code)
);
CREATE TRIGGER trg_src_derived_node_no_update BEFORE UPDATE ON src_derived_node
BEGIN SELECT RAISE(ABORT, 'immutable: src_derived_node is append-only'); END;
CREATE TRIGGER trg_src_derived_node_no_delete BEFORE DELETE ON src_derived_node
BEGIN SELECT RAISE(ABORT, 'immutable: src_derived_node is append-only'); END;

-- ===== operations =====
CREATE TABLE run (
  run_id TEXT PRIMARY KEY,
  feature TEXT NOT NULL,
  run_key TEXT NOT NULL,
  mode TEXT NOT NULL,
  model TEXT,
  status TEXT NOT NULL,
  base_baseline_id TEXT,
  params TEXT NOT NULL DEFAULT '{}',
  notes TEXT NOT NULL DEFAULT '[]',
  result TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX ix_run_0 ON run(run_key);
CREATE INDEX ix_run_1 ON run(feature);
CREATE TABLE run_step (
  run_id TEXT NOT NULL REFERENCES run(run_id),
  step TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  llm_calls INTEGER NOT NULL DEFAULT 0,
  cache_hits INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  PRIMARY KEY(run_id, step)
);
CREATE TABLE stage_item (
  run_id TEXT NOT NULL REFERENCES run(run_id),
  step TEXT NOT NULL,
  code TEXT NOT NULL,
  class TEXT NOT NULL,
  payload TEXT NOT NULL,
  PRIMARY KEY(run_id, step, code)
);
CREATE TABLE rendered_file (
  feature TEXT NOT NULL,
  kind TEXT NOT NULL,
  path TEXT NOT NULL,
  sha TEXT NOT NULL,
  version INTEGER NOT NULL,
  PRIMARY KEY(feature, kind)
);
CREATE TABLE schema_version (
  version INTEGER NOT NULL,
  yaml_hash TEXT NOT NULL,
  applied_at TEXT NOT NULL
);

-- ===== test KG core =====
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
CREATE TRIGGER trg_tso_based_on_immutable_ins BEFORE INSERT ON tso_based_on
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = NEW.tbl AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;
CREATE TRIGGER trg_tso_based_on_immutable_del BEFORE DELETE ON tso_based_on
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = OLD.tbl AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;
CREATE TRIGGER trg_tso_based_on_immutable_upd BEFORE UPDATE ON tso_based_on
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = OLD.tbl AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;
CREATE TRIGGER trg_tso_evidence_immutable_ins BEFORE INSERT ON tso_evidence
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = NEW.tbl AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;
CREATE TRIGGER trg_tso_evidence_immutable_del BEFORE DELETE ON tso_evidence
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = OLD.tbl AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;
CREATE TRIGGER trg_tso_evidence_immutable_upd BEFORE UPDATE ON tso_evidence
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = OLD.tbl AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: basis of a baselined node'); END;

-- ----- TestBasisItem (layer A): Pointer to a TDD node in a given baseline
CREATE TABLE tso_test_basis_item (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_basis_item(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "tdd_code" TEXT NOT NULL,
  "tdd_class" TEXT NOT NULL,
  "baseline_id" TEXT NOT NULL,
  "in_scope" INTEGER NOT NULL CHECK ("in_scope" IN (0, 1)),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_basis_item_node ON tso_test_basis_item(node_id);
CREATE TRIGGER trg_tso_test_basis_item_immutable_upd BEFORE UPDATE ON tso_test_basis_item
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_basis_item' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_basis_item is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_basis_item_immutable_del BEFORE DELETE ON tso_test_basis_item
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_basis_item' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_basis_item is baselined and cannot be deleted'); END;

-- ----- TestObligation (layer A): What must be proven
CREATE TABLE tso_test_obligation (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_obligation(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "question" TEXT NOT NULL,
  "origin" TEXT NOT NULL CHECK ("origin" IN ('designObligation', 'seedRule', 'risk', 'llmProposal')),
  "applicability" TEXT NOT NULL CHECK ("applicability" IN ('applicable', 'notApplicable', 'unknown')),
  "priority" TEXT NOT NULL CHECK ("priority" IN ('P0', 'P1', 'P2', 'P3')),
  rated_by_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_obligation_node ON tso_test_obligation(node_id);
CREATE INDEX ix_tso_test_obligation_rated_by ON tso_test_obligation(rated_by_node_id);
CREATE TRIGGER trg_tso_test_obligation_immutable_upd BEFORE UPDATE ON tso_test_obligation
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_obligation' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_obligation is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_obligation_immutable_del BEFORE DELETE ON tso_test_obligation
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_obligation' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_obligation is baselined and cannot be deleted'); END;
CREATE TABLE tso_test_obligation__subject (
  row_id INTEGER NOT NULL REFERENCES tso_test_obligation(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_obligation__subject_node ON tso_test_obligation__subject(node_id);
CREATE TRIGGER trg_tso_test_obligation__subject_immutable_ins BEFORE INSERT ON tso_test_obligation__subject
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_obligation' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_obligation is baselined, relation subject cannot change'); END;
CREATE TRIGGER trg_tso_test_obligation__subject_immutable_del BEFORE DELETE ON tso_test_obligation__subject
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_obligation' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_obligation is baselined, relation subject cannot change'); END;

-- ----- TestCondition (layer A): One testable aspect of an obligation
CREATE TABLE tso_test_condition (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_condition(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "condition_kind" TEXT NOT NULL CHECK ("condition_kind" IN ('functional', 'inputValidation', 'stateTransition', 'guardViolation', 'invariant', 'decisionRule', 'errorHandling', 'idempotency', 'concurrency', 'timing', 'atomicity', 'compensation', 'migration', 'compatibility', 'access', 'secretExposure', 'audit', 'performance', 'resilience', 'observability', 'rollout')),
  "statement" TEXT NOT NULL,
  "expected_effect" TEXT NOT NULL,
  "pre_state" TEXT,
  "trigger" TEXT,
  "key" TEXT NOT NULL,
  obligation_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  subject_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  oracle_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  fault_model_node_id INTEGER REFERENCES tso_node(id),
  partition_node_id INTEGER REFERENCES tso_node(id),
  path_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_condition_node ON tso_test_condition(node_id);
CREATE INDEX ix_tso_test_condition_obligation ON tso_test_condition(obligation_node_id);
CREATE INDEX ix_tso_test_condition_subject ON tso_test_condition(subject_node_id);
CREATE INDEX ix_tso_test_condition_oracle ON tso_test_condition(oracle_node_id);
CREATE INDEX ix_tso_test_condition_fault_model ON tso_test_condition(fault_model_node_id);
CREATE INDEX ix_tso_test_condition_partition ON tso_test_condition(partition_node_id);
CREATE INDEX ix_tso_test_condition_path ON tso_test_condition(path_node_id);
CREATE TRIGGER trg_tso_test_condition_immutable_upd BEFORE UPDATE ON tso_test_condition
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_condition' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_condition is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_condition_immutable_del BEFORE DELETE ON tso_test_condition
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_condition' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_condition is baselined and cannot be deleted'); END;

-- ----- TestOracle (layer A): Source of the correct result
CREATE TABLE tso_test_oracle (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_oracle(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "oracle_kind" TEXT NOT NULL CHECK ("oracle_kind" IN ('postcondition', 'invariant', 'errorContract', 'threshold', 'stateMachine', 'decisionRow', 'schema', 'acceptanceCriterion', 'reference', 'humanJudgment')),
  "expected" TEXT NOT NULL,
  "tolerance" TEXT,
  "measurement" TEXT,
  source_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_oracle_node ON tso_test_oracle(node_id);
CREATE INDEX ix_tso_test_oracle_source ON tso_test_oracle(source_node_id);
CREATE TRIGGER trg_tso_test_oracle_immutable_upd BEFORE UPDATE ON tso_test_oracle
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_oracle' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_oracle is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_oracle_immutable_del BEFORE DELETE ON tso_test_oracle
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_oracle' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_oracle is baselined and cannot be deleted'); END;

-- ----- FaultModel (layer A): Injected fault
CREATE TABLE tso_fault_model (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_fault_model(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "fault_kind" TEXT NOT NULL,
  "injection_point" TEXT NOT NULL,
  "timing" TEXT NOT NULL CHECK ("timing" IN ('before', 'during', 'afterCommit', 'beforeAck')),
  "expected_handling" TEXT NOT NULL,
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_fault_model_node ON tso_fault_model(node_id);
CREATE TRIGGER trg_tso_fault_model_immutable_upd BEFORE UPDATE ON tso_fault_model
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fault_model' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fault_model is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_fault_model_immutable_del BEFORE DELETE ON tso_fault_model
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fault_model' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fault_model is baselined and cannot be deleted'); END;

-- ----- Partition (layer A): Equivalence class or boundary
CREATE TABLE tso_partition (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_partition(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "parameter" TEXT NOT NULL,
  "class_kind" TEXT NOT NULL CHECK ("class_kind" IN ('equivalence', 'boundary', 'enum', 'null', 'format', 'missing')),
  "values" TEXT NOT NULL CHECK (json_valid("values") AND json_array_length("values") >= 1),
  "valid" INTEGER NOT NULL CHECK ("valid" IN (0, 1)),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_partition_node ON tso_partition(node_id);
CREATE TRIGGER trg_tso_partition_immutable_upd BEFORE UPDATE ON tso_partition
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_partition' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_partition is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_partition_immutable_del BEFORE DELETE ON tso_partition
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_partition' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_partition is baselined and cannot be deleted'); END;

-- ----- ScenarioPath (layer A): Path through a flow or state machine
CREATE TABLE tso_scenario_path (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_scenario_path(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "path_kind" TEXT NOT NULL CHECK ("path_kind" IN ('main', 'alternative', 'failure', 'negative')),
  "steps" TEXT NOT NULL CHECK (json_valid("steps") AND json_array_length("steps") >= 1),
  scenario_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_scenario_path_node ON tso_scenario_path(node_id);
CREATE INDEX ix_tso_scenario_path_scenario ON tso_scenario_path(scenario_node_id);
CREATE TRIGGER trg_tso_scenario_path_immutable_upd BEFORE UPDATE ON tso_scenario_path
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_scenario_path' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_scenario_path is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_scenario_path_immutable_del BEFORE DELETE ON tso_scenario_path
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_scenario_path' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_scenario_path is baselined and cannot be deleted'); END;

-- ----- RiskRating (layer A): Risk-based priority
CREATE TABLE tso_risk_rating (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_risk_rating(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "likelihood" TEXT NOT NULL CHECK ("likelihood" IN ('Low', 'Medium', 'High')),
  "impact" TEXT NOT NULL CHECK ("impact" IN ('Low', 'Medium', 'High')),
  "priority" TEXT NOT NULL CHECK ("priority" IN ('P0', 'P1', 'P2', 'P3')),
  "rationale" TEXT NOT NULL,
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_risk_rating_node ON tso_risk_rating(node_id);
CREATE TRIGGER trg_tso_risk_rating_immutable_upd BEFORE UPDATE ON tso_risk_rating
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_risk_rating' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_risk_rating is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_risk_rating_immutable_del BEFORE DELETE ON tso_risk_rating
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_risk_rating' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_risk_rating is baselined and cannot be deleted'); END;

-- ----- TestGap (layer A): Information missing to write a condition or oracle
CREATE TABLE tso_test_gap (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_gap(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "missing" TEXT NOT NULL,
  "owner" TEXT NOT NULL,
  "asked_question" TEXT NOT NULL,
  "due_date" TEXT,
  blocks_obligation_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_gap_node ON tso_test_gap(node_id);
CREATE INDEX ix_tso_test_gap_blocks_obligation ON tso_test_gap(blocks_obligation_node_id);
CREATE TRIGGER trg_tso_test_gap_immutable_upd BEFORE UPDATE ON tso_test_gap
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_gap' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_gap is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_gap_immutable_del BEFORE DELETE ON tso_test_gap
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_gap' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_gap is baselined and cannot be deleted'); END;
CREATE TABLE tso_test_gap__blocks (
  row_id INTEGER NOT NULL REFERENCES tso_test_gap(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_gap__blocks_node ON tso_test_gap__blocks(node_id);
CREATE TRIGGER trg_tso_test_gap__blocks_immutable_ins BEFORE INSERT ON tso_test_gap__blocks
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_gap' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_gap is baselined, relation blocks cannot change'); END;
CREATE TRIGGER trg_tso_test_gap__blocks_immutable_del BEFORE DELETE ON tso_test_gap__blocks
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_gap' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_gap is baselined, relation blocks cannot change'); END;

-- ----- TestRequirement (layer B): Groups conditions of the same subject and level
CREATE TABLE tso_test_requirement (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_requirement(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "statement" TEXT NOT NULL,
  "level" TEXT NOT NULL CHECK ("level" IN ('L1', 'L2', 'L3')),
  "test_type" TEXT NOT NULL CHECK ("test_type" IN ('functional', 'reliability', 'performance', 'security', 'compatibility', 'maintainability', 'usability')),
  "priority" TEXT NOT NULL CHECK ("priority" IN ('P0', 'P1', 'P2', 'P3')),
  "verification_method" TEXT NOT NULL CHECK ("verification_method" IN ('test', 'benchmark', 'analysis', 'inspection')),
  "scope_note" TEXT NOT NULL,
  subject_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_requirement_node ON tso_test_requirement(node_id);
CREATE INDEX ix_tso_test_requirement_subject ON tso_test_requirement(subject_node_id);
CREATE TRIGGER trg_tso_test_requirement_immutable_upd BEFORE UPDATE ON tso_test_requirement
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_requirement' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_requirement is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_requirement_immutable_del BEFORE DELETE ON tso_test_requirement
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_requirement' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_requirement is baselined and cannot be deleted'); END;
CREATE TABLE tso_test_requirement__groups (
  row_id INTEGER NOT NULL REFERENCES tso_test_requirement(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_requirement__groups_node ON tso_test_requirement__groups(node_id);
CREATE TRIGGER trg_tso_test_requirement__groups_immutable_ins BEFORE INSERT ON tso_test_requirement__groups
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_requirement' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_requirement is baselined, relation groups cannot change'); END;
CREATE TRIGGER trg_tso_test_requirement__groups_immutable_del BEFORE DELETE ON tso_test_requirement__groups
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_requirement' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_requirement is baselined, relation groups cannot change'); END;

-- ----- PassCriterion (layer B): Numbered criterion item of a TR
CREATE TABLE tso_pass_criterion (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_pass_criterion(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "ordinal" INTEGER NOT NULL,
  "text" TEXT NOT NULL,
  requirement_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  condition_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_pass_criterion_node ON tso_pass_criterion(node_id);
CREATE INDEX ix_tso_pass_criterion_requirement ON tso_pass_criterion(requirement_node_id);
CREATE INDEX ix_tso_pass_criterion_condition ON tso_pass_criterion(condition_node_id);
CREATE TRIGGER trg_tso_pass_criterion_immutable_upd BEFORE UPDATE ON tso_pass_criterion
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_pass_criterion' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_pass_criterion is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_pass_criterion_immutable_del BEFORE DELETE ON tso_pass_criterion
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_pass_criterion' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_pass_criterion is baselined and cannot be deleted'); END;

-- ----- TestDesign (layer B): Technique and coverage criterion
CREATE TABLE tso_test_design (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_design(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "technique" TEXT NOT NULL CHECK ("technique" IN ('equivalencePartitioning', 'boundaryValue', 'stateTransition', 'decisionTable', 'causeEffect', 'pairwise', 'scenario', 'useCase', 'errorGuessing', 'faultInjection', 'loadModel', 'accessMatrix')),
  "coverage_criterion" TEXT NOT NULL,
  "coverage_target" INTEGER NOT NULL,
  "rationale" TEXT NOT NULL,
  addresses_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_design_node ON tso_test_design(node_id);
CREATE INDEX ix_tso_test_design_addresses ON tso_test_design(addresses_node_id);
CREATE TRIGGER trg_tso_test_design_immutable_upd BEFORE UPDATE ON tso_test_design
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_design' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_design is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_design_immutable_del BEFORE DELETE ON tso_test_design
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_design' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_design is baselined and cannot be deleted'); END;

-- ----- CoverageItem (layer B): Smallest coverage unit
CREATE TABLE tso_coverage_item (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_coverage_item(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "coverage_kind" TEXT NOT NULL,
  "description" TEXT NOT NULL,
  "expected" TEXT NOT NULL,
  "value" TEXT CHECK ("value" IS NULL OR (json_valid("value"))),
  design_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  realizes_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  expected_from_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_coverage_item_node ON tso_coverage_item(node_id);
CREATE INDEX ix_tso_coverage_item_design ON tso_coverage_item(design_node_id);
CREATE INDEX ix_tso_coverage_item_realizes ON tso_coverage_item(realizes_node_id);
CREATE INDEX ix_tso_coverage_item_expected_from ON tso_coverage_item(expected_from_node_id);
CREATE TRIGGER trg_tso_coverage_item_immutable_upd BEFORE UPDATE ON tso_coverage_item
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_coverage_item' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_coverage_item is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_coverage_item_immutable_del BEFORE DELETE ON tso_coverage_item
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_coverage_item' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_coverage_item is baselined and cannot be deleted'); END;

-- ----- TestCase (layer C): Test case
CREATE TABLE tso_test_case (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_case(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "objective" TEXT NOT NULL,
  "level" TEXT NOT NULL CHECK ("level" IN ('L1', 'L2', 'L3')),
  "priority" TEXT NOT NULL CHECK ("priority" IN ('P0', 'P1', 'P2', 'P3')),
  "automatable" INTEGER NOT NULL CHECK ("automatable" IN (0, 1)),
  "preconditions" TEXT CHECK ("preconditions" IS NULL OR (json_valid("preconditions"))),
  in_set_node_id INTEGER REFERENCES tso_node(id),
  needs_data_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_case_node ON tso_test_case(node_id);
CREATE INDEX ix_tso_test_case_in_set ON tso_test_case(in_set_node_id);
CREATE INDEX ix_tso_test_case_needs_data ON tso_test_case(needs_data_node_id);
CREATE TRIGGER trg_tso_test_case_immutable_upd BEFORE UPDATE ON tso_test_case
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_case_immutable_del BEFORE DELETE ON tso_test_case
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined and cannot be deleted'); END;
CREATE TABLE tso_test_case__covers (
  row_id INTEGER NOT NULL REFERENCES tso_test_case(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_case__covers_node ON tso_test_case__covers(node_id);
CREATE TRIGGER trg_tso_test_case__covers_immutable_ins BEFORE INSERT ON tso_test_case__covers
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation covers cannot change'); END;
CREATE TRIGGER trg_tso_test_case__covers_immutable_del BEFORE DELETE ON tso_test_case__covers
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation covers cannot change'); END;
CREATE TABLE tso_test_case__needs_fixture (
  row_id INTEGER NOT NULL REFERENCES tso_test_case(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_case__needs_fixture_node ON tso_test_case__needs_fixture(node_id);
CREATE TRIGGER trg_tso_test_case__needs_fixture_immutable_ins BEFORE INSERT ON tso_test_case__needs_fixture
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation needsFixture cannot change'); END;
CREATE TRIGGER trg_tso_test_case__needs_fixture_immutable_del BEFORE DELETE ON tso_test_case__needs_fixture
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation needsFixture cannot change'); END;
CREATE TABLE tso_test_case__needs_stub (
  row_id INTEGER NOT NULL REFERENCES tso_test_case(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_case__needs_stub_node ON tso_test_case__needs_stub(node_id);
CREATE TRIGGER trg_tso_test_case__needs_stub_immutable_ins BEFORE INSERT ON tso_test_case__needs_stub
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation needsStub cannot change'); END;
CREATE TRIGGER trg_tso_test_case__needs_stub_immutable_del BEFORE DELETE ON tso_test_case__needs_stub
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation needsStub cannot change'); END;
CREATE TABLE tso_test_case__injects (
  row_id INTEGER NOT NULL REFERENCES tso_test_case(id) ON DELETE CASCADE,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  PRIMARY KEY(row_id, node_id)
);
CREATE INDEX ix_tso_test_case__injects_node ON tso_test_case__injects(node_id);
CREATE TRIGGER trg_tso_test_case__injects_immutable_ins BEFORE INSERT ON tso_test_case__injects
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = NEW.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation injects cannot change'); END;
CREATE TRIGGER trg_tso_test_case__injects_immutable_del BEFORE DELETE ON tso_test_case__injects
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_case' AND row_id = OLD.row_id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_case is baselined, relation injects cannot change'); END;

-- ----- TestStep (layer C): A given/when/then step
CREATE TABLE tso_test_step (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_step(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "ordinal" INTEGER NOT NULL,
  "step_kind" TEXT NOT NULL CHECK ("step_kind" IN ('given', 'when', 'then', 'cleanup')),
  "action" TEXT NOT NULL,
  "input" TEXT CHECK ("input" IS NULL OR (json_valid("input"))),
  "target_code" TEXT,
  test_case_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_step_node ON tso_test_step(node_id);
CREATE INDEX ix_tso_test_step_test_case ON tso_test_step(test_case_node_id);
CREATE TRIGGER trg_tso_test_step_immutable_upd BEFORE UPDATE ON tso_test_step
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_step' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_step is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_step_immutable_del BEFORE DELETE ON tso_test_step
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_step' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_step is baselined and cannot be deleted'); END;

-- ----- Assertion (layer C): Result check; the expected result comes from the oracle
CREATE TABLE tso_assertion (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_assertion(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "ordinal" INTEGER NOT NULL,
  "assertion_kind" TEXT NOT NULL CHECK ("assertion_kind" IN ('status', 'errorCode', 'state', 'record', 'event', 'header', 'latency', 'count', 'absence', 'value', 'threshold')),
  "operator" TEXT NOT NULL,
  "expected" TEXT NOT NULL,
  "unit" TEXT,
  "description" TEXT,
  test_case_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  oracle_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_assertion_node ON tso_assertion(node_id);
CREATE INDEX ix_tso_assertion_test_case ON tso_assertion(test_case_node_id);
CREATE INDEX ix_tso_assertion_oracle ON tso_assertion(oracle_node_id);
CREATE TRIGGER trg_tso_assertion_immutable_upd BEFORE UPDATE ON tso_assertion
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_assertion' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_assertion is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_assertion_immutable_del BEFORE DELETE ON tso_assertion
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_assertion' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_assertion is baselined and cannot be deleted'); END;

-- ----- TestData (layer C): Test data
CREATE TABLE tso_test_data (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_data(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "entity" TEXT NOT NULL,
  "lifecycle" TEXT NOT NULL,
  "sensitivity" TEXT NOT NULL CHECK ("sensitivity" IN ('none', 'internal', 'secret')),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_data_node ON tso_test_data(node_id);
CREATE TRIGGER trg_tso_test_data_immutable_upd BEFORE UPDATE ON tso_test_data
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_data' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_data is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_data_immutable_del BEFORE DELETE ON tso_test_data
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_data' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_data is baselined and cannot be deleted'); END;

-- ----- DataField (layer C): Data field
CREATE TABLE tso_data_field (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_data_field(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "value_class" TEXT NOT NULL,
  "example" TEXT NOT NULL,
  "sensitive" INTEGER NOT NULL CHECK ("sensitive" IN (0, 1)),
  data_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_data_field_node ON tso_data_field(node_id);
CREATE INDEX ix_tso_data_field_data ON tso_data_field(data_node_id);
CREATE TRIGGER trg_tso_data_field_immutable_upd BEFORE UPDATE ON tso_data_field
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_data_field' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_data_field is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_data_field_immutable_del BEFORE DELETE ON tso_data_field
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_data_field' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_data_field is baselined and cannot be deleted'); END;

-- ----- Fixture (layer C): Background state
CREATE TABLE tso_fixture (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_fixture(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "kind" TEXT NOT NULL,
  "setup" TEXT NOT NULL,
  "teardown" TEXT NOT NULL,
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_fixture_node ON tso_fixture(node_id);
CREATE TRIGGER trg_tso_fixture_immutable_upd BEFORE UPDATE ON tso_fixture
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fixture' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fixture is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_fixture_immutable_del BEFORE DELETE ON tso_fixture
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fixture' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fixture is baselined and cannot be deleted'); END;

-- ----- StubSpec (layer C): Dependency stub
CREATE TABLE tso_stub_spec (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_stub_spec(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "target_code" TEXT NOT NULL,
  "behavior" TEXT NOT NULL CHECK ("behavior" IN ('success', 'errorTransient', 'errorPermanent', 'timeout', 'slow', 'duplicate', 'malformedPayload')),
  "status_code" INTEGER,
  "delay_ms" INTEGER,
  "response" TEXT CHECK ("response" IS NULL OR (json_valid("response"))),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_stub_spec_node ON tso_stub_spec(node_id);
CREATE TRIGGER trg_tso_stub_spec_immutable_upd BEFORE UPDATE ON tso_stub_spec
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_stub_spec' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_stub_spec is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_stub_spec_immutable_del BEFORE DELETE ON tso_stub_spec
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_stub_spec' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_stub_spec is baselined and cannot be deleted'); END;

-- ----- FaultInjection (layer C): Realises a FaultModel
CREATE TABLE tso_fault_injection (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_fault_injection(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "mechanism" TEXT NOT NULL,
  "trigger" TEXT NOT NULL,
  realizes_node_id INTEGER NOT NULL REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_fault_injection_node ON tso_fault_injection(node_id);
CREATE INDEX ix_tso_fault_injection_realizes ON tso_fault_injection(realizes_node_id);
CREATE TRIGGER trg_tso_fault_injection_immutable_upd BEFORE UPDATE ON tso_fault_injection
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fault_injection' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fault_injection is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_fault_injection_immutable_del BEFORE DELETE ON tso_fault_injection
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_fault_injection' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_fault_injection is baselined and cannot be deleted'); END;

-- ----- TestEnvironment (layer C): Execution environment
CREATE TABLE tso_test_environment (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_environment(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "name" TEXT NOT NULL,
  "services" TEXT CHECK ("services" IS NULL OR (json_valid("services"))),
  "secret_ref" TEXT CHECK ("secret_ref" IS NULL OR (json_valid("secret_ref"))),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_environment_node ON tso_test_environment(node_id);
CREATE TRIGGER trg_tso_test_environment_immutable_upd BEFORE UPDATE ON tso_test_environment
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_environment' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_environment is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_environment_immutable_del BEFORE DELETE ON tso_test_environment
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_environment' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_environment is baselined and cannot be deleted'); END;

-- ----- TestSet (layer C): Test case group
CREATE TABLE tso_test_set (
  id INTEGER PRIMARY KEY,
  node_id INTEGER NOT NULL REFERENCES tso_node(id),
  node_version INTEGER NOT NULL CHECK (node_version >= 1),
  content_hash TEXT NOT NULL,
  supersedes_id INTEGER REFERENCES tso_test_set(id),
  run_id TEXT NOT NULL,
  derivation TEXT NOT NULL CHECK (derivation IN ('stated', 'rule', 'llm', 'human')),
  confidence REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  model TEXT,
  prompt_hash TEXT,
  created_at TEXT NOT NULL,
  "purpose" TEXT NOT NULL,
  "ordinal" INTEGER NOT NULL,
  "entry_criteria" TEXT NOT NULL CHECK (json_valid("entry_criteria") AND json_array_length("entry_criteria") >= 1),
  "exit_criteria" TEXT NOT NULL CHECK (json_valid("exit_criteria") AND json_array_length("exit_criteria") >= 1),
  runs_in_node_id INTEGER REFERENCES tso_node(id),
  UNIQUE(node_id, node_version)
);
CREATE INDEX ix_tso_test_set_node ON tso_test_set(node_id);
CREATE INDEX ix_tso_test_set_runs_in ON tso_test_set(runs_in_node_id);
CREATE TRIGGER trg_tso_test_set_immutable_upd BEFORE UPDATE ON tso_test_set
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_set' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_set is baselined and cannot be updated'); END;
CREATE TRIGGER trg_tso_test_set_immutable_del BEFORE DELETE ON tso_test_set
WHEN (SELECT status FROM tso_lifecycle WHERE tbl = 'tso_test_set' AND row_id = OLD.id) IN ('Baselined','Superseded','Retired')
BEGIN SELECT RAISE(ABORT, 'immutable: tso_test_set is baselined and cannot be deleted'); END;

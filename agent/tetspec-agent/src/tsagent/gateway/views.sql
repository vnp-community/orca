-- GENERATED from tsagent/ontology/tso.yaml (version 1, sha256 827d5259050344fed165347904c5b9d357c7b16d77ccb6c2ff9977cb6cce9982).
-- Do not edit by hand: edit tso.yaml, then run `make codegen`.

-- Read-only views for MCP sql_select (MECH-14). Each view takes the latest version of each feature.
CREATE VIEW IF NOT EXISTS v_spec_current AS
SELECT sv.* FROM tso_spec_version sv
WHERE sv.version = (SELECT MAX(version) FROM tso_spec_version x WHERE x.feature = sv.feature);
CREATE VIEW IF NOT EXISTS v_source_node AS
SELECT b.feature, n.baseline_id, n.code, n.class, n.base, n.props, n.test_scope
FROM src_node n JOIN src_baseline b ON b.id = n.baseline_id
WHERE b.ordinal = (SELECT MAX(ordinal) FROM src_baseline x WHERE x.feature = b.feature);
CREATE VIEW IF NOT EXISTS v_test_basis_item AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."tdd_code", t."tdd_class", t."baseline_id", t."in_scope"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_basis_item'
JOIN tso_test_basis_item t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_basis_item' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_obligation AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."question", t."origin", t."applicability", t."priority", (SELECT code FROM tso_node WHERE id = t.rated_by_node_id) AS rated_by, (SELECT json_group_array(nn.code) FROM tso_test_obligation__subject j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS subject
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_obligation'
JOIN tso_test_obligation t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_obligation' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_condition AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."condition_kind", t."statement", t."expected_effect", t."pre_state", t."trigger", t."key", (SELECT code FROM tso_node WHERE id = t.obligation_node_id) AS obligation, (SELECT code FROM tso_node WHERE id = t.subject_node_id) AS subject, (SELECT code FROM tso_node WHERE id = t.oracle_node_id) AS oracle, (SELECT code FROM tso_node WHERE id = t.fault_model_node_id) AS fault_model, (SELECT code FROM tso_node WHERE id = t.partition_node_id) AS partition, (SELECT code FROM tso_node WHERE id = t.path_node_id) AS path
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_condition'
JOIN tso_test_condition t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_condition' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_oracle AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."oracle_kind", t."expected", t."tolerance", t."measurement", (SELECT code FROM tso_node WHERE id = t.source_node_id) AS source
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_oracle'
JOIN tso_test_oracle t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_oracle' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_fault_model AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."fault_kind", t."injection_point", t."timing", t."expected_handling"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_fault_model'
JOIN tso_fault_model t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_fault_model' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_partition AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."parameter", t."class_kind", t."values", t."valid"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_partition'
JOIN tso_partition t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_partition' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_scenario_path AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."path_kind", t."steps", (SELECT code FROM tso_node WHERE id = t.scenario_node_id) AS scenario
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_scenario_path'
JOIN tso_scenario_path t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_scenario_path' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_risk_rating AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."likelihood", t."impact", t."priority", t."rationale"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_risk_rating'
JOIN tso_risk_rating t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_risk_rating' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_gap AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."missing", t."owner", t."asked_question", t."due_date", (SELECT code FROM tso_node WHERE id = t.blocks_obligation_node_id) AS blocks_obligation, (SELECT json_group_array(nn.code) FROM tso_test_gap__blocks j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS blocks
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_gap'
JOIN tso_test_gap t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_gap' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_requirement AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."statement", t."level", t."test_type", t."priority", t."verification_method", t."scope_note", (SELECT code FROM tso_node WHERE id = t.subject_node_id) AS subject, (SELECT json_group_array(nn.code) FROM tso_test_requirement__groups j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS groups
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_requirement'
JOIN tso_test_requirement t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_requirement' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_pass_criterion AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."ordinal", t."text", (SELECT code FROM tso_node WHERE id = t.requirement_node_id) AS requirement, (SELECT code FROM tso_node WHERE id = t.condition_node_id) AS condition
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_pass_criterion'
JOIN tso_pass_criterion t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_pass_criterion' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_design AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."technique", t."coverage_criterion", t."coverage_target", t."rationale", (SELECT code FROM tso_node WHERE id = t.addresses_node_id) AS addresses
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_design'
JOIN tso_test_design t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_design' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_coverage_item AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."coverage_kind", t."description", t."expected", t."value", (SELECT code FROM tso_node WHERE id = t.design_node_id) AS design, (SELECT code FROM tso_node WHERE id = t.realizes_node_id) AS realizes, (SELECT code FROM tso_node WHERE id = t.expected_from_node_id) AS expected_from
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_coverage_item'
JOIN tso_coverage_item t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_coverage_item' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_case AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."objective", t."level", t."priority", t."automatable", t."preconditions", (SELECT code FROM tso_node WHERE id = t.in_set_node_id) AS in_set, (SELECT code FROM tso_node WHERE id = t.needs_data_node_id) AS needs_data, (SELECT json_group_array(nn.code) FROM tso_test_case__covers j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS covers, (SELECT json_group_array(nn.code) FROM tso_test_case__needs_fixture j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS needs_fixture, (SELECT json_group_array(nn.code) FROM tso_test_case__needs_stub j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS needs_stub, (SELECT json_group_array(nn.code) FROM tso_test_case__injects j JOIN tso_node nn ON nn.id = j.node_id WHERE j.row_id = t.id) AS injects
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_case'
JOIN tso_test_case t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_case' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_step AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."ordinal", t."step_kind", t."action", t."input", t."target_code", (SELECT code FROM tso_node WHERE id = t.test_case_node_id) AS test_case
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_step'
JOIN tso_test_step t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_step' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_assertion AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."ordinal", t."assertion_kind", t."operator", t."expected", t."unit", t."description", (SELECT code FROM tso_node WHERE id = t.test_case_node_id) AS test_case, (SELECT code FROM tso_node WHERE id = t.oracle_node_id) AS oracle
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_assertion'
JOIN tso_assertion t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_assertion' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_data AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."entity", t."lifecycle", t."sensitivity"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_data'
JOIN tso_test_data t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_data' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_data_field AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."name", t."value_class", t."example", t."sensitive", (SELECT code FROM tso_node WHERE id = t.data_node_id) AS data
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_data_field'
JOIN tso_data_field t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_data_field' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_fixture AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."kind", t."setup", t."teardown"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_fixture'
JOIN tso_fixture t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_fixture' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_stub_spec AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."target_code", t."behavior", t."status_code", t."delay_ms", t."response"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_stub_spec'
JOIN tso_stub_spec t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_stub_spec' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_fault_injection AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."mechanism", t."trigger", (SELECT code FROM tso_node WHERE id = t.realizes_node_id) AS realizes
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_fault_injection'
JOIN tso_fault_injection t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_fault_injection' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_environment AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."name", t."services", t."secret_ref"
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_environment'
JOIN tso_test_environment t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_environment' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_test_set AS
SELECT sv.feature, sv.version AS spec_version, n.code, t.node_version, t.derivation, t.confidence, lc.status, lc.review_status, t."purpose", t."ordinal", t."entry_criteria", t."exit_criteria", (SELECT code FROM tso_node WHERE id = t.runs_in_node_id) AS runs_in
FROM v_spec_current sv JOIN tso_spec_version_item i ON i.spec_version_id = sv.id AND i.tbl = 'tso_test_set'
JOIN tso_test_set t ON t.id = i.row_id JOIN tso_node n ON n.id = t.node_id
LEFT JOIN tso_lifecycle lc ON lc.tbl = 'tso_test_set' AND lc.row_id = t.id;
CREATE VIEW IF NOT EXISTS v_gap AS SELECT * FROM v_test_gap;
CREATE VIEW IF NOT EXISTS v_trace AS
SELECT r.feature, r.code AS requirement, r.level, pc.ordinal, c.code AS condition, c.condition_kind,
  b.tdd_code AS source, ci.code AS coverage_item, tc.code AS test_case
FROM v_test_requirement r
JOIN v_pass_criterion pc ON pc.feature = r.feature AND pc.requirement = r.code
JOIN v_test_condition c ON c.feature = r.feature AND c.code = pc.condition
LEFT JOIN v_test_basis_item b ON b.feature = r.feature AND b.code = c.subject
LEFT JOIN v_coverage_item ci ON ci.feature = r.feature AND ci.realizes = c.code
LEFT JOIN v_test_case tc ON tc.feature = r.feature AND EXISTS (SELECT 1 FROM json_each(tc.covers) WHERE value = ci.code);

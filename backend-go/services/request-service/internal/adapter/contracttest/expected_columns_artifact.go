package contracttest

// withArtifactColumns adds the columns and tables of migrations 0060 (CR-REQ-027) and 0061 (CR-REQ-028)
// so those features' schema stays in their own file.
func withArtifactColumns(m map[string][]Column) map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	m["requests"] = append(m["requests"], n("content_schema_version"), n("acceptance_criteria"), n("type_fields"), n("content_revision"), n("content_digest"))
	m["solutions"] = append(m["solutions"], n("seq"), n("schema_version"), n("provenance"), n("input_request_revision"), n("content_digest"))
	m["request_revisions"] = []Column{n("id"), n("tenant_id"), n("request_id"), n("revision"), n("cause"), n("snapshot"), n("digest"), y("actor_id"), n("actor_kind"), y("clarification_id"), n("created_at")}
	m["artifact_index"] = []Column{n("tenant_id"), n("display_id"), n("kind"), n("request_id"), n("artifact_id"), n("created_at")}
	m["artifact_relations"] = []Column{n("id"), n("tenant_id"), n("request_id"), n("rel"), n("from_kind"), n("from_id"), n("to_kind"), n("to_id"), y("created_by_run_id"), n("created_at")}
	m["request_coverage"] = []Column{n("id"), n("tenant_id"), n("request_id"), n("plan_task_id"), n("ac_id"), n("task_id"), y("check_id"), n("created_at")}
	m["clarifications"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("seq"), n("source"), n("source_ref"), n("status"), n("resume_status"), n("round"),
		n("asked_request_revision"), y("answered_request_revision"), n("due_at"), y("reminded_at"), n("cancel_reason"), n("created_by"),
		n("created_at"), y("answered_at"), n("version"),
	}
	m["clarification_questions"] = []Column{
		n("id"), n("tenant_id"), n("clarification_id"), n("seq"), n("question_key"), n("kind"), n("prompt"), n("reason"), y("options"),
		y("suggested_default"), n("required"), y("target_path"), y("answer"), y("answer_source"), y("answered_by"), y("answered_at"),
	}
	m["clarification_assignees"] = []Column{n("clarification_id"), n("tenant_id"), n("principal_kind"), n("principal_id")}
	m["decisions"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("seq"), n("subject_kind"), n("subject_id"), n("subject_digest"), n("question"), n("options"),
		y("recommended_option_id"), n("recommendation_reason"), y("chosen_option_id"), y("chooser_id"), y("chosen_at"), n("rationale"),
		n("risk_level"), y("confirmed_by"), y("confirmed_at"), n("status"), n("version"), n("created_at"),
	}
	m["decision_history"] = []Column{n("id"), n("tenant_id"), n("decision_id"), n("action"), y("option_id"), y("actor_id"), n("rationale"), n("at")}
	return m
}

package contracttest

// withOutcomeReviewColumns adds the tables of migrations 0105 to 0109 (CR-OR-001, 002, 011, 012, 013, 022).
func withOutcomeReviewColumns(m map[string][]Column) map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	// project_engine_settings gains the project's review choice (migration 0106).
	m["project_engine_settings"] = append(m["project_engine_settings"], n("outcome_review"))
	m["outcome_reviews"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("phase"), n("head_key"), n("head_commit"), n("base_commit"), n("inputs_digest"), n("state"),
		n("model"), n("kg_snapshot"), n("design_digest"), n("context_digest"), y("created_by"), n("created_at"), n("updated_at"), n("version"),
		y("approved_by"), y("approved_at"), n("approval_note"),
	}
	m["outcome_review_runs"] = []Column{
		n("id"), n("tenant_id"), n("review_id"), n("request_id"), n("kind"), n("status"), n("scope"), n("agent_run_ref"), n("cost_estimate"),
		n("cost_actual"), n("error_code"), y("active_key"), n("started_at"), y("finished_at"), n("reason"), y("client_key"), y("created_by"),
	}
	m["outcome_review_contexts"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("kind"), n("digest"), n("prompt_version"), n("payload"), n("created_at"), n("expires_at"),
	}
	m["review_artifact_blobs"] = []Column{n("tenant_id"), n("request_id"), n("digest"), n("media_type"), n("size_bytes"), n("content"), n("created_at")}
	m["review_artifacts"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("section"), n("variant"), n("name"), n("diagram_type"), n("input_digest"), n("preset_digest"),
		n("skill_version"), n("revision"), n("state"), n("origin"), n("reason"), n("error_code"), n("blob_digest"), n("size_bytes"), n("view_box"),
		n("title"), n("description"), n("self_check"), n("model"), y("created_by"), n("created_at"), y("active_key"),
	}
	m["outcome_review_decisions"] = []Column{
		n("id"), n("tenant_id"), n("review_id"), n("request_id"), n("item_key"), n("decision"), n("note"), y("child_request_id"), n("decided_by"), n("decided_at"), y("active_key"),
	}
	m["review_drawing_profiles"] = []Column{n("id"), n("tenant_id"), n("project_key"), n("spec"), n("spec_digest"), n("version"), n("updated_by"), n("updated_at")}
	return m
}

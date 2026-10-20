package contracttest

// withOutcomeReviewColumns adds the tables of migrations 0105, 0107 and 0108 (CR-OR-001, 002, 011).
func withOutcomeReviewColumns(m map[string][]Column) map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	// project_engine_settings gains the project's review choice (migration 0106).
	m["project_engine_settings"] = append(m["project_engine_settings"], n("outcome_review"))
	m["outcome_reviews"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("phase"), n("head_key"), n("head_commit"), n("base_commit"), n("inputs_digest"), n("state"),
		n("model"), n("kg_snapshot"), n("design_digest"), n("context_digest"), y("created_by"), n("created_at"), n("updated_at"), n("version"),
	}
	m["outcome_review_runs"] = []Column{
		n("id"), n("tenant_id"), n("review_id"), n("request_id"), n("kind"), n("status"), n("scope"), n("agent_run_ref"), n("cost_estimate"),
		n("cost_actual"), n("error_code"), y("active_key"), n("started_at"), y("finished_at"),
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
	return m
}

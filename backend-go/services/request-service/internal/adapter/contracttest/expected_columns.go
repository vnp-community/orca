package contracttest

// Column is one expected information_schema row.
type Column struct {
	Name     string
	Nullable bool
}

// ExpectedColumns is the foundation schema both dialects must expose (CR-REQ-002 section 2.1).
// solution_engine comes from the solution-engines migration that sits later in the same chain.
func ExpectedColumns() map[string][]Column {
	return withArtifactColumns(foundationColumns())
}

func foundationColumns() map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	return map[string][]Column{
		"request_counters": {n("tenant_id"), n("next_number")},
		"requests": {
			n("id"), n("tenant_id"), y("project_id"), n("number"), n("title"), n("body"), n("source_provider"), n("source_ref"),
			n("source_url"), n("source_site"), y("type"), y("type_source"), y("size"), n("urgency"), y("confidence"),
			n("classification_reason"), n("status"), y("returned_from_stage"), n("return_reason"), y("plan_task_id"),
			n("reporter_id"), n("created_at"), n("updated_at"), n("version"), y("solution_engine"), y("returned_category"), y("source_hints"), n("classification_attempts"),
		},
		"request_type_history": {n("tenant_id"), n("request_id"), n("at"), y("from_type"), n("to_type"), n("actor_id"), y("actor_kind"), n("reason")},
		"solutions": {
			n("id"), n("tenant_id"), n("request_id"), n("options"), y("chosen_option"), n("version"), n("created_at"), n("updated_at"),
			n("kind"), n("status"), n("content_ref"), y("generation_run_id"),
		},
		"request_links":          {n("tenant_id"), n("parent_request_id"), n("child_request_id"), y("reason"), y("created_by"), n("created_at")},
		"request_return_history": {n("id"), n("tenant_id"), n("request_id"), n("action"), y("stage"), y("category"), n("reason"), y("actor_id"), n("actor_kind"), n("at")},
		"classification_runs": {
			n("id"), n("tenant_id"), n("request_id"), n("trigger_name"), y("source_event_id"), n("manual"), n("actor_id"), n("status"),
			n("claims"), n("lease_owner"), n("lease_expires_at"), n("error_code"), n("started_at"), y("finished_at"), y("active"),
		},
		"request_idempotency": {n("tenant_id"), n("source_provider"), n("source_site"), n("source_ref"), n("request_id"), n("created_at")},
	}
}

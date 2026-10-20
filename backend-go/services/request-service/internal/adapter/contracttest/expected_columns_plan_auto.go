package contracttest

// withPlanAutoColumns adds the table and the setting column of migration 0101 (CR-REQ-044). The MySQL-only generated
// active_key is index plumbing and is left out, as the schema check there already does.
func withPlanAutoColumns(m map[string][]Column) map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	m["plan_auto_jobs"] = []Column{
		n("tenant_id"), n("request_id"), n("anchor"), n("origin"), n("project_id"), n("state"), n("attempt"), n("max_attempts"), y("run_id"),
		y("reason_code"), n("commit_attempts"), n("next_check_at"), y("lease_owner"), y("lease_expires_at"), y("requested_by"), n("created_at"), n("updated_at"),
	}
	m["project_engine_settings"] = []Column{
		n("tenant_id"), n("project_id"), n("solution_engine"), y("openspec_min_version"), n("updated_by"), n("updated_at"), n("version"),
		n("solution_agent"), n("execution_mode"), n("plan_auto_generate"),
	}
	return m
}

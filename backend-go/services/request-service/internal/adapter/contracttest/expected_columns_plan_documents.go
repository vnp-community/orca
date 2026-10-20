package contracttest

// withPlanDocumentColumns adds the OpenSpec change table, which no earlier check listed, with the masked-secrets count of
// migration 0102, and the version history table of migration 0103 (CR-REQ-045).
func withPlanDocumentColumns(m map[string][]Column) map[string][]Column {
	n := func(name string) Column { return Column{Name: name} }
	y := func(name string) Column { return Column{Name: name, Nullable: true} }
	m["openspec_changes"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("change_id"), y("branch"), n("status"), n("tasks_sync_state"), y("tasks_sync_digest"),
		y("tasks_sync_error"), y("tasks_sync_next_retry_at"), y("commit_sha"), n("created_at"), n("updated_at"), n("version"),
		y("tasks_md_digest"), n("kg_available"), y("kg_snapshot"), y("kg_read_ids"), y("kg_gaps"), y("kg_conflicts"), y("skill_hash"),
		y("bundle_json"), y("workspace_id"), n("tasks_sync_attempts"), n("redacted_secrets"),
	}
	m["openspec_change_revisions"] = []Column{
		n("id"), n("tenant_id"), n("request_id"), n("revision"), n("run_id"), n("commit_sha"), n("tasks_md_digest"), n("bundle_digest"),
		n("files_json"), y("bundle_json"), n("created_at"),
	}
	return m
}

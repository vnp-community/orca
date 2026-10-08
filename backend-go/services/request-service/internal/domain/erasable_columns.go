package domain

// EraseMode says what anonymization writes into a free-text column.
type EraseMode string

const (
	EraseClearText       EraseMode = "text"       // ''
	EraseClearJSONArray  EraseMode = "json_array" // '[]'
	EraseClearJSONObject EraseMode = "json_obj"   // '{}'
	EraseNull            EraseMode = "null"
	EraseMarker          EraseMode = "marker" // '[erased]' so a list still shows the row
)

// ErasableColumn is one column that can hold customer content. Rows are found by KeyColumn = request id.
type ErasableColumn struct {
	Table     string
	Column    string
	KeyColumn string
	Mode      EraseMode
	// Parent is set for child tables without request_id: KeyColumn is then the foreign key to Parent.id
	// and Parent carries the request_id.
	Parent string
}

// ErasableColumns drives retention and EraseRequest. Tables of later waves (AI trace blobs, usage ledger)
// register their columns here; TestEveryTextColumnDeclaredErasableOrExempt fails until they do.
var ErasableColumns = []ErasableColumn{
	{"requests", "title", "id", EraseMarker, ""},
	{"requests", "body", "id", EraseClearText, ""},
	{"requests", "classification_reason", "id", EraseClearText, ""},
	{"requests", "return_reason", "id", EraseClearText, ""},
	{"requests", "source_url", "id", EraseClearText, ""},
	{"requests", "source_hints", "id", EraseNull, ""},
	{"request_type_history", "reason", "request_id", EraseClearText, ""},
	{"request_return_history", "reason", "request_id", EraseClearText, ""},
	{"solutions", "options", "request_id", EraseClearJSONObject, ""},
	{"analysis_runs", "raw_output", "request_id", EraseNull, ""},
	{"analysis_runs", "error_message", "request_id", EraseNull, ""},
	{"approvals", "comment", "request_id", EraseClearText, ""},
	{"context_packs", "body", "request_id", EraseClearText, ""},
	{"context_packs", "items", "request_id", EraseClearJSONArray, ""},
	{"context_packs", "missing", "request_id", EraseClearJSONArray, ""},
	{"evidence", "title", "request_id", EraseClearText, ""},
	{"evidence", "excerpt", "request_id", EraseClearText, ""},
	{"openspec_changes", "tasks_sync_error", "request_id", EraseNull, ""},
	{"requests", "acceptance_criteria", "id", EraseClearJSONArray, ""},
	{"requests", "type_fields", "id", EraseClearJSONObject, ""},
	{"analysis_runs", "feedback", "request_id", EraseNull, ""},
	{"request_revisions", "snapshot", "request_id", EraseClearJSONObject, ""},
	{"clarifications", "cancel_reason", "request_id", EraseClearText, ""},
	{"clarification_questions", "prompt", "clarification_id", EraseClearText, "clarifications"},
	// The reason column has a CHECK (reason <> ''), so it takes the marker instead of an empty string.
	{"clarification_questions", "reason", "clarification_id", EraseMarker, "clarifications"},
	{"clarification_questions", "options", "clarification_id", EraseNull, "clarifications"},
	{"clarification_questions", "suggested_default", "clarification_id", EraseNull, "clarifications"},
	{"clarification_questions", "answer", "clarification_id", EraseNull, "clarifications"},
	{"decisions", "question", "request_id", EraseClearText, ""},
	{"decisions", "options", "request_id", EraseClearJSONArray, ""},
	{"decisions", "recommendation_reason", "request_id", EraseClearText, ""},
	{"decisions", "rationale", "request_id", EraseClearText, ""},
	{"decision_history", "rationale", "decision_id", EraseClearText, "decisions"},
	{"task_run_outcomes", "error_message", "request_id", EraseClearText, ""},
	{"request_checks", "summary", "request_id", EraseClearText, ""},
	{"request_checks", "metrics", "request_id", EraseClearJSONObject, ""},
}

func exempt(reason string, columns ...string) map[string]string {
	m := make(map[string]string, len(columns))
	for _, c := range columns {
		m[c] = reason
	}
	return m
}

func mergeExempt(parts ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

// ExemptTextColumns are text-like columns that hold no customer content, each with the reason.
var ExemptTextColumns = mergeExempt(
	exempt("enum or state value",
		"analysis_runs.engine", "analysis_runs.kind", "analysis_runs.mode", "analysis_runs.status", "analysis_runs.error_code",
		"approvals.stage", "approvals.status", "approvals.subject_type", "approval_approvers.principal_kind",
		"approval_policies.request_type", "approval_policies.size", "approval_policies.urgency", "approval_policies.subject_type",
		"classification_runs.status", "classification_runs.error_code", "classification_runs.trigger_name",
		"context_packs.stage", "context_packs.cp_version", "context_sources.adapter", "context_sources.kind", "context_sources.status",
		"context_sources.transport", "context_sources.trust", "evidence.freshness", "evidence.trust",
		"openspec_changes.status", "openspec_changes.tasks_sync_state", "project_engine_settings.openspec_min_version",
		"project_engine_settings.solution_engine", "request_audit_outbox.actor_type", "request_audit_outbox.outcome",
		"request_links.reason", "request_return_history.action", "request_return_history.actor_kind", "request_return_history.category",
		"request_return_history.stage", "request_type_history.actor_kind", "request_type_history.from_type", "request_type_history.to_type",
		"requests.returned_category", "requests.returned_from_stage", "requests.size", "requests.solution_engine", "requests.status",
		"requests.type", "requests.type_source", "requests.urgency", "requests.source_provider"),
	exempt("opaque identifier, digest or lease owner",
		"analysis_runs.idempotency_key", "analysis_runs.lease_owner", "approval_approvers.principal_id", "approvals.decided_by",
		"approvals.idempotency_key", "approvals.requested_by", "approvals.subject_digest", "approvals.subject_id",
		"approval_policies.created_by", "classification_runs.lease_owner", "evidence.ref", "evidence.source_id",
		"openspec_changes.branch", "openspec_changes.change_id", "openspec_changes.commit_sha", "openspec_changes.tasks_sync_digest",
		"request_idempotency.source_provider", "request_idempotency.source_ref", "request_idempotency.source_site",
		"requests.source_ref", "requests.source_site", "context_sources.source_key"),
	exempt("tenant or project configuration, not Request content",
		"approval_policies.approvers", "context_sources.enabled_for", "context_sources.redaction", "context_sources.scopes"),
	exempt("ids and trust labels only, no content", "evidence.used_by"),
	exempt("enum or state value (artifact model, clarification, decision)",
		"analysis_runs.enforcement", "analysis_runs.repo_check", "artifact_index.kind", "artifact_relations.rel",
		"artifact_relations.from_kind", "artifact_relations.to_kind", "clarification_assignees.principal_kind",
		"clarification_questions.kind", "clarification_questions.answer_source", "clarifications.source", "clarifications.status",
		"clarifications.resume_status", "decision_history.action", "decisions.risk_level", "decisions.status", "decisions.subject_kind",
		"request_revisions.actor_kind", "request_revisions.cause", "solutions.kind", "solutions.status"),
	exempt("opaque identifier, key, digest, path or reference; no customer content",
		"artifact_index.display_id", "artifact_relations.from_id", "artifact_relations.to_id", "clarification_assignees.principal_id",
		"clarification_questions.answered_by", "clarification_questions.question_key", "clarification_questions.target_path",
		"clarifications.created_by", "clarifications.source_ref", "decision_history.actor_id", "decision_history.option_id",
		"decisions.chooser_id", "decisions.chosen_option_id", "decisions.confirmed_by", "decisions.recommended_option_id",
		"decisions.subject_digest", "decisions.subject_id", "request_coverage.ac_id", "request_coverage.check_id",
		"request_revisions.actor_id", "request_revisions.digest", "requests.content_digest", "solutions.content_digest",
		"solutions.content_ref", "tenant_settings.updated_by"),
	exempt("enum, reason code or lease owner of the execution loop; no customer content",
		"request_checks.kind", "request_checks.source", "request_checks.status", "task_run_outcomes.cause", "task_run_outcomes.outcome",
		"execution_reconcile_state.lease_owner"),
	exempt("provenance has no content or credential (ParseProvenance refuses unknown members)", "solutions.provenance"),
	exempt("event envelope; payloads carry ids and counts, never content (outbox payload golden tests)", "outbox_events.payload", "outbox_events.subject", "processed_events.subject"),
	exempt("security bookkeeping: ids, names and counts, never content (MarshalAuditMetadata)",
		"request_audit_outbox.action", "request_audit_outbox.actor_id", "request_audit_outbox.ip_address", "request_audit_outbox.metadata_json",
		"request_audit_outbox.target_id", "request_audit_outbox.target_type", "request_webhook_nonces.source"),
)

// MySQLOnlyExemptTextColumns are uuid and generated key columns that MySQL stores as VARCHAR where Postgres uses uuid.
var MySQLOnlyExemptTextColumns = exempt("opaque identifier or generated key (VARCHAR in MySQL, uuid in Postgres)",
	"analysis_runs.id", "analysis_runs.request_id", "analysis_runs.tenant_id", "analysis_runs.active_key",
	"openspec_changes.id", "openspec_changes.tenant_id", "openspec_changes.request_id", "approvals.pending_key",
	"project_engine_settings.project_id", "project_engine_settings.tenant_id", "project_engine_settings.updated_by",
	"clarifications.open_key", "decisions.live_key")

// IsExemptTextColumn tells whether table.column holds no customer content.
func IsExemptTextColumn(column string) bool {
	if _, ok := ExemptTextColumns[column]; ok {
		return true
	}
	_, ok := MySQLOnlyExemptTextColumns[column]
	return ok
}

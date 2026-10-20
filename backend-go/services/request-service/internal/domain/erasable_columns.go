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
	{"openspec_changes", "bundle_json", "request_id", EraseNull, ""},
	{"openspec_changes", "kg_gaps", "request_id", EraseNull, ""},
	{"openspec_changes", "kg_conflicts", "request_id", EraseNull, ""},
	{"openspec_change_revisions", "bundle_json", "request_id", EraseNull, ""},
	{"openspec_change_revisions", "files_json", "request_id", EraseClearJSONArray, ""},
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
	// The review model quotes acceptance criteria and plan task titles, so it is customer content.
	{"outcome_reviews", "model", "request_id", EraseClearJSONObject, ""},
	// A decision note and an approval note are written by a person about the content of the review.
	{"outcome_review_decisions", "note", "request_id", EraseClearText, ""},
	{"outcome_reviews", "approval_note", "request_id", EraseClearText, ""},
	// The cached design and context results quote the same text as the review model.
	{"outcome_review_contexts", "payload", "request_id", EraseClearJSONObject, ""},
	// A drawing shows the labels of the review model, so its content, text and model are cleared with the Request.
	{"review_artifact_blobs", "content", "request_id", EraseClearText, ""},
	{"review_artifacts", "title", "request_id", EraseClearText, ""},
	{"review_artifacts", "description", "request_id", EraseClearText, ""},
	{"review_artifacts", "reason", "request_id", EraseClearText, ""},
	{"review_artifacts", "model", "request_id", EraseClearJSONObject, ""},
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
		"requests.type", "requests.type_source", "requests.urgency", "requests.source_provider",
		"requests.execution_mode", "project_engine_settings.execution_mode", "phase_starts.start_trigger"),
	exempt("opaque identifier, digest or lease owner",
		"analysis_runs.idempotency_key", "analysis_runs.lease_owner", "approval_approvers.principal_id", "approvals.decided_by",
		"approvals.idempotency_key", "approvals.requested_by", "approvals.subject_digest", "approvals.subject_id",
		"approval_policies.created_by", "classification_runs.lease_owner", "evidence.ref", "evidence.source_id",
		"openspec_changes.branch", "openspec_changes.change_id", "openspec_changes.commit_sha", "openspec_changes.tasks_sync_digest",
		"openspec_changes.tasks_md_digest", "openspec_changes.kg_snapshot", "openspec_changes.kg_read_ids", "openspec_changes.skill_hash",
		"openspec_changes.workspace_id", "openspec_change_revisions.commit_sha", "openspec_change_revisions.tasks_md_digest",
		"openspec_change_revisions.bundle_digest",
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
		"request_revisions.actor_id", "request_revisions.digest", "requests.base_ref", "requests.content_digest", "solutions.content_digest",
		"solutions.content_ref", "tenant_settings.updated_by", "plan_auto_jobs.anchor"),
	exempt("enum, reason code or lease owner of the execution loop; no customer content",
		"request_checks.kind", "request_checks.source", "request_checks.status", "task_run_outcomes.cause", "task_run_outcomes.outcome",
		"execution_reconcile_state.lease_owner", "plan_auto_jobs.origin", "plan_auto_jobs.state", "plan_auto_jobs.reason_code",
		"plan_auto_jobs.lease_owner"),
	exempt("enum, key, digest or reference of an outcome review or its agent run; the content is in outcome_reviews.model",
		"outcome_reviews.phase", "outcome_reviews.state", "outcome_reviews.head_key", "outcome_reviews.head_commit",
		"outcome_reviews.base_commit", "outcome_reviews.inputs_digest", "outcome_reviews.kg_snapshot", "outcome_reviews.design_digest",
		"outcome_reviews.context_digest", "outcome_review_runs.kind", "outcome_review_runs.status", "outcome_review_runs.scope",
		"outcome_review_runs.agent_run_ref", "outcome_review_runs.cost_estimate", "outcome_review_runs.cost_actual",
		"outcome_review_runs.error_code", "outcome_review_runs.active_key", "outcome_review_contexts.kind", "outcome_review_contexts.digest",
		"outcome_review_contexts.prompt_version",
		"outcome_review_decisions.item_key", "outcome_review_decisions.decision", "outcome_review_decisions.active_key", "outcome_review_runs.reason", "outcome_review_runs.client_key",
		"review_drawing_profiles.project_key", "review_drawing_profiles.spec", "review_drawing_profiles.spec_digest",
		"review_artifact_blobs.digest", "review_artifact_blobs.media_type", "review_artifacts.section", "review_artifacts.variant", "review_artifacts.name",
		"review_artifacts.diagram_type", "review_artifacts.input_digest", "review_artifacts.preset_digest", "review_artifacts.skill_version",
		"review_artifacts.state", "review_artifacts.origin", "review_artifacts.error_code", "review_artifacts.blob_digest", "review_artifacts.view_box",
		"review_artifacts.self_check", "review_artifacts.active_key", "project_engine_settings.outcome_review"),
	exempt("provenance has no content or credential (ParseProvenance refuses unknown members)", "solutions.provenance"),
	exempt("tool names and counts and usage figures of an agent run; never what went into or came out of a tool", "analysis_runs.run_stats"),
	exempt("event envelope; payloads carry ids and counts, never content (outbox payload golden tests)", "outbox_events.payload", "outbox_events.subject", "processed_events.subject"),
	exempt("security bookkeeping: ids, names and counts, never content (MarshalAuditMetadata)",
		"request_audit_outbox.action", "request_audit_outbox.actor_id", "request_audit_outbox.ip_address", "request_audit_outbox.metadata_json",
		"request_audit_outbox.target_id", "request_audit_outbox.target_type", "request_webhook_nonces.source"),
)

// MySQLOnlyExemptTextColumns are uuid and generated key columns that MySQL stores as VARCHAR where Postgres uses uuid.
var MySQLOnlyExemptTextColumns = exempt("opaque identifier or generated key (VARCHAR in MySQL, uuid in Postgres)",
	"analysis_runs.id", "analysis_runs.request_id", "analysis_runs.tenant_id", "analysis_runs.active_key",
	"openspec_changes.id", "openspec_changes.tenant_id", "openspec_changes.request_id", "approvals.pending_key",
	"openspec_change_revisions.id", "openspec_change_revisions.tenant_id", "openspec_change_revisions.request_id", "openspec_change_revisions.run_id",
	"project_engine_settings.project_id", "project_engine_settings.tenant_id", "project_engine_settings.updated_by",
	"clarifications.open_key", "decisions.live_key", "plan_auto_jobs.tenant_id", "plan_auto_jobs.request_id", "plan_auto_jobs.project_id",
	"plan_auto_jobs.run_id", "plan_auto_jobs.requested_by", "plan_auto_jobs.active_key")

// IsExemptTextColumn tells whether table.column holds no customer content.
func IsExemptTextColumn(column string) bool {
	if _, ok := ExemptTextColumns[column]; ok {
		return true
	}
	_, ok := MySQLOnlyExemptTextColumns[column]
	return ok
}

package postgres

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const requestColumns = `id, tenant_id, project_id, number, title, body, source_provider, source_ref, source_url, source_site,
	type, type_source, size, urgency, confidence, classification_reason, status, returned_from_stage, return_reason,
	plan_task_id, reporter_id, created_at, updated_at, version, solution_engine, returned_category, source_hints, classification_attempts,
	content_schema_version, acceptance_criteria, type_fields, content_revision, content_digest`

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func scanRequest(row pgx.Row) (domain.Request, error) {
	var (
		r                                                            domain.Request
		project, typ, typeSource, size, returned, plan, se, category *string
		provider, urgency, status                                    string
		hints                                                        []byte
		created, updated                                             time.Time
	)
	err := row.Scan(&r.ID, &r.TenantID, &project, &r.Number, &r.Title, &r.Body, &provider, &r.SourceRef, &r.SourceURL, &r.SourceSite,
		&typ, &typeSource, &size, &urgency, &r.Confidence, &r.ClassificationReason, &status, &returned, &r.ReturnReason,
		&plan, &r.ReporterID, &created, &updated, &r.Version, &se, &category, &hints, &r.ClassificationAttempts,
		&r.ContentSchemaVersion, &r.AcceptanceCriteriaJSON, &r.TypeFieldsJSON, &r.ContentRevision, &r.ContentDigest)
	if err != nil {
		return domain.Request{}, err
	}
	r.ProjectID = derefString(project)
	r.SourceProvider = domain.SourceProvider(provider)
	r.Type = domain.RequestType(derefString(typ))
	r.TypeSource = domain.TypeSource(derefString(typeSource))
	r.Size = domain.RequestSize(derefString(size))
	r.Urgency = domain.Urgency(urgency)
	r.Status = domain.RequestStatus(status)
	r.ReturnedFromStage = domain.ReturnStage(derefString(returned))
	r.PlanTaskID = derefString(plan)
	r.ReturnedCategory = domain.ReturnCategory(derefString(category))
	if se != nil {
		name := domain.EngineName(*se)
		r.SolutionEngine = &name
	}
	if len(hints) > 0 {
		if err := json.Unmarshal(hints, &r.SourceHints); err != nil {
			return domain.Request{}, err
		}
	}
	r.CreatedAt = created.UTC()
	r.UpdatedAt = updated.UTC()
	return r, nil
}

// requestArgs lists the mutable columns in the order used by INSERT and UPDATE.
func requestArgs(r domain.Request) []any {
	var se any
	if r.SolutionEngine != nil {
		se = string(*r.SolutionEngine)
	}
	return []any{
		nullIfEmpty(r.ProjectID), r.Title, r.Body, string(r.SourceProvider), r.SourceRef, r.SourceURL, r.SourceSite,
		nullIfEmpty(string(r.Type)), nullIfEmpty(string(r.TypeSource)), nullIfEmpty(string(r.Size)), string(r.Urgency),
		r.Confidence, r.ClassificationReason, string(r.Status), nullIfEmpty(string(r.ReturnedFromStage)), r.ReturnReason,
		nullIfEmpty(r.PlanTaskID), r.ReporterID, se, nullIfEmpty(string(r.ReturnedCategory)), hintsArg(r.SourceHints), r.ClassificationAttempts,
	}
}

// hintsArg stores NULL for empty hints so "never set" and "set to nothing" read the same.
func hintsArg(h domain.SourceHints) any {
	if h.IsZero() {
		return nil
	}
	b, _ := json.Marshal(h)
	return string(b)
}

// contentArgs lists the content columns for INSERT; a request built without NewRequest gets the column defaults.
func contentArgs(r domain.Request) []any {
	ac, tf := string(r.AcceptanceCriteriaJSON), string(r.TypeFieldsJSON)
	if ac == "" {
		ac = "[]"
	}
	if tf == "" {
		tf = "{}"
	}
	return []any{max(r.ContentSchemaVersion, 1), ac, tf, max(r.ContentRevision, 1), r.ContentDigest}
}

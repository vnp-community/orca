# TASK-WF-005-03: `SearchTemplates`, `RateTemplate`, `usage_count` increment, "Import to My Workflows"

**From Solution:** BE-SOL-005
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`SearchTemplates`, `RateTemplate` RPCs), `backend-go/services/workflow-service/internal/usecase/search_templates.go`, `rate_template.go` (new), `backend-go/services/workflow-service/internal/usecase/execute.go` (usage_count increment), `backend-go/services/workflow-service/internal/adapter/postgres/repository.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
**Depends on:** TASK-WF-005-02 (needs the `tags`/`visibility`/`rating_sum`/`rating_count`/`usage_count` columns and widened `WorkflowTemplate` that task adds), TASK-WF-004-02 (`CloneTemplate` — "Import to My Workflows" reuses it, no new import mechanism)
**Status:** `[ ]` TODO

---

## Context

`Execute` usecase (`internal/usecase/execute.go`, full file read
directly) is where `usage_count` must increment — confirmed this is a
new addition, no such increment exists today (the whole `usage_count`
column doesn't exist before TASK-WF-005-02 adds it). BE-SOL-005 is
explicit this must happen "inside `Execute`'s existing usecase (one line,
at the point an execution actually starts) — not on
`ListTemplates`/preview, which must never inflate the counter." Confirmed
via direct read of `ListTemplates`' usecase and `ResolveTemplate`'s
usecase: neither should be touched by this task.

No `SearchTemplates`/`RateTemplate`/`GenerateShareLink`/`UpdateVisibility`
RPC exists anywhere today (confirmed via direct grep across
`workflow-service`, `proto/orca/workflow/`) — `SearchTemplates`/
`RateTemplate` are this task's job; the other two are TASK-WF-005-02's.

`ListTemplates` (`internal/usecase/ports.go:23-27`,
`TemplateRepository.ListTemplates(ctx, tenantID, scope, pageToken string,
pageSize int32)`) is the closest existing precedent for
`SearchTemplates`' pagination shape — reuse its keyset-pagination
convention rather than inventing a new one.

## Changes to make

**1. `workflow.proto`**:

```protobuf
service WorkflowService {
  ...
  rpc SearchTemplates(SearchTemplatesRequest) returns (SearchTemplatesResponse);
  rpc RateTemplate(RateTemplateRequest) returns (google.protobuf.Empty);
}

message SearchTemplatesRequest {
  string query = 1;       // matched against name/description substring
  repeated string tags = 2;
  string sort = 3;         // "trending" (usage_count desc) | "recent" (created_at desc)
  string page_token = 4;
  int32 page_size = 5;
}
message SearchTemplatesResponse {
  repeated WorkflowTemplate templates = 1;
  string next_page_token = 2;
}

message RateTemplateRequest {
  string template_id = 1;
  int32 rating = 2; // 1-5, validate range in the usecase
}
```

**2. `internal/usecase/search_templates.go`** (new) — tenant-scoped
(searches only templates visible to the caller's tenant plus
`visibility=public` templates from any tenant — confirm the exact
cross-tenant visibility rule against BE-SOL-005/CR-WF-005's intent before
implementing, since this is the one place a search legitimately crosses
tenant boundaries by design):

```go
type SearchTemplatesInput struct {
	Query, Sort, PageToken string
	Tags                   []string
	PageSize               int32
}

func (uc *SearchTemplates) Execute(ctx context.Context, in SearchTemplatesInput) (SearchTemplatesOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	...
	// repo query: name/description ILIKE, tags && $tags (array overlap),
	// visibility IN ('public') OR tenant_id = $tenantID, ORDER BY
	// usage_count DESC or created_at DESC per in.Sort.
}
```

**3. `internal/usecase/rate_template.go`** (new):

```go
func (uc *RateTemplate) Execute(ctx context.Context, in RateTemplateInput) error {
	if in.Rating < 1 || in.Rating > 5 {
		return apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_RATING", "rating must be between 1 and 5", nil)
	}
	// repo: UPDATE workflow.templates SET rating_sum = rating_sum + $rating,
	// rating_count = rating_count + 1 WHERE id = $templateId — an atomic
	// increment, not a read-modify-write, to avoid a lost-update race
	// between two concurrent raters.
	...
}
```

Average rating (`rating_sum / rating_count`) is computed at read time
(e.g. in `toProtoTemplate`), not stored — avoids a second source of truth
that can drift from the two counters.

**4. `internal/usecase/execute.go`** — add the `usage_count` increment.
Confirmed exact insertion point: inside `Execute.Execute`, after the DAG
is validated/waves built but before (or alongside) `CreateExecution`'s
call — re-read `execute.go`'s full body at implementation time to find
the precise line, since BE-SOL-005 only specifies "at the point an
execution actually starts," not an exact line number. Add a repository
method (e.g. `TemplateRepository.IncrementUsageCount(ctx, tenantID,
templateID string) error`) called once per `Execute` call — confirm it is
NOT called from `ExecuteAdHocStep` (a synthetic single-step run against a
step config, not a template-driven execution) since that path has no
`template_id` to attribute usage to.

**5. `internal/adapter/postgres/repository.go`** — implement
`SearchTemplates`' query, `IncrementUsageCount` (atomic `UPDATE ...
SET usage_count = usage_count + 1`), and `RateTemplate`'s atomic
increments.

**6. "Import to My Workflows"** — no new mechanism. The frontend flow is:
resolve a `share_token` via `GetTemplateByShareToken`
(TASK-WF-005-02) to get the source template's ID, then call
`CloneTemplate(source_template_id)` (TASK-WF-004-02) — this task adds NO
backend code for import itself, only confirms (via an integration test)
that the two RPCs compose correctly end to end: share-token resolution →
clone → the clone is a fully independent template the importing tenant
owns.

**7. `api-gateway`'s `channels_workflow.go`** — add
`workflow.template.search` and `workflow.template.rate`, following
`workflow.template.list`'s pattern for the former and
`workflow.hasActiveExecutions`'s simple-mutation pattern for the latter.

## Test plan

- `SearchTemplates` matches by name/description substring and by tag;
  sorts correctly by `usage_count` (trending) and `created_at` (recent).
- `usage_count` increments exactly once per real `Execute` call, zero
  times on `ListTemplates`/`ResolveTemplate`/preview calls — an explicit
  regression test asserting the counter is untouched by list/preview
  paths, matching BE-SOL-005's own test-plan item.
- `RateTemplate` with an out-of-range rating (0, 6) is rejected; two
  concurrent `RateTemplate` calls against the same template both land
  (no lost update) — a `-race`-run concurrency test.
- Import-from-share-link (`GetTemplateByShareToken` → `CloneTemplate`)
  produces an independent clone; mutating the source template afterward
  does not affect the imported clone.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run "TestSearchTemplates|TestRateTemplate|TestExecute_UsageCount" -race -v
go test ./services/workflow-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; search/sort cases pass; usage-count isolation
test (list/preview never increments) passes; concurrent-rating test
passes under `-race`; import-via-clone end-to-end test passes.

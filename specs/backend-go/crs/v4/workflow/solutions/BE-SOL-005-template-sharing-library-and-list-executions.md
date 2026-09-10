# BE-SOL-005: Template sharing/library schema+RPCs + `ListExecutions` fix

**Resolves:** [CR-WF-005](../../../../../../docs/crs/v4/workflow/CR-WF-005-template-sharing-library-and-list-executions.md)
**Service:** `workflow-service` + `api-gateway` (wscompat channel)
**Depends on:** [BE-SOL-004](./BE-SOL-004-template-inheritance-merge-and-clone.md) ("Import to My Workflows" reuses Clone)
**Affected files (proposed):**
- `backend-go/services/workflow-service/internal/domain/template.go` (new fields)
- `backend-go/proto/orca/workflow/v1/workflow.proto` (`ListExecutions`, sharing RPCs, widened `WorkflowTemplate` message)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go` (`listExecutions` channel — currently missing)
- `backend-go/services/workflow-service/migrations/0007_template_sharing_and_library.up.sql` (new — next free number after `0006_template_version`)
**Status:** 📋 Proposed — not yet implemented

---

## Current state — two independently confirmed gaps, one more urgent than the other

**1. `workflow.listExecutions` — RPC does not exist, frontend is calling
into nothing.** Confirmed directly:

```
$ grep -n "rpc " workflow.proto        → 11 RPCs, GetExecution (singular) but no ListExecutions
$ grep -noE "'workflow\.[a-zA-Z.]+'" channels_workflow.go → exactly 11 channels registered,
    listExecutions is not one of them (execute, cancel, template.create,
    template.update, getExecution, pause, resume, template.list,
    template.resolve, hasActiveExecutions, executeAdHocStep)
```

`WorkflowMonitor.tsx` (the one workflow component actually mounted in the
app, per `WorkspaceLayout.tsx`) calls `workflow.listExecutions` — this
means the Workflow tab **fails to load for every user on backend-go**
today. This is worth shipping as a fast-follow ahead of the rest of this
solution if the team needs it fixed sooner (see §Sequencing).

**2. Sharing/library — confirmed absent from both code and the TDD.**
`WorkflowTemplate` (`template.go:56-77`) has exactly `ID, TenantID, Name,
DAGJSON, Scope, ParentTemplateID, Version` — no `owner_id`, `description`,
`tags`, `visibility`, `share_token`, `rating`, `usage_count`. Migration
history (`0001_init` through `0006_template_version`) confirms none of
these were ever added. **Even `workflow-service.md` §5's own intended
schema doesn't include them** — only `description`/`owner_id` beyond
what's built. This is a genuine net-new design, not a "finish what the TDD
sketched" change — flagged per this series' convention for anything beyond
literal TDD text.

## Design — `ListExecutions` (fast-follow candidate)

```protobuf
rpc ListExecutions(ListExecutionsRequest) returns (ListExecutionsResponse);
message ListExecutionsRequest { string project_id = 1; int32 limit = 2; string cursor = 3; }
```

Wire into `channels_workflow.go` following the exact pattern the other 11
channels already use — no new pattern.

## Design — schema widening

```sql
-- 0007_template_sharing_and_library.up.sql
ALTER TABLE workflow.templates
  ADD COLUMN owner_id      TEXT,
  ADD COLUMN description   TEXT,
  ADD COLUMN tags          TEXT[] DEFAULT '{}',
  ADD COLUMN visibility    TEXT NOT NULL DEFAULT 'private', -- private|team|company|public
  ADD COLUMN share_token   TEXT,
  ADD COLUMN rating_sum    INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count  INT NOT NULL DEFAULT 0,
  ADD COLUMN usage_count   INT NOT NULL DEFAULT 0;
```

## Design — RPCs

```protobuf
rpc UpdateVisibility(UpdateVisibilityRequest) returns (google.protobuf.Empty);
rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
rpc GetTemplateByShareToken(GetTemplateByShareTokenRequest) returns (GetTemplateByShareTokenResponse); // unauthenticated, narrow projection — see security note below
rpc RateTemplate(RateTemplateRequest) returns (google.protobuf.Empty);
rpc SearchTemplates(SearchTemplatesRequest) returns (SearchTemplatesResponse); // text + tag filter, sort trending (usage_count) | recent (created_at)
```

`usage_count` increments inside `Execute`'s existing usecase (one line, at
the point an execution actually starts) — not on `ListTemplates`/preview,
which must never inflate the counter.

**Security note, same standard as [BE-SOL-003](../../task-graph/solutions/BE-SOL-003-task-access-control-team-scope-and-sharing.md)'s
task share-link:** `GetTemplateByShareToken` bypasses tenant-scoped access
by design (public) — its response message must be a dedicated narrow
projection (`TemplateShareView`), never the full `WorkflowTemplate`, and
needs a security review pass before merge.

## Design — "Import to My Workflows" reuses Clone, no new import mechanism

```go
// GetTemplateByShareToken's caller-side flow: resolve share_token -> template metadata,
// then call CloneTemplate(source_template_id) — BE-SOL-004's existing RPC, not duplicated here.
```

## Test plan

- `ListExecutions` returns real data, paginated; `WorkflowMonitor.tsx`
  loads without error against a real backend-go target (manual/E2E check,
  not just a unit test — this bug was invisible to unit tests before).
- `SearchTemplates` matches by name/description substring and by tag;
  sorts correctly by each of `usage_count`/`created_at`.
- `usage_count` increments exactly once per real execution start, zero
  times on list/preview.
- `GetTemplateByShareToken` response asserted to exclude every field not on
  `TemplateShareView` — regression test that fails loudly on a future
  `WorkflowTemplate` field addition forgotten in the share view.
- Import-from-share-link produces an independent Clone (source edits don't
  propagate).

## Sequencing note

Given `WorkflowMonitor.tsx` is broken in production today for any
backend-go target, consider splitting `ListExecutions` into its own
fast-follow PR ahead of the rest of this solution's sharing/library scope,
which is larger and needs more review (security pass on the share-link
path).

## Not in scope (per the CR)

- UI (Library browse, share dialog) — [FE-SOL-001](../../../../../frontend/crs/v4/workflow/solutions/FE-SOL-001-frontend-builder-library-pause-resume.md).
- `visibility=company` admin-approval workflow, if the product wants one —
  not designed here, flagged as an open product question.
- Re-mapping `CR-TRACE-017`'s `workflowShareFlow` tracer to real backend-go
  files — small follow-up once this solution ships, not part of it.

## References

- [CR-WF-005](../../../../../../docs/crs/v4/workflow/CR-WF-005-template-sharing-library-and-list-executions.md)
- `backend-go/services/workflow-service/internal/domain/template.go:56-77`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`

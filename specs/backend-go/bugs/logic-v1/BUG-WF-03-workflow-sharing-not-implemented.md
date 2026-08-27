# BUG-WF-03: Workflow sharing, approval, share-link import, and library search/rating are all unimplemented

**Business Logic:** [BL-WF-03](../../../../docs/logic/workflow-orchestration/BL-WF-03-workflow-sharing.md) — Workflow Sharing & Library Discovery
**Priority (per spec):** P1
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** There is no way for a user to change a template's visibility, request/approve a company-wide publish, generate a public share link, preview or import someone else's shared workflow, search the library by text/tags, sort by trending, or see a rating/usage count — none of this data or any of these operations exist in backend-go at all.

---

## Spec summary

BL-WF-03 covers: visibility escalation (private → team → company → public) with a lead-requires-admin-approval gate for company scope, public share-link generation + token-based read-only preview + one-click import to the visitor's own personal workflows, full-text/tag search plus trending/recent sort in the library, 1-5 star rating display, and usage-count incrementing on every execution.

## What backend-go has

Nothing. Checked every layer:

- **Schema**: `workflow.templates` has only `id, tenant_id, name, dag_json, scope, parent_template_id, version, created_at, updated_at` — no `visibility`, `owner_id`, `share_token`, `rating`, or `usage_count` column in any migration (`backend-go/services/workflow-service/migrations/0001_init.up.sql`, `0003_template_parent_chain.up.sql`, `0006_template_version.up.sql` — the only three migrations that ever touch `workflow.templates`).
- **Proto**: `WorkflowTemplate` message has `id, tenant_id, name, dag_json, scope, parent_template_id, version` only (`backend-go/proto/orca/workflow/v1/workflow.proto:59-66`) — no visibility/share/rating/usage fields, and `WorkflowService` has no approval-related RPC.
- **Usecases**: `ListTemplates` (`backend-go/services/workflow-service/internal/usecase/list_templates.go`) filters by `scope` only — no text search, no tag filter, no sort-by-trending/recent option (spec's `searchWorkflows` needs both). No `orca_workflow_approvals`-equivalent table, repository, or usecase exists anywhere in `workflow-service` or `orchestration-service` (grep for `approval`/`Approval`/`visibility`/`share_token`/`usage_count`/`rating` across `services/workflow-service` returns zero matches).
- **REST/WS**: `workflow_routes.go` exposes only `POST /templates`, `GET /templates`, `GET /templates/resolve`, plus the execution endpoints (`create/list/resolve` only — `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:27-29`). No share-link, approval, import, search, or rating endpoint exists. `wscompat` registers zero `workflow.*` channels at all (see BUG-030).

## What's missing

- Visibility field/state machine (private → team → company → public) — no column, no usecase.
- Admin-approval flow for company-scope publish when requester is a lead (`orca_workflow_approvals` table equivalent) — no table, no usecase, no RPC.
- Public share-link generation (`share_token`) and token-based read-only preview endpoint for anonymous/external visitors.
- "Import to My Workflows" flow that creates a new personal-scope template from a shared one (with either a `parent_template` reference or a full clone — this also depends on BUG-WF-01's missing Clone mode).
- Library search: no text search (name/description/tags), no tag filter — `ListTemplates` supports scope filtering only.
- Sort by `trending` (usage_count desc, rating desc) or `recent` (updated_at desc) — `ListTemplates` has no sort parameter at all; its only ordering is whatever the keyset-pagination query imposes (by id, implicitly).
- Rating (1-5 stars) — no field, no submit-rating usecase, no average-rating computation.
- Usage-count increment on every execution — `Execute`'s usecase (`execute.go`) has no counter-increment call anywhere in its flow.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` — the narrower "wscompat wrapper missing" finding for the 3-4 RPCs that do exist (`execute`/`cancel`/`template.create`); this bug is about the much larger set of RPCs BL-WF-03 needs that were never built server-side at all, so there is nothing for a wscompat wrapper to even wrap.
- This bug and `BUG-WF-01-workflow-template-sharing-fields-missing.md` share the same root cause: `workflow.templates`' schema was deliberately narrowed at scaffold time to `id/tenant_id/name/dag_json/scope` (+ later `parent_template_id`/`version`) and visibility/sharing/rating columns were never added in any subsequent migration.

## References

- `backend-go/docs/logic/workflow-orchestration/BL-WF-03-workflow-sharing.md` — spec
- `backend-go/services/workflow-service/migrations/0001_init.up.sql`, `0003_template_parent_chain.up.sql`, `0006_template_version.up.sql` — full column history of `workflow.templates` (no visibility/share/rating/usage columns ever added)
- `backend-go/proto/orca/workflow/v1/workflow.proto:59-66` — `WorkflowTemplate` message, full field list
- `backend-go/services/workflow-service/internal/usecase/list_templates.go:11-53` — `ListTemplates`, scope-filter only, no text/tag search or sort option
- `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:27-29` — full `/v1/workflows/templates*` REST surface (create/list/resolve only)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — no `workflow.*` registrations

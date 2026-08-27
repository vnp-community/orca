# BUG-WF-01: Template inheritance is real but scoped merge, clone mode, publish/share, and version-bump-on-breaking-change are all missing

**Business Logic:** [BL-WF-01](../../../../docs/logic/workflow-orchestration/BL-WF-01-workflow-template.md) — Workflow Template Management (Create/Inherit/Share)
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** High
**Symptom:** A user can create a template and make a child template that references a parent via `parent_template_id`, but they cannot: set a description/tags, clone a template as a disconnected copy, override specific parent fields (`overrides`/`inject_steps`/`remove_steps`), change visibility (private/team/company/public), get a share link, have a company-scope publish go through admin approval, or see usage/rating stats — none of that data exists in the schema at all.

---

## Spec summary

BL-WF-01 describes full template lifecycle management: create with `name/description/tags/scope/visibility`, a visual/YAML editor with Zod-style validation, two distinct inheritance modes (**Clone** = disconnected copy, **Inherit** = live parent reference with `overrides`/`inject_steps`/`remove_steps` resolved via `resolveTemplate()`'s `deepMerge`), a publish/share flow with visibility escalation (private → team → company → public), admin approval for company-scope publishing, rating (1-5 stars) + `usage_count` tracking, and automatic minor-version bumps when a template with active usage receives a breaking change.

## What backend-go has

- `WorkflowService.CreateTemplate`/`UpdateTemplate`/`ListTemplates`/`ResolveTemplate` RPCs — `backend-go/proto/orca/workflow/v1/workflow.proto:12-49`
- `domain.NewWorkflowTemplate` validates tenant/name/scope and rejects a template naming itself as its own parent — `backend-go/services/workflow-service/internal/domain/template.go:82-113`
- DAG structural validation (unique step IDs, no self-reference, dangling `dependsOn`, full cycle detection via Kahn's algorithm) — `backend-go/services/workflow-service/internal/domain/dag.go:76-146`
- Parent-chain inheritance: `parent_template_id` column + `ResolveChain` recursive CTE (depth ≤ 5) — `backend-go/services/workflow-service/migrations/0003_template_parent_chain.up.sql`, `backend-go/services/workflow-service/internal/usecase/resolve_template.go:47-107`
- `UpdateTemplate` bumps `templates.version` on every write with optimistic-concurrency (`expected_version`) and re-validates multi-hop parent cycles — `backend-go/services/workflow-service/internal/usecase/update_template.go:26-71`, schema in `backend-go/services/workflow-service/migrations/0006_template_version.up.sql`
- REST wiring for create/list/resolve — `backend-go/services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:27-29,51-129`

## What's missing

- **No `owner_id`, `description`, `tags`, or `visibility` columns at all** on `workflow.templates` — the table has only `id, tenant_id, name, dag_json, scope, parent_template_id, version, created_at, updated_at` (`backend-go/services/workflow-service/migrations/0001_init.up.sql`, confirmed by every later migration only adding `parent_template_id`/`version`). The entire "Publish & Share" flow (private → team → company → public, share-link generation, admin approval, `orca_workflow_approvals`) has no data model to sit on.
- **No Clone mode.** Only one creation path exists (`CreateTemplate`), and it only supports the Inherit shape (`parent_template_id` reference). There is no distinct "copy, no parent link" endpoint/flag, and no client-side deep-copy semantics documented or enforced server-side.
- **`resolveTemplate()` does not do `overrides`/`inject_steps`/`remove_steps` deep-merge.** The spec's inheritance model is field-level overrides + step injection + step removal merged onto the parent. Backend-go's actual policy (`resolveEffectiveTemplate`, `resolve_template.go:88-107`) is coarser: walk the chain and return the **entire DAG of the nearest ancestor that has any steps** — an all-or-nothing swap, not a merge. There is no `overrides` map, no `inject_steps`, no `remove_steps` concept anywhere in `domain.WorkflowTemplate`, `domain.Step`, or the proto.
- **No rating or usage_count tracking.** No column, no increment-on-run usecase, no rating RPC/endpoint.
- **No admin-approval workflow** for company-scope publishing — no `orca_workflow_approvals`-equivalent table or usecase.
- **Version bump policy diverges from spec.** Spec: bump only on breaking change to a template with active usage, and existing executions keep running the old version. Backend-go: bump on **every** `UpdateTemplate` call unconditionally (`update_template.go:38,61` — always constructs a new template row and calls `Update`), with no breaking-change detection and no distinction based on whether `usage_count > 0`. (Executions do freeze their own DAG snapshot at `Execute` time per the RPC doc comment, so that half of the isolation guarantee holds by different means — see `workflow.proto:16-20`.)
- **YAML editor / Zod-style schema validation partially covered**: step-ID uniqueness and cycle detection exist (`dag.go`), but there is no "server refs format valid" / "provider refs format valid" validation — because there's no server/provider reference concept in a step's config at all (see BUG-WF-02).

## See also

- `specs/backend-go/bugs/missing-v1/BUG-030-workflow-channels-not-implemented.md` — `workflow.template.create`/`workflow.execute`/`workflow.cancel` REST RPCs exist but have no `wscompat` WS wrapper yet; `workflow.template.update` has a real `UpdateTemplate` RPC now (this bug's write predates that; re-verify before reusing its "no backing RPC" claim for `template.update`).

## References

- `backend-go/docs/logic/workflow-orchestration/BL-WF-01-workflow-template.md` — spec
- `backend-go/proto/orca/workflow/v1/workflow.proto:59-66` — `WorkflowTemplate` message (no owner_id/description/tags/visibility fields)
- `backend-go/services/workflow-service/internal/domain/template.go:60-77` — `WorkflowTemplate` struct
- `backend-go/services/workflow-service/migrations/0001_init.up.sql` — original schema, only `id/tenant_id/name/dag_json/scope`
- `backend-go/services/workflow-service/migrations/0003_template_parent_chain.up.sql`, `0006_template_version.up.sql` — the only two columns ever added
- `backend-go/services/workflow-service/internal/usecase/resolve_template.go:88-107` — `resolveEffectiveTemplate`'s "nearest non-empty ancestor wins" policy (not a field-level deepMerge)
- `backend-go/services/workflow-service/internal/usecase/update_template.go:26-71` — unconditional version bump on every update
- `backend-go/services/workflow-service/internal/usecase/create_template.go:43-72` — the only creation path (no clone variant)
- `backend-go/services/workflow-service/README.md:75-76` — confirms `StreamExecutionEvents` and other §3 design-doc scope was deliberately trimmed from this scaffold

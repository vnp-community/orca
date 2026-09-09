# TASK-WF-004-02: `CloneTemplate` RPC

**From Solution:** BE-SOL-004
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`CloneTemplate` RPC), `backend-go/services/workflow-service/internal/usecase/clone_template.go` (new), `backend-go/services/workflow-service/internal/adapter/grpc/server.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
**Depends on:** None (independent of TASK-WF-004-01; both feed BE-SOL-005's "Import to My Workflows")
**Status:** `[ ]` TODO

---

## Context

Confirmed no `CloneTemplate` RPC, message, or usecase exists anywhere
today (`grep -rn "CloneTemplate" backend-go/` returns nothing outside the
solution docs). `domain.NewWorkflowTemplate` (`template.go:84-113`,
re-read directly) has the exact signature BE-SOL-004 assumes: `(id,
tenantID, name, dagJSON string, scope Scope, parentTemplateID string)
(WorkflowTemplate, error)` — confirmed, `ErrTemplateSelfParent`
(`template.go:39-50`) guards `parentTemplateID == id`, which is
irrelevant here since Clone always passes `""` for the new template's
parent.

`ResolveTemplate` (`internal/usecase/resolve_template.go:47-86`) is the
existing usecase this task's `CloneTemplate` reuses to compute the
source's effective (post-inheritance) DAG before severing inheritance —
confirmed its `Execute(ctx, ResolveTemplateInput{TemplateID})` returns
`ResolveTemplateOutput{Template, Chain}`, and `Template` is exactly the
resolved `domain.WorkflowTemplate` this task needs.

`TemplateRepository.CreateTemplate(ctx, tmpl domain.WorkflowTemplate)
error` (`ports.go:19`) is the existing repository method this task's
`Create` call uses — no new repository method needed for Clone itself.

## Changes to make

**1. `workflow.proto`**:

```protobuf
service WorkflowService {
  ...
  rpc CloneTemplate(CloneTemplateRequest) returns (CloneTemplateResponse);
}

message CloneTemplateRequest {
  string source_template_id = 1;
  string new_name = 2;
  string scope = 3; // company | team | personal
}

message CloneTemplateResponse {
  WorkflowTemplate template = 1;
}
```

**2. `internal/usecase/clone_template.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type CloneTemplateInput struct {
	SourceTemplateID string
	NewName          string
	Scope            string
}

// CloneTemplate produces a fresh, parent-less copy of an effective
// (post-inheritance) template — the clone's dag_json is the SOURCE's
// fully-resolved DAG (parent chain already folded in by ResolveTemplate),
// not just the source row's own raw dag_json, so a clone of a
// steps-inheriting-from-parent template still has real steps. The clone
// has NO ParentTemplateID — this is a deliberate severing of inheritance,
// matching ErrTemplateSelfParent's existing guard: a clone is a fresh
// root, never a child of its source, so future edits to the source never
// propagate to the clone (or vice versa).
type CloneTemplate struct {
	resolveTemplate *ResolveTemplate
	templates       TemplateRepository
}

func NewCloneTemplate(resolveTemplate *ResolveTemplate, templates TemplateRepository) *CloneTemplate {
	return &CloneTemplate{resolveTemplate: resolveTemplate, templates: templates}
}

func (uc *CloneTemplate) Execute(ctx context.Context, in CloneTemplateInput) (domain.WorkflowTemplate, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	resolved, err := uc.resolveTemplate.Execute(ctx, ResolveTemplateInput{TemplateID: in.SourceTemplateID})
	if err != nil {
		return domain.WorkflowTemplate{}, err // ResolveTemplate already wraps NotFound/etc. in apperrors
	}

	scope := domain.Scope(in.Scope)
	clone, err := domain.NewWorkflowTemplate(uuid.NewString(), tenantID, in.NewName, resolved.Template.DAGJSON, scope, "")
	if err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_TEMPLATE", err.Error(), err)
	}

	if err := uc.templates.CreateTemplate(ctx, clone); err != nil {
		return domain.WorkflowTemplate{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_CLONE_TEMPLATE_FAILED", "failed to create cloned template", err)
	}
	return clone, nil
}
```

Note: `TemplateRepository.CreateTemplate` returns only `error` (no
returned row, per `ports.go:19`) — this task returns the locally-built
`clone` value directly rather than re-fetching, matching
`domain.NewWorkflowTemplate`'s "constructor already IS the created value"
convention used elsewhere in this codebase (e.g. `CreateTemplate`
usecase, if one exists — confirm its exact pattern at implementation
time and mirror it for consistency).

**3. `internal/adapter/grpc/server.go`** — add the handler, following
`Execute`'s exact pattern (`:76-87`):

```go
func (s *Server) CloneTemplate(ctx context.Context, req *workflowv1.CloneTemplateRequest) (*workflowv1.CloneTemplateResponse, error) {
	tmpl, err := s.cloneTemplate.Execute(ctx, usecase.CloneTemplateInput{
		SourceTemplateID: req.GetSourceTemplateId(),
		NewName:          req.GetNewName(),
		Scope:            req.GetScope(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.CloneTemplateResponse{Template: toProtoTemplate(tmpl)}, nil
}
```

**4. `cmd/server/main.go`** — construct `usecase.NewCloneTemplate(...)`
and wire it into `Server`'s constructor alongside the other usecases.

**5. `api-gateway`'s `channels_workflow.go`** — add a
`workflow.template.clone` channel, following `workflow.template.create`'s
exact pattern (`channels_workflow.go:85-106`): tenant/user from `Identity`
attached via `gatewaygrpc.AttachIdentity`, args decoded via
`decodeArg[cloneArgs](args, 0)`.

## Test plan

- `CloneTemplate` on a template with its own steps → clone's `dag_json`
  matches the source's resolved DAG, `ParentTemplateID == ""`.
- `CloneTemplate` on a personal template that inherits steps from a
  parent (empty own `dag_json`) → clone's `dag_json` is the PARENT's
  resolved steps (proving Clone uses `ResolveTemplate`'s effective output,
  not the source row's raw `dag_json`).
- Mutate source after cloning → clone unaffected, and vice versa (proves
  the severed-parent, copy-not-reference semantics).
- `CloneTemplate` on a nonexistent `source_template_id` → NotFound,
  propagated from `ResolveTemplate`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go build ./services/api-gateway/...
go test ./services/workflow-service/internal/usecase/... -run TestCloneTemplate -v
go test ./services/workflow-service/internal/adapter/grpc/... -run TestCloneTemplate -v
```

Expected: clean build; clone/mutate-independence test passes; NotFound
propagates correctly for a missing source template.

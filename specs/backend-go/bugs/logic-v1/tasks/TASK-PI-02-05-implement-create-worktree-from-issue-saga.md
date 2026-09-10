# TASK-PI-02-05: Implement `CreateWorktreeFromIssue` saga usecase

**From Solution:** SOL-PI-02
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/create_worktree_from_issue.go` (new)
**Depends on:** TASK-PI-02-01, TASK-PI-02-03, TASK-PI-02-04
**Status:** `[x] DONE — CreateWorktreeFromIssue saga implemented, wired into grpc/server.go + main.go; ProjectClient.IsIssueStatusSyncEnabled added.`

---

## Context

BUG-PI-02: no worktree-from-issue saga exists. This composes the existing
`CreateWorktree` saga (`create_worktree.go`, reused as a step, not
duplicated) with issue fetch, branch derivation, agent spawn, and — per
BR-PI-06 — a durable opt-out check against `project-service`'s new
`issue_status_sync_enabled` field (TASK-PI-02-06) before deciding whether to
even record the issue link in `Lineage`.

## Changes to make

```go
// internal/usecase/create_worktree_from_issue.go
package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeFromIssueInput struct {
	ProjectID, RepoID, BaseRef string
	IssueRef                   domain.IssueRef
	SkipStatusUpdate           bool
	SkipAgentStart             bool
}

type WorktreeFromIssueResult struct {
	Worktree             domain.WorktreeResult
	BranchName           string
	AgentSessionID       string
	AgentStartError      string
	StatusUpdateEnqueued bool
}

// CreateWorktreeFromIssue is CreateWorktree's saga with two steps prepended
// (fetch issue, derive branch) and two appended (spawn agent, thread the
// issue link through Lineage for project-service to publish — see
// SOL-PI-03; this service owns no database and cannot itself enqueue an
// outbox event).
type CreateWorktreeFromIssue struct {
	issues   IssueSourceClient
	createWT *CreateWorktree
	agents   AgentSpawner
	projects ProjectClient
}

func NewCreateWorktreeFromIssue(issues IssueSourceClient, createWT *CreateWorktree, agents AgentSpawner, projects ProjectClient) *CreateWorktreeFromIssue {
	return &CreateWorktreeFromIssue{issues: issues, createWT: createWT, agents: agents, projects: projects}
}

func (uc *CreateWorktreeFromIssue) Execute(ctx context.Context, in CreateWorktreeFromIssueInput) (WorktreeFromIssueResult, error) {
	issue, err := uc.issues.GetIssue(ctx, in.IssueRef)
	if err != nil {
		return WorktreeFromIssueResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_FROM_ISSUE_ISSUE_NOT_FOUND", "issue not found", err)
	}

	branch := generateBranchName(issue.Title, issue.Labels, issue.ExternalRef) // BR-PI-04

	lineage := domain.WorktreeLineageCapture{
		Origin: "issue", CaptureSource: "createWorktreeFromIssue",
		LinkedIssueProvider: issue.Provider, LinkedIssueRef: issue.ExternalRef,
	}
	if in.SkipStatusUpdate {
		lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", ""
	} else if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, in.ProjectID); err != nil || !enabled {
		// BR-PI-06: don't even record the link if sync is off for this project.
		lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", ""
	}

	wtResult, err := uc.createWT.Execute(ctx, CreateWorktreeInput{
		ProjectID: in.ProjectID, RepoID: in.RepoID, Branch: branch, BaseRef: in.BaseRef,
		Lineage: lineage,
	})
	if err != nil {
		return WorktreeFromIssueResult{}, err // CreateWorktree's own compensation already ran
	}

	result := WorktreeFromIssueResult{
		Worktree: wtResult, BranchName: branch,
		StatusUpdateEnqueued: lineage.LinkedIssueRef != "",
	}

	if !in.SkipAgentStart {
		prompt := buildAgentPrompt(issue.Title, issue.Description, issue.AcceptanceCriteria, issue.Comments) // BR-PI-05
		sessionID, err := uc.agents.SpawnAndInject(ctx, wtResult.WorktreeID, prompt)
		if err != nil {
			// Non-fatal: the worktree exists; agent spawn is a separate
			// BL-AG-01 concern with its own failure modes (e.g. Dev Server
			// not connected) — never roll back a successful worktree for it.
			result.AgentStartError = err.Error()
		} else {
			result.AgentSessionID = sessionID
		}
	}
	return result, nil
}
```

### `ProjectClient` extension (`ports.go`)

Add `IsIssueStatusSyncEnabled(ctx context.Context, projectID string) (bool, error)`
to the `ProjectClient` interface (`internal/usecase/ports.go`); implement it
in `internal/adapter/grpcclient/project_client.go` via `project-service`'s
`GetProject` RPC, reading the new `issue_status_sync_enabled` field
(TASK-PI-02-02/TASK-PI-02-06).

### gRPC handler wiring (`internal/adapter/grpc/server.go`)

Add a `CreateWorktreeFromIssue` method translating
`CreateWorktreeFromIssueRequest`'s `oneof issue_source` into `domain.IssueRef`
and the usecase's result into `CreateWorktreeFromIssueResponse` — follow the
existing `CreateWorktree` handler's exact shape (lines ~678-695).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go vet ./services/git-gateway-service/...
```

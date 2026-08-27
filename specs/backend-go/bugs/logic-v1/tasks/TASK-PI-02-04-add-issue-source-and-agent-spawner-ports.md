# TASK-PI-02-04: `WorktreeLineageCapture` issue fields + `IssueSourceClient`/`AgentSpawner` ports

**From Solution:** SOL-PI-02
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/domain/domain.go`, `backend-go/services/git-gateway-service/internal/usecase/ports.go`, `backend-go/services/git-gateway-service/internal/adapter/grpcclient/scm_client.go` (new), `backend-go/services/git-gateway-service/internal/adapter/grpcclient/issuetracking_client.go` (new), `backend-go/services/git-gateway-service/internal/adapter/grpcclient/infrafleet_client.go` (new)
**Depends on:** TASK-PI-02-02
**Status:** `[x] DONE — WorktreeLineageCapture/IssueRef/Issue domain types + IssueSourceClient/AgentSpawner ports; scm_client.go/issuetracking_client.go/infrafleet_client.go adapters added.`

---

## Context

`domain.WorktreeLineageCapture` (currently `ParentWorktreeID`/`Origin`/
`CaptureSource`/`TaskID`/`OrchestrationRunID`/`CoordinatorHandle`/
`CreatedByTerminalHandle`, see `domain.go`'s existing struct) needs two more
fields to carry the linked-issue reference through to `ProjectClient.RecordWorktreeCreated`.
This task also adds the two new outbound ports the saga
(TASK-PI-02-05) needs — issue fetch and agent spawn — flagged in SOL-PI-02's
rationale as new dependency edges (`git --> scm`, `git --> issue`,
`git --> infra`) beyond `02-microservices-decomposition.md`'s current graph.

## Changes to make

### 1. `domain.go` — extend `WorktreeLineageCapture`

```go
type WorktreeLineageCapture struct {
	ParentWorktreeID        string
	Origin                  string
	CaptureSource           string
	TaskID                  string
	OrchestrationRunID      string
	CoordinatorHandle       string
	CreatedByTerminalHandle string
	LinkedIssueProvider     string // NEW — "github" | "gitlab" | "jira" | "linear"
	LinkedIssueRef          string // NEW — provider-native ref
}

// IssueRef identifies an issue in either an SCM (GitHub/GitLab) or an
// issue-tracker (Jira/Linear), resolved by IssueSourceClient.
type IssueRef struct {
	Provider   string // "github" | "gitlab" | "jira" | "linear"
	Repo       string // scm only
	Number     int32  // scm only
	TrackerRef string // tracker only, e.g. "ENG-123"
}

// Issue is the minimal shape create_worktree_from_issue.go needs from
// either issue source — title/labels feed branch-name derivation,
// description/AC/comments feed the agent prompt.
type Issue struct {
	Title              string
	Description        string
	AcceptanceCriteria string
	Labels             []string
	Comments           []string
	Provider           string
	ExternalRef         string // "owner/repo#123" or "ENG-123", matches Worktree.linked_issue_ref
}
```

### 2. `ports.go` — add two new ports

```go
// IssueSourceClient abstracts scm-integration-service vs.
// issue-tracking-service — resolved by the caller's oneof
// (ScmIssueRef vs TrackerIssueRef).
type IssueSourceClient interface {
	GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error)
}

// AgentSpawner wraps infra-fleet-service.SpawnTerminalSession (BL-AG-01's
// agent.spawn) plus the follow-up prompt-injection write once the PTY
// reports idle — git-gateway-service does not implement PTY spawn itself.
type AgentSpawner interface {
	SpawnAndInject(ctx context.Context, worktreeID, prompt string) (sessionID string, err error)
}
```

Also extend `ProjectClient.RecordWorktreeCreated`'s existing signature
(`internal/usecase/ports.go`) — no new method, `lineage` already carries the
two new fields through unchanged.

### 3. `internal/adapter/grpcclient/scm_client.go` / `issuetracking_client.go` (new)

Both implement `IssueSourceClient` for their half of the `oneof`:

```go
// scm_client.go
type ScmSourceClient struct {
	client scmintegrationv1.ScmIntegrationServiceClient
}

func NewScmSourceClient(client scmintegrationv1.ScmIntegrationServiceClient) *ScmSourceClient {
	return &ScmSourceClient{client: client}
}

func (c *ScmSourceClient) GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error) {
	ctx, err := withTenantMetadata(ctx) // reuse project_client.go's existing helper
	if err != nil {
		return domain.Issue{}, err
	}
	resp, err := c.client.ListIssues(ctx, &scmintegrationv1.ListIssuesRequest{
		Provider: parseScmProvider(ref.Provider), Repo: ref.Repo,
	})
	if err != nil {
		return domain.Issue{}, err
	}
	for _, iss := range resp.GetIssues() {
		if iss.GetNumber() == ref.Number {
			return domain.Issue{
				Title: iss.GetTitle(), Provider: ref.Provider,
				ExternalRef: fmt.Sprintf("%s#%d", ref.Repo, ref.Number),
			}, nil
		}
	}
	return domain.Issue{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_FROM_ISSUE_ISSUE_NOT_FOUND", "issue not found", nil)
}
```

`issuetracking_client.go` follows the identical shape against
`issue-tracking-service`'s `GetIssue`-equivalent RPC (use whatever that
service's proto currently names it — confirm before wiring, do not assume).

### 4. `internal/adapter/grpcclient/infrafleet_client.go` (new)

```go
type InfraFleetAgentSpawner struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetAgentSpawner(client infrafleetv1.InfraFleetServiceClient) *InfraFleetAgentSpawner {
	return &InfraFleetAgentSpawner{client: client}
}

func (s *InfraFleetAgentSpawner) SpawnAndInject(ctx context.Context, worktreeID, prompt string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := s.client.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{WorktreeId: worktreeID})
	if err != nil {
		return "", err
	}
	// Follow-up prompt-injection write once the PTY reports idle — see
	// BL-AG-01:127-142 for the exact wait-for-idle-then-write contract;
	// implement via infra-fleet-service's existing write/subscribe RPCs.
	return resp.GetSessionId(), nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go vet ./services/git-gateway-service/...
```

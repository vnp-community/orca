# TASK-105: Linear-only metadata-lookup usecase group (`ListTeams`/`ListTeamLabels`/`ListTeamMembers`/`GetCustomView`/`ListWorkflowStates`)

**From Solution:** SOL-016
**Priority:** P1
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/{domain,usecase,adapter/linear,adapter/jira,adapter/grpc}/*.go`
**Depends on:** TASK-099, TASK-102, TASK-104
**Status:** `[x]` DONE — implemented in worktree `agent-a412325f0d1276bb5` (branch `worktree-agent-a412325f0d1276bb5`), **committed** as `c29ca9e6a`. `go build`/`go vet`/`buf generate`/`buf breaking` clean. Pending merge.

---

## Context

The 5 `linear.*` methods with no Jira-shared usecase yet: the 4 with no
Jira analog at all (per SOL-016's "genuinely diverges" table — `listTeams`,
`teamLabels`, `teamMembers`, `getCustomView`) plus `teamStates`, which
SOL-016's mapping table maps onto `ListWorkflowStates` ("Linear's per-team
ordered state list is exactly the `WorkflowState` concept §4 already
generalizes across both providers"). TASK-102 added the
`ListWorkflowStates` RPC to the proto; this task adds its usecase and
adapter implementation alongside the other 4 team-scoped lookups, since it
shares their exact shape (resolve `domain.ProviderLinear`, one provider
call, no mutation).

## Changes to make

### 1. `internal/domain/issue.go` — add `Team`, `TeamLabel`, `TeamMember`, `CustomView`

```go
type Team struct {
	ID          string
	WorkspaceID string
	Name        string
	Key         string
}

type TeamLabel struct {
	ID    string
	Name  string
	Color string
}

type TeamMember struct {
	ID          string
	DisplayName string
	AvatarURL   string
}

type CustomView struct {
	ID          string
	WorkspaceID string
	Name        string
	Model       string // "issue" | "project"
	TeamID      string
}
```

### 2. `internal/usecase/ports.go` — extend `IssueTrackerProvider`

```go
type IssueTrackerProvider interface {
	// ... previous methods, plus (Linear-only — see jira/client.go's
	// unimplemented stubs below):
	ListTeams(ctx context.Context, cred Credential, workspaceID string) ([]domain.Team, error)
	ListTeamLabels(ctx context.Context, cred Credential, teamID string) ([]domain.TeamLabel, error)
	ListTeamMembers(ctx context.Context, cred Credential, teamID string) ([]domain.TeamMember, error)
	GetCustomView(ctx context.Context, cred Credential, viewID, model string) (domain.CustomView, error)
	ListWorkflowStates(ctx context.Context, cred Credential, teamID string) ([]domain.WorkflowState, error)
}
```

### 3. `internal/usecase/list_teams.go`, `list_team_labels.go`, `list_team_members.go`, `get_custom_view.go` (new)

Always resolve `domain.ProviderLinear` — same Jira-only-shape convention
`ListCreateFields` (TASK-099) established, mirrored for Linear:

```go
// list_teams.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

type ListTeamsInput struct {
	WorkspaceID string
}

type ListTeams struct {
	registry    ProviderRegistry
	credentials CredentialResolver
}

func NewListTeams(registry ProviderRegistry, credentials CredentialResolver) *ListTeams {
	return &ListTeams{registry: registry, credentials: credentials}
}

func (uc *ListTeams) Execute(ctx context.Context, in ListTeamsInput) ([]domain.Team, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "ISSUETRACKING_NO_USER", "no user in request context", nil)
	}
	provider, err := uc.registry.Resolve(domain.ProviderLinear)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_PROVIDER_UNAVAILABLE", "no adapter registered for linear", err)
	}
	cred, err := uc.credentials.Resolve(ctx, tenantID, userID, domain.ProviderLinear, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "ISSUETRACKING_NOT_CONNECTED", "no linear credential available", err)
	}
	teams, err := provider.ListTeams(ctx, cred, in.WorkspaceID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "ISSUETRACKING_LIST_TEAMS_FAILED", "failed to list teams", err)
	}
	return teams, nil
}
```

`list_team_labels.go` (`ListTeamLabelsInput{TeamID, WorkspaceID}`,
`provider.ListTeamLabels(ctx, cred, in.TeamID)`, code
`ISSUETRACKING_LIST_TEAM_LABELS_FAILED`), `list_team_members.go`
(`ListTeamMembersInput{TeamID, WorkspaceID}`,
`provider.ListTeamMembers(ctx, cred, in.TeamID)`, code
`ISSUETRACKING_LIST_TEAM_MEMBERS_FAILED`), and `get_custom_view.go`
(`GetCustomViewInput{ViewID, Model, WorkspaceID}`,
`provider.GetCustomView(ctx, cred, in.ViewID, in.Model)`, code
`ISSUETRACKING_GET_CUSTOM_VIEW_FAILED`), and `list_workflow_states.go`
(`ListWorkflowStatesInput{TeamID, WorkspaceID}`,
`provider.ListWorkflowStates(ctx, cred, in.TeamID)`, code
`ISSUETRACKING_LIST_WORKFLOW_STATES_FAILED`) follow the identical shape.

### 4. `internal/adapter/linear/client.go` — implement the 4 methods

```go
const listTeamsQuery = `query Teams {
  teams {
    nodes { id name key }
  }
}`

type linearTeamNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Key  string `json:"key"`
}

type linearAllTeamsData struct {
	Teams struct {
		Nodes []linearTeamNode `json:"nodes"`
	} `json:"teams"`
}

func (c *Client) ListTeams(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.Team, error) {
	var resp graphQLResponse[linearAllTeamsData]
	if err := c.do(ctx, cred.Token, listTeamsQuery, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list teams: %s", resp.Errors[0].Message)
	}
	out := make([]domain.Team, 0, len(resp.Data.Teams.Nodes))
	for _, t := range resp.Data.Teams.Nodes {
		out = append(out, domain.Team{ID: t.ID, WorkspaceID: workspaceID, Name: t.Name, Key: t.Key})
	}
	return out, nil
}

const listTeamLabelsQuery = `query TeamLabels($teamId: String!) {
  team(id: $teamId) {
    labels {
      nodes { id name color }
    }
  }
}`

type linearTeamLabelsData struct {
	Team struct {
		Labels struct {
			Nodes []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"nodes"`
		} `json:"labels"`
	} `json:"team"`
}

func (c *Client) ListTeamLabels(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamLabel, error) {
	var resp graphQLResponse[linearTeamLabelsData]
	if err := c.do(ctx, cred.Token, listTeamLabelsQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list team labels: %s", resp.Errors[0].Message)
	}
	out := make([]domain.TeamLabel, 0, len(resp.Data.Team.Labels.Nodes))
	for _, l := range resp.Data.Team.Labels.Nodes {
		out = append(out, domain.TeamLabel{ID: l.ID, Name: l.Name, Color: l.Color})
	}
	return out, nil
}

const listTeamMembersQuery = `query TeamMembers($teamId: String!) {
  team(id: $teamId) {
    members {
      nodes { id name avatarUrl }
    }
  }
}`

type linearTeamMembersData struct {
	Team struct {
		Members struct {
			Nodes []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatarUrl"`
			} `json:"nodes"`
		} `json:"members"`
	} `json:"team"`
}

func (c *Client) ListTeamMembers(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamMember, error) {
	var resp graphQLResponse[linearTeamMembersData]
	if err := c.do(ctx, cred.Token, listTeamMembersQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list team members: %s", resp.Errors[0].Message)
	}
	out := make([]domain.TeamMember, 0, len(resp.Data.Team.Members.Nodes))
	for _, m := range resp.Data.Team.Members.Nodes {
		out = append(out, domain.TeamMember{ID: m.ID, DisplayName: m.Name, AvatarURL: m.AvatarURL})
	}
	return out, nil
}

// GetCustomView queries customView (workspace-scoped) when model=="project"
// callers expect a project custom view — Linear's public schema exposes a
// single customView(id) lookup regardless of model; model is round-tripped
// from the input since the API response has no separate discriminator
// field to read it back from reliably in this scaffold.
const getCustomViewQuery = `query CustomView($id: String!) {
  customView(id: $id) {
    id
    name
    team { id }
  }
}`

type linearCustomViewData struct {
	CustomView struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Team *struct {
			ID string `json:"id"`
		} `json:"team"`
	} `json:"customView"`
}

func (c *Client) GetCustomView(ctx context.Context, cred usecase.Credential, viewID, model string) (domain.CustomView, error) {
	var resp graphQLResponse[linearCustomViewData]
	if err := c.do(ctx, cred.Token, getCustomViewQuery, map[string]any{"id": viewID}, &resp); err != nil {
		return domain.CustomView{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.CustomView{}, fmt.Errorf("linear: get custom view: %s", resp.Errors[0].Message)
	}
	cv := domain.CustomView{ID: resp.Data.CustomView.ID, Name: resp.Data.CustomView.Name, Model: model}
	if resp.Data.CustomView.Team != nil {
		cv.TeamID = resp.Data.CustomView.Team.ID
	}
	return cv, nil
}
```

### 4b. `internal/adapter/linear/client.go` — `ListWorkflowStates`

```go
const listWorkflowStatesQuery = `query TeamStates($teamId: String!) {
  team(id: $teamId) {
    states {
      nodes { id name type position }
    }
  }
}`

type linearTeamStatesData struct {
	Team struct {
		States struct {
			Nodes []struct {
				ID       string  `json:"id"`
				Name     string  `json:"name"`
				Type     string  `json:"type"` // triage|backlog|unstarted|started|completed|cancelled
				Position float64 `json:"position"`
			} `json:"nodes"`
		} `json:"states"`
	} `json:"team"`
}

// ListWorkflowStates backs linear.teamStates — Linear's states are already
// returned ordered by position (ascending) by the API, matching
// teamStates' expected ordered-list contract; no client-side sort needed.
func (c *Client) ListWorkflowStates(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.WorkflowState, error) {
	var resp graphQLResponse[linearTeamStatesData]
	if err := c.do(ctx, cred.Token, listWorkflowStatesQuery, map[string]any{"teamId": teamID}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("linear: list workflow states: %s", resp.Errors[0].Message)
	}
	out := make([]domain.WorkflowState, 0, len(resp.Data.Team.States.Nodes))
	for _, s := range resp.Data.Team.States.Nodes {
		out = append(out, domain.WorkflowState{ID: s.ID, Name: s.Name, Category: s.Type})
	}
	return out, nil
}
```

### 5. `internal/adapter/jira/client.go` — unimplemented stubs

```go
// ListTeams/ListTeamLabels/ListTeamMembers/GetCustomView are Linear-only
// concepts (SOL-016's "genuinely diverges" table) — implemented only to
// satisfy IssueTrackerProvider, never reached by any jira.* channel.
func (c *Client) ListTeams(ctx context.Context, cred usecase.Credential, workspaceID string) ([]domain.Team, error) {
	return nil, fmt.Errorf("jira: ListTeams is not applicable to jira — use listProjects")
}
func (c *Client) ListTeamLabels(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamLabel, error) {
	return nil, fmt.Errorf("jira: ListTeamLabels is not applicable to jira")
}
func (c *Client) ListTeamMembers(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.TeamMember, error) {
	return nil, fmt.Errorf("jira: ListTeamMembers is not applicable to jira — use listAssignableUsers")
}
func (c *Client) GetCustomView(ctx context.Context, cred usecase.Credential, viewID, model string) (domain.CustomView, error) {
	return domain.CustomView{}, fmt.Errorf("jira: GetCustomView is not applicable to jira")
}
func (c *Client) ListWorkflowStates(ctx context.Context, cred usecase.Credential, teamID string) ([]domain.WorkflowState, error) {
	return nil, fmt.Errorf("jira: ListWorkflowStates is not applicable to jira — use getProjectStatusOrder")
}
```

### 6. `internal/adapter/grpc/server.go` — wire the 5 new RPCs

Same translate-request → call-usecase → `apperrors.ToGRPCStatus` →
translate-response shape as every other handler.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/...
go vet ./services/issue-tracking-service/...
```

Expected: clean build — this is the last piece `linear/client.go`/
`jira/client.go` need to fully satisfy the widened `IssueTrackerProvider`
port. Confirm both adapters compile as `usecase.IssueTrackerProvider`:

```bash
cat <<'EOF' > /tmp/ifacecheck.go
package main
import (
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/jira"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/adapter/linear"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)
var _ usecase.IssueTrackerProvider = jira.New(nil)
var _ usecase.IssueTrackerProvider = linear.New(nil)
func main() {}
EOF
go run /tmp/ifacecheck.go && rm /tmp/ifacecheck.go
```

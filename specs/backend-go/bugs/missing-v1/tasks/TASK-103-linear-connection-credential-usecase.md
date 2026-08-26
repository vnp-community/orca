# TASK-103: Wire `linear.*` into the connection/credential usecase group (`Whoami` only — everything else reused as-is)

**From Solution:** SOL-016
**Priority:** P1
**Service:** `issue-tracking-service`
**File:** `services/issue-tracking-service/internal/adapter/linear/client.go`, `cmd/server/main.go`
**Depends on:** TASK-097, TASK-102
**Status:** `[x]` DONE — implemented in worktree `agent-a412325f0d1276bb5` (branch `worktree-agent-a412325f0d1276bb5`), **committed** as `c29ca9e6a`. `go build`/`go vet`/`buf generate`/`buf breaking` clean. Pending merge.

---

## Context

SOL-016 is explicit: `Connect`/`Disconnect`/`SelectWorkspace`/
`GetConnectionStatus`/`TestConnection` (TASK-097) are **already
provider-agnostic** — every usecase there takes a `domain.Provider`
parameter and every branch that differs by provider (Jira's `SiteURL`
requirement in `Connect.Execute`, the composite `owner_id` format) already
handles Linear correctly with zero new code, because `domain.ProviderLinear`
was always a valid `domain.Provider` value. The only genuinely missing
piece is `linear/client.go` implementing `IssueTrackerProvider.Whoami` —
`jira/client.go` got one in TASK-097; `linear/client.go` did not yet.

This task is intentionally small — it exists as its own step (per the
assignment's "connection/credential usecase" category) precisely because
SOL-016 itself calls out that the design work is already done; only the
one missing adapter method needs writing.

## Changes to make

### `internal/adapter/linear/client.go` — add `Whoami`

```go
const viewerQuery = `query Viewer {
  viewer {
    id
    name
    email
  }
}`

type linearViewerData struct {
	Viewer struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"viewer"`
}

// Whoami calls Linear's viewer{} GraphQL query to verify cred.Token and
// identify the authenticated account — the first call Connect makes,
// before anything is persisted (see usecase/connect.go, TASK-097).
func (c *Client) Whoami(ctx context.Context, cred usecase.Credential) (domain.Viewer, error) {
	var resp graphQLResponse[linearViewerData]
	if err := c.do(ctx, cred.Token, viewerQuery, nil, &resp); err != nil {
		return domain.Viewer{}, err
	}
	if len(resp.Errors) > 0 {
		return domain.Viewer{}, fmt.Errorf("linear: whoami: %s", resp.Errors[0].Message)
	}
	return domain.Viewer{ID: resp.Data.Viewer.ID, DisplayName: resp.Data.Viewer.Name, Email: resp.Data.Viewer.Email}, nil
}
```

No new imports needed — `context`, `fmt` are already imported by this file.

### No other changes required

Confirm (do not modify) that:

- `usecase.Connect.Execute` (TASK-097) already skips the `SiteURL`
  requirement for `domain.ProviderLinear` (`in.Provider == domain.ProviderJira
  && in.SiteURL == ""` only fires for Jira) — `ConnectInput{Provider:
  domain.ProviderLinear, Token: "..."}`  with empty `SiteURL`/`Email` is
  already accepted.
- `workspaceIDFor` (TASK-097, `connect.go`) already falls back to
  `fmt.Sprintf("%s:%s", provider, viewerID)` when `siteURL == ""` — Linear
  connections get a workspace id derived from the viewer id, since Linear's
  API key is not scoped to a single named workspace the way a Jira site
  URL is.
- `credential.Resolver`'s composite `owner_id` (`"<userID>:<provider>"`,
  TASK-097) already differentiates Jira/Linear credentials for the same
  user — no Linear-specific branch needed there either.

### `cmd/server/main.go`

No change — `providerregistry.New().Register(domain.ProviderJira,
jira.New(nil)).Register(domain.ProviderLinear, linear.New(nil))` already
registers `linear.New(nil)`; it now satisfies `IssueTrackerProvider`'s
`Whoami` requirement (previously it would have failed to compile as
`ProviderRegistry`'s `usecase.IssueTrackerProvider` value once TASK-097
added `Whoami` to the interface — this task is what makes that compile
again for the Linear adapter specifically; `jira.New(nil)` already compiled
after TASK-097).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/issue-tracking-service/... 2>&1 | head -50
```

Expected: `linear/client.go` no longer errors on a missing `Whoami` method.
The build still fails on `linear/client.go`'s `ListIssues`/`CreateIssue`
(old positional signatures) and the metadata-lookup methods — TASK-104 and
TASK-105 fix those. Confirm the *specific* remaining compile errors are
only about those methods, not `Whoami`:

```bash
go build ./services/issue-tracking-service/internal/adapter/linear/... 2>&1 | grep -v "ListIssues\|CreateIssue\|ListProjects\|ListIssueTypes\|ListCreateFields\|ListAssignableUsers\|ListPriorities\|ListTransitions\|GetProjectStatusOrder\|UpdateIssue\|AddIssueComment\|ListIssueComments\|SearchIssues\|GetIssue\|ListTeams\|ListTeamLabels\|ListTeamMembers\|GetCustomView"
```

(Should print nothing beyond the "missing method" summary line, or
nothing at all once TASK-104/TASK-105 land.)

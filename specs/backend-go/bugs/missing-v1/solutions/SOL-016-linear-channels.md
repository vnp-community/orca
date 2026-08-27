# SOL-016: Build `linear.*` on the same `issue-tracking-service` surface as SOL-015, diverging only where Linear's model actually differs

**Resolves:** [BUG-016](../BUG-016-linear-channels-not-implemented.md)
**Service:** `issue-tracking-service` (proto + usecase extension) + `api-gateway` (`wscompat` wiring)
**Affected files (proposed):**
- `backend-go/proto/orca/issuetracking/v1/issuetracking.proto` (same file SOL-015 extends — see "Relationship to SOL-015" below)
- `backend-go/services/issue-tracking-service/internal/domain/*.go` (`Team`/`CustomView` — Linear-only additions)
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go` (extend `IssueTrackerProvider`)
- `backend-go/services/issue-tracking-service/internal/usecase/*.go` (new use cases, one per RPC)
- `backend-go/services/issue-tracking-service/internal/adapter/external/linear/client.go` (extend beyond `ListIssues`/`CreateIssue`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_linear.go` (new file)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_linear_test.go` (new file)
- `backend-go/services/api-gateway/cmd/server/main.go` (`registerLinearChannels` call alongside SOL-015's `registerJiraChannels`)
**Status:** ✅ Implemented — all 6 task(s) (TASK-102–107) DONE; see each task file's own Status/Verify section for evidence.

---

## Relationship to SOL-015: same service, same proto file, same credential model — implement together

`jira.*` and `linear.*` are BUG-015/BUG-016's sibling reports against the
same owning service, same thin 3-RPC proto, same "no connect flow exists
yet" gap (`ports.go:42-57`'s doc comment names both providers in one
sentence). This solution does not repeat SOL-015's connection/credential
design (composite `owner_id`, multi-site/multi-workspace schema widening,
`Workspace` message, `ConnectionStatus` message, the `wscompat`
view-translation pattern) — all of that is provider-agnostic already and
applies to Linear unchanged, just substituting "workspace" for "site." Read
SOL-015 first; this document covers only what's genuinely different.

## What's shared vs. what genuinely diverges

`issue-tracking-service.md` §4 already draws this line for the domain
model: `Issue`, `Project`, `WorkflowState`, `IssueComment`,
`ConnectionStatus` are provider-agnostic value objects both adapters map
onto. Extending that same judgment to the 19 `linear.*` methods:

**Shared with `jira.*` (same RPC, no Linear-specific proto shape needed):**

| `linear.*` method | RPC (shared with `jira.*`, SOL-015) | Notes |
|---|---|---|
| `status` | `GetConnectionStatus` | |
| `testConnection` | `TestConnection` | |
| `connect` | `Connect` | Linear only needs `Token` (bearer) — no `SiteURL`/`Email`, per `issue-tracking-service.md` §9 |
| `disconnect` | `Disconnect` | |
| `selectWorkspace` | `SelectWorkspace` | same RPC as `jira.selectSite` — TDD's own unification |
| `searchIssues` | `SearchIssues` | |
| `listIssues` | `ListIssues` | partial backing today, same gap as `jira.listIssues` |
| `createIssue` | `CreateIssue` | Linear-specific fields (`teamId`, `stateId`, `parentIssueId`) ride the same additive fields SOL-015 adds |
| `getIssue` | `GetIssue` | |
| `updateIssue` | `UpdateIssue` | |
| `addIssueComment` | `AddIssueComment` | |
| `issueComments` | `ListIssueComments` | |
| `createProject` | `CreateProject` | already in the TDD's RPC list (§3: "linear.createProject; no Jira equivalent today") — **no Jira analog, and none forced here** |
| `getProject` | `GetProject` | already in the TDD's RPC list |
| `teamStates` | `ListWorkflowStates` | Linear's per-team ordered state list is exactly the `WorkflowState` concept §4 already generalizes across both providers |

**Genuinely Linear-specific — no forced unification:**

| `linear.*` method | Proposed RPC | Why this doesn't fold into a shared RPC |
|---|---|---|
| `listTeams` | `ListTeams` (new) | The TDD's own comment reads `ListProjects // Jira projects, Linear teams` — i.e. it proposes reusing `ListProjects` for teams. This solution deliberately **does not** take that shortcut: a Jira `Project` and a Linear `Team` share only "a grouping issues belong to." A `Team` also carries workflow-state ownership (`teamStates`), label ownership (`teamLabels`), and membership (`teamMembers`) — concepts Jira's `Project` doesn't have at all in this proto. Force-fitting `Team` through `ListProjects`/`Project` would mean either padding `Project` with Linear-only fields no Jira caller ever populates, or overloading one message to mean two different domain concepts depending on which provider answered — exactly the "false unification" this task was asked to avoid. Propose a distinct `Team` message and `ListTeams` RPC instead; `GetProject`/`CreateProject` stay shared because Linear's "project" genuinely is the same grouping concept as Jira's (both are a bounded set of issues with its own name/lead/status), unlike "team." |
| `teamLabels` | `ListTeamLabels` (new) | No Jira equivalent — Jira labels are global per-project text tags with no owning entity to list from; Linear labels are owned by a `Team` and have their own `id`/`color`. |
| `teamMembers` | `ListTeamMembers` (new) | Distinct from `ListAssignableUsers` (Jira/Linear shared, per-issue "who can be assigned *this* issue") — `teamMembers` is a team roster query with no issue in scope at all. Reusing `ListAssignableUsers` here would require synthesizing a fake issue reference just to satisfy its request shape; a dedicated RPC is more honest about what's actually being asked. |
| `getCustomView` | `GetCustomView` (new) | Linear "custom views" (saved filtered issue/project lists, `model: 'issue' \| 'project'`) have no Jira concept at all — not even Jira's saved JQL filters map cleanly, since a Linear custom view is schema-typed (`model`) in a way a JQL string isn't. New `CustomView` domain type, entirely Linear-scoped. |

---

## Design — Proto additions specific to `linear.*` (on top of SOL-015's `issuetracking.proto` changes)

```protobuf
service IssueTrackingService {
  // ... SOL-015's RPCs, plus:
  rpc CreateProject(CreateProjectRequest) returns (Project); // already TDD-sketched, §3
  rpc GetProject(GetProjectRequest) returns (Project);        // already TDD-sketched, §3

  // Linear-only — no forced Jira mapping, see table above.
  rpc ListTeams(ListTeamsRequest) returns (ListTeamsResponse);
  rpc ListTeamLabels(ListTeamLabelsRequest) returns (ListTeamLabelsResponse);
  rpc ListTeamMembers(ListTeamMembersRequest) returns (ListTeamMembersResponse);
  rpc GetCustomView(GetCustomViewRequest) returns (CustomView);
}

// Team is deliberately its own message, not a repurposed Project — see
// "genuinely diverges" table above.
message Team {
  string id = 1;
  string workspace_id = 2;
  string name = 3;
  string key = 4; // Linear's short team key, e.g. "ENG"
}

message Label { string id = 1; string name = 2; string color = 3; }

message Member { string id = 1; string display_name = 2; string avatar_url = 3; }

// CustomView has no Jira analog — Linear-only concept, not force-fit into
// any shared message.
message CustomView {
  string id = 1;
  string workspace_id = 2;
  string name = 3;
  string model = 4; // "issue" | "project"
  string team_id = 5; // optional — a view can be workspace-scoped or team-scoped
}

message ListTeamsRequest { string tenant_id = 1; string workspace_id = 2; }
message ListTeamsResponse { repeated Team teams = 1; }

message ListTeamLabelsRequest { string team_id = 1; string workspace_id = 2; }
message ListTeamLabelsResponse { repeated Label labels = 1; }

message ListTeamMembersRequest { string team_id = 1; string workspace_id = 2; }
message ListTeamMembersResponse { repeated Member members = 1; }

message GetCustomViewRequest { string view_id = 1; string model = 2; string workspace_id = 3; }
```

`CreateIssueRequest` (SOL-015's extended message) already carries the
fields `linear.createIssue` needs beyond the Jira set — `assignee_id`,
`label_ids`, `parent_issue_id` are shared field names; add `team_id` and
`state_id` (Linear-specific, unused by Jira):

```protobuf
message CreateIssueRequest {
  // ... SOL-015's fields, plus:
  string team_id = 13;  // Linear: replaces project_key as the primary grouping
  string state_id = 14; // Linear: initial workflow state, no Jira equivalent (Jira issues start in a fixed default status)
}
```

---

## Design — `usecase/` layer

Same shape as SOL-015's groups; only the genuinely-divergent RPCs get new
usecases here.

```go
// internal/usecase/list_teams.go
func (uc *ListTeams) Execute(ctx context.Context, in ListTeamsInput) ([]domain.Team, error) {
    cred, err := uc.credentials.Resolve(ctx, in.TenantID, in.UserID, domain.ProviderLinear, in.WorkspaceID)
    if err != nil {
        return nil, err
    }
    provider, err := uc.registry.Resolve(domain.ProviderLinear)
    if err != nil {
        return nil, err
    }
    return provider.ListTeams(ctx, cred)
}
```

`IssueTrackerProvider` (`ports.go`, shared with SOL-015) grows `ListTeams`/
`ListTeamLabels`/`ListTeamMembers`/`GetCustomView` as new methods —
`jira/client.go`'s implementation of these four simply isn't called for
`ISSUE_PROVIDER_JIRA` requests (the `wscompat` layer never routes
`jira.listTeams` because no such channel exists), so no stub/error path is
needed on the Jira side; the interface is satisfied by `linear/client.go`
alone being the one adapter these four usecases resolve to. (If Go's
interface satisfaction requires `jira/client.go` to implement all
`IssueTrackerProvider` methods too, its four Linear-only methods return
`apperrors.KindUnimplemented` — a real "this doesn't apply to Jira"
response, not silently ignored.)

`linear/client.go`'s GraphQL client (hand-rolled, per §4's "no official
Linear Go SDK" note) extends its existing typed-request-struct pattern with
four new queries — `teams`, `team.labels`, `team.members`,
`customView`/`customViews` — against Linear's public GraphQL schema.

---

## Design — `wscompat` wiring (`channels_linear.go`)

Same convention as SOL-015's `channels_jira.go`:

```go
func registerLinearChannels(r *Registry, client issuetrackingv1.IssueTrackingServiceClient) {
    r.Register("linear.listTeams", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type listTeamsArgs struct {
            WorkspaceID string `json:"workspaceId"`
        }
        in, _ := decodeArg[listTeamsArgs](args, 0) // workspaceId optional, per runtime-linear-client.ts:365
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.ListTeams(rpcCtx, &issuetrackingv1.ListTeamsRequest{WorkspaceId: in.WorkspaceID})
        if err != nil {
            return nil, err
        }
        return toLinearTeamViews(resp.GetTeams()), nil
    })

    r.Register("linear.createIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        type createArgs struct {
            TeamID      string   `json:"teamId"`
            Title       string   `json:"title"`
            StateID     string   `json:"stateId"`
            AssigneeID  string   `json:"assigneeId"`
            LabelIDs    []string `json:"labelIds"`
            WorkspaceID string   `json:"workspaceId"`
        }
        in, err := decodeArg[createArgs](args, 0)
        if err != nil {
            return nil, err
        }
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        resp, err := client.CreateIssue(rpcCtx, &issuetrackingv1.CreateIssueRequest{
            Provider: issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR,
            TeamId: in.TeamID, Title: in.Title, StateId: in.StateID,
            AssigneeId: in.AssigneeID, LabelIds: in.LabelIDs, WorkspaceId: in.WorkspaceID,
        })
        if err != nil {
            return nil, err
        }
        return toLinearIssueView(resp), nil
    })

    // linear.status, .testConnection, .connect, .disconnect,
    // .selectWorkspace, .searchIssues, .listIssues, .getIssue,
    // .updateIssue, .addIssueComment, .issueComments, .createProject,
    // .getProject, .teamStates, .teamLabels, .teamMembers, .getCustomView
    // — same shape, one handler each.
}
```

`toLinearIssueView` differs from SOL-015's `toJiraIssueView` in field names
only (`identifier` not `key`, `state{name,type,color}` object not a flat
string, `team{id,name,key}` not `project`) — both translate the same shared
`Issue` proto message, per `frontend/src/shared/types.ts:1598-1618`'s
`LinearIssue` shape vs. `jira-types.ts`'s `JiraIssue` shape. Note the
frontend's `linear.listIssues` also expects a `{items, errors, hasMore}`
collection envelope (`LinearCollectionResult<T>`,
`runtime-linear-client.ts:247` via `normalizeLinearIssueCollectionResult`)
where `jira.listIssues` returns a bare array — this is a `wscompat`-layer
response-shape difference, not a proto difference; `ListIssuesResponse`
stays one shared message, and `registerLinearChannels`'s `listIssues`
handler wraps it in `{items: [...], hasMore: false}` while
`registerJiraChannels`'s returns the array directly.

`RegisterRealChannels` adds `registerLinearChannels(r, issueTrackingClient)`
next to SOL-015's `registerJiraChannels(r, issueTrackingClient)` call — same
client, no new dial.

---

## Test plan

- `services/issue-tracking-service/internal/usecase/list_teams_test.go`, `get_custom_view_test.go` — the four Linear-only usecases, fakes for `IssueTrackerProvider`/`CredentialResolver`, same pattern as SOL-015's usecase tests.
- `services/issue-tracking-service/internal/adapter/external/linear/client_test.go` — GraphQL request/response mapping for the four new queries, mocked HTTP transport (matches this adapter's existing test shape for `ListIssues`/`CreateIssue`).
- `services/api-gateway/internal/adapter/wscompat/channels_linear_test.go` — one test per channel; specifically assert `linear.listIssues`'s `{items, hasMore}` envelope shape (regression guard distinguishing it from `jira.listIssues`'s bare-array shape, since both route through the same proto `ListIssuesResponse`).
- Regression test: `linear.listTeams` never returns Jira `Project` rows and `jira.listProjects` never returns Linear `Team` rows even if a tenant has both providers connected — guards against the "false unification" this design deliberately avoided at the proto level from leaking back in at the `wscompat` layer via a shared helper by accident.

## References

- `specs/backend-go/bugs/missing-v1/BUG-016-linear-channels-not-implemented.md` — the gap this solution closes
- `specs/backend-go/bugs/missing-v1/solutions/SOL-015-jira-channels.md` — shared connection/credential/proto-extension design this solution builds on
- `specs/backend-go/tdd/services/issue-tracking-service.md` §3 ("linear.createProject; no Jira equivalent today", "Jira projects, Linear teams" comment), §4 (domain model boundary) — the target design and its own acknowledgment of where the two providers diverge
- `backend-go/services/issue-tracking-service/internal/usecase/ports.go:1-77` — `IssueTrackerProvider` port to extend
- `backend-go/services/issue-tracking-service/internal/adapter/linear/client.go:1-42` — hand-rolled GraphQL client to extend
- `frontend/src/shared/types.ts:1555-1753` — `LinearWorkspace`/`LinearIssue`/`LinearTeam`/`LinearCustomViewSummary`/`LinearComment` wire shapes
- `frontend/src/renderer/src/runtime/runtime-linear-client.ts:113-597` — all 19 frontend call sites and their exact arg shapes
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:221-235,390-406` — the `register<Namespace>Channels` pattern both this file and SOL-015's mirror

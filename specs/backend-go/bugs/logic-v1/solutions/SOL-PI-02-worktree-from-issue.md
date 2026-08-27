# SOL-PI-02: `CreateWorktreeFromIssue` saga in `git-gateway-service`

**Resolves:** [BUG-PI-02](../BUG-PI-02-worktree-from-issue-not-implemented.md)
**Service:** `git-gateway-service` (new saga usecase, owning service — see rationale) calling out to `scm-integration-service` / `issue-tracking-service` (issue fetch), `project-service` (existing, via `CreateWorktree`), and `infra-fleet-service` (agent spawn, BL-AG-01); publishes the event `SOL-PI-03` designs a consumer for
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` (new `CreateWorktreeFromIssue` RPC)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree_from_issue.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/branch_name.go` (new — BR-PI-04, pure function)
- `backend-go/services/git-gateway-service/internal/usecase/agent_prompt.go` (new — BR-PI-05, pure function)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` (new `IssueSourceClient`, `AgentSpawner` ports)
- `backend-go/services/git-gateway-service/internal/domain/domain.go` (`WorktreeLineageCapture` gains `LinkedIssueProvider`/`LinkedIssueRef`)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/scm_client.go`, `issuetracking_client.go` (new)
- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/infrafleet_client.go` (new)
- `backend-go/services/project-service/internal/adapter/postgres/...` — per-project opt-out flag (BR-PI-06) + `linked_issue_provider`/`linked_issue_ref` columns, see below
- `backend-go/proto/orca/project/v1/project.proto` (new `issue_status_sync_enabled` field on `Project`; new `linked_issue_provider`/`linked_issue_ref` fields on `Worktree`/`RecordWorktreeCreatedRequest` — shared with SOL-PI-03, which owns the publish side)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` (new `worktree.createFromIssue` channel)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Where this saga lives: `git-gateway-service`, not a new service, not `api-gateway`

`05-data-architecture.md`'s "Cross-service data consistency" section frames
a synchronous saga as living in "A's usecase layer" when "A's operation
[can't be] considered complete" without "B also succeed[ing]"
(`05-data-architecture.md:100-112`) — the saga owner is the service whose
operation is the one the caller is actually waiting on. Here that's worktree
creation: steps (a)-(d) (branch name, existence check, worktree creation,
prompt build) all exist to produce one worktree, and `git-gateway-service`
already owns `CreateWorktree` as a saga (its own doc comment: "the saga:
resolve host, run `git worktree add`, then record bookkeeping... best-effort
compensate" — `create_worktree.go:16-29`, cited verbatim). `CreateWorktreeFromIssue`
is that same saga with two new steps prepended (fetch + derive) and two
appended (spawn agent, publish status-sync event) — not a different saga
in a different service.

This also matches `02-microservices-decomposition.md`'s dependency graph
literally: `git-gateway-service` already depends on `project-service` and
`infra-fleet-service` (`git --> proj`, `git --> infra`,
`02-microservices-decomposition.md:150-151`). This solution adds two new
edges — `git --> scm` and `git --> issue` — to fetch the issue being
worktree'd from. **Flagged explicitly as a scope addition to the dependency
graph**, the same posture SOL-009 took for extending `git-gateway-service`'s
proto: not something the graph already specified, but the smallest addition
that keeps the saga in the service that owns its outcome, rather than
duplicating `CreateWorktree`'s saga machinery in a brand-new
`project-integration-service` for one composite RPC. `api-gateway` is
explicitly *not* the right home per `08-inter-service-communication.md`'s
API Gateway responsibilities list (`08-inter-service-communication.md:47-70`)
— every listed responsibility there is edge/transport concern (TLS, JWT,
REST translation, WS piping, rate limiting), never multi-step business
sagas; `05-data-architecture.md`'s saga guidance places saga logic inside a
service's usecase layer, not the gateway.

### Step (e), "start the agent (BL-AG-01)" — delegates to `infra-fleet-service`, does not reimplement it

`project-service.md`'s boundary decision is explicit and directly on point:
"`project.agentSpawn` does not port here... Copying it into Go's
`project-service` would make this a second, inconsistent home for
agent-dispatch logic alongside `git-gateway-service`... the spawn RPC
itself belongs to `infra-fleet-service`" (`project-service.md:42-52`).
`infra-fleet-service.md`'s `SpawnTerminalSession` RPC
(`infra-fleet-service.md:124`) is that spawn primitive — BL-AG-01's
`agent.spawn` JSON-RPC to the Dev Server Agent
(`docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:131-140`) is
exactly the payload `SpawnTerminalSession` routes, per
`infra-fleet-service.md`'s framing of PTY/agent spawn as the same
"coordination, not execution" concern as every other routed op
(`infra-fleet-service.md:47-52`, the "Terminal/PTY session routing" bullet
explicitly covers "dispatching spawn/write/resize/kill calls"). This
solution's saga calls `SpawnTerminalSession` with the agent binary/args/env
BL-AG-01 already specifies, then a follow-up prompt-injection write once
the PTY reports "idle" — it does not reimplement PTY spawn logic inside
`git-gateway-service`.

### Step (f), issue status update — feeds SOL-PI-03's event, does not duplicate it

BUG-PI-02 itself flags this: "instead of calling issue-status-update
itself" is exactly the wrong shape — this saga must not call
`UpdateIssue`/`TransitionIssue` synchronously. But `git-gateway-service`
itself has no Postgres database of its own (SOL-009's "Same 'owns no data'
shape... no new database, no new migrations" — this solution doesn't
revisit that), so it cannot durably enqueue a transactional-outbox event
the way `issue-tracking-service.LinkIssue` does — there is no local
transaction to make the enqueue atomic with. This saga therefore does not
publish `worktree.created` itself: it passes the linked-issue reference
through to `CreateWorktree`'s existing `Lineage` parameter (extended below),
and **`project-service` publishes the event, in the same transaction as its
own `RecordWorktreeCreated` write** — see SOL-PI-03's design for the full
rationale (project-service already durably owns worktree existence per
`project-service.md`'s domain model, §4, so it's the natural transactional
-outbox writer here, not `git-gateway-service`). BR-PI-06's opt-out (see
below) still gates whether this saga even asks for the link to be recorded;
the publish-and-consume mechanics live entirely in SOL-PI-03.

### BR-PI-04/BR-PI-05 — pure domain functions, no TDD precedent, small enough to not need one

Branch-name generation and prompt sanitization are stateless string
transforms with no external call and no ownership question — they don't
need grounding beyond "this is `git-gateway-service`'s saga, so its helper
functions live in its `usecase/` package." Flagged as this solution's two
genuinely new pieces of logic (not a port of anything).

---

## Design — proto (`gitgateway.proto`)

```protobuf
rpc CreateWorktreeFromIssue(CreateWorktreeFromIssueRequest) returns (CreateWorktreeFromIssueResponse);

message CreateWorktreeFromIssueRequest {
  string project_id = 1;
  string repo_id = 2;
  string base_ref = 3;

  // Issue source — exactly one of these two blocks is set.
  oneof issue_source {
    ScmIssueRef scm_issue = 4;          // GitHub/GitLab issue
    TrackerIssueRef tracker_issue = 5;  // Jira/Linear issue
  }

  bool skip_status_update = 6;   // BR-PI-06 per-call opt-out
  bool skip_agent_start = 7;     // caller may want the worktree without auto-starting an agent
}

message ScmIssueRef {
  string provider = 1;   // "github" | "gitlab"
  string repo = 2;
  int32 number = 3;
}
message TrackerIssueRef {
  string provider = 1;   // "jira" | "linear"
  string issue_ref = 2;  // provider-native key, e.g. "ENG-123"
}

message CreateWorktreeFromIssueResponse {
  Worktree worktree = 1;          // same shape CreateWorktree already returns
  string branch_name = 2;         // the derived name, surfaced for the UI's worktree card (step 4)
  string agent_session_id = 3;    // empty if skip_agent_start or spawn failed non-fatally
  bool status_update_enqueued = 4;
}
```

`skip_status_update`/`skip_agent_start` are per-call escape hatches; the
durable per-project opt-out (BR-PI-06's actual requirement — "must be
disableable," not just skippable per-call) is a `Project` field, below.

---

## Design — `usecase/` layer

```go
// internal/usecase/create_worktree_from_issue.go
type CreateWorktreeFromIssue struct {
    issues       IssueSourceClient   // scm-integration-service or issue-tracking-service, by provider
    createWT     *CreateWorktree     // existing saga, reused as a step — not duplicated
    agents       AgentSpawner        // infra-fleet-service.SpawnTerminalSession + write
    projects     ProjectClient       // existing port; extended to read issue_status_sync_enabled
}

func (uc *CreateWorktreeFromIssue) Execute(ctx context.Context, in CreateWorktreeFromIssueInput) (WorktreeFromIssueResult, error) {
    issue, err := uc.issues.GetIssue(ctx, in.IssueRef) // step (a) precursor: need title+body first
    if err != nil {
        return WorktreeFromIssueResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_FROM_ISSUE_ISSUE_NOT_FOUND", "issue not found", err)
    }

    branch := generateBranchName(issue) // BR-PI-04 — "type/description-issueId"
    // BR-PI-02 step 2b: pre-flight duplicate-branch check, issue-aware
    if exists, err := uc.createWT.local.BranchExists(ctx, in.RepoID, branch); err != nil {
        return WorktreeFromIssueResult{}, err
    } else if exists {
        return WorktreeFromIssueResult{}, apperrors.New(apperrors.KindAlreadyExists, "WORKTREE_FROM_ISSUE_BRANCH_EXISTS",
            fmt.Sprintf("branch %q already exists — rename or delete it first", branch), nil)
    }

    // linkedIssue is threaded through CreateWorktree's Lineage param (extended
    // with LinkedIssueProvider/LinkedIssueRef below) purely so project-service
    // can persist it and publish worktree.created with the link attached in
    // the SAME transaction as RecordWorktreeCreated — see SOL-PI-03. This
    // saga never touches an outbox itself (git-gateway-service has none).
    lineage := domain.WorktreeLineageCapture{
        Origin: "issue", CaptureSource: "createWorktreeFromIssue",
        LinkedIssueProvider: issue.Provider, LinkedIssueRef: issue.ExternalRef(),
    }
    if !in.SkipStatusUpdate {
        if enabled, err := uc.projects.IsIssueStatusSyncEnabled(ctx, in.ProjectID); err != nil || !enabled {
            lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", "" // BR-PI-06 opt-out: don't even record the link if sync is off for this project
        }
    } else {
        lineage.LinkedIssueProvider, lineage.LinkedIssueRef = "", ""
    }

    wtResult, err := uc.createWT.Execute(ctx, CreateWorktreeInput{ // reuses the existing saga verbatim
        ProjectID: in.ProjectID, RepoID: in.RepoID, Branch: branch, BaseRef: in.BaseRef,
        Lineage: lineage,
    })
    if err != nil {
        return WorktreeFromIssueResult{}, err // CreateWorktree's own compensation already ran
    }

    result := WorktreeFromIssueResult{
        Worktree: wtResult, BranchName: branch,
        StatusUpdateEnqueued: lineage.LinkedIssueRef != "", // project-service publishes; this just reports intent
    }

    if !in.SkipAgentStart {
        prompt := buildAgentPrompt(issue) // BR-PI-05 — sanitize + compose title/description/AC/comments
        sessionID, err := uc.agents.SpawnAndInject(ctx, wtResult.WorktreeID, prompt)
        if err != nil {
            // Non-fatal: the worktree exists; log and surface a partial result rather than
            // rolling back a successful worktree because agent spawn (a separate BL-AG-01
            // concern with its own failure modes, e.g. Dev Server not connected) failed.
            result.AgentStartError = err.Error()
        } else {
            result.AgentSessionID = sessionID
        }
    }
    return result, nil
}
```

`generateBranchName`/`buildAgentPrompt` are pure functions in their own
files (`branch_name.go`, `agent_prompt.go`) so they're unit-testable
without any port fakes:

```go
// internal/usecase/branch_name.go
// BR-PI-04: "type/description-issueId" — type inferred from issue labels
// ("bug"->"fix", "enhancement"->"feat", default "chore"), description is
// the title kebab-cased and truncated, issueId is the provider's issue number/key.
func generateBranchName(issue domain.Issue) string {
    kind := inferBranchType(issue.Labels) // "fix" | "feat" | "chore"
    desc := kebabCase(truncate(issue.Title, 40))
    return fmt.Sprintf("%s/%s-%s", kind, desc, issue.ExternalRef())
}

// internal/usecase/agent_prompt.go
// BR-PI-05: sanitize before composing — strip HTML/script content, collapse
// provider markdown quirks (Jira ADF, already normalized by
// issue-tracking-service.md §4's own Issue.Description contract), truncate
// any single field to a bounded length so one hostile/huge issue body can't
// blow the agent's context budget.
func buildAgentPrompt(issue domain.Issue) string {
    return renderPromptTemplate(sanitize(issue.Title), sanitize(issue.Description),
        sanitize(issue.AcceptanceCriteria), sanitizeComments(issue.Comments))
}
```

### New ports

```go
// IssueSourceClient abstracts scm-integration-service vs. issue-tracking-service —
// resolved by the request's oneof (ScmIssueRef vs TrackerIssueRef), same
// per-provider-adapter shape both TDD docs already use internally.
type IssueSourceClient interface {
    GetIssue(ctx context.Context, ref IssueRef) (domain.Issue, error)
}

// AgentSpawner wraps infra-fleet-service.SpawnTerminalSession (BL-AG-01's
// agent.spawn) plus the follow-up prompt-injection write once the PTY
// reports idle — git-gateway-service does not implement PTY spawn itself.
type AgentSpawner interface {
    SpawnAndInject(ctx context.Context, worktreeID, prompt string) (sessionID string, err error)
}
```

### `project-service` addition — BR-PI-06 durable opt-out

```sql
-- migration on project-service's own DB (project.projects)
ALTER TABLE projects ADD COLUMN issue_status_sync_enabled BOOLEAN NOT NULL DEFAULT true;
```

Exposed via a field on the existing `Project` message
(`project.proto`) and a narrow read path (`IsIssueStatusSyncEnabled`) rather
than a new RPC — `UpdateProject`'s existing field mask covers writing it,
matching the same "extend `UpdateProject`'s mask" pattern
`RebindDevServer`'s carve-out already established for `dev_server_id`
(`project-service.md:159-161`, "field mask rejects `dev_server_id`... exactly
one code path"). This same field is read by both this solution (to decide
whether to publish the event at all) and SOL-PI-03 (as the belt-and-braces
consumer-side check) — see that solution's design for why both checks
exist.

---

## Design — wiring (`wscompat`)

```go
// channels_worktree.go — new channel, same pattern as worktree.create
r.Register("worktree.createFromIssue", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type args struct {
        ProjectID string `json:"projectId"`
        RepoID    string `json:"repoId"`
        BaseRef   string `json:"baseRef"`
        Provider  string `json:"provider"`   // "github"|"gitlab"|"jira"|"linear"
        Repo      string `json:"repo"`       // scm only
        Number    int32  `json:"number"`     // scm only
        IssueRef  string `json:"issueRef"`   // tracker only
        SkipAgentStart   bool `json:"skipAgentStart"`
        SkipStatusUpdate bool `json:"skipStatusUpdate"`
    }
    // ... decode, build oneof, call gitClient.CreateWorktreeFromIssue, return resp
})
```

---

## Test plan

- `branch_name_test.go`: label→type inference table (bug→fix, enhancement→feat,
  no matching label→chore), kebab-casing, truncation at 40 chars, exact
  `type/description-issueId` shape assertion (BR-PI-04 regression guard).
- `agent_prompt_test.go`: sanitize strips `<script>`/HTML, truncates an
  oversized field, empty acceptance-criteria/comments omitted cleanly from
  the template (BR-PI-05).
- `create_worktree_from_issue_test.go`: fake `IssueSourceClient`/`CreateWorktree`
  (reuse `worktree_fakes_test.go`'s existing fakes)/`AgentSpawner`/`OutboxEnqueuer`/`ProjectClient`:
  - happy path: issue fetched, branch derived, `CreateWorktree.Execute` called with `Lineage.LinkedIssueRef` populated, agent spawned — response has all result fields populated.
  - duplicate branch: `BranchExists=true` short-circuits before any worktree is created — assert `CreateWorktree.Execute` never called.
  - agent spawn failure: worktree still returned successfully, `AgentStartError` populated, no rollback of the worktree (regression guard against over-eager compensation).
  - `skip_agent_start`: `AgentSpawner.SpawnAndInject` never called.
  - `skip_status_update` or `IsIssueStatusSyncEnabled=false`: `Lineage.LinkedIssueProvider`/`LinkedIssueRef` passed to `CreateWorktree` are both empty — regression guard that BR-PI-06's opt-out reaches project-service's persisted row, not just this saga's own state.
- `wscompat` channel test: both `oneof` shapes (scm vs. tracker) decode correctly; malformed combination (both or neither set) returns a client error before any gRPC call.
- Contract test: `RebindDevServer`-style saga guard is *not* needed here since agent-spawn/status-update failures are explicitly non-fatal — assert this in a comment-adjacent test named for the invariant (`TestCreateWorktreeFromIssue_AgentAndStatusFailuresNeverFailTheSaga`).

## Agent (`agent/`) impact

**None required for this solution's own code** — `SpawnTerminalSession`
already routes to the Dev Server Agent's existing `agent.spawn`/PTY output
stream (BL-AG-01, already implemented per that doc's precondition list).
This solution is a pure `backend-go` orchestration addition that calls
already-scoped `infra-fleet-service`/Dev Server Agent capabilities — it
adds no new agent-side RPC or protocol surface.

## References

- `specs/backend-go/tdd/services/project-service.md:42-52` (§2 boundary decision: agent spawn belongs to `infra-fleet-service`, two-step saga), `:122-161` (`RebindDevServer`'s field-mask-single-path precedent this solution's `issue_status_sync_enabled` field follows)
- `specs/backend-go/tdd/services/infra-fleet-service.md:47-52` (PTY/agent spawn as routed coordination, not execution), `:124` (`SpawnTerminalSession` RPC)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:150-151` (existing `git --> proj`, `git --> infra` edges; this solution's two new edges are a flagged addition)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:100-112` (saga pattern — lives in the caller's usecase layer)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:47-70` (API Gateway responsibilities — no business-saga logic there)
- `docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:39-62,127-142` (`agent.spawn` JSON-RPC contract, prompt-injection-after-idle flow)
- `docs/logic/project-integration/BL-PI-02-tao-worktree-tu-task.md:21-49` (main flow, BR-PI-04/05/06 verbatim)
- `docs/logic/project-integration/BL-PI-03-update-issue-status.md:38` (BR-PI-09, non-blocking constraint honored by publishing an event instead of a sync call)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:16-71` — existing saga reused as a step, not duplicated
- `specs/backend-go/bugs/logic-v1/BUG-PI-02-worktree-from-issue-not-implemented.md` — problem statement

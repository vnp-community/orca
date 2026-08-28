# TASK-PI-02-08: Tests for the `CreateWorktreeFromIssue` saga

**From Solution:** SOL-PI-02
**Priority:** P1
**Service:** `git-gateway-service` + `api-gateway`
**File:** `services/git-gateway-service/internal/usecase/branch_name_test.go` (new), `agent_prompt_test.go` (new), `create_worktree_from_issue_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`
**Depends on:** TASK-PI-02-05, TASK-PI-02-06, TASK-PI-02-07
**Status:** `[x] DONE — branch_name_test.go, agent_prompt_test.go, create_worktree_from_issue_test.go (7 cases incl. TestCreateWorktreeFromIssue_AgentAndStatusFailuresNeverFailTheSaga) all new; channels_worktree_test.go extended with 5 worktree.createFromIssue cases (both oneof shapes + 3 malformed-input rejections).`

---

## Tests to add

### `branch_name_test.go`

Table-driven: label->type inference (`bug`->`fix`, `enhancement`/`feature`->`feat`,
no matching label->`chore`), kebab-casing, truncation at 40 chars, exact
`type/description-issueId` shape assertion (BR-PI-04 regression guard).

### `agent_prompt_test.go`

`sanitize` strips `<script>`/HTML tags, truncates an oversized field to
`maxPromptFieldLen`, empty acceptance-criteria/comments are omitted cleanly
from the rendered template (BR-PI-05).

### `create_worktree_from_issue_test.go`

Fake `IssueSourceClient`/`AgentSpawner`/`ProjectClient` (reuse
`worktree_fakes_test.go`'s existing `CreateWorktree` fakes where possible):

- Happy path: issue fetched, branch derived, `CreateWorktree.Execute` called
  with `Lineage.LinkedIssueRef` populated, agent spawned — response has all
  result fields populated.
- Duplicate branch: `CreateWorktree`'s own pre-flight check rejects before
  any worktree is created — assert the saga surfaces that error unchanged.
- Agent spawn failure: worktree still returned successfully,
  `AgentStartError` populated, no rollback of the worktree (regression guard
  against over-eager compensation).
- `skip_agent_start`: `AgentSpawner.SpawnAndInject` never called.
- `skip_status_update` or `IsIssueStatusSyncEnabled=false`:
  `Lineage.LinkedIssueProvider`/`LinkedIssueRef` passed to `CreateWorktree`
  are both empty — regression guard that BR-PI-06's opt-out reaches
  project-service's persisted row, not just this saga's own state.
- Named invariant test:
  `TestCreateWorktreeFromIssue_AgentAndStatusFailuresNeverFailTheSaga`.

### `channels_worktree_test.go`

- Both `oneof` shapes (scm vs. tracker) decode correctly.
- Malformed combination (unknown provider, or scm provider missing
  `repo`/`number`) returns a client error before any gRPC call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/git-gateway-service/internal/usecase/... -run "BranchName|AgentPrompt|CreateWorktreeFromIssue" -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestChannelsWorktree -v
go build ./... && go vet ./...
```

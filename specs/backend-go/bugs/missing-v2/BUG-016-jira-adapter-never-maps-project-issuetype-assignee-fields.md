# BUG-016: Jira adapter's read path (`SearchIssues`/`GetIssue`) never maps `project`/`issueType`/`assignee`/`reporter`/`priority`/`labels` — frontend crashes reading `issue.project.key`

**Service:** `issue-tracking-service`
**File:** `internal/adapter/jira/client.go` (`jiraIssue` struct, `toRichIssue`)
**Severity:** 🔴 High — every Jira issue fetched into the Task page crashes the whole list load, not just degrades one field
**Status:** ✅ Root cause CONFIRMED end-to-end (2026-09-15), traced from a live frontend error message through 3 files. ✅ Fixed, unit-tested (2 new tests: mapping + unassigned-issue nil-safety), build/vet/test all pass, not yet deployed/live-verified.

## Symptom (user-reported, live)

After CR-TSRC-001's capability fix unblocked the Jira source from showing as "unavailable", the Task page now attempts to fetch Jira issues and fails with:
```
Couldn't load Jira issues. Try again in a moment.
Details: Cannot read properties of undefined (reading 'key')
```

## Root cause — confirmed end-to-end

1. **`frontend/src/renderer/src/components/task-page-jira-status-order.ts:21`**: `issueProjectScope` reads `issue.project.key` directly — `JiraIssue.project` (`frontend/src/shared/jira-types.ts:102`) is typed as a REQUIRED (non-optional) field, so this code never null-checks `issue.project` itself.
2. **`backend-go/services/issue-tracking-service/internal/adapter/grpc/server.go:607-609`** (`toProtoIssue`): only sets `out.Project` when `i.Project.ID/Key/Name` is non-empty — otherwise leaves the proto field `nil`, which serializes as absent/`undefined` on the wire (not an empty object).
3. **`backend-go/services/issue-tracking-service/internal/adapter/jira/client.go:186-204`**: the actual root cause — `jiraIssue`'s `Fields` struct only parses `summary` and `status.name` from Jira's real API response:
   ```go
   type jiraIssue struct {
       Key    string `json:"key"`
       Fields struct {
           Summary string `json:"summary"`
           Status  struct{ Name string } `json:"status"`
       } `json:"fields"`
   }
   func toRichIssue(baseURL string, key, summary, status string) domain.Issue {
       return domain.Issue{ID: key, ProviderIssueID: key, Key: key, Title: summary, State: status, WorkflowState: domain.WorkflowState{Name: status}, URL: issueBrowseURL(baseURL, key)}
   }
   ```
   `project`, `issuetype`, `assignee`, `reporter`, `priority`, `labels` are ALL present in Jira's real API response (confirmed shape: `fields.project.{id,key,name}`, `fields.issuetype.{id,name,subtask}`, `fields.assignee/reporter.{accountId|key|name, displayName, emailAddress}`, `fields.priority.{id,name}`, `fields.labels: string[]`) but this adapter never reads any of them — `domain.Issue`'s `Project`/`IssueType`/`Assignee`/`Reporter`/`Priority`/`Labels` fields (all of which exist in the domain type, `issue.go:52-70`) stay permanently zero-valued for every issue `SearchIssues`/`GetIssue` ever return.

`project`/`issueType` are the two REQUIRED (non-optional) fields on the frontend's `JiraIssue` type — a `nil` proto message for either crashes the first code path that reads it (confirmed for `project`; `issueType` is the same shape and would crash the next code path that reads it, not yet hit live only because `issueProjectScope` runs first).

## Fix (implemented in this pass)

Extend `jiraIssue.Fields` to parse `project`/`issuetype`/`assignee`/`reporter`/`priority`/`labels`, and `toRichIssue` (now taking the full `jiraIssue`, not 4 loose strings) to map them into `domain.Issue`. `assignee`/`reporter` accept either Jira Cloud's `accountId` or Server/Data Center's `key`/`name` as the user id (CR-JIRA-001 already established this Cloud-vs-Server/DC duality for auth; the same duality applies to user identifiers in issue payloads).

## Related

- [CR-JIRA-001](../../../docs/crs/v3/jira-integration/CR-JIRA-001-support-self-hosted-jira-server-data-center.md) / [BUG-013](./BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) — this adapter's auth/API-version fix; this bug is the next gap in the same file, found only once a real Jira connection could actually fetch issues.
- [CR-TSRC-001](../../../docs/crs/v4/task-source-integrations/CR-TSRC-001-port-task-source-capability-to-backend-go.md) / [BUG-FE-TASKV1-009](../../frontend/bugs/task-v1/BUG-FE-TASKV1-009-jira-linear-task-source-capability-never-advertised-by-backend-go.md) — the capability-gate fix that unblocked the UI far enough to hit this bug for the first time.

# BUG-018: `channels_jira.go`/`channels_linear.go` send `labels: null` (not `[]`) for an issue with no labels — frontend crashes on `.slice()`

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_jira.go` (`toJiraIssueView`), `channels_linear.go` (`toLinearIssueView`, same shape)
**Severity:** 🔴 High — crashes the entire Task page render for any Jira/Linear issue with no labels (the common case)
**Status:** ✅ Root cause CONFIRMED, ✅ Fixed + unit-tested (build/vet/test all pass), ✅ **deployed** (version `2026.09.15-jira-labels-fix`). Same bug class as `channels_scm.go`'s already-fixed Labels handling (line 1233-1234) — just never applied to Jira/Linear. Live user-facing verification (reload Task page, retry loading Jira issues) still pending — this bug lives in api-gateway's JSON translation layer, not directly reachable via `grpcurl` to issue-tracking-service, so a browser reload is the real verification step.

## Symptom (live, user-reported)

```
TypeError: Cannot read properties of null (reading 'slice')
    at T1 (TaskPage-CE_TmxAP.js:...)
```
Surfaced immediately after BUG-016/BUG-017 were deployed and Jira issues finally loaded with real data for the first time.

## Root cause

`TaskPage.tsx` reads `issue.labels.slice(...)` in 3 places (lines ~5042, 6188, 6351) with no null-guard — `JiraIssue.labels: string[]` is a required (non-optional) field.

`channels_jira.go`'s `toJiraIssueView`:
```go
Labels: i.GetLabels(), Status: jiraStatusView{Name: i.GetState()},
```
`i.GetLabels()` (a proto-generated getter for a `repeated string` field) returns the raw Go slice — `nil` when the issue has no labels (the common case — confirmed live: neither `DEV-278` nor `DEV-277`, the first real issues fetched, had any labels). Go's `encoding/json.Marshal` serializes a nil slice as JSON `null` (not `[]`) when the struct field has no `omitempty` tag (`jiraIssueView.Labels []string \`json:"labels"\`` here does not).

**This exact bug class was already found and fixed once in this same file family** — `channels_scm.go:1233-1234`:
```go
if view.Labels == nil {
    view.Labels = []string{}
}
```
— just never applied to `channels_jira.go` or `channels_linear.go` (same `Labels: i.GetLabels()` pattern, same latent crash, not yet hit live for Linear only because Linear's capability gate is presumably still closed/untested).

## Fix

Apply the identical, already-proven `channels_scm.go` pattern to both `toJiraIssueView` (`channels_jira.go`) and `toLinearIssueView` (`channels_linear.go`).

## Related

- [BUG-016](./BUG-016-jira-adapter-never-maps-project-issuetype-assignee-fields.md) / [BUG-017](./BUG-017-jira-list-projects-uses-cloud-only-project-search-endpoint.md) — the two preceding layers in this same "Jira issue loading" investigation chain; this is the third and (so far) final layer found live, in the same session.
- `channels_scm.go`'s own already-fixed Labels handling — the precedent this fix copies exactly.

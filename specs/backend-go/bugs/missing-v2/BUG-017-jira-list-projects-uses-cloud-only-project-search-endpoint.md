# BUG-017: `ListProjects` uses Cloud-only `/project/search` endpoint — 404s on Jira Server/Data Center without it, breaking the project picker step of the Jira issue-loading flow

**Service:** `issue-tracking-service`
**File:** `internal/adapter/jira/client.go` (`ListProjects`)
**Severity:** 🔴 High — blocks the project-picker step the Task page's Jira browse flow (`use-task-page-jira-browse-state.ts`) needs before it can search/load any issues at all, on this exact self-hosted Jira instance (`jr.servicehub.vn`)
**Status:** ✅ Root cause CONFIRMED live via direct `curl` against the real site. ✅ Fixed, unit-tested (v2 flat-array + v3 paginated regression tests), build/vet/test all pass. Not yet deployed/live-verified.

## Symptom (live logs, `issue-tracking-service`)

```json
{"code":"ISSUETRACKING_LIST_PROJECTS_FAILED","cause":"jira: list projects: unexpected status 404: {\"errorMessages\":[\"No project could be found with key 'search'.\"],\"errors\":{}}"}
```
Repeated ~10 times over several minutes (user retrying) — every `ListProjects` call fails.

## Root cause — confirmed via direct `curl` against `jr.servicehub.vn`

```
GET /rest/api/2/project/search  → 404 {"errorMessages":["No project could be found with key 'search'."]}
GET /rest/api/2/project         → 200 [{"id":"17501","key":"ATNYCAGBM","name":"...",...}, ...]
```

`/rest/api/2/project/search` (the paginated, Cloud-shaped `{values: [...]}` endpoint `ListProjects` hardcodes) was only added to Jira Server/Data Center in a later version (8.4+); this instance predates it, so Jira's router doesn't recognize `/project/search` as a distinct sub-resource at all — it falls through to the OLDER, single-project-lookup route `/project/{projectIdOrKey}`, treating the literal string `"search"` as a project key, hence the misleading "No project could be found with key 'search'" error.

`internal/adapter/jira/client.go`'s `ListProjects` (line ~606) always calls `apiURL(cred.BaseURL, apiVersion, "/project/search")` regardless of `apiVersion` — the same Cloud-vs-Server/DC duality CR-JIRA-001 already fixed for auth/issue endpoints was never applied here.

## Fix

For `apiVersion == "2"` (Server/Data Center — same detection `resolveAPIVersion` already does), call the older `/rest/api/2/project` endpoint instead, which returns a flat JSON array (not `{values: [...]}`) — parse accordingly. Cloud (`v3`) keeps using `/project/search` unchanged.

## Related

- [CR-JIRA-001](../../../docs/crs/v3/jira-integration/CR-JIRA-001-support-self-hosted-jira-server-data-center.md) / [BUG-013](./BUG-013-jira-adapter-cloud-only-rejects-self-hosted-jira.md) — the same Cloud-vs-Server/DC API-shape duality, same file, this is the next endpoint that needed the same treatment and wasn't caught because `ListProjects` wasn't exercised by BUG-013's live verification (only `Whoami`/`serverInfo` were).
- [BUG-016](./BUG-016-jira-adapter-never-maps-project-issuetype-assignee-fields.md) — a DIFFERENT, already-fixed bug in the same adapter file, in the same user-facing flow (both surfaced as the frontend's identical generic "Couldn't load Jira issues... Cannot read properties of undefined (reading 'key')" message — misleading, since the actual failures are unrelated to each other and happen at different steps of the same flow: BUG-016 was in issue search's field mapping, this one is in the preceding project-list step).

# BUG-005: Empty list results serialize as JSON `null` instead of `[]` across multiple `wscompat` channels

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_tenant_project.go`, `channels_repo_ssh_status_workspace.go`, `channels_team.go`, `channels_credentials.go` (pattern likely present in more files — not exhaustively checked, see below)
**Severity:** Medium — every real frontend consumer of these lists does `.length`/`.map()`/spread on the result; a bare `null` where `[]` is expected crashes the UI on the very first empty-state render (a brand-new tenant, an account with no groups/targets/teams/credentials yet)
**Symptom:**
  - `projectGroup.list` → `{"ok":true,"result":null}` (frontend expects `{groups: [...]}`)
  - `ssh.listTargets` → `{"ok":true,"result":null}` (frontend expects `{targets: [...]}`)
  - `team.list` → `{"ok":true,"result":null}` (frontend expects an array)
  - `credentials.list` → `{"ok":true,"result":{"services":null,"mode":"server"}}` (frontend expects `services: []`)
**Status:** 🔴 Open — found live 2026-08-27 via `tests/client/rpc-catalog.spec.ts` against `172.20.2.39:6769`; root cause confirmed by source inspection (Go language semantics, not deployment-specific).

---

## Description

Each affected handler returns a proto-generated slice getter (or a
locally-declared `var xs []T`) directly as the RPC result:

```go
// channels_tenant_project.go:406-415 (projectGroup.list)
resp, err := client.ListProjectGroups(rpcCtx, &projectv1.ListProjectGroupsRequest{})
// ...
return resp.GetGroups(), nil

// channels_repo_ssh_status_workspace.go:331-340 (ssh.listTargets)
resp, err := client.ListSshTargets(rpcCtx, &infrafleetv1.ListSshTargetsRequest{})
// ...
return resp.GetSshTargets(), nil

// channels_team.go:45-52 (team.list)
resp, err := client.ListTeams(ctx, &tenantv1.ListTeamsRequest{})
// ...
return resp.GetTeams(), nil

// channels_credentials.go:226-236 (credentials.list)
var services []string
// ... appended to conditionally, e.g. only if a provider has a credential ...
```

Proto3's generated getter for an unset/empty `repeated` field returns Go's
`nil` slice (proto3 has no "empty vs. unset" distinction for repeated
fields — the getter just returns the underlying struct field). Likewise, a
plain `var services []string` that nothing gets appended to stays `nil`.
Go's `encoding/json` marshals a `nil` slice as JSON `null`, not `[]` —
different from an explicitly-initialized `[]T{}`, which marshals to `[]`.

`devServer.list` is the counterexample proving this isn't universal:
live-verified the same session's `devServer.list` call correctly returned
`result: []` for an empty list — some handler in this codebase already
gets this right (worth checking its exact pattern as a reference for the
fix, though not read as part of this report).

## Confirmed

- Source-read all 4 handler sites above; each returns a Go `nil` slice on
  the empty-result path, purely from Go/proto3 semantics — no deployment
  state needed to confirm this half of the bug.
- Live-verified 2026-08-27 against `172.20.2.39:6769`: all 4 calls above
  reproduced the exact `null` results quoted, for the same authenticated
  admin session used in BUG-002/BUG-003/BUG-004 (a tenant with no
  project groups, SSH targets, teams, or credentials yet — the natural
  empty state this bug hits).
- **Not exhaustively checked**: this pattern (`return resp.GetXxx(), nil`
  for a list RPC) likely recurs in other `wscompat` channel files beyond
  the 4 confirmed here — a full sweep (`grep -rn 'return resp\.Get.*(), nil'`
  across `internal/adapter/wscompat/channels_*.go`, cross-checked against
  which ones can plausibly return empty) would find the full set. Flagged
  as follow-up scope, not done here to keep this report grounded only in
  what was actually reproduced live.

## Suggested Fix

Fix at the boundary rather than per-handler, to close the whole class at
once and prevent regressions in new channels:
- Add a response-encoding step in the dispatch path (`handler.go`'s
  write-back, or a wrapper `Registry.Dispatch` applies to every handler's
  return value) that recursively replaces nil slices with empty slices
  before JSON-encoding — e.g. via a small reflection-based normalizer, or
  by having `Registry.Register` wrap handlers so their `any` return value
  passes through a `normalizeNilSlices(v any) any` pass.
- Failing a boundary-level fix, each handler needs
  `if x := resp.GetXxx(); x != nil { return x, nil }; return []T{}, nil`
  (or the credentials.list equivalent: initialize `services :=
  []string{}` instead of `var services []string`) — mechanical but must be
  applied at every call site, including ones not yet found by this report.

## Regression Test Gap

None of the existing `channels_*_test.go` fake-client tests appear to
exercise the **empty-result** case for these 4 handlers (based on the
`repo.list`/`worktree.list` test excerpts seen while investigating BUG-001/
BUG-003, which always stub a non-empty response) — a test asserting
`json.Marshal(result) == "[]"` (not `"null"`) for an empty upstream
response would have caught this before deploy.

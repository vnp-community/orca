# BUG-003: `project.rego` OPA policy evaluation fails for any project ID — blocks `repo.list`/`worktree.list`/every `requireProjectAccess`-gated RPC

**Service:** `project-service`
**File:** `internal/usecase/authorization.go`
**Severity:** High — blocks every RPC gated by `requireProjectAccess` (`repo.list`, `worktree.list`, and per `authorization.go`'s own doc comment, `GetProject`/`UpdateProject`/`DeleteProject`/`AddMember`/`RebindDevServer`/`AddRepo`/`RemoveRepo`/`ReorderRepos` too), not just the two confirmed live
**Symptom:** `repo.list`/`worktree.list`, called with a syntactically valid (but non-existent) `projectId`, both fail identically:
```
Internal: PROJECT_POLICY_EVAL_FAILED: failed to evaluate authorization policy
```
**Status:** 🔴 Open, root cause confirmed — found live 2026-08-27 via `tests/client/rpc-catalog.spec.ts` against `172.20.2.39:6769`; see [SOL-003](./solutions/SOL-003-embed-opa-bundle-in-container-images.md).

---

## Description

`requireProjectAccess` (`authorization.go:50`) resolves the caller's
membership role for a project, then asks OPA to decide:

```go
// authorization.go:57-73
callerProjectRole := ""
m, err := membership.GetMembership(ctx, projectID, actorID)
switch {
case errors.Is(err, domain.ErrMembershipNotFound):
	// callerProjectRole stays ""
case err != nil:
	return apperrors.New(apperrors.KindInternal, "PROJECT_MEMBERSHIP_LOOKUP_FAILED", "failed to resolve caller's project membership", err)
default:
	callerProjectRole = string(m.Role)
}

allowed, err := opa.Decision(ctx, callerProjectRole, callerGlobalRole(ctx), action)
if err != nil {
	return apperrors.New(apperrors.KindInternal, "PROJECT_POLICY_EVAL_FAILED", "failed to evaluate authorization policy", err)
}
```

For a project that genuinely doesn't exist, `GetMembership` correctly
returns `ErrMembershipNotFound` (handled — `callerProjectRole` stays `""`,
no error). The failure is in the **next** step: `opa.Decision(ctx, "", "",
action)` itself errors. This should be a normal, cheap "deny" decision
(empty role + empty global role + `project.rego`'s `action_roles` table has
no `""` entry ⇒ not allowed) — not an error. An error here means the OPA
call itself is failing (client misconfiguration, unreachable OPA
sidecar/bundle, or a bug in how `callerProjectRole`/`callerGlobalRole` are
marshaled into the query), not a policy decision.

Because this is a generic evaluation failure (not "denied"), it fails
**every** `requireProjectAccess`-gated call, for every project, regardless
of whether the caller has real membership — this isn't scoped to
non-existent projects specifically, just the only case cheaply
reproducible without a seeded real project (blocked by BUG-002/pre-existing
membership state on this deployment).

## Confirmed

- `services/project-service/internal/usecase/authorization.go:50-77` — full
  `requireProjectAccess` body, confirmed the exact error site and that
  `ErrMembershipNotFound` is NOT the branch that fired (that returns no
  error at all).
- Live-verified 2026-08-27 against `172.20.2.39:6769`, two independent
  channels, same result:
  - `repo.list` with `{"projectId":"00000000-0000-0000-0000-000000000000"}`
    → `PROJECT_POLICY_EVAL_FAILED`
  - `worktree.list` with the same params → `PROJECT_POLICY_EVAL_FAILED`
  - Differential check: the SAME calls with an **empty** `projectId`
    (`{}`, i.e. `ProjectID: ""`) instead fail one step earlier with
    `PROJECT_MEMBERSHIP_LOOKUP_FAILED` (a DB-level error, plausibly an
    invalid-UUID-literal error from passing `""` where the repository
    expects a UUID column value — a separate, minor input-validation gap:
    empty/malformed `projectId` should arguably be rejected as
    `invalid_argument` before ever reaching the repository query, not leak
    an opaque `Internal` DB error). This confirms `PROJECT_POLICY_EVAL_FAILED`
    is reached only once `projectId` is at least well-formed, i.e. it's
    specifically the OPA call that's broken, not an artifact of bad test input.

## Root Cause — CONFIRMED (updated after initial filing)

The client side can't see `err`'s underlying message, but the server-side
code is fully readable, and it resolves this cleanly — no logs needed.

`project-service`'s `OPAClient` (`internal/adapter/opaclient/client.go`)
correctly uses the shared **embedded, in-process** evaluator
(`common/policy.Evaluator`, `NewEvaluator(bundlePath)` +
`rego.PreparedEvalQuery`) per `07-security-architecture.md`'s "each service
calls OPA (embedded, in-process — no network hop...)" requirement — so the
earlier "OPA sidecar unreachable" hypothesis is **ruled out**; there is no
sidecar in this design at all.

The actual break is a **container packaging gap**:

- `internal/config/config.go:41` — `OPABundlePath` defaults to
  `commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz")`, a
  path relative to the process's working directory, correct only when run
  from `services/project-service/` in the monorepo checkout (e.g. local
  `go run`/`go test`).
- `services/project-service/deploy/Dockerfile` (multi-stage, distroless
  final image) copies only the compiled binary and
  `services/project-service/migrations` into the final stage:
  ```dockerfile
  FROM gcr.io/distroless/static-debian12:nonroot
  COPY --from=build /out/project-service /project-service
  COPY services/project-service/migrations /migrations
  USER nonroot:nonroot
  ENTRYPOINT ["/project-service"]
  ```
  `policy/orca-authz/` (the Rego bundle) is **never copied into the
  image**, and no `WORKDIR` is set, so at runtime `OPABundlePath`'s default
  relative path resolves against `/` and finds nothing regardless.
- `common/policy.Evaluator.Decision` → `preparedQuery` compiles the bundle
  lazily, on first use — a missing/unreadable bundle path fails there,
  wrapped by `authorization.go:34` into exactly `PROJECT_POLICY_EVAL_FAILED`.

**This is systemic, not project-service-specific**: `auth-service`,
`task-service`, and `annotation-service` all construct the same
`common/policy.Evaluator` (per that package's own doc comment listing all
4 consumers) and their `deploy/Dockerfile`s have the byte-for-byte
identical `COPY` list — none of them copies `policy/orca-authz` either.
Every service using embedded OPA is affected the same way.

See [SOL-003](./solutions/SOL-003-embed-opa-bundle-in-container-images.md)
for the fix.

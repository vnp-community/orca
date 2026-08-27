# SOL-CLI-01: A new `orca-cli` binary + CLI-token auth + REST worktree-create route

**Resolves:** [BUG-CLI-01](../BUG-CLI-01-tao-worktree-cli-not-implemented.md)
**Service:** `api-gateway` (REST route + CLI-token auth route) + `git-gateway-service` (idempotency) + a genuinely new client binary, `backend-go/cmd/orca-cli/`
**Affected files (proposed):**
- `backend-go/cmd/orca-cli/` (new module: `main.go`, `internal/apiclient/`, `internal/command/`, `internal/output/`, `internal/config/`)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` (new `POST /auth/cli-token`)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go` (new `POST /v1/worktrees`)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`, `ports.go` (idempotency check)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree_test.go`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes_test.go`, `auth_routes_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**Why a new binary, not reusing `desktop/src/cli/`.** `api-gateway.md` §10 states Electron desktop mode is out of this doc set's scope and never routes through `api-gateway` at all; `desktop/src/cli/`'s `RuntimeClient` talks to the Electron app's own `runtime.sock` (confirmed live: `desktop/src/cli/runtime/transport.test.ts:33` — `join(userDataPath, 'runtime.sock')`), a different transport for a different deployment target. BUG-CLI-01 already establishes there is no CLI package anywhere under `backend-go/`. §7 of `api-gateway.md` explicitly lists "CLI clients (REST, JWT)" as one of the three caller classes `api-gateway` is designed to serve — the TDD assumes a REST+JWT CLI exists, it just was never built. This solution builds it.

**Why REST, not a new WS client.** BUG-CLI-01's gap list is explicit: `worktree.create` is real but reachable only over the stateful WS JSON-RPC protocol `wscompat` speaks with the frontend, "not something a shell script can `curl`." `api-gateway.md` §3 draws the REST/WS line by *use case*, not by capability: "Real-time surfaces are WS at the edge" (terminal streams, notifications, agent status *push*) — worktree creation is not a streaming operation, so it belongs on the REST facade, consistent with every other synchronous git/project mutation already on `mountGitRoutes`/`mountProjectRoutes`.

**Why hand-written REST, not `grpc-gateway` codegen.** `api-gateway.md` §3 states the *target* architecture generates REST from `google.api.http` proto annotations. The actual code at HEAD does not do this yet — `usage_routes.go`'s doc comment (the "ONE real end-to-end REST→gRPC reverse-proxy path in this scaffold") states plainly: "Production wiring should replace this hand-written translation with a grpc-gateway-generated mux... this scaffold hand-writes the equivalent routes to demonstrate the pattern without that codegen step." `git_routes.go`/`project_routes.go`/`infra_routes.go` all follow this same hand-written pattern today. This solution's new route matches the codebase's actual current convention, not the aspirational one — introducing `grpc-gateway` codegen for one route while sixteen others remain hand-written would be an inconsistent, out-of-scope infrastructure change for this bug.

**Why the CLI composes worktree-create → agent-spawn → prompt-inject itself, not `api-gateway`.** `api-gateway.md` §2 is explicit: "if a REST call needs data from two services, that composition belongs to the calling client or to a service exposing a composed read, not to a gateway orchestration layer" — and worktree-create (`git-gateway-service`) + agent-spawn + prompt-send (`infra-fleet-service`, per BUG-CLI-02/BUG-AG-01) are two different services with no shared transaction. `orca-cli` is exactly "the calling client" that clause anticipates; it performs the three-call sequence itself and reports a single JSON result, never asking `api-gateway` to grow a saga endpoint spanning `git-gateway-service` + `infra-fleet-service`.

**`--agent`/`--prompt` scope boundary, stated explicitly.** Per BUG-CLI-01's own "See also", `--agent <type>` has no real spawn primitive to bind to until [BUG-AG-01](../BUG-AG-01-khoi-dong-agent-partial.md) is resolved (today's only spawn RPC, `SpawnTerminalSession`, launches a bare shell, not an agent binary). This solution's CLI wiring calls whatever spawn RPC exists at implementation time behind one seam (`internal/apiclient.SpawnAgent`) — today that means `orca worktree create --agent claude` degrades to spawning a generic shell in the new worktree and issuing a clear `AGENT_SPAWN_NOT_SUPPORTED` warning (exit 0 for the worktree, non-fatal), not a hard failure of the whole command; once BUG-AG-01 lands a real `agent.spawn`-shaped RPC, only `apiclient.SpawnAgent`'s implementation changes. This bug is scoped to making the worktree-creation and CLI-plumbing gap real; it does not re-solve BUG-AG-01.

---

## Design — proto & usecase: idempotency (BR-CLI-01)

`create_worktree.go:41-71`'s `Execute` has no dedupe check today — a second call with the same `(project_id, repo_id, branch)` re-runs `git worktree add` and fails, per BUG-WT-01's own duplicate-path finding (that bug owns the general validation gaps; this solution adds only the idempotency check BR-CLI-01 needs for a scriptable CLI, which is meaningless for a GUI click but essential for CI retries).

```protobuf
// gitgateway.proto — CreateWorktreeRequest, extended
message CreateWorktreeRequest {
  // ... existing fields unchanged ...
  optional string idempotency_key = 12; // caller-supplied; orca-cli defaults
                                         // to sha256(project_id|repo_id|branch)
                                         // when the user passes none
}
```

```go
// internal/usecase/ports.go (extended)
type ProjectClient interface {
    // ... existing methods ...
    FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error)
}
```

```go
// internal/usecase/create_worktree.go (extended Execute, prepended step)
func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
    if in.IdempotencyKey != "" {
        if existing, found, err := uc.projects.FindWorktreeByIdempotencyKey(ctx, in.ProjectID, in.IdempotencyKey); err != nil {
            return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_IDEMPOTENCY_LOOKUP_FAILED", "failed to check for existing worktree", err)
        } else if found {
            // BR-CLI-01: same inputs -> return the existing worktree, not a
            // second git-worktree-add attempt.
            return domain.WorktreeResult{WorktreeID: existing.ID, Path: existing.Path, HeadSHA: existing.HeadSHA}, nil
        }
    }
    repo, err := uc.projects.GetRepo(ctx, in.RepoID)
    // ... unchanged from here ...
}
```

`FindWorktreeByIdempotencyKey`'s `project-service` implementation stores `idempotency_key` as a new nullable, uniquely-indexed column on `worktrees` (one migration, `project-service/migrations/`), set from `RecordWorktreeCreated`'s existing lineage-capture parameters (`CreateWorktreeRequest`'s `optional` fields already forward context to that call per `gitgateway.proto:610-621` — `idempotency_key` follows the identical forwarding path, no new saga step).

---

## Design — `api-gateway` REST wiring

### `POST /v1/worktrees` — the actual git operation, not bookkeeping

`project_routes.go:37`'s `POST /{id}/worktrees` calls `RecordWorktreeCreated` only (bookkeeping). This solution adds the sibling route that performs the real `git worktree add`, following `git_routes.go:22-30`'s exact pattern (chi router, `gatewaygrpc.AttachIdentity`, `writeGRPCError`):

```go
// git_routes.go — mountGitRoutes, new route
func mountGitRoutes(r chi.Router, client gitgatewayv1.GitGatewayServiceClient) {
	r.Route("/v1/git", func(sub chi.Router) { /* unchanged */ })
	r.Post("/v1/worktrees", handleCreateWorktree(client)) // new
}

type createWorktreeRequestBody struct {
	ProjectID       string `json:"project_id"`
	RepoID          string `json:"repo_id"`
	Branch          string `json:"branch"`
	BaseRef         string `json:"base_ref"`
	IdempotencyKey  string `json:"idempotency_key"`
}

func handleCreateWorktree(client gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		var body createWorktreeRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}
		if body.ProjectID == "" || body.RepoID == "" || body.Branch == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "project_id, repo_id, and branch are required")
			return
		}
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: body.ProjectID, RepoId: body.RepoID, Branch: body.Branch,
			BaseRef: body.BaseRef, IdempotencyKey: &body.IdempotencyKey,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}
```

Placed under `/v1/worktrees` (top-level, not nested under `/v1/git`) because the resource is a worktree, not a git operation — consistent with `project_routes.go`'s existing `/v1/projects/{id}/worktrees` naming for the bookkeeping-only view; this route is the creation-with-side-effects counterpart `git-gateway-service` (not `project-service`) owns, so it does not nest under `/v1/projects/{id}` (that would misattribute ownership to `project-service`, the same mistake the two-split-endpoints history under `BUG-031` already documents).

### `POST /auth/cli-token` — JWT issuance for non-browser callers

`api-gateway.md` §9 / `04-tech-stack.md`'s Auth row require CLI callers to authenticate via short-lived RS256 JWT, not the browser session cookie `/auth/local` issues today (`auth_routes.go:57-78` calls `Login` then `setSessionCookie`, never returning a bearer token in the body). `auth-service`'s proto already has the RPC this needs, unused by any route today: `IssueServiceToken(user_id, audience) -> {jwt, expires_at}` (`backend-go/proto/orca/auth/v1/auth.proto:82-90`). This solution wires it:

```go
// auth_routes.go — mountAuthRoutes, new route
mux.Post("/auth/cli-token", func(w http.ResponseWriter, r *http.Request) {
	var body loginRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}
	loginResp, err := authClient.Login(r.Context(), &authv1.LoginRequest{Email: body.Email, Password: body.Password})
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	tokenResp, err := authClient.IssueServiceToken(r.Context(), &authv1.IssueServiceTokenRequest{
		UserId: loginResp.GetUser().GetId(), Audience: "cli",
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	// No cookie set here — deliberate. A CLI/CI caller stores the JWT
	// itself (orca-cli writes it to ~/.config/orca/credentials.json,
	// 0600); a cookie would be silently dropped by any non-browser client.
	writeJSON(w, http.StatusOK, map[string]any{
		"jwt": tokenResp.GetJwt(), "expires_at": tokenResp.GetExpiresAt(),
		"user": toAuthUserResponse(loginResp.GetUser()),
	})
})
```

Not behind `authMiddleware` (same as `/auth/local`) — this route *establishes* identity, matching `router.go`'s existing carve-out for `/auth/*`.

---

## Design — `backend-go/cmd/orca-cli/`: a new client binary

Not a service: no `.proto` of its own, no gRPC/HTTP server, no database — it is a REST+JWT client, so it does not fit any of the 17 entries in `02-microservices-decomposition.md`'s catalog and is deliberately placed outside `services/`, at repo-top-level `backend-go/cmd/`, its own `go.mod` per `04-tech-stack.md`'s "one Go module per deployable" convention extended to a client tool (no service depends on it; it only imports generated proto/REST-shape types, never `internal/` packages of any service, so it cannot violate the dependency-inversion rule `03-clean-architecture-guidelines.md` states for services).

```
backend-go/cmd/orca-cli/
├── main.go                        # composition root: parse flags, build apiclient, dispatch command
├── internal/
│   ├── config/
│   │   └── credentials.go         # ~/.config/orca/credentials.json (JWT + expiry), 0600, ORCA_API_URL / ORCA_API_TOKEN env overrides
│   ├── apiclient/                 # thin REST client — one method per endpoint this CLI calls
│   │   ├── client.go              # http.Client + bearer-token injection + JSON marshal/unmarshal
│   │   ├── auth.go                # Login(email, password) -> stores JWT
│   │   ├── worktree.go            # CreateWorktree(...)
│   │   └── errors.go              # maps {code, message} JSON error bodies (writeJSONError's shape) -> typed CLIError
│   ├── command/
│   │   ├── worktree_create.go     # `orca worktree create` — composes CreateWorktree -> (best-effort) SpawnAgent -> SendPrompt
│   │   └── root.go                # command tree registration (cobra), shared --json/--worktree flags
│   └── output/
│       └── output.go              # JSON vs. human formatting, exit-code mapping (see BR-CLI-02/03 table below)
└── go.mod
```

`worktree_create.go`'s composition, the shape every subsequent CLI command in BUG-CLI-02/03 follows for multi-call flows:

```go
func RunWorktreeCreate(ctx context.Context, cli *apiclient.Client, opts WorktreeCreateOptions) (Result, error) {
	wt, err := cli.CreateWorktree(ctx, apiclient.CreateWorktreeInput{
		ProjectID: opts.ProjectID, RepoID: opts.RepoID, Branch: opts.Name, BaseRef: opts.Base,
		IdempotencyKey: opts.IdempotencyKey(), // sha256(project|repo|branch) if user passed none — BR-CLI-01
	})
	if err != nil {
		return Result{}, err // exit 1, see output.go
	}
	result := Result{WorktreeID: wt.WorktreeID, Path: wt.Path, HeadSHA: wt.HeadSHA}
	if opts.Agent == "" {
		return result, nil
	}
	// Best-effort — see "Design rationale" §"--agent/--prompt scope boundary".
	spawn, err := cli.SpawnAgent(ctx, wt.WorktreeID, opts.Agent)
	if err != nil {
		result.Warnings = append(result.Warnings, "AGENT_SPAWN_NOT_SUPPORTED: "+err.Error())
		return result, nil // worktree succeeded; exit 0 with a warning, not exit 1
	}
	result.PtyID = spawn.PtyID
	if opts.Prompt != "" {
		if err := cli.SendTerminalInput(ctx, spawn.PtyID, opts.Prompt); err != nil {
			result.Warnings = append(result.Warnings, "PROMPT_SEND_FAILED: "+err.Error())
		}
	}
	return result, nil
}
```

### BR-CLI-02/03 — `--json` and exit codes

| Outcome | `--json` stdout | Exit code |
|---|---|---|
| Success (worktree created, agent/prompt steps either skipped or succeeded) | `{"worktreeId","path","headSha","ptyId?","warnings":[]}` | 0 |
| Success with a non-fatal `--agent`/`--prompt` warning | same shape, `warnings` populated | 0 (per BR-CLI-01's framing: the requested worktree operation succeeded) |
| Client-side / usage error (bad flags, missing required arg) | `{"error":{"code":"INVALID_ARGUMENT","message":...}}` | 2 |
| Server error (4xx/5xx from `api-gateway`, network failure) | `{"error":{"code":<gateway's code>,"message":...}}` | 1 |

`writeJSONError`'s existing `{code, message}` shape (`git_routes.go` and every other handler already use it) is reused verbatim as the CLI's machine-parseable error body — BR-CLI-04's "dual human+machine-parseable error messages" is satisfied by printing the same JSON to stdout in `--json` mode and a formatted `<code>: <message>` line to stderr otherwise, one `output.ReportError` call, not two divergent error paths.

---

## Test plan

- `git-gateway-service/internal/usecase/create_worktree_test.go` — new case: second `Execute` call with the same `idempotency_key` returns the first call's `WorktreeResult` without invoking the fake `GitExecutor` a second time (assert zero additional calls) — the core BR-CLI-01 regression guard.
- `project-service/internal/adapter/postgres/worktree_repository_test.go` — `FindWorktreeByIdempotencyKey` round-trips against `testcontainers-go` Postgres; unique-index violation on duplicate key surfaces as a typed conflict, not a raw SQL error.
- `api-gateway/internal/adapter/httpgateway/git_routes_test.go` — `POST /v1/worktrees`: happy path returns 201 + the RPC response; missing `branch` returns 400 `INVALID_ARGUMENT` without calling the fake gRPC client; `AttachIdentity` invoked before the call (never trusts a body-supplied tenant/project mismatch).
- `api-gateway/internal/adapter/httpgateway/auth_routes_test.go` — `POST /auth/cli-token`: valid credentials -> `{jwt, expires_at, user}` with **no** `Set-Cookie` header (regression guard distinguishing this from `/auth/local`); invalid credentials -> 401, `IssueServiceToken` never called (assert on the fake).
- `backend-go/cmd/orca-cli/internal/command/worktree_create_test.go` — fake `apiclient.Client`: `--agent` failure produces exit 0 + populated `warnings`, never exit 1; missing required flags produce exit 2 without any HTTP call; `--json` output is valid JSON matching the table above byte-for-byte on a fixed fake response.
- `backend-go/cmd/orca-cli/internal/apiclient/errors_test.go` — a `writeJSONError`-shaped 400/401/500 body maps to the correct `CLIError.Code`/exit-code bucket.
- End-to-end (per `03-clean-architecture-guidelines.md`'s testing-implications, "a small number of cross-service scenarios... against a full docker-compose stack"): `orca-cli worktree create --base main --json` against a `docker-compose`-launched `api-gateway` + `git-gateway-service` + `project-service`, asserting a real worktree directory exists on disk afterward.

## References

- `specs/backend-go/bugs/logic-v1/BUG-CLI-01-tao-worktree-cli-not-implemented.md` — problem statement and the "no CLI binary in backend-go's scope" finding
- `specs/backend-go/tdd/services/api-gateway.md:2` (§1 caller classes incl. "CLI clients"), `§2 lines 31-39` (cross-service composition belongs to the client), `§3 lines 65-100` (REST facade / WS-vs-REST split), `§9 lines 284-315` (JWT auth for CLI/mobile), `§10 lines 351-357` (Electron desktop out of scope)
- `specs/backend-go/tdd/architecture/04-tech-stack.md` — "Auth & policy" row (short-lived RS256 JWT for CLI), "Module strategy" row (one Go module per deployable)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — dependency-inversion rule cited for why `orca-cli` sits outside every service's `internal/`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/git_routes.go:22-30` — existing hand-written REST→gRPC pattern this solution's new route follows
- `backend-go/services/api-gateway/internal/adapter/httpgateway/usage_routes.go:20-28` — doc comment establishing hand-written-not-grpc-gateway as the actual current convention
- `backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go:37` — the bookkeeping-only sibling route this solution's `POST /v1/worktrees` complements
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go:56-78` — `/auth/local`'s cookie-only pattern, contrasted with this solution's new `/auth/cli-token`
- `backend-go/proto/orca/auth/v1/auth.proto:16,82-90` — `IssueServiceToken`, real and already defined, previously unwired to any route
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:41-71` — `Execute`, extended with the idempotency-key check
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:604-627` — `CreateWorktreeRequest`/`Response`, extended with `idempotency_key`
- `desktop/src/cli/runtime/transport.test.ts:33` — confirms the real `orca` CLI's transport target is `runtime.sock`, not backend-go
- `specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md`, `BUG-WT-01-tao-worktree-partial.md` — explicitly out of this solution's scope, cited for the `--agent` degrade-gracefully boundary

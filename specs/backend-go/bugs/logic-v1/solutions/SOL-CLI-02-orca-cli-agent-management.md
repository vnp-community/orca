# SOL-CLI-02: Worktree-scoped REST agent endpoints + `orca agent`/`orca snapshot` in `orca-cli`

**Resolves:** [BUG-CLI-02](../BUG-CLI-02-quan-ly-agent-cli-not-implemented.md)
**Service:** `infra-fleet-service` (new resolution/send/scrollback RPCs) + `api-gateway` (REST wiring) + `backend-go/cmd/orca-cli/` (new commands, built on [SOL-CLI-01](./SOL-CLI-01-orca-cli-worktree-create.md)'s binary)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (3 new RPCs)
- `backend-go/services/infra-fleet-service/internal/usecase/get_agent_terminal_session.go`, `send_terminal_input.go`, `get_terminal_scrollback.go` (new use cases)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend if needed for scrollback buffer access)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (3 new RPC handlers)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go` (new `/v1/worktrees/{worktreeId}/agent/*` routes)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes_test.go`
- `backend-go/cmd/orca-cli/internal/apiclient/agent.go`, `internal/command/agent_status.go`, `agent_wait.go`, `agent_send.go`, `snapshot.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

BUG-CLI-02 itself draws the line precisely: "this is a 'no CLI wrapper exists', not a 'no backend capability exists' gap" — `terminal.agentStatus`, `terminal.wait`, `terminal.send`, and scrollback capture are all real, gRPC-backed, tested operations on `infra-fleet-service` (`channels_terminal.go:284-301,392-418,434-478`). What's actually missing is threefold, and each closes with a different amount of new surface:

1. **`--worktree <id>` → `ptyId` resolution** — closeable with **zero new state**, using an RPC that already exists for a different caller. `ResolveConnection` (`infrafleet.proto:139-155`) already resolves `worktree_id → {connection_id, repo_path}` — its doc comment states it backs "api-gateway's `browser.*` relay channels", a different feature, but the resolution semantics are identical to what `--worktree <id>` needs. Combined with `ListTerminalSessions(connection_id)` (`infrafleet.proto:318-319`, `TerminalSession.cwd`), matching `cwd == repo_path` finds "the" terminal for that worktree without inventing a new worktree↔pty mapping to keep consistent. This composition is genuine business logic (matching, tie-breaking by `last_active_at_unix_ms`), so per `03-clean-architecture-guidelines.md`'s usecase layer contract ("if an inbound handler has an `if` statement deciding business behavior... belongs in `usecase/`") it is added as one new `infra-fleet-service` usecase, not as `if`-logic in `api-gateway`'s REST handler or in `orca-cli` itself.
2. **REST route surface** — `api-gateway.md` §3's REST-vs-WS split (real-time push → WS; request/response → REST) puts all four of these squarely on REST: `agentStatus`/`wait`/`send`/scrollback are each one request, one response, no server-push framing, unlike `terminal.multiplex`'s live-viewport streaming. `infra_routes.go` already hand-writes exactly this shape for `ResolveConnection` (`POST /v1/infra/connections/resolve`) and four other `infra-fleet-service` RPCs — this solution extends the same file with the same pattern, resource-nested per `api-gateway.md` §3's "nested along the logical-FK relationships" rule: `/v1/worktrees/{worktreeId}/agent/*`, not a flat `?worktreeId=` query param, and not nested under `/v1/infra` (the resource is "this worktree's agent", not "this fleet operation").
3. **Plain-text scrollback** — `SnapshotRequest`'s binary multiplex-frame buffer (`channels_terminal_multiplex.go:18-28,257`) already holds the bytes `orca snapshot` needs; this solution adds a thin RPC that reads the *same* buffer and reassembles it as one string, rather than a second, competing capture mechanism — flagged explicitly as new surface, but a read-only view over existing state, not new state.

**Unary send, no held stream.** `terminal.send` writes into a live, client-attached `AttachPty` bidi stream (`channels_terminal.go:284-301`) that only exists while a WS session holds it open — a stateless REST call from a CI script cannot attach to or reuse that stream. `GetTerminalAgentStatus`/`WaitTerminalSession`/`ResizeTerminalSession` are already unary RPCs that operate on a `pty_id` without requiring an attached stream (`infrafleet.proto:315,321-325,329-334`) — a unary `SendTerminalInput(pty_id, data)` is the same shape, not a new pattern, and is this solution's only proposed extension to the RPC surface beyond what §"1" above already needs.

---

## Design — proto additions (`infrafleet.proto`)

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged ...

  // GetAgentTerminalSession resolves worktree_id -> the live TerminalSession
  // whose cwd matches that worktree's path, if one exists. Composes
  // ResolveConnection + ListTerminalSessions server-side (see this
  // solution's rationale §1) so no caller re-derives the match/tie-break
  // logic itself.
  rpc GetAgentTerminalSession(GetAgentTerminalSessionRequest) returns (GetAgentTerminalSessionResponse);

  // SendTerminalInput writes directly to the pty's input, bypassing
  // AttachPty's stream — for stateless (REST/CLI) callers that never
  // attach. GUI callers keep using terminal.send/AttachPty for lower
  // latency; this is not a replacement for that path.
  rpc SendTerminalInput(SendTerminalInputRequest) returns (google.protobuf.Empty);

  // GetTerminalScrollback reads the same recovery buffer SnapshotRequest
  // uses (channels_terminal_multiplex.go) and reassembles it as flat text,
  // for callers that want a redirectable string, not multiplex frames.
  rpc GetTerminalScrollback(GetTerminalScrollbackRequest) returns (GetTerminalScrollbackResponse);
}

message GetAgentTerminalSessionRequest  { string worktree_id = 1; }
message GetAgentTerminalSessionResponse {
  bool found = 1;
  TerminalSession session = 2; // unset when found=false
}

message SendTerminalInputRequest { string pty_id = 1; bytes data = 2; }

message GetTerminalScrollbackRequest  { string pty_id = 1; }
message GetTerminalScrollbackResponse {
  string text = 1;
  bool   truncated = 2; // true if the buffer's own retention bound (per
                         // channels_terminal_multiplex.go) already dropped
                         // earlier output — an honest signal, not silently
                         // partial data
}
```

---

## Design — `infra-fleet-service` usecase layer

```go
// internal/usecase/get_agent_terminal_session.go
type GetAgentTerminalSession struct {
    connections ConnectionResolver   // existing port, backs ResolveConnection
    sessions    TerminalSessionStore // existing in-memory registry, backs ListTerminalSessions
}

func (uc *GetAgentTerminalSession) Execute(ctx context.Context, worktreeID string) (domain.TerminalSession, bool, error) {
    conn, err := uc.connections.Resolve(ctx, domain.ResolveConnectionInput{WorktreeID: worktreeID})
    if err != nil {
        return domain.TerminalSession{}, false, err
    }
    sessions, err := uc.sessions.List(ctx, conn.ConnectionID)
    if err != nil {
        return domain.TerminalSession{}, false, err
    }
    var best domain.TerminalSession
    found := false
    for _, s := range sessions {
        // Exact match only — a subdirectory cwd is a different terminal
        // (e.g. the user cd'd into a subfolder), not "the" agent session.
        if s.Cwd != conn.RepoPath {
            continue
        }
        if !found || s.LastActiveAtUnixMs > best.LastActiveAtUnixMs {
            best, found = s, true
        }
    }
    return best, found, nil
}
```

`SendTerminalInput`'s usecase is a one-line pass-through to the same `DevServerAgentClient`/local-pty write path `terminal.send`'s server side already uses for `AttachPty`'s `PtyInput` frame — no new I/O primitive, just a second entry point to it (`pty.write`, per `BUG-AG-01`'s own grep of `devserveragent/methods.go`'s known JSON-RPC methods).

`GetTerminalScrollback`'s usecase reads `channels_terminal_multiplex.go`'s existing recovery buffer through whatever port already backs `SnapshotRequest` server-side, concatenates the buffered output chunks in order, and returns the flat string plus a `truncated` flag sourced from that buffer's existing retention-bound signal (no new buffer, no new retention policy).

---

## Design — `api-gateway` REST wiring

```go
// infra_routes.go — mountInfraRoutes, extended
func mountInfraRoutes(r chi.Router, client infrafleetv1.InfraFleetServiceClient) {
	r.Route("/v1/infra", func(sub chi.Router) { /* unchanged */ })

	r.Route("/v1/worktrees/{worktreeId}/agent", func(sub chi.Router) {
		sub.Get("/status", handleGetAgentStatus(client))
		sub.Post("/wait", handleWaitAgent(client))
		sub.Post("/send", handleSendAgentInput(client))
		sub.Get("/snapshot", handleGetAgentSnapshot(client))
	})
}

// resolveAgentPtyID is the one shared step all four handlers below open
// with — GetAgentTerminalSession, mapped to a clear 404 when no live
// session exists (an ephemeral PTY that already exited is not a bug, it's
// the expected state for a finished agent run).
func resolveAgentPtyID(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, worktreeID string) (string, error) {
	resp, err := client.GetAgentTerminalSession(ctx, &infrafleetv1.GetAgentTerminalSessionRequest{WorktreeId: worktreeID})
	if err != nil {
		return "", err
	}
	if !resp.GetFound() {
		return "", errNoActiveAgentSession // handler maps to 404 NOT_FOUND
	}
	return resp.GetSession().GetPtyId(), nil
}

func handleGetAgentStatus(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err) // 404 NOT_FOUND / passthrough gRPC error
			return
		}
		resp, err := client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: ptyID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleWaitAgent(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		var body struct{ TimeoutMs int32 `json:"timeout_ms"` }
		if !decodeJSONBody(w, r, &body) {
			return
		}
		resp, err := client.WaitTerminalSession(ctx, &infrafleetv1.WaitTerminalSessionRequest{PtyId: ptyID, TimeoutMs: body.TimeoutMs})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp) // {exited, exit_code, timed_out} — BR-CLI-05's exit-code-2 mapping happens in orca-cli, see below
	}
}

func handleGetAgentSnapshot(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		resp, err := client.GetTerminalScrollback(ctx, &infrafleetv1.GetTerminalScrollbackRequest{PtyId: ptyID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		// BR-CLI-06's "flat scrollback file" — text/plain, not JSON, so
		// `orca snapshot --output result.txt` is a literal body copy, not a
		// JSON field the CLI has to unwrap.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if resp.GetTruncated() {
			w.Header().Set("X-Orca-Snapshot-Truncated", "true")
		}
		_, _ = w.Write([]byte(resp.GetText()))
	}
}

// handleSendAgentInput follows the same resolve-then-call shape; omitted
// for brevity — decodes {"text": "..."} and calls SendTerminalInput.
```

`writeAgentResolveError` maps `errNoActiveAgentSession` to `404 NOT_FOUND` with a message the CLI surfaces verbatim ("no active agent session for worktree <id> — has an agent been started?"), distinct from `resp.GetSession()` being merely idle (that's a `200` with `agent_running=false`, not a 404).

---

## Design — `orca-cli` commands

Built on [SOL-CLI-01](./SOL-CLI-01-orca-cli-worktree-create.md)'s `internal/apiclient`/`internal/output` scaffolding — `agent.go` adds four thin methods (`GetAgentStatus`, `WaitAgent`, `SendAgentInput`, `GetAgentSnapshot`), each one REST call, no client-side composition needed since `resolveAgentPtyID` already moved server-side.

### BR-CLI-05 — `orca agent wait --timeout` exit-code-2 mapping

```go
// internal/command/agent_wait.go
func RunAgentWait(ctx context.Context, cli *apiclient.Client, worktreeID string, timeout time.Duration) (Result, int) {
	resp, err := cli.WaitAgent(ctx, worktreeID, timeout)
	if err != nil {
		return Result{}, exitCodeForError(err) // 1 or 2 per SOL-CLI-01's table
	}
	if resp.TimedOut {
		return Result{Exited: false, TimedOut: true}, 2 // BR-CLI-05, the one CLI-specific exit-code rule beyond SOL-CLI-01's generic table
	}
	return Result{Exited: resp.Exited, ExitCode: resp.ExitCode}, 0
}
```

### BR-CLI-07 — GUI-concurrent verification

Nothing in the design above is exclusive: `GetAgentTerminalSession`/`GetTerminalAgentStatus`/`WaitTerminalSession` are all read/wait operations against the same `infra-fleet-service` state a GUI's `AttachPty` stream also reads; `SendTerminalInput` writes to the same pty the GUI's `terminal.send` writes to, both ending at the same underlying `pty.write` JSON-RPC call — concurrent writers were already possible before this solution (two GUI tabs attached to the same session), so no new synchronization is required. This closes BUG-CLI-02's "unverifiable" flag with a concrete test (below) rather than leaving it asserted.

---

## Test plan

- `infra-fleet-service/internal/usecase/get_agent_terminal_session_test.go` — fake `ConnectionResolver`/`TerminalSessionStore`: exact `cwd` match returns `found=true`; subdirectory `cwd` does not match; two sessions with matching `cwd` return the one with the higher `last_active_at_unix_ms`; no connection resolved (`worktree_id` unknown) returns `found=false`, not an error.
- `infra-fleet-service/internal/usecase/get_terminal_scrollback_test.go` — buffer at/under retention bound returns `truncated=false`; buffer that already dropped early chunks returns `truncated=true`; assembled text preserves chunk order.
- `infra-fleet-service/internal/usecase/send_terminal_input_test.go` — writes reach the fake `DevServerAgentClient`'s `pty.write` call with the exact bytes given, no framing added.
- `infra-fleet-service/internal/adapter/grpc/server_test.go` — contract tests for the three new RPCs against the extended proto.
- `api-gateway/internal/adapter/httpgateway/infra_routes_test.go` — one test per new route using a fake `InfraFleetServiceClient`: `GetAgentTerminalSession{found:false}` → `404` with the documented message, verified `GetTerminalAgentStatus`/`WaitTerminalSession`/etc. is never called afterward (regression guard against calling a stale/wrong `pty_id`); `GET .../snapshot` asserts `Content-Type: text/plain` and the truncated header only when the fake response says `truncated=true`.
- `orca-cli/internal/command/agent_wait_test.go` — `timed_out=true` maps to exit code 2 exactly, every other outcome maps per SOL-CLI-01's table; a fake clock/timeout does not leak into the exit-code decision (decided purely from the response body, not from client-side elapsed time).
- Cross-service concurrency test (docker-compose, per `03-clean-architecture-guidelines.md`'s testing-implications end-to-end tier): spawn an agent PTY, attach a simulated GUI WS client via `terminal.create`+`AttachPty`, concurrently call `orca-cli agent send` against the same worktree, assert both the GUI stream and the REST caller observe the same resulting PTY output — the concrete BR-CLI-07 verification BUG-CLI-02 flagged as missing.

## References

- `specs/backend-go/bugs/logic-v1/BUG-CLI-02-quan-ly-agent-cli-not-implemented.md` — problem statement; explicitly frames this as a wrapper gap, not a capability gap
- `specs/backend-go/tdd/services/api-gateway.md:65-100` (§3, REST-vs-WS split and resource-nesting rule)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase-owns-business-decisions rule, cited for why match/tie-break logic is a new `infra-fleet-service` usecase, not gateway `if`-logic
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:136-174` (`ResolveConnection`, reused as-is), `:297-334` (`SpawnTerminalSessionRequest.cwd`, `TerminalSession`, `WaitTerminalSessionRequest/Response`, `GetTerminalAgentStatus*`), `:315` (existing unary-RPC-without-a-stream precedent for the new `SendTerminalInput`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:284-301` (`terminal.send`'s stream-attached shape, contrasted with the new unary `SendTerminalInput`), `:392-418,434-478` (`terminal.wait`/`agentStatus`, reused via the new resolve-then-call REST handlers)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal_multiplex.go:18-28,257` (`SnapshotRequest`'s existing recovery buffer, reused by the new `GetTerminalScrollback`)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:20-31` (`mountInfraRoutes`'s existing pattern, incl. the already-wired `POST /v1/infra/connections/resolve` this solution's resolution logic builds on)
- `specs/backend-go/bugs/logic-v1/BUG-AG-01-khoi-dong-agent-partial.md` — cited for why "agent status" is limited to generic-PTY liveness until that bug is separately resolved; out of this solution's scope
- [`SOL-CLI-01`](./SOL-CLI-01-orca-cli-worktree-create.md) — the `orca-cli` binary, `apiclient`/`output` scaffolding, and exit-code table this solution's commands are built on

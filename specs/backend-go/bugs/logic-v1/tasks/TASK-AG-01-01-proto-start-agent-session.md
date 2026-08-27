# TASK-AG-01-01: Add `StartAgentSession` RPC + `AgentSession` message to `infrafleet.proto`

**From Solution:** SOL-AG-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`BUG-AG-01` needs a way to spawn an AI-CLI agent (not a bare shell) via the Dev Server Agent's already-real `agent.spawn` RPC. This task adds the proto surface only — additive, no breaking change.

## Changes to make

Add to the `InfraFleetService` service block, after the existing `SpawnTerminalSession`/`KillTerminalSession`/`StopTerminalProcess` RPCs:

```protobuf
  // StartAgentSession spawns an AI-CLI agent (Claude Code, Codex, ...) via
  // the Dev Server Agent's real agent.spawn RPC — sibling to
  // SpawnTerminalSession, not a replacement (a bare shell PTY still uses
  // SpawnTerminalSession).
  rpc StartAgentSession(StartAgentSessionRequest) returns (AgentSession);
```

Append the messages at the bottom of the file:

```protobuf
message StartAgentSessionRequest {
  string connection_id = 1;     // -> DevServer, same resolution as SpawnTerminalSession
  string worktree_id   = 2;     // logical FK -> project-service; BR-AG-01 scope key
  string user_id       = 3;     // BR-AG-01: one agent per worktree PER USER
  string cwd           = 4;     // worktree path on the dev server
  string model_id      = 5;     // e.g. "claude", "gpt-4o", "gemini", "opencode", "ollama"
  string account_id    = 6;     // ai_provider.accounts.id — from aiProvider.resolve; "" for localInference models
  string trust_preset  = 7;     // "standard" | "full" | "none" — forwarded as-is, applied agent-side as CLI args
  int32  cols          = 8;
  int32  rows          = 9;
  // resolved_api_key is DELIBERATELY ABSENT — see TASK-AG-01-04 (credential
  // injection blocker). Adding it here would let a caller populate
  // agent.spawn's resolvedApiKey param, silently reintroducing Gap 2.
}

message AgentSession {
  string id              = 1;   // sessionId — agent_sessions.id
  string pty_id          = 2;
  string connection_id   = 3;   // -> DevServer, same resolution key as TerminalSession.connection_id
  string worktree_id     = 4;
  string dev_server_id   = 5;   // snapshot of the dev server resolved at spawn time, for display only
  string user_id         = 6;
  string model_id        = 7;
  string account_id      = 8;
  string status          = 9;   // spawning|idle|running|waiting|completed|error|stopped — see TASK-AG-05-*
  int64  started_at_unix_ms      = 10;
  int64  last_active_at_unix_ms  = 11;
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only additions).

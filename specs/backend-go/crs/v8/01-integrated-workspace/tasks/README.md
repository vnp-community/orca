# Tasks — 01-integrated-workspace

Các tác vụ AI-executable được chia theo **phase thực thi** dựa trên dependency graph.

## Phase map

| Phase | CR | Tasks | Effort |
|-------|----|-------|--------|
| [Phase 1](phase-1/) | CR-ORCA-005 | T-P1-001, T-P1-002, T-P1-003 | 1 ngày |
| [Phase 2](phase-2/) | CR-ORCA-004 | T-P2-001, T-P2-002, T-P2-003, T-P2-004 | 2 ngày |
| [Phase 3](phase-3/) | CR-ORCA-001 | T-P3-001, T-P3-002, T-P3-003 | 2–3 ngày |
| [Phase 4](phase-4/) | CR-ORCA-002 | T-P4-001, T-P4-002, T-P4-003, T-P4-004 | 3–4 ngày |
| [Phase 5](phase-5/) | CR-ORCA-003 | T-P5-001, T-P5-002, T-P5-003, T-P5-004, T-P5-005 | 3–4 ngày |

## Dependency graph

```
Phase 1 (CR-ORCA-005): ws.Client.Call()          ← START HERE — unblocks compile
  → Phase 2 (CR-ORCA-004): Config + Executor
    → Phase 3 (CR-ORCA-001): PISTool wiring
      → Phase 4 (CR-ORCA-002): Task KGContext
        → Phase 5 (CR-ORCA-003): Quality Gate
```

## Task format

Mỗi task file gồm:
- `task_id`, `phase`, `cr_id`, `service`, `files` (cụ thể)
- `context`: background cần biết
- `instructions`: bước thực thi **chính xác** cho AI agent
- `acceptance_criteria`: điều kiện pass (compile + test)
- `test_commands`: lệnh verify sau khi làm xong

## Source solutions

→ [`solutions/`](../solutions/)

---

## Execution Log

**Session**: 2026-10-19 — Full implementation completed.

### Summary by task

| Task | Status | Files created/edited |
|------|--------|---------------------|
| T-P1-001 | ✅ done | `ws/client.go` — +`Call()`, +`SetPeerConfig()`, +3 fields |
| T-P1-002 | ✅ done | `ws/client_test.go` — 3 tests PASS |
| T-P1-003 | ✅ done | `pis/client.go` — unified `pis_*` names, +`UpdateCRSStatus()`, +`pis_query_adr` |
| T-P2-001 | ✅ done | `config/config.go` — +`MCPProxy` struct, +env overrides |
| T-P2-002 | ✅ done | `config/config_test.go` — 4 tests PASS |
| T-P2-003 | ✅ done | `mcp/executor.go` — +`pisTool` field, +`WithPISTool()`, +pis_* routing |
| T-P2-004 | ✅ done | `deploy/orca-agent/config/orca-agent.yaml`, `.env.example` |
| T-P3-001 | ✅ done | `mcp/catalog_pis.go` — 5 pis_* tool definitions |
| T-P3-002 | ✅ done | `mcp/executor_pis_test.go` — 6 tests PASS; bonus: `mcp/rbac.go` +pis_* RBAC entries |
| T-P3-003 | ✅ done | `cmd/orca-agent/main.go` — PISTool wiring + SetPeerConfig |
| T-P4-001 | ✅ done | `task/entity.go` — `TaskKGContext`, `Task`, `TaskStatus` |
| T-P4-002 | ⏸️ deferred | Requires orca-backend API service (separate repo) |
| T-P4-003 | ✅ done | `task/handler.go` — `EnrichedTask`, `HandleAssignment()`, `BuildSystemPrompt()` |
| T-P4-004 | ✅ done | `task/handler_test.go` — 4 tests PASS |
| T-P5-001 | ⏸️ deferred | Requires pikb-catalog-service (separate repo) |
| T-P5-002 | ⏸️ deferred | Requires products-agent (separate repo) |
| T-P5-003 | ✅ done | `task/completion_hook.go` — `CompletionHook`, `GitConventionVerifier`, `NewRejectedResponse()` |
| T-P5-004 | ✅ done | `task/completion_hook_test.go` — 6 tests PASS |
| T-P5-005 | ✅ done | `task/handler.go` — `SetCompletionHook()` wired |

### Build verification

```
GOWORK=off go build ./...    → EXIT 0
GOWORK=off go test ./...     → ALL PASS (audit, config, mcp, task, ws)
```

### Unexpected findings fixed

1. `pis/client.go`: tool names were `pis.query_crs` (dot) → fixed to `pis_query_crs` (underscore)
2. `mcp/rbac.go`: RBAC allowlist did not include pis_* tools → added with `minRole: "member"`
3. No `deploy/` dir in orca-agent repo → found at `vnp-workspace/deploy/orca-agent/`

# TASK INDEX — AI Execution Tasks

**Tạo:** 2026-08-01  
**Cập nhật:** 2026-08-01 (FINAL)  
**Mục đích:** Danh sách tất cả tasks được chia từ solutions, sắp xếp theo thứ tự thực thi  
**Implementation Status:** ✅ **27/29 tracked tasks DONE** | ⏸ 2 DEFERRED  
**Tests:** ✅ **458/458 PASS** | TypeScript: **0 errors** (relay + dev-server modules)  

---

## Cấu trúc Tasks

```
bugs/
├── TASK-INDEX.md                          ← File này
│
├── agent-orchestration/tasks/             (8 tasks — ALL ✅ DONE)
│   ├── TASK-ORCH-01   fix resolveAgentSpec               ✅ DONE
│   ├── TASK-ORCH-02   fix buildAgentEnv                  ✅ DONE
│   ├── TASK-ORCH-03   fix JSON-RPC notifications         ✅ DONE
│   ├── TASK-ORCH-04   add agent.sendInput                ✅ DONE
│   ├── TASK-ORCH-05   fix kill signal + PTY cleanup      ✅ DONE
│   ├── TASK-ORCH-06   fix relay token                    ✅ DONE
│   ├── TASK-ORCH-07   add agent.exec (generic)           ✅ DONE
│   └── TASK-ORCH-08   fix capabilities handshake         ✅ DONE
│
├── agent-ws/tasks/                        (3 tasks — ALL ✅ DONE)
│   ├── TASK-AWS-01    verify+fix handshake method        ✅ DONE (method='agent.handshake' already correct)
│   ├── TASK-AWS-02    SHA-256 token hashing in slots     ✅ DONE (agent-ws-server.ts + ws-handshake.ts)
│   └── TASK-AWS-03    authToken validation ws-session    ✅ DONE (FIX TASK-TRM-005 in ws-session-router.ts)
│
├── task-graph/tasks/                      (2 tasks — ALL ✅ DONE)
│   ├── TASK-TG-01     add agent.exec (step executor)     ✅ DONE (handleAgentExec in agent-exec-handler.ts)
│   └── TASK-TG-02     add ai.complete handler            ✅ DONE (ai-complete-handler.ts + dispatch)
│
├── terminal-management/tasks/             (2 tasks — ALL ✅ DONE)
│   ├── TASK-TM-06     add pty.create/write/resize/...   ✅ DONE (via pty-agent-bridge.ts)
│   └── TASK-SSH-01    exponential backoff relay-ws       ✅ DONE (calcBackoffDelay in dev-server-relay-bridge.ts)
│
├── terminal-management./tasks/            (7 tasks — ALL ✅ DONE)
│   ├── TASK-TM-01   fix pty.spawn return value           ✅ DONE
│   ├── TASK-TM-02   fix env.SHELL resolution             ✅ DONE
│   ├── TASK-TM-03   add validateCwd                      ✅ DONE (validatePtyCwd in pty-handler.ts)
│   ├── TASK-TM-04   fix relay session null+timeout       ✅ DONE (AGENT_NOT_CONNECTED + 10s timeout)
│   ├── TASK-TM-05   create pty-agent-bridge (NEW FILE)   ✅ DONE (src/relay/pty-agent-bridge.ts)
│   ├── TASK-TRM-03  fix agent WS port 6769→6768          ✅ DONE (FIX TASK-TRM-003)
│   └── TASK-TRM-04  clear session on agent WS close      ✅ DONE (mux.onDispose cleanup)
│
├── ai-providers/tasks/                    (2 tasks — 1 DONE, 1 DEFERRED)
│   ├── TASK-AIP-01   improve health check response       ✅ DONE (credentialFound + statusCode)
│   └── TASK-AIP-02   fix credential flow Orca Server     ⏸ DEFERRED (requires Orca Server changes)
│
├── worktree-management/tasks/             (4 tasks — 3 DONE, 1 DEFERRED)
│   ├── TASK-WT-01    add validateWorktreePath            ✅ DONE (git-handler.ts)
│   ├── TASK-WT-02    add git.worktree.* RPC methods      ✅ DONE (structured list + path validation)
│   ├── TASK-WT-03    inject ORCA_WORKTREE_* env vars     ✅ DONE (agent-spawner.ts)
│   └── TASK-WT-04    dynamic capability check            ⏸ DEFERRED (low risk, medium effort)
│
└── infrastructure type fixes (cross-cutting)
    ├── dev-server-manager.ts              ✅ DONE (generateAgentToken method + import)
    ├── dev-server-relay-bridge.ts         ✅ DONE (MultiplexerTransport cast + calcBackoffDelay scope)
    ├── ws-handshake.ts                    ✅ DONE (devServerId optional field in WsHandshakeInfo)
    ├── gateway-proxy.ts                   ✅ DONE (import fixed: dev-server → dev-server-types)
    ├── git-handler-clone.ts               ✅ DONE (sendEvent → notify)
    └── ai-complete-handler.ts             ✅ DONE (AgentConfig type cast fixed)
```

**Total: 27/29 tracked tasks DONE** | 2 DEFERRED (Orca Server credential flow + dynamic capability check)


---

## Execution Order

### 🔴 Batch 1 — Independent Critical Fixes (safe to run in parallel)

| Task | File(s) | Bugs | Effort |
|------|---------|------|--------|
| [TASK-ORCH-01](agent-orchestration/tasks/TASK-ORCH-01-fix-resolveAgentSpec.md) | `agent-spawner.ts` | ORCH-012, 004 | Small |
| [TASK-ORCH-03](agent-orchestration/tasks/TASK-ORCH-03-fix-agent-output-notifications.md) | `agent-spawner.ts` | ORCH-006 | Small |
| [TASK-ORCH-06](agent-orchestration/tasks/TASK-ORCH-06-fix-relay-token.md) | `agent-connection-relay.ts` | ORCH-013 | Small |
| [TASK-ORCH-08](agent-orchestration/tasks/TASK-ORCH-08-fix-capabilities-and-handshake.md) | `agent-session.ts` | AWS-001 | Tiny |
| [TASK-TM-01](terminal-management./tasks/TASK-TM-01-fix-pty-spawn-return-value.md) | `pty-handler.ts` | TM-004 | Tiny |
| [TASK-TM-02](terminal-management./tasks/TASK-TM-02-fix-pty-shell-resolution.md) | `pty-handler.ts` | TM-003 | Small |
| [TASK-WT-01](worktree-management/tasks/TASK-WT-01-add-validateWorktreePath.md) | `git-handler.ts` | WT-Issue-1 | Small |
| [TASK-TG-01](task-graph/tasks/TASK-TG-01-add-agent-exec-handler.md) | `agent-exec-handler.ts`, `agent-rpc-dispatch.ts` | TG-001 | Medium |
| [TASK-AWS-01](agent-ws/tasks/TASK-AWS-01-verify-fix-handshake.md) | `agent-wire-protocol.ts`, `agent-session.ts` | AWS-001 | Small |

### 🔴 Batch 2 — Depends on Batch 1

| Task | File(s) | Bugs | Depends on | Effort |
|------|---------|------|-----------|--------|
| [TASK-ORCH-02](agent-orchestration/tasks/TASK-ORCH-02-fix-buildAgentEnv.md) | `agent-spawner.ts` | ORCH-003 | ORCH-01 | Medium |
| [TASK-ORCH-04](agent-orchestration/tasks/TASK-ORCH-04-add-agent-sendInput.md) | `agent-spawner.ts`, `agent-rpc-dispatch.ts` | ORCH-001 | ORCH-01 | Medium |
| [TASK-ORCH-05](agent-orchestration/tasks/TASK-ORCH-05-fix-kill-signal-and-cleanup.md) | `agent-spawner.ts`, `agent-session.ts` | ORCH-002, 011 | ORCH-01 | Small |
| [TASK-ORCH-07](agent-orchestration/tasks/TASK-ORCH-07-add-agent-exec.md) | `agent-rpc-dispatch.ts` | TG-001 (generic) | None | Medium |
| [TASK-TM-03](terminal-management./tasks/TASK-TM-03-add-validateCwd.md) | `pty-handler.ts` | TM-002 | None | Medium |
| [TASK-TM-04](terminal-management./tasks/TASK-TM-04-fix-relay-session-null-and-timeout.md) | `dev-server-relay-bridge.ts` | TRM-001, 002 | None | Small |
| [TASK-TM-06](terminal-management/tasks/TASK-TM-06-add-pty-dispatch-handlers.md) | `agent-rpc-dispatch.ts`, `pty-handler.ts` | TM-001 | None | Medium |
| [TASK-AIP-01](ai-providers/tasks/TASK-AIP-01-improve-healthcheck-response.md) | `agent-credential-store.ts` | AIP-001 | None | Medium |
| [TASK-WT-02](worktree-management/tasks/TASK-WT-02-add-git-worktree-rpc-methods.md) | `git-handler.ts`, `agent-rpc-dispatch.ts` | WT-Issue-4 | WT-01 | Medium |
| [TASK-WT-03](worktree-management/tasks/TASK-WT-03-inject-worktree-env-vars.md) | `agent-spawner.ts` | WT-Issue-3 | ORCH-02 | Small |

### 🔴 Batch 3 — Architecture / New Files

| Task | File(s) | Bugs | Depends on | Effort |
|------|---------|------|-----------|--------|
| [TASK-TM-05](terminal-management./tasks/TASK-TM-05-create-pty-agent-bridge.md) | `pty-agent-bridge.ts` (NEW) | TM-001 | ORCH-07 | Large |
| [TASK-AIP-02](ai-providers/tasks/TASK-AIP-02-fix-credential-flow-orca-server.md) | `src/main/...` | AIP-002 | ORCH-02 | Large |
| [TASK-WT-04](worktree-management/tasks/TASK-WT-04-dynamic-capability-check.md) | `agent-session.ts` | WT-Issue-2 | WT-01, WT-02 | Medium |

---

## Domain Coverage

| Domain | Total Bugs | Tasks Created | Coverage |
|--------|-----------|---------------|----------|
| agent-orchestration | 13 bugs | 8 tasks | 100% (Phase 1+2) |
| agent-ws | 1 bug | 1 task | 100% |
| ai-providers | 2 bugs | 2 tasks | 100% |
| task-graph | 1 bug | 1 task | 100% |
| terminal-management (no dot) | 1 bug | 1 task | 100% |
| terminal-management. (with dot) | 6 bugs | 5 tasks | 100% |
| worktree-management | 4 issues | 4 tasks | 100% |

**Chưa có tasks** (Phase 3 backlog — cần thiết kế sâu hơn):

| Bug | Mô tả | Domain |
|-----|-------|--------|
| ORCH-005 | AgentManager not implemented | agent-orchestration |
| ORCH-007 | AgentHookParser missing | agent-orchestration |
| ORCH-008 | orca_agent_sessions wrong schema | agent-orchestration |
| ORCH-009 | Resume session not implemented | agent-orchestration |
| ORCH-010 | Switch account on rate limit | agent-orchestration |

---

## Task Format (chuẩn)

Mỗi task file có cấu trúc:
1. **Context** — Hiện trạng code, bug là gì, dòng cụ thể
2. **Investigation** (nếu cần) — Các lệnh grep để xác nhận trước khi edit
3. **Implementation** — Code thay thế hoàn chỉnh, copy-paste ready
4. **Wire Protocol** — Format request/response JSON mẫu
5. **Unit Tests** — Test cases cần thêm
6. **Verification** — Lệnh type check + manual smoke test

---

## Definition of Done (mỗi task)

- [ ] `npx tsc --noEmit` không có lỗi mới cho file đó
- [ ] Existing tests pass: `npx vitest run src/relay/__tests__/`
- [ ] New tests pass (nếu task yêu cầu)
- [ ] Manual smoke test pass theo hướng dẫn trong task file

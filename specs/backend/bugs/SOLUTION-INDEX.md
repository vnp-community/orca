# BACKEND SOLUTION INDEX — Tất cả Domains

**Cập nhật:** 2026-08-01  
**Phiên bản:** v1.0  
**Căn cứ:** Backend TDD v5 specs (21 TDD documents)  
**Tổng bugs:** 57 bugs across 22 domains

---

## Cấu trúc Solutions

```
bugs/
├── SOLUTION-INDEX.md                                       ← File này
│
├── agent-orchestration/solutions/
│   └── SOLUTION-agent-orchestration.md                    ← No bugs, analysis only
│
├── agent-ws/solutions/
│   └── SOLUTION-agent-ws.md                               ← 5 bugs (AWS-001~004, BE-AWS-001)
│
├── ai-providers/solutions/
│   └── SOLUTION-ai-providers.md                           ← 6 bugs (AIP-001~004, BE-AIP-001~002)
│
├── auth/solutions/
│   └── SOLUTION-auth.md                                   ← 6 bugs (AUTH-001~003, BE-AUTH-001~003)
│
├── automation/solutions/
│   └── SOLUTION-automation.md                             ← 5 bugs (AT-001~003, BE-AT-001~002)
│
├── cli-headless/solutions/
│   └── SOLUTION-cli-headless.md                           ← 2 bugs (CLI-001, BE-CLI-001)
│
├── code-review/solutions/
│   └── SOLUTION-code-review.md                            ← 2 bugs (CR-001, BE-CR-001)
│
├── dev-server-v1/solutions/  [EXISTS]
│   ├── SOL-DS-001~005.md                                  ← 8 bugs (DS-001~008) DONE
│
├── dev-server-v2/solutions/
│   └── (empty — no bugs)
│
├── fleet/solutions/
│   └── SOLUTION-fleet.md                                  ← 2 bugs (BE-FLEET-001~002)
│
├── mobile-companion/solutions/
│   └── SOLUTION-mobile-companion.md                       ← 1 bug (BE-MB-001)
│
├── paircode-v1/solutions/  [EXISTS]
│   ├── SOL-PC-001~002.md                                  ← 3 bugs (PC-001~003) DONE
│
├── profile/solutions/
│   └── SOLUTION-profile.md                                ← 1 bug (BE-PRF-001)
│
├── project-integration/solutions/
│   └── SOLUTION-project-integration.md                    ← 1 bug (PI-001)
│
├── project-workspace/solutions/
│   └── SOLUTION-project-workspace.md                      ← 2 bugs (PW-001~002)
│
├── remote-development/solutions/
│   └── SOLUTION-remote-development.md                     ← 2 bugs (BE-SSH-001~002)
│
├── remote-integration/solutions/
│   └── SOLUTION-remote-integration.md                     ← 1 bug (RI-001)
│
├── task-graph/solutions/
│   └── SOLUTION-task-graph.md                             ← 3 bugs (TG-002, BE-TG-001~002)
│
├── terminal-management/solutions/
│   └── SOLUTION-terminal-management.md                    ← 1 bug (TM-002)
│
├── terminal-management./solutions/
│   └── SOLUTION-terminal-management.md                    ← 9 bugs (BE-TM-001~006, TRM-BE-001~002)
│
├── workflow-orchestration/solutions/
│   └── SOLUTION-workflow-orchestration.md                 ← 4 bugs (WF-001, WF-003~004, BE-WF-001)
│
└── worktree-management/solutions/
    └── SOLUTION-worktree-management.md                    ← 3 bugs (WT-001~002, BE-WT-001)
```

---

## Bugs Coverage Table — Tất cả Domains

### Domain: agent-ws (5 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| AWS-001 | relay-ws mode topology wrong | 🔴 HIGH | Fix agent-ws-server.ts topology (inbound only) |
| AWS-002 | token không SHA-256 hash | 🔴 CRITICAL | AgentTokenManager.hash() trước khi lưu DB |
| AWS-003 | token TTL quá ngắn, không có refresh | 🟠 HIGH | DB-persistent token + keepalive |
| AWS-004 | x-orca-admin auth bypass | 🔴 CRITICAL | Xóa bypass header hoàn toàn |
| BE-AWS-001 | agent token verify in-memory | 🔴 HIGH | Lưu hashed token trong DB |

### Domain: ai-providers (6 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| AIP-001 | pending status never resolved | 🟠 HIGH | writeCredential() update status → active |
| AIP-002 | credential not decrypted before relay | 🔴 CRITICAL | Decrypt Layer 1 trên Orca Server |
| AIP-003 | health checker no status change alert | 🟡 MEDIUM | Emit event khi status thay đổi |
| AIP-004 | health checker unused relay pool | 🟡 MEDIUM | Inject DevServerManager pool |
| BE-AIP-001 | AIProviderService not implemented | 🔴 CRITICAL | Implement đầy đủ tất cả methods |
| BE-AIP-002 | credential relay security design flaw | 🔴 CRITICAL | 2-layer encryption architecture |

### Domain: auth (6 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| AUTH-001 | login không có audit log | 🟠 HIGH | AuditLogger.logAction() trên login |
| AUTH-002 | cookie SameSite Lax (CSRF) | 🟠 HIGH | SameSite: 'strict' |
| AUTH-003 | session không có idle timeout | 🟡 MEDIUM | SessionManager cleanup sau 4h |
| BE-AUTH-001 | login missing audit log (backend) | 🟠 HIGH | Giống AUTH-001 |
| BE-AUTH-002 | cookie SameSite Lax (backend) | 🟠 HIGH | Giống AUTH-002 |
| BE-AUTH-003 | per-user process isolation not implemented | 🔴 CRITICAL | SessionManager fork per userId |

### Domain: automation (5 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| AT-001 | automation service electron-dependent | 🔴 HIGH | Platform abstraction layer |
| AT-002 | event-based trigger not implemented | 🟠 HIGH | EventAutomationTrigger class |
| AT-003 | remote host scheduling disabled | 🟡 MEDIUM | Enable for all modes |
| BE-AT-001 | event-based automation not implemented | 🟠 HIGH | Giống AT-002 + DB rules |
| BE-AT-002 | worktree cleanup service not implemented | 🟠 HIGH | WorktreeCleanupService |

### Domain: cli-headless (2 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| CLI-001 | headless automation dispatcher not wired | 🟠 HIGH | HeadlessDispatcher Unix socket client |
| BE-CLI-001 | daemon Unix socket not implemented | 🔴 HIGH | PtyDaemon Unix socket server |

### Domain: code-review (2 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| CR-001 | diff service missing remote path | 🟠 HIGH | Remote path via relay bridge |
| BE-CR-001 | annotation diff service not implemented | 🟠 HIGH | AnnotationDiffService class |

### Domain: dev-server-v1 (8 bugs) [DONE]

| Bug ID | Mô tả | Status |
|--------|-------|--------|
| DS-001 ~ DS-008 | Various relay/reconnect/config bugs | ✅ Có solutions (SOL-DS-001~005) |

### Domain: fleet (2 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-FLEET-001 | fleet health monitor not implemented | 🔴 HIGH | FleetHealthMonitor full implementation |
| BE-FLEET-002 | health monitor no relay metrics | 🟡 MEDIUM | Add relay RTT + throughput metrics |

### Domain: mobile-companion (1 bug)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-MB-001 | mobile companion not implemented | 🟡 MEDIUM | MobileCompanionService + Web Push |

### Domain: paircode-v1 (3 bugs) [DONE]

| Bug ID | Mô tả | Status |
|--------|-------|--------|
| PC-001 ~ PC-003 | Browser, WS router, devserver list bugs | ✅ Có solutions (SOL-PC-001~002) |

### Domain: profile (1 bug)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-PRF-001 | profile resolver not implemented | 🔴 HIGH | ProfileResolver with hierarchy merge |

### Domain: project-integration (1 bug)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| PI-001 | credential service missing GitHub | 🟠 HIGH | GitHub/GitLab PAT + OAuth support |

### Domain: project-workspace (2 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| PW-001 | teardown no PTY cleanup | 🔴 HIGH | Kill all PTYs before teardown |
| PW-002 | relay no per-user isolation | 🔴 CRITICAL | userId enforcement in relay calls |

### Domain: remote-development (2 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-SSH-001 | reconnect no exponential backoff | 🟠 HIGH | ExponentialBackoffReconnect |
| BE-SSH-002 | port forward no DB persistence | 🟡 MEDIUM | PortForwardService persist + restore |

### Domain: remote-integration (1 bug)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| RI-001 | credential store fixed salt | 🔴 CRITICAL | Per-credential random salt (PBKDF2) |

### Domain: task-graph (3 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| TG-002 | task executor missing dep check | 🟠 HIGH | Topological sort + dep validation |
| BE-TG-001 | agent.exec method name mismatch | 🔴 CRITICAL | Add agent.exec handler in relay |
| BE-TG-002 | ai.complete relay handler missing | 🔴 CRITICAL | Add ai.complete handler + AI callers |

### Domain: terminal-management (1 bug)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| TM-002 | workspace init relative path | 🟡 MEDIUM | Convert to absolute paths |

### Domain: terminal-management. (9 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-TM-001 | WS router không forward binary frames | 🔴 HIGH | Forward binary as-is |
| BE-TM-002 | WS keepalive corrupts Unix socket | 🔴 HIGH | Filter ping/pong frames |
| BE-TM-003 | auth token injection diverges from HLD | 🟠 HIGH | ORCA_SESSION_TOKEN alignment |
| BE-TM-004 | Agent WS port mismatch | 🟡 MEDIUM | Use /agent path on port 6768 |
| BE-TM-005 | direct WS missing disconnect handler | 🟠 HIGH | Cleanup on WS close |
| BE-TM-006 | terminal.create missing RBAC check | 🔴 CRITICAL | RBAC permission check |
| TRM-BE-001 | auth route mismatch | 🔴 HIGH | Align /rpc route paths |
| TRM-BE-002 | WS auth close wrong code | 🟡 MEDIUM | Standard WS close codes |

### Domain: workflow-orchestration (4 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| WF-001 | server spec not implemented | 🔴 HIGH | WorkflowServer HTTP routes |
| WF-003 | condition step code injection | 🔴 CRITICAL | Replace eval() với safe evaluator |
| WF-004 | resume orphan step execution | 🔴 HIGH | resetOrphanedSteps + resume logic |
| BE-WF-001 | workflow orchestrator not implemented | 🔴 CRITICAL | Full WorkflowOrchestrator impl |

### Domain: worktree-management (3 bugs)

| Bug ID | Mô tả | Mức độ | Fix |
|--------|-------|--------|-----|
| BE-WT-001 | worktree.create no disk check | 🟡 MEDIUM | Check disk > 100MB + path conflict |
| WT-001 | git.worktree API inconsistent | 🔴 HIGH | Align API to specific methods |
| WT-002 | agent spawner provider interface mismatch | 🔴 HIGH | Fix resolveForProject signature |

---

## Bug Fix Priority

### 🔴 Phase 1 — CRITICAL Security (Implement ngay)

| Priority | Bug | Domain | Risk |
|----------|-----|--------|------|
| 1 | AWS-004 | agent-ws | Auth bypass — bất kỳ user → admin |
| 2 | RI-001 | remote-integration | Credential encryption flaw |
| 3 | BE-AUTH-003 | auth | No user isolation |
| 4 | AIP-002/BE-AIP-002 | ai-providers | Credential relay flaw |
| 5 | WF-003 | workflow | Code injection via eval() |
| 6 | BE-TM-006 | terminal-management | Missing RBAC |
| 7 | PW-002 | project-workspace | No per-user relay isolation |

### 🔴 Phase 2 — Critical Functionality

| Priority | Bug | Domain |
|----------|-----|--------|
| 8 | BE-WF-001 | workflow — core feature missing |
| 9 | BE-AIP-001 | ai-providers — service not implemented |
| 10 | BE-PRF-001 | profile — resolver not implemented |
| 11 | BE-TG-001/BE-TG-002 | task-graph — critical method mismatches |
| 12 | BE-CLI-001/CLI-001 | cli-headless — Unix socket missing |
| 13 | AWS-002 | agent-ws — token hashing |
| 14 | BE-AWS-001 | agent-ws — DB-based token verify |

### 🟠 Phase 3 — High Priority

| Priority | Bug | Domain |
|----------|-----|--------|
| 15 | PW-001 | project-workspace — PTY cleanup |
| 16 | WF-004 | workflow — orphan resume |
| 17 | BE-TM-001/002 | terminal — binary + keepalive |
| 18 | BE-FLEET-001 | fleet — health monitor |
| 19 | WT-002 | worktree — spawner interface |
| 20 | TG-002 | task-graph — dep check |

### 🟡 Phase 4 — Medium Priority

| Priority | Bug | Domain |
|----------|-----|--------|
| 21 | BE-SSH-001 | remote-development — backoff |
| 22 | AIP-003/004 | ai-providers — health alerting |
| 23 | BE-AT-002 | automation — worktree cleanup |
| 24 | AT-001 | automation — Electron dependency |
| 25+ | Remaining | Various |

---

## New Files cần tạo (NEW files)

```
src/main/dev-server/agent-token-manager.ts          ← AWS-002 (SHA-256 hashing)
src/main/automation/EventAutomationTrigger.ts        ← AT-002/BE-AT-001
src/main/automation/CronAutomationTrigger.ts         ← AT-002
src/main/worktree/WorktreeCleanupService.ts          ← BE-AT-002
src/main/code-review/AnnotationDiffService.ts        ← BE-CR-001
src/main/ssh/fleet-health-monitor.ts                 ← BE-FLEET-001 (extend existing)
src/main/ssh/ssh-reconnect-manager.ts                ← BE-SSH-001
src/main/ssh/PortForwardService.ts                   ← BE-SSH-002
src/main/mobile/MobileCompanionService.ts            ← BE-MB-001
src/server/mobile-api-routes.ts                      ← BE-MB-001
src/main/workflow/WorkflowServer.ts                  ← WF-001
src/main/workflow/WorkflowOrchestrator.ts            ← BE-WF-001 (major rewrite)
src/main/session/user-process-entry.ts               ← BE-AUTH-003
src/main/auth/web-credential-store.ts                ← RI-001 (major rewrite)
src/relay/ai-provider-caller.ts                      ← BE-TG-002
```

## New DB Migrations cần tạo

```
0008_agent_token_hash.ts          ← BE-AWS-001
0009_automation_rules.ts          ← BE-AT-001
0010_fleet_health_metrics.ts      ← BE-FLEET-001
0011_mobile_devices.ts            ← BE-MB-001
0012_profiles.ts                  ← BE-PRF-001
0013_web_credentials.ts           ← PI-001
0014_port_forwards.ts             ← BE-SSH-002
0015_migrate_credential_salts.ts  ← RI-001
0016_workflow.ts                  ← BE-WF-001
```

---

## TDD v5 Cross-Reference

| TDD | Domain(s) |
|-----|----------|
| TDD-04 (RPC Server) | terminal-management, agent-ws |
| TDD-05 (SSH Relay) | remote-development, remote-integration, fleet |
| TDD-07 (Runtime Service) | terminal-management, worktree-management |
| TDD-08 (Agent Orchestration) | task-graph, automation |
| TDD-11 (Web Server Mode) | auth, cli-headless, mobile-companion |
| TDD-12 (Database Layer) | All (migrations) |
| TDD-13 (Dev Server Onboarding) | fleet, mobile-companion |
| TDD-14 (Profile Hierarchy) | profile |
| TDD-15 (Project Binding) | project-integration, project-workspace |
| TDD-16 (AI Provider Management) | ai-providers |
| TDD-17 (Workflow Orchestration) | workflow-orchestration |
| TDD-18 (Task Graph) | task-graph |
| TDD-19 (Project Workspace) | project-workspace, worktree-management |
| TDD-20 (Remote Git UI) | code-review, worktree-management |

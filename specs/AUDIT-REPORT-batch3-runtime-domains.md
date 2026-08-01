# AUDIT REPORT — terminal-management, worktree-management, task-graph, workflow-orchestration, project-workspace, project-integration, remote-integration

**Ngày:** 2026-08-01  
**Phạm vi:** 7 domains runtime (phần thực thi trên Dev Server)  
**Phương pháp:** HLD-to-Code Mapping với kiến trúc 3-tier đã được xác nhận

---

## Nguyên tắc kiến trúc đã xác nhận

> **Toàn bộ runtime execution (git, PTY, agent, fs operations) PHẢI chạy trên Dev Server thông qua relay.call().**
> Orca Server chỉ là orchestrator (nhận từ Browser → điều phối → relay → Dev Server).

```
Browser ──WS/HTTP──► Orca Server ──relay.call()──► Dev Server
                          │
                          ├─ auth, permission check
                          ├─ DB (Server DB)
                          └─ routing (ProjectServerRouter)
                     
Dev Server thực hiện:
  node-pty (PTY sessions)
  git CLI (all git operations)
  agent process (Claude/GPT/etc)
  fs operations (readDir, readFile, grep)
  gh/glab CLI (GitHub/GitLab ops)
```

---

## Tóm tắt Bugs

| Domain | Bugs | Severity cao nhất |
|--------|------|-------------------|
| terminal-management | 3 | 🔴 CRITICAL |
| worktree-management | 2 | 🔴 HIGH |
| task-graph | 2 | 🔴 CRITICAL |
| workflow-orchestration | 4 | 🔴 HIGH (Security) |
| project-workspace | 2 | 🟡 MEDIUM |
| project-integration | 1 | 🔴 HIGH |
| remote-integration | 1 | 🟡 MEDIUM |
| **TOTAL** | **15** | |

---

## Critical Findings

### 1. 🔴 CRITICAL — relay dispatch thiếu `pty.create/destroy/resize/write` handlers (BUG-TM-001)
**Ảnh hưởng:** Terminal trên remote Dev Server **hoàn toàn không hoạt động** — không thể tạo PTY session.  
**File:** `src/relay/agent-rpc-dispatch.ts`  
**Fix:** Thêm pty.* handlers vào relay dispatch.

### 2. 🔴 CRITICAL — relay dispatch thiếu `agent.exec` handler (BUG-TG-001)
**Ảnh hưởng:** `ProfileAwareAgentSpawner.spawn()` và `StepExecutors.executeAgent()` đều fail — **không có agent nào chạy được**.  
**File:** `src/relay/agent-rpc-dispatch.ts`  
**Fix:** Đăng ký `case 'agent.exec'` trong dispatch.

### 3. 🔴 CRITICAL — `AIProviderResolver` interface mismatch với `AIProviderService` (BUG-WT-002)
**Ảnh hưởng:** `ProfileAwareAgentSpawner` gọi `providerService.resolveForProject(projectId, preferredModel)` nhưng actual signature là `(devServerId, projectId, userId, modelHint)` → TypeScript compile error hoặc runtime fail.  
**File:** `src/main/project/ProfileAwareAgentSpawner.ts:22-26`

### 4. 🔴 HIGH (Security) — `StepExecutors.executeCondition()` dùng `new Function()` (BUG-WF-003)
**Ảnh hưởng:** RCE via malicious workflow condition expression — attacker có thể execute arbitrary code trên Orca Server.  
**File:** `src/main/workflow/StepExecutors.ts:164-168`

### 5. 🔴 HIGH — `server:` và `fleet:tag:` workflow specs ném error (BUG-WF-001)
**Ảnh hưởng:** Multi-server workflow chỉ hoạt động với `project:<id>` spec. `server:` và `fleet:tag:` đều fail với NotImplemented error.  
**File:** `src/main/workflow/StepExecutors.ts:197-204`

### 6. 🔴 HIGH — `WebCredentialStore` thiếu `github` và `gitlab` types (BUG-PI-001)
**Ảnh hưởng:** Không thể lưu GitHub/GitLab tokens — BL-PI-01, BL-PI-04 broken.  
**File:** `src/main/credentials/web-credential-store.ts:13-18`

---

## Documents đã cập nhật

| Document | Thay đổi |
|----------|---------|
| [terminal-management.md](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/docs/flows/logic/terminal-management.md) | Cập nhật từ Electron Daemon sang 3-tier: Browser→WS→Orca Server→relay→Dev Server (node-pty) |
| [worktree-management.md](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/docs/flows/logic/worktree-management.md) | Cập nhật BL-WT-01→05: tất cả git CLI và PTY phải qua relay đến Dev Server |
| [component-mapping.md](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/docs/flows/component-mapping.md) | Thêm `Y (runtime)` annotation, bổ sung 7 chú thích kiến trúc mới |

---

## Tất cả Bugs theo Priority

| Bug ID | Domain | Component | Severity | Mô tả ngắn |
|--------|--------|-----------|----------|------------|
| BUG-TM-001 | terminal | AGENT | 🔴 CRITICAL | relay thiếu pty.* handlers |
| BUG-TG-001 | task-graph | AGENT | 🔴 CRITICAL | relay thiếu agent.exec handler |
| BUG-WT-002 | worktree | BACKEND | 🔴 CRITICAL | AIProviderResolver interface mismatch |
| BUG-WF-003 | workflow | BACKEND | 🔴 HIGH (Security) | new Function() RCE via condition step |
| BUG-WF-001 | workflow | BACKEND | 🔴 HIGH | server:/fleet:tag: spec NotImplemented |
| BUG-PI-001 | project-integration | BACKEND | 🔴 HIGH | CredentialService missing github/gitlab |
| BUG-TM-002 | terminal | BACKEND | 🔴 HIGH | fs.readDir relative path '.' without cwd |
| BUG-TG-002 | task-graph | BACKEND | 🟡 MEDIUM | TaskAgentExecutor missing dep check |
| BUG-WF-004 | workflow | BACKEND | 🟡 MEDIUM | resume orphan step re-execution |
| BUG-PW-001 | project-workspace | BACKEND | 🟡 MEDIUM | teardownWorkspace no PTY cleanup |
| BUG-PW-002 | project-workspace | BACKEND | 🟡 MEDIUM | relay shared across users, no isolation |
| BUG-WT-001 | worktree | BACKEND | 🟡 MEDIUM | git.exec vs git.worktree.list inconsistent |
| BUG-RI-001 | remote-integration | BACKEND | 🟡 MEDIUM | credential store fixed salt |
| BUG-TM-003 | terminal | FRONTEND | 🟡 MEDIUM | terminal session not persisted to DB |
| BUG-WF-002 | workflow | AGENT | 🔴 HIGH | relay agent.exec handler missing (covered by BUG-TG-001) |

---

## Liên kết Bug Reports

- [terminal-management bugs (agent)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/agent/bugs/terminal-management/)
- [terminal-management bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/terminal-management/)
- [terminal-management bugs (frontend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/frontend/bugs/terminal-management/)
- [worktree-management bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/worktree-management/)
- [task-graph bugs (agent)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/agent/bugs/task-graph/)
- [task-graph bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/task-graph/)
- [workflow-orchestration bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/workflow-orchestration/)
- [project-workspace bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/project-workspace/)
- [project-integration bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/project-integration/)
- [remote-integration bugs (backend)](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/remote-integration/)

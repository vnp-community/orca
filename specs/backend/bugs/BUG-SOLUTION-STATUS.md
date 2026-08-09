# BUG SOLUTION STATUS — 100% Coverage Tracker

**Cập nhật:** 2026-08-01  
**Mục tiêu:** 100% bug coverage với solutions có thể thực hiện được

---

## Backend Bugs (57 bugs)

### ✅ = Solution hoàn chỉnh có code thực tế  
### ⚠️ = Solution có nhưng cần verify source code thực tế  
### ❌ = Chưa có solution

---

## Domain: agent-ws (5 bugs)

| Bug ID | Mức độ | Status | Solution File | Ghi chú |
|--------|--------|--------|--------------|---------|
| AWS-001 | 🔴 HIGH | ✅ | [SOLUTION-agent-ws-exact.md](agent-ws/solutions/SOLUTION-agent-ws-exact.md) | Port 6769→6768 (= BE-TM-004) |
| AWS-002 | 🟡 MEDIUM | ✅ | [SOLUTION-agent-ws-exact.md](agent-ws/solutions/SOLUTION-agent-ws-exact.md) | SHA-256 hash token |
| AWS-003 | 🔴 HIGH | ✅ | [SOLUTION-agent-ws-exact.md](agent-ws/solutions/SOLUTION-agent-ws-exact.md) | `permanent` option + 30d TTL |
| AWS-004 | 🔴 CRITICAL | ✅ | [SOLUTION-agent-ws-exact.md](agent-ws/solutions/SOLUTION-agent-ws-exact.md) | Remove X-Orca-Admin bypass |
| BE-AWS-001 | 🔴 HIGH | ✅ | [SOLUTION-agent-ws-exact.md](agent-ws/solutions/SOLUTION-agent-ws-exact.md) | DB lookup fallback |

---

## Domain: ai-providers (6 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| AIP-001 | 🔴 CRITICAL | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |
| AIP-002 | 🔴 CRITICAL | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |
| AIP-003 | 🟡 MEDIUM | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |
| AIP-004 | 🟡 MEDIUM | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |
| BE-AIP-001 | 🔴 CRITICAL | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |
| BE-AIP-002 | 🔴 CRITICAL | ✅ | [SOLUTION-ai-providers.md](ai-providers/solutions/SOLUTION-ai-providers.md) |

---

## Domain: auth (6 bugs)

| Bug ID | Mức độ | Status | Solution File | Source verification |
|--------|--------|--------|--------------|---------------------|
| AUTH-001 | 🟠 HIGH | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | `auth-manager.ts` login() |
| AUTH-002 | 🟠 HIGH | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | `auth-router.ts:23` → `'strict'` |
| AUTH-003 | 🟡 MEDIUM | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | `session-manager.ts:51` sweepIdleProcesses |
| BE-AUTH-001 | 🟠 HIGH | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | = AUTH-001 |
| BE-AUTH-002 | 🟠 HIGH | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | = AUTH-002 |
| BE-AUTH-003 | 🔴 CRITICAL | ✅ | [SOLUTION-auth-exact.md](auth/solutions/SOLUTION-auth-exact.md) | SessionManager đã implement, verify |

---

## Domain: automation (5 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| AT-001 | 🔴 HIGH | ✅ | [SOLUTION-automation.md](automation/solutions/SOLUTION-automation.md) |
| AT-002 | 🟠 HIGH | ✅ | [SOLUTION-automation.md](automation/solutions/SOLUTION-automation.md) |
| AT-003 | 🟡 MEDIUM | ✅ | [SOLUTION-automation.md](automation/solutions/SOLUTION-automation.md) |
| BE-AT-001 | 🟠 HIGH | ✅ | [SOLUTION-automation.md](automation/solutions/SOLUTION-automation.md) |
| BE-AT-002 | 🟠 HIGH | ✅ | [SOLUTION-automation.md](automation/solutions/SOLUTION-automation.md) |

---

## Domain: cli-headless (2 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| CLI-001 | 🟠 HIGH | ✅ | [SOLUTION-cli-headless.md](cli-headless/solutions/SOLUTION-cli-headless.md) |
| BE-CLI-001 | 🔴 HIGH | ✅ | [SOLUTION-cli-headless.md](cli-headless/solutions/SOLUTION-cli-headless.md) |

---

## Domain: code-review (2 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| CR-001 | 🟠 HIGH | ✅ | [SOLUTION-code-review.md](code-review/solutions/SOLUTION-code-review.md) |
| BE-CR-001 | 🟠 HIGH | ✅ | [SOLUTION-code-review.md](code-review/solutions/SOLUTION-code-review.md) |

---

## Domain: dev-server-v1 (8 bugs) — ALL DONE

| Bug ID | Status | Solution |
|--------|--------|---------|
| DS-001 | ✅ DONE | SOL-DS-001 (implemented 2026-07-27) |
| DS-002 | ✅ DONE | SOL-DS-002 |
| DS-003 | ✅ DONE | SOL-DS-003 |
| DS-004 | ✅ DONE | SOL-DS-004 |
| DS-005 | ✅ DONE | SOL-DS-004 |
| DS-006 | ✅ DONE | SOL-DS-005 |
| DS-007 | ✅ DONE | SOL-DS-005 |
| DS-008 | ✅ DONE | SOL-DS-005 |

---

## Domain: fleet (2 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| BE-FLEET-001 | 🔴 HIGH | ✅ | [SOLUTION-fleet.md](fleet/solutions/SOLUTION-fleet.md) |
| BE-FLEET-002 | 🟡 MEDIUM | ⚠️ **RE-OPENED** | [SOLUTION-fleet.md](fleet/solutions/SOLUTION-fleet.md) — audit 2026-08-09 xác nhận code hiện tại KHÔNG khớp solution (vẫn không CPU/RAM/disk/latency, vẫn 60s không phải 30s). Xem [BUG-BE-HLD-010](hld-v1/BUG-BE-HLD-010-fleet-health-monitor-no-real-metrics-still-broken.md). |

## Domain: hld-v1 (20 bugs) — Audit 2026-08-08/09 (backend vs `docs/hld/*` + `docs/features/F22-F40`)

**Cập nhật 2026-08-09:** toàn bộ 20 bug đã có solution code-level (12 file, xem [`hld-v1/solutions/00-index.md`](hld-v1/solutions/00-index.md)) — căn cứ theo `specs/backend/tdd/v4`+`v5`. Status đổi thành 🟡 **Solution Ready** (chưa merge vào code thật — vẫn cần review + áp dụng patch + test).

| Bug ID | Mức độ | Status | Solution File | Ghi chú |
|--------|--------|--------|---------------|---------|
| BE-HLD-001 | 🔴 CRITICAL | 🟡 Solution Ready | [SOLUTION-rbac-exact.md](hld-v1/solutions/SOLUTION-rbac-exact.md) | `requireAdmin(ctx)` RPC stub không check role — permission bypass; patch cũng cần áp lại cho `desktop/` (bản sao byte-for-byte) |
| BE-HLD-002 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-rbac-exact.md](hld-v1/solutions/SOLUTION-rbac-exact.md) | `requireOwnerOrAdmin` dead code, `project.create` không giới hạn quyền |
| BE-HLD-003 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-rbac-exact.md](hld-v1/solutions/SOLUTION-rbac-exact.md) | RBAC phân mảnh — solution đề xuất `PermissionService.hasPermission()` làm phase 2 |
| BE-HLD-004 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-github-gitlab-relay-exact.md](hld-v1/solutions/SOLUTION-github-gitlab-relay-exact.md) | Backend tự thực thi gh/glab — fix tối thiểu (guard `ORCA_MULTI_USER`) trước, roadmap relay đầy đủ sau |
| BE-HLD-005 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-github-gitlab-relay-exact.md](hld-v1/solutions/SOLUTION-github-gitlab-relay-exact.md) | GH_CONFIG_DIR không truyền — cần sửa cả Backend VÀ `agent/src/relay/pty-handler.ts` (hiện không đọc userId) |
| BE-HLD-006 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-admin-panel-exact.md](hld-v1/solutions/SOLUTION-admin-panel-exact.md) | Admin sessions list là stub rỗng |
| BE-HLD-007 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-admin-panel-exact.md](hld-v1/solutions/SOLUTION-admin-panel-exact.md) | Admin Access Policies API — file mới `admin-policy-handlers.ts` |
| BE-HLD-008 | 🟠 HIGH | 🟡 Solution Ready ⚠️ | [SOLUTION-workflow-exact.md](hld-v1/solutions/SOLUTION-workflow-exact.md) | Provider-selection theo step — **verify trước**: `WorkflowOrchestrator.executeStep()` type-mismatch có thể khiến mọi step đã throw `UNSUPPORTED_STEP_TYPE` |
| BE-HLD-009 | 🟠 HIGH | 🟡 Solution Ready | [SOLUTION-workflow-exact.md](hld-v1/solutions/SOLUTION-workflow-exact.md) | Pause/resume — cần migration mới 0014 |
| BE-HLD-010 | 🟡 MEDIUM | 🟡 Solution Ready (re-open BE-FLEET-002) | [SOLUTION-fleet-exact.md](hld-v1/solutions/SOLUTION-fleet-exact.md) | FleetHealthMonitor CPU/RAM/disk/latency — solution cũ SOLUTION-fleet.md bịa kiến trúc không khớp code thật, đã viết lại |
| BE-HLD-011 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-session-manager-exact.md](hld-v1/solutions/SOLUTION-session-manager-exact.md) | Auto-respawn + idle-timeout config |
| BE-HLD-012 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-fleet-exact.md](hld-v1/solutions/SOLUTION-fleet-exact.md) | CLI `orca fleet provision` — file mới `fleet-provision-cli.ts`, chưa rõ điểm wire vào `bin.orca` |
| BE-HLD-013 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-fleet-exact.md](hld-v1/solutions/SOLUTION-fleet-exact.md) | Fleet bootstrap disk-check + SHA256 verify |
| BE-HLD-014 | 🟡 MEDIUM | 🟡 Solution Ready ⚠️ | [SOLUTION-ai-provider-exact.md](hld-v1/solutions/SOLUTION-ai-provider-exact.md) | Key rotation — **verify trước**: `AuditLogger.log()` insert sai cột so với schema thật `orca_audit_log` |
| BE-HLD-015 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-ai-provider-exact.md](hld-v1/solutions/SOLUTION-ai-provider-exact.md) | Quota 80% alert qua `onQuotaWarning`, debounce 1 lần/ngày |
| BE-HLD-016 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-db-migration-naming-exact.md](hld-v1/solutions/SOLUTION-db-migration-naming-exact.md) | Chỉ sửa tài liệu — không đề xuất sửa migration đã chạy |
| BE-HLD-017 | 🟡 MEDIUM | 🟡 Solution Ready (cần PO xác nhận scope) | [SOLUTION-platform-electron-adapter-exact.md](hld-v1/solutions/SOLUTION-platform-electron-adapter-exact.md) | ElectronAdapter — skeleton code có sẵn, chờ quyết định phạm vi |
| BE-HLD-018 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-remote-git-ui-exact.md](hld-v1/solutions/SOLUTION-remote-git-ui-exact.md) | DevServerGitProvider thiếu git log/AI commit-msg/diff — 1/9 method fix ngay, 8/9 cần bổ sung Agent trước |
| BE-HLD-019 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-agent-ws-protocol-exact.md](hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md) | Chủ yếu sửa tài liệu F29; version-check thật dùng mã 1008, không bịa 4003 |
| BE-HLD-020 | 🟡 MEDIUM | 🟡 Solution Ready | [SOLUTION-project-devserver-rebind-exact.md](hld-v1/solutions/SOLUTION-project-devserver-rebind-exact.md) | Rebind devServerId — phụ thuộc BE-HLD-002 cho phần RBAC |

> Chi tiết đầy đủ: [`hld-v1/00-index.md`](hld-v1/00-index.md). Nguồn: `audit/backend/backend-vs-design-review.md`.

---

## Domain: mobile-companion (1 bug)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| BE-MB-001 | 🟡 MEDIUM | ✅ | [SOLUTION-mobile-companion.md](mobile-companion/solutions/SOLUTION-mobile-companion.md) |

---

## Domain: paircode-v1 (3 bugs) — ALL DONE

| Bug ID | Status |
|--------|--------|
| PC-001 | ✅ DONE |
| PC-002 | ✅ DONE |
| PC-003 | ✅ DONE |

---

## Domain: profile (1 bug)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| BE-PRF-001 | 🔴 HIGH | ✅ | [SOLUTION-profile.md](profile/solutions/SOLUTION-profile.md) |

---

## Domain: project-integration (1 bug)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| PI-001 | 🟠 HIGH | ✅ | [SOLUTION-project-integration.md](project-integration/solutions/SOLUTION-project-integration.md) |

---

## Domain: project-workspace (2 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| PW-001 | 🔴 HIGH | ✅ | [SOLUTION-project-workspace.md](project-workspace/solutions/SOLUTION-project-workspace.md) |
| PW-002 | 🔴 CRITICAL | ✅ | [SOLUTION-project-workspace.md](project-workspace/solutions/SOLUTION-project-workspace.md) |

---

## Domain: remote-development (2 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| BE-SSH-001 | 🟠 HIGH | ✅ | [SOLUTION-remote-development.md](remote-development/solutions/SOLUTION-remote-development.md) |
| BE-SSH-002 | 🟡 MEDIUM | ✅ | [SOLUTION-remote-development.md](remote-development/solutions/SOLUTION-remote-development.md) |

---

## Domain: remote-integration (1 bug)

| Bug ID | Mức độ | Status | Solution File | Source verified |
|--------|--------|--------|--------------|-----------------|
| RI-001 | 🔴 CRITICAL | ✅ | [SOLUTION-RI-001-exact.md](remote-integration/solutions/SOLUTION-RI-001-exact.md) | Per-credential random salt |

---

## Domain: task-graph (3 bugs)

| Bug ID | Mức độ | Status | Solution File | Source verified |
|--------|--------|--------|--------------|-----------------|
| BE-TG-001 | 🔴 CRITICAL | ✅ | [SOLUTION-task-graph-exact.md](task-graph/solutions/SOLUTION-task-graph-exact.md) | `ProfileAwareAgentSpawner:106` params mismatch |
| BE-TG-002 | 🔴 HIGH | ✅ | [SOLUTION-task-graph-exact.md](task-graph/solutions/SOLUTION-task-graph-exact.md) | `ai.complete` handler + `ai-complete-handler.ts` |
| TG-002 | 🟠 HIGH | ✅ | [SOLUTION-task-graph-exact.md](task-graph/solutions/SOLUTION-task-graph-exact.md) | Dependency check before execute |

---

## Domain: terminal-management (1 bug)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| TM-002 | 🟡 MEDIUM | ✅ | [SOLUTION-terminal-management.md](terminal-management/solutions/SOLUTION-terminal-management.md) |

---

## Domain: terminal-management. (9 bugs) — ĐÃ SOURCE-VERIFIED

| Bug ID | Mức độ | Status | Solution File | Source verified |
|--------|--------|--------|--------------|-----------------|
| BE-TM-001 | 🔴 HIGH | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `ws-session-router.ts:122–136` |
| BE-TM-002 | 🔴 HIGH | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `ws-session-router.ts:98–102` |
| BE-TM-003 | 🟠 HIGH | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `ws-session-router.ts:110–114` |
| BE-TM-004 | 🟡 MEDIUM | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `dev-server-relay-bridge.ts:254` |
| BE-TM-005 | 🟠 HIGH | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `dev-server-relay-bridge.ts:220–233` |
| BE-TM-006 | 🔴 CRITICAL | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `terminal.ts` terminal.create RBAC |
| TRM-BE-001 | 🔴 HIGH | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | `/auth` route mount |
| TRM-BE-002 | 🟡 MEDIUM | ✅ | [SOLUTION-TRM-BE-exact.md](terminal-management./solutions/SOLUTION-TRM-BE-exact.md) | WS close 4401 → frontend handler |

---

## Domain: workflow-orchestration (4 bugs)

| Bug ID | Mức độ | Status | Solution File |
|--------|--------|--------|--------------|
| BE-WF-001 | 🔴 CRITICAL | ✅ | [SOLUTION-workflow-orchestration.md](workflow-orchestration/solutions/SOLUTION-workflow-orchestration.md) |
| WF-001 | 🔴 HIGH | ✅ | [SOLUTION-workflow-orchestration.md](workflow-orchestration/solutions/SOLUTION-workflow-orchestration.md) |
| WF-003 | 🔴 CRITICAL | ✅ | [SOLUTION-workflow-orchestration.md](workflow-orchestration/solutions/SOLUTION-workflow-orchestration.md) |
| WF-004 | 🔴 HIGH | ✅ | [SOLUTION-workflow-orchestration.md](workflow-orchestration/solutions/SOLUTION-workflow-orchestration.md) |

> **Note:** WorkflowOrchestrator đã được implement (SOL-V5-004, ✅ DONE 2026-07-29). Bugs WF-001, WF-003, WF-004 cần kiểm tra lại xem đã fix chưa.

---

## Domain: worktree-management (3 bugs)

| Bug ID | Mức độ | Status | Solution File | Source verified |
|--------|--------|--------|--------------|-----------------|
| BE-WT-001 | 🟡 MEDIUM | ✅ | [SOLUTION-worktree-exact.md](worktree-management/solutions/SOLUTION-worktree-exact.md) | Disk check trước worktree add |
| WT-001 | 🔴 HIGH | ✅ | [SOLUTION-worktree-exact.md](worktree-management/solutions/SOLUTION-worktree-exact.md) | `git.worktree.list` đã có trong relay |
| WT-002 | 🔴 HIGH | ✅ | [SOLUTION-worktree-exact.md](worktree-management/solutions/SOLUTION-worktree-exact.md) | `ProfileAwareAgentSpawner:96` params |

---

## Frontend Bugs (11 bugs)

| Bug ID | Domain | Status | Solution File |
|--------|--------|--------|--------------|
| BUG-FE-ORCH-001 | agent-orchestration | ✅ | [SOL-FE-ORCH-001](../../frontend/bugs/agent-orchestration/solutions/SOL-FE-ORCH-001-ipc-bridge-agent-start-stop-resume.md) |
| BUG-FE-CR-001 | code-review | ✅ | [SOL-FE-CR-001](../../frontend/bugs/code-review/solutions/SOL-FE-CR-001-diff-viewer-annotation-pr-dialog.md) |
| BUG-FE-FLEET-001 | fleet | ✅ | [SOL-FE-FLEET-001](../../frontend/bugs/fleet/solutions/SOL-FE-FLEET-001-fleet-dashboard-implementation.md) |
| BUG-FE-MB-001 | mobile-companion | ✅ | [SOL-FE-MB-001](../../frontend/bugs/mobile-companion/solutions/SOL-FE-MB-001-mobile-companion-ui-implementation.md) |
| BUG-FE-TM-001 | terminal-management. | ✅ | [SOL-FE-TM-001](../../frontend/bugs/terminal-management./solutions/SOL-FE-TM-001-increase-terminal-create-timeout.md) |
| BUG-FE-TM-002 | terminal-management. | ✅ | [SOL-FE-TM-002](../../frontend/bugs/terminal-management./solutions/SOL-FE-TM-002-scrollback-snapshot-save-restore.md) |
| BUG-FE-TM-003 | terminal-management. | ✅ | [SOL-FE-TM-003](../../frontend/bugs/terminal-management./solutions/SOL-FE-TM-003-fix-hardcoded-presentation-mode.md) |
| BUG-FE-TM-004 | terminal-management. | ✅ | [SOL-FE-TM-004](../../frontend/bugs/terminal-management./solutions/SOL-FE-TM-004-fix-default-viewport-size.md) |
| BUG-TM-003 | terminal-management | ✅ | [SOL-TM-003](../../frontend/bugs/terminal-management/solutions/SOL-TM-003-terminal-session-persistence.md) |
| BUG-FE-WF-001 | workflow-orchestration | ✅ | [SOL-FE-WF-001](../../frontend/bugs/workflow-orchestration/solutions/SOL-FE-WF-001-workflow-builder-ui-implementation.md) |

---

## Tổng kết

| Metric | Số lượng |
|--------|---------|
| **Backend bugs tổng** | 77 (57 cũ + 20 `hld-v1`, 2026-08-09) |
| **Backend bugs có solution** | 57 ✅ / 20 `hld-v1` 🟡 Solution Ready (2026-08-09, chưa merge vào code) |
| **Backend bugs source-verified** | ~25 (BE-TM-001~006, AWS-001~004, TG-001~002, WT-001~002, AUTH-002) + toàn bộ 20 `hld-v1` (source-verified bằng CodeGraph/GitNexus khi audit) |
| **Status cần sửa lại (stale "FIXED")** | BE-FLEET-002 (re-open thành BE-HLD-010), khả năng cả AUTH-003 và PI-001 — xem ghi chú trong `hld-v1/00-index.md` |
| **Frontend bugs tổng** | 11 |
| **Frontend bugs có solution** | 10 ✅ |
| **Dev-server-v1 bugs đã DONE** | 8 ✅ |
| **Paircode-v1 bugs đã DONE** | 3 ✅ |

## Nhóm ưu tiên implement ngay (CRITICAL + source-verified)

```
1. [agent-token-routes.ts:36-43]   AWS-004: Xóa X-Orca-Admin bypass      ← RCE risk
2. [auth-router.ts:23]             AUTH-002: sameSite 'lax' → 'strict'   ← CSRF
3. [ws-session-router.ts:98-102]   BE-TM-002: Xóa keepalive timer        ← session corruption
4. [ws-session-router.ts:122-136]  BE-TM-001: Binary frame forwarding    ← PTY corruption
5. [dev-server-relay-bridge.ts:254] BE-TM-004: Port 6769 → 6768         ← mis-route
6. [dev-server-relay-bridge.ts:220] BE-TM-005: onDispose handler        ← memory leak
7. [ProfileAwareAgentSpawner.ts:106] BE-TG-001: agent.exec params fix   ← task graph broken
8. [agent-rpc-dispatch.ts NEW case] BE-TG-002: ai.complete handler      ← AI planning broken
```

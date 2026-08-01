# Bug Index — Toàn bộ nghiệp vụ (dựa trên docs/flows/logic/)

**Cập nhật:** 2026-08-01 (Audit chi tiết lần 2)  
**Phương pháp:** So sánh HLD (docs/flows/logic/) với implementation thực tế trong src/

---

## Phân loại theo Domain và Thành phần

### 🤖 Agent (Dev Server)

| ID | File | Domain | Severity | Mô tả ngắn |
|----|------|--------|----------|------------|
| [BUG-AG-ORCH-001](./agent/bugs/agent-orchestration/) | agent-orchestration.md | Agent Orchestration | 🔴 HIGH | `agent.sendInput` RPC không có → Ctrl+C không đến PTY |
| [BUG-AG-ORCH-002](./agent/bugs/agent-orchestration/) | agent-orchestration.md | Agent Orchestration | 🟡 MEDIUM | `agent.kill` dùng SIGTERM thay vì signal từ params |
| [BUG-AG-ORCH-003](./agent/bugs/agent-orchestration/) | agent-orchestration.md | Agent Orchestration | 🔴 HIGH | `buildAgentEnv` dùng hardcode `'placeholder-key'` |
| [BUG-AG-ORCH-004](./agent/bugs/agent-orchestration/) | agent-orchestration.md | Agent Orchestration | 🟡 MEDIUM | `resolveAgentSpec` thiếu Codex và OpenCode |
| [BUG-AG-AIP-001](./agent/bugs/ai-providers/) | ai-providers.md | AI Provider Mgmt | 🟡 MEDIUM | `ai.provider.healthCheck` chỉ check reachability, không call API thực |
| [BUG-AG-AIP-002](./agent/bugs/ai-providers/) | ai-providers.md | AI Provider Mgmt | 🔴 HIGH | `readDecryptedKey` trả về encrypted blob, không phải plaintext apiKey |
| [BUG-AG-AWS-001](./agent/bugs/agent-ws/) | agent-ws.md | Agent WebSocket | 🟡 MEDIUM | Handshake method name khác HLD |

---

### 🖥️ Backend (Orca Server)

| ID | Domain | Severity | Mô tả ngắn |
|----|--------|----------|------------|
| [BUG-BE-AUTH-001](./backend/bugs/auth/) | Auth | 🟡 MEDIUM | Login không ghi audit log |
| [BUG-BE-AUTH-002](./backend/bugs/auth/) | Auth | 🟡 MEDIUM | Session cookie `SameSite=Lax` thay vì `Strict` |
| [BUG-BE-AUTH-003](./backend/bugs/auth/) | Auth | 🔴 HIGH | Per-user process isolation chưa implement |
| [BUG-BE-AWS-001](./backend/bugs/agent-ws/) | Agent WebSocket | 🔴 HIGH | Agent token verify dùng in-memory, không phải DB |
| [BUG-BE-AIP-001](./backend/bugs/ai-providers/) | AI Provider Mgmt | 🔴 HIGH | AIProviderService backend (server REST side) chưa implement |
| [BUG-BE-AIP-002](./backend/bugs/ai-providers/) | AI Provider Mgmt | 🟡 MEDIUM | Security design flaw: không thể derive browser AES-GCM key |
| [BUG-BE-WT-001](./backend/bugs/worktree-management/) | Worktree Mgmt | 🟡 MEDIUM | worktree.create không check disk space (BR-WT-01) |
| [BUG-BE-CR-001](./backend/bugs/code-review/) | Code Review | 🔴 HIGH | AnnotationService, DiffService chưa implement |
| [BUG-BE-PRF-001](./backend/bugs/profile/) | Profile Mgmt | ~~🔴~~ ✅ FIXED | ProfileResolver đã implement (TDD-14) |
| [BUG-BE-MB-001](./backend/bugs/mobile-companion/) | Mobile Companion | 🔴 CRITICAL | Toàn bộ Mobile Companion backend chưa implement |
| [BUG-BE-FLEET-001](./backend/bugs/fleet/) | Fleet Mgmt | 🟡 MEDIUM | FleetManager, YAML loader chưa implement (health monitor có) |
| [BUG-BE-FLEET-002](./backend/bugs/fleet/) | Fleet Mgmt | 🟡 MEDIUM | FleetHealthMonitor không gọi relay metrics, sai cron interval |
| [BUG-BE-WF-001](./backend/bugs/workflow-orchestration/) | Workflow Orch. | ~~🔴~~ ✅ FIXED | WorkflowOrchestrator đã implement (TDD-17) |
| [BUG-BE-CLI-001](./backend/bugs/cli-headless/) | CLI & Headless | 🟡 MEDIUM | orca daemon Unix Socket chưa implement đúng |
| **NEW** [BUG-BE-TG-001](./backend/bugs/task-graph/BUG-BE-TG-001-agent-exec-method-name-mismatch.md) | Task Graph | 🔴 **CRITICAL** | `relay.call('agent.exec')` → relay chỉ có `agent.execNonInteractive` |
| **NEW** [BUG-BE-TG-002](./backend/bugs/task-graph/BUG-BE-TG-002-ai-complete-relay-handler-missing.md) | Task Graph | 🔴 HIGH | `relay.call('ai.complete')` → relay không có handler `ai.complete` |
| **NEW** [BUG-BE-AT-001](./backend/bugs/automation/BUG-BE-AT-001-event-based-automation-not-implemented.md) | Automation | 🟡 MEDIUM | Event-based triggers (EventBus, GitHub Webhook) chưa implement |
| **NEW** [BUG-BE-AT-002](./backend/bugs/automation/BUG-BE-AT-002-worktree-cleanup-service-not-implemented.md) | Automation | 🟡 MEDIUM | WorktreeCleanupService chưa implement |
| **NEW** [BUG-BE-SSH-001](./backend/bugs/remote-development/BUG-BE-SSH-001-reconnect-no-exponential-backoff.md) | Remote Dev | 🟡 MEDIUM | SSH relay reconnect không có exponential backoff |
| **NEW** [BUG-BE-SSH-002](./backend/bugs/remote-development/BUG-BE-SSH-002-port-forward-no-db-persistence.md) | Remote Dev | 🟡 MEDIUM | Port forward không persist vào SQLite |

---

### 🌐 Frontend (Browser/Renderer)

| ID | Domain | Severity | Mô tả ngắn |
|----|--------|----------|------------|
| [BUG-FE-MB-001](./frontend/bugs/mobile-companion/) | Mobile Companion | 🔴 CRITICAL | Toàn bộ Mobile Companion UI chưa implement |
| [BUG-FE-CR-001](./frontend/bugs/code-review/) | Code Review | 🔴 HIGH | DiffViewer, Annotation UI chưa implement |
| [BUG-FE-FLEET-001](./frontend/bugs/fleet/) | Fleet Mgmt | 🔴 HIGH | Fleet Dashboard Admin SPA chưa implement |
| [BUG-FE-WF-001](./frontend/bugs/workflow-orchestration/) | Workflow Orch. | ~~🔴~~ ✅ PARTIAL | WorkflowBuilder — cần xác nhận lại |

---

## ✅ Đã implement tốt hơn dự kiến (false positives từ lần 1)

| Domain | Đánh giá lại |
|--------|-------------|
| Profile Management | `ProfileResolver` (TDD-14) ĐÃ implement đầy đủ 3-layer merge |
| Workflow Orchestration | `WorkflowOrchestrator` (TDD-17) ĐÃ implement với DAG builder |
| Task Graph | `TaskService`, `TaskGrantService`, `TaskAIPlanner`, `TaskAgentExecutor` ĐÃ implement |
| AI Provider (backend) | `AIProviderService` (TDD-16) ĐÃ implement với DB + relay write |
| Automation | `AutomationService` ĐÃ implement schedule-based triggers |
| Remote Development | SSH code rất đầy đủ — relay deploy, port forward, session management |
| fs.readDir/readFile/search | ĐÃ có handlers trong relay (`fs-handler.ts`) |
| git operations | ĐÃ có handlers trong relay (`git-handler.ts`) |

---

## 🔴 Critical Bugs cần fix ngay

| # | Bug ID | Severity | Tại sao Critical |
|---|--------|----------|-----------------|
| 1 | BUG-BE-TG-001 | 🔴 CRITICAL | `agent.exec` method không tồn tại trong relay → task execution broken |
| 2 | BUG-BE-TG-002 | 🔴 HIGH | `ai.complete` không tồn tại trong relay → AI planning broken |
| 3 | BUG-BE-MB-001 | 🔴 CRITICAL | Toàn bộ Mobile Companion domain missing |
| 4 | BUG-AG-AIP-002 | 🔴 HIGH | AI credentials không thể decrypt → AI không chạy được |
| 5 | BUG-BE-AWS-001 | 🔴 HIGH | Agent WS auth không verify DB → security hole |

---

## Tổng kết theo Mức độ (sau audit chi tiết)

| Severity | Số lượng |
|----------|---------|
| 🔴 CRITICAL | 3 |
| 🔴 HIGH | 12 |
| 🟡 MEDIUM | 11 |
| ~~False Positive~~ (đã implement) | 4 |
| **Tổng bugs thực** | **26** |

# CR v2 — Full-Flow Tracing Change Requests

**Phiên bản:** v1.0
**Ngày:** 2026-08-01
**Trạng thái:** Proposed
**Feature liên quan:** [F40 — Full-Flow Tracing (Observability)](../../../features/F40-full-flow-tracing.md)
**Nguồn luồng nghiệp vụ:** [`docs/flows/logic/`](../../../flows/logic/README.md)
**Core tracing module:** [`src/shared/trace/`](../../../../src/shared/trace/index.ts) (`index.ts`, `tracers.ts`, `browser.ts`)

---

## Mục tiêu

[F40](../../../features/F40-full-flow-tracing.md) đã ship core API tracing isomorphic (Node.js + browser) nhưng chỉ 3 business operation thực sự được instrument (`devServer:browseDir/mkdir/rmdir`). Bộ CR này áp dụng tracing đó lên **toàn bộ 19 luồng nghiệp vụ** trong `docs/flows/logic/` để hỗ trợ troubleshoot: khi một thao tác chậm hoặc fail, xác định được chính xác layer nào (Browser? RPC? Relay? Dev Server? SSH? DB?) là nguyên nhân.

---

## Change Requests

| CR | Domain | Flow doc | BL code | Priority | Phụ thuộc |
|----|--------|----------|---------|----------|-----------|
| [CR-TRACE-000](./CR-TRACE-000-tracing-rollout-overview.md) | **Foundational** — Gap Analysis & Propagation Architecture | — | — | P1 | — |
| [CR-TRACE-001](./CR-TRACE-001-worktree-management.md) | Worktree Management | [worktree-management.md](../../../flows/logic/worktree-management.md) | BL-WT-01→05 | P1 | CR-TRACE-000 |
| [CR-TRACE-002](./CR-TRACE-002-agent-orchestration.md) | Agent Orchestration | [agent-orchestration.md](../../../flows/logic/agent-orchestration.md) | BL-AG-01→05 | P1 | CR-TRACE-000 |
| [CR-TRACE-003](./CR-TRACE-003-terminal-management.md) | Terminal Management | [terminal-management.md](../../../flows/logic/terminal-management.md) | BL-TM-01→04 | P1 | CR-TRACE-000 |
| [CR-TRACE-004](./CR-TRACE-004-remote-development.md) | Remote Development (SSH) | [remote-development.md](../../../flows/logic/remote-development.md) | BL-SSH-01→04 | P1 | CR-TRACE-000 |
| [CR-TRACE-005](./CR-TRACE-005-code-review.md) | Code Review | [code-review.md](../../../flows/logic/code-review.md) | BL-CR-01→05 | P1 | CR-TRACE-000 |
| [CR-TRACE-006](./CR-TRACE-006-project-integration.md) | Project Integration (GitHub/GitLab/Linear) | [project-integration.md](../../../flows/logic/project-integration.md) | BL-PI-01→04 | P2 | CR-TRACE-000 |
| [CR-TRACE-007](./CR-TRACE-007-mobile-companion.md) | Mobile Companion | [mobile-companion.md](../../../flows/logic/mobile-companion.md) | BL-MB-01→04 | P2 | CR-TRACE-000 |
| [CR-TRACE-008](./CR-TRACE-008-automation.md) | Automation | [automation.md](../../../flows/logic/automation.md) | BL-AT-01→04 | P2 | CR-TRACE-000 |
| [CR-TRACE-009](./CR-TRACE-009-design-browser.md) | Design & Browser | [design-browser.md](../../../flows/logic/design-browser.md) | BL-DB-01→03 | P2 | CR-TRACE-000 |
| [CR-TRACE-010](./CR-TRACE-010-cli-headless.md) | CLI & Headless | [cli-headless.md](../../../flows/logic/cli-headless.md) | BL-CLI-01→03 | P2 | CR-TRACE-000 |
| [CR-TRACE-011](./CR-TRACE-011-auth.md) | Auth & User Management | [auth.md](../../../flows/logic/auth.md) | BL-AUTH-01→05 | P1 | CR-TRACE-000 |
| [CR-TRACE-012](./CR-TRACE-012-fleet.md) | Fleet Management | [fleet.md](../../../flows/logic/fleet.md) | BL-FLEET-01→04 | P2 | CR-TRACE-000 |
| [CR-TRACE-013](./CR-TRACE-013-agent-ws.md) | Agent WebSocket Protocol | [agent-ws.md](../../../flows/logic/agent-ws.md) | BL-AWS-01→03 | P2 | CR-TRACE-000 |
| [CR-TRACE-014](./CR-TRACE-014-remote-integration.md) | Remote Source Control Integrations | [remote-integration.md](../../../flows/logic/remote-integration.md) | BL-INT-01→03 | P2 | CR-TRACE-000 |
| [CR-TRACE-015](./CR-TRACE-015-profile.md) | Profile & Project Management | [profile.md](../../../flows/logic/profile.md) | BL-PRF-01→04 | P3 | CR-TRACE-000 |
| [CR-TRACE-016](./CR-TRACE-016-ai-providers.md) | AI Provider Management | [ai-providers.md](../../../flows/logic/ai-providers.md) | BL-AIP-01→03 | P3 | CR-TRACE-000 |
| [CR-TRACE-017](./CR-TRACE-017-workflow-orchestration.md) | Workflow Orchestration | [workflow-orchestration.md](../../../flows/logic/workflow-orchestration.md) | BL-WF-01→03 | P3 | CR-TRACE-000 |
| [CR-TRACE-018](./CR-TRACE-018-task-graph.md) | Task Graph | [task-graph.md](../../../flows/logic/task-graph.md) | BL-TG-01→04 | P3 | CR-TRACE-000 |
| [CR-TRACE-019](./CR-TRACE-019-project-workspace.md) | Project Workspace | [project-workspace.md](../../../flows/logic/project-workspace.md) | BL-PW-01→04 | P3 | CR-TRACE-000 |

---

## Đọc theo thứ tự nào?

1. **CR-TRACE-000 trước tiên, luôn luôn.** Nó định nghĩa 3 thứ mọi CR khác phụ thuộc vào:
   - Core API change: `Tracer.start(fields?, resume?: { id })` để nối tiếp span `id` qua process boundary.
   - Quy ước lan truyền `traceId` cho 6 loại transport (WS RPC, `relay.call()`, Agent WS JSON-RPC, HTTP/WS CLI, Mobile WS mã hoá NaCl, SSH exec).
   - Quy ước đặt tên tracer `domain:operation` (1 tracer / 1 `BL-XXX-NN`) và nguyên tắc khi nào dùng `span.step()`.
2. Sau đó triển khai theo **Rollout Phases** (CR-TRACE-000 §6):

| Phase | CRs | Vì sao ưu tiên |
|-------|-----|-----------------|
| 1 | 001–004 | Worktree, Agent Orchestration, Terminal, SSH — luồng core hàng ngày |
| 2 | 005–008 | Code Review, Project Integration, Mobile, Automation — tích hợp bên ngoài dễ fail |
| 3 | 009–012 | Design/Browser, CLI/Headless, Auth, Fleet — vận hành/admin |
| 4 | 013–016 | Agent WS, Remote Integration, Profile, AI Providers — nền tảng ít user-facing hơn |
| 5 | 017–019 | Workflow Orchestration, Task Graph, Project Workspace — luồng DAG/multi-step, phức tạp nhất |

---

## Phát hiện quan trọng khi viết bộ CR này: Doc/Code Drift

Khi nghiên cứu implementation thực tế cho từng flow (grep + đọc source, không đoán), nhiều tên component trong `docs/flows/logic/*.md` **không tồn tại trong code** — chúng mô tả kiến trúc mức HLD, còn code thực tế dùng module/class khác. Ví dụ tiêu biểu (chi tiết đầy đủ nằm trong từng CR và ở CR-TRACE-000 §8):

| Flow doc mô tả | Code thực tế |
|-----------------|----------------|
| `WorktreeManager` | `src/main/runtime/rpc/methods/worktree.ts` + `git-remote.ts` (qua `ProjectServerRouter`/`RelayConnectionPool`) |
| `AgentManager.spawn()` / relay `agent.spawn` | `ProfileAwareAgentSpawner.spawn()` → relay method `agent.exec` |
| `SshManager`/`RelayManager`/`PortForwardManager` | `SshConnectionManager`, `SshRelaySession`, `SshPortForwardManager` |
| `PairingManager`/`MobileDispatchHandler` | Dùng chung `OrcaRuntimeRpcServer`/`RpcDispatcher` + `MOBILE_RPC_METHOD_ALLOWLIST`; push dùng Web Push (RFC 8030), không phải APNs/FCM |
| `FleetManager`/`FleetProvisioner` | `fleet-health-monitor.ts`, `fleet-status-service.ts`, `fleet-bootstrap-service.ts` |
| `CliAuthProxy`, `PreflightService` (CI/PR merge) | Không tồn tại; preflight thực tế chỉ check CLI tool/auth detection |
| `TaskGraphBuilder`, `WorkflowServerResolver`, `TemplateRegistry` | Không tồn tại riêng — logic nằm trong `TaskService`, chưa có cascade routing, đã gộp vào `TemplateResolver` |

Một số sub-flow được flow doc mô tả nhưng **chưa có implementation** (BL-CR-02/03 review-feedback injection, BL-AWS-03 admin token CRUD, BL-WF-03 workflow sharing, BL-PW-04 task-workspace integration, BL-DB-02/03 QA viewport matrix) — các CR liên quan đánh dấu rõ "chưa xác định file cụ thể" thay vì đưa instrumentation giả định cho code chưa tồn tại.

**Hệ quả:** đọc CR-TRACE-001…019 theo bảng "File:function" thực tế trong từng CR, không theo tên component ở flow doc gốc. Khuyến nghị cập nhật lại `docs/flows/logic/*.md` (ngoài phạm vi bộ CR này) theo mẫu ghi chú "⚠️ Kiến trúc thực tế" mà `worktree-management.md` và `cli-headless.md` đã áp dụng.

---

## Tracer registry sau khi rollout đầy đủ

Trước rollout: 11 tracer đã tồn tại (`devServer:browseDir/mkdir/rmdir`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`, `wsSession:route`, `session:spawn`, `agentToken:register`, `agent:tokenManager` — xem CR-TRACE-000 §2 GAP-3).

Sau rollout: mỗi CR-TRACE-001…019 thêm tracer riêng theo namespace domain (`worktree:*`, `agentOrch:*`, `terminal:*`, `ssh:*`, `codeReview:*`, `projectIntegration:*`, `mobile:*`, `automation:*`, `designBrowser:*`, `cli:*`, `auth:*`, `fleet:*`, `agentWs:*` mở rộng, `remoteIntegration:*`, `profile:*`, `aiProvider:*`, `workflow:*`, `taskGraph:*`, `projectWorkspace:*`) — chi tiết từng tracer nằm trong mục 3 của mỗi CR.

---

## Ràng buộc bảo mật xuyên suốt

Một số CR (Auth, Remote Integration, AI Providers) có ràng buộc bảo mật riêng, áp dụng chung cho toàn bộ rollout:

- **Không bao giờ** đưa password, session token, credential đã decrypt, hoặc API key vào `TraceFields` của bất kỳ span nào — chỉ metadata (`accountId`, `provider`, `userId`) được phép.
- `fail()` events luôn log ra console bất kể `ORCA_TRACE` flag (theo thiết kế F40) — vì vậy field nào lọt vào `span.fail(err, fields)` chắc chắn sẽ log, kể cả khi tracing đang tắt. Rà soát kỹ trước khi thêm field vào các call `fail()` ở luồng auth/credential.

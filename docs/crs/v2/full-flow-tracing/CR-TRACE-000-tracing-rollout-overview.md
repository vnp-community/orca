# CR-TRACE-000 — Full-Flow Tracing Rollout: Gap Analysis & Propagation Architecture

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-000 |
| **Tên** | Full-Flow Tracing Rollout — Gap Analysis & Cross-Boundary Propagation Architecture |
| **Loại** | Architecture / Observability |
| **Priority** | P1 — Should Have |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | — (foundational; đọc trước khi triển khai CR-TRACE-001 → CR-TRACE-019) |
| **Tác động** | `src/shared/trace/*`, `docs/features/F40-full-flow-tracing.md`, toàn bộ `docs/flows/logic/*.md` |

---

## 1. Bối cảnh

[F40 — Full-Flow Tracing](../../../features/F40-full-flow-tracing.md) đã ship core API isomorphic (`src/shared/trace/index.ts`, `tracers.ts`, `browser.ts`) và SSE bridge (`trace-sse-routes.ts`). Tuy nhiên **implementation coverage hiện tại rất hẹp**: chỉ 3 business operations có tracer thực sự (`devServer:browseDir`, `devServer:mkdir`, `devServer:rmdir`) cộng với hạ tầng nội bộ (`agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`).

19 luồng nghiệp vụ trong [`docs/flows/logic/`](../../../flows/logic/README.md) (≈70+ sub-flow `BL-XXX-NN`) — worktree, agent orchestration, terminal, remote dev (SSH), code review, project integration, mobile companion, automation, design/browser, CLI/headless, auth, fleet, agent WS protocol, remote integration, profile, AI providers, workflow orchestration, task graph, project workspace — **không có bất kỳ instrumentation nào**. Khi một trong các luồng này chậm hoặc fail, không có cách nào biết bước nào (Browser? RPC? Relay? Dev Server? SSH? DB?) là nguyên nhân — đúng vấn đề mà F40 được tạo ra để giải quyết, nhưng chưa được áp dụng ra ngoài `devServer:*`.

CR này (1) chỉ ra các gap kiến trúc cần vá **trước khi** viết CR cho từng luồng, và (2) định nghĩa quy ước đặt tên + lan truyền `traceId` xuyên suốt các loại transport khác nhau mà 19 luồng này dùng (không chỉ WS RPC như `devServer:*`).

---

## 2. Gap Analysis

### GAP-1: Span `id` KHÔNG nhất quán xuyên process, dù F40 acceptance criteria yêu cầu

Code hiện tại (`src/main/runtime/rpc/methods/dev-server.ts:208`):

```typescript
const span = Tracers.browseDirFlow.start({ devServerId: params.id, path: params.path })
```

`Tracer.start()` (`src/shared/trace/index.ts:150-156`) luôn gọi `shortId()` nội bộ — **không có tham số nào để tiếp nhận một id đã tồn tại từ layer trước**. Sơ đồ minh hoạ trong F40 (`id=abc123` xuyên suốt Browser → Main → Relay → Agent) là **mô hình mong muốn, chưa phải hiện trạng**: mỗi layer hiện tại tạo span độc lập với id ngẫu nhiên riêng. Đây là lý do acceptance criteria "Span `id` nhất quán xuyên suốt toàn bộ stack" trong F40 vẫn để trống `[ ]`.

### GAP-2: Không có wire-envelope convention để mang `traceId` qua boundary

19 luồng trong `docs/flows/logic/` dùng ít nhất 6 loại transport khác nhau để băng qua process/host boundary, và **không loại nào có chỗ định nghĩa sẵn cho `traceId`**:

| Transport | Luồng dùng | File đại diện |
|-----------|-----------|----------------|
| WebSocket RPC (Browser ↔ Orca Server) | Worktree, Terminal, Task Graph, Project Workspace, ... | `src/main/runtime/rpc/methods/*.ts` |
| `relay.call()` (Orca Server ↔ Dev Server, qua SSH exec hoặc Agent WS) | Worktree, Agent Orchestration, Terminal, Automation | `src/main/dev-server/dev-server-relay-bridge.ts` |
| Agent WS JSON-RPC 2.0 (Orca ↔ Custom Agent) | Agent WebSocket Protocol | `src/main/dev-server/agent-ws-server.ts`, `src/relay/agent-rpc-dispatch.ts` |
| HTTP/WS `:6768` (CLI ↔ Electron Main — **không phải Unix Socket** theo ghi chú thực tế trong `cli-headless.md`) | CLI & Headless | `src/main/automations/service.ts` |
| WebSocket + TweetNaCl box (Mobile ↔ Main) | Mobile Companion | `PairingManager`, `MobileDispatchHandler` |
| SSH exec / `SshChannelMultiplexer` (Main ↔ Remote Host) | Remote Development | `SshManager`, `RelayManager` |

Không có quy ước → mỗi CR luồng sẽ tự nghĩ ra field name khác nhau nếu không chuẩn hoá trước.

### GAP-3: Chỉ 5/~70+ sub-flow có tracer; naming convention chưa dự phòng va chạm

`tracers.ts` hiện chỉ export `browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow`. Các `createTracer()` ad-hoc khác đã xác nhận tồn tại trong code (do các CR-TRACE-001…019 phát hiện thêm khi grep, ngoài `relay:agentCall`/`agent:rpc` đã biết từ trước):

| Flow name | File | Ghi chú |
|-----------|------|---------|
| `relay:agentCall` | `dev-server-relay-bridge.ts` | Đã biết từ F40 |
| `agent:rpc` | `src/relay/agent-rpc-dispatch.ts:21` | Đã biết từ F40 |
| `wsSession:route` | `src/main/session/ws-session-router.ts` | Phát hiện khi viết CR-TRACE-011 (Auth) |
| `session:spawn` | `src/main/session/session-manager.ts` | Phát hiện khi viết CR-TRACE-011 (Auth) |
| `agentToken:register` | `src/server/agent-token-routes.ts:29` | Phát hiện khi viết CR-TRACE-013 (Agent WS) |
| `agent:tokenManager` | `src/relay/agent-token-manager.ts:24` | Phát hiện khi viết CR-TRACE-013 (Agent WS) |

Tổng cộng **11 tracer đã tồn tại** trước khi rollout này bắt đầu — nhiều hơn con số 5 nêu trong F40. Khi 19 domain mới thêm tracer riêng, cần namespace rõ ràng để không đụng các tên trên (đặc biệt `agent:*` đã dùng cho JSON-RPC dispatch nội bộ, dễ nhầm với Agent Orchestration domain — xem `agentOrch:` prefix ở mục 4).

### GAP-4: Chưa có hướng dẫn "khi nào nên `step()`" để tránh over-instrumentation

Nhiều bước trong các luồng là single DB query (`SELECT`/`INSERT`/`UPDATE` một dòng) — không phải lúc nào cũng đáng một `span.step()` riêng. Nếu không có nguyên tắc chung, một số CR sẽ instrument quá dày (noise), số khác quá thưa (thiếu thông tin debug).

---

## 3. Core API Change (bắt buộc trước CR-TRACE-001 → 019)

### 3.1 `Tracer.start()` nhận optional resume id

```typescript
// src/shared/trace/index.ts
export interface Tracer {
  start(fields?: TraceFields, resume?: { id: string }): TraceSpan
}

export function createTracer(flow: string): Tracer {
  return {
    start(fields: TraceFields = {}, resume?: { id: string }): TraceSpan {
      const id = resume?.id ?? shortId()
      const startMs = Date.now()
      emit({ id, flow, level: 'start', fields, ts: startMs })
      // step/ok/fail giữ nguyên logic, dùng `id` ở trên
      ...
    }
  }
}
```

- Backward compatible: mọi call site hiện tại (`Tracers.browseDirFlow.start(fields)`) không cần đổi.
- Khi có `resume.id`, span **tiếp nối** id từ layer trước thay vì random mới → cùng một `id` xuất hiện xuyên suốt Browser/Main/Relay/Agent trong TracePanel và log.
- `elapsedMs` mỗi layer vẫn tính từ `startMs` cục bộ của chính layer đó (không phải từ layer đầu tiên) — mỗi layer đo latency riêng của nó; muốn tổng latency end-to-end thì lấy `ts` của `start` event đầu tiên trừ `ts` của `ok`/`fail` cuối cùng (TracePanel/log aggregation làm việc này, không phải `TraceSpan`).

### 3.2 Quy ước field `traceId` trong wire envelope

`traceId` là **tên field chuẩn** dùng ở lớp giao thức (không phải trong `TraceFields` của span) để mang span `id` qua boundary. Call site đọc `traceId` từ payload nhận được và truyền vào `resume`:

```typescript
const span = Tracers.worktreeCreateFlow.start(
  { projectId, baseRef },
  params.traceId ? { id: params.traceId } : undefined
)
// Khi forward tiếp xuống layer sau, đính kèm span.id:
relay.call('git.worktree.add', { ...gitParams, traceId: span.id })
```

### 3.3 Bảng lan truyền theo từng transport

| Transport | Vị trí đặt `traceId` | Ghi chú |
|-----------|----------------------|---------|
| WebSocket RPC (Browser ↔ Orca Server) | Sibling field cạnh `method`/`params` trong request envelope; response echo lại | Browser tạo `traceId` đầu tiên bằng tracer riêng của nó, gửi kèm RPC call |
| `relay.call()` (Orca Server ↔ Dev Server) | Field `traceId` trong params envelope của `DevServerRelayBridge.call()` | `relayCallTracer` (`relay:agentCall`) resume bằng `traceId` nhận từ RPC method layer |
| Agent WS JSON-RPC 2.0 | Nested trong `params._trace.id` (không đụng field `id` sẵn có của JSON-RPC dùng cho request/response matching) | Áp dụng cho cả `relay-websocket` và `direct-websocket` mode |
| HTTP/WS `:6768` (CLI ↔ Electron Main) | Giống convention WS RPC ở trên — CLI tự tạo `traceId` bằng `createTracer('cli:<command>').start()` trước khi gửi request | Không có Unix Socket thật trong code hiện tại (xem ghi chú "Khác biệt HLD vs Implementation" trong `cli-headless.md`) |
| WebSocket + TweetNaCl box (Mobile ↔ Main) | Field `traceId` nằm **trong** payload JSON trước khi encrypt (cùng cấp với `type`) | Không đặt ngoài envelope mã hoá vì toàn bộ payload là opaque ciphertext ở tầng transport |
| SSH exec / `SshChannelMultiplexer` (Main ↔ Remote Host) | Không lan truyền vào remote shell process — chỉ trace các bước phía Main (`ssh2.connect`, `exec`, tunnel setup) | Remote shell không chạy code Orca nên không thể nhận span |

---

## 4. Quy ước đặt tên Tracer cho 19 domain

Namespace theo `domain:operation`, tránh trùng với tracer nội bộ đã có (`devServer:*`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`):

| Domain (flow doc) | Prefix | Ví dụ flow name |
|--------------------|--------|------------------|
| worktree-management.md | `worktree:` | `worktree:create`, `worktree:fanOut`, `worktree:delete`, `worktree:compare`, `worktree:merge` |
| agent-orchestration.md | `agentOrch:` | `agentOrch:spawn`, `agentOrch:promptInject`, `agentOrch:statusPoll`, `agentOrch:kill` |
| terminal-management.md | `terminal:` | `terminal:create`, `terminal:resize`, `terminal:destroy`, `terminal:reconnect` |
| remote-development.md | `ssh:` | `ssh:connect`, `ssh:deployRelay`, `ssh:reconnect`, `ssh:portForward` |
| code-review.md | `codeReview:` | `codeReview:createPr`, `codeReview:aiReview`, `codeReview:comment`, `codeReview:merge` |
| project-integration.md | `projectIntegration:` | `projectIntegration:linkRepo`, `projectIntegration:syncIssues`, `projectIntegration:credentialRefresh` |
| mobile-companion.md | `mobile:` | `mobile:pair`, `mobile:push`, `mobile:dispatch`, `mobile:statusQuery` |
| automation.md | `automation:` | `automation:scheduleRun`, `automation:trigger`, `automation:eventDispatch` |
| design-browser.md | `designBrowser:` | `designBrowser:cdpConnect`, `designBrowser:screenshot`, `designBrowser:inspect` |
| cli-headless.md | `cli:` | `cli:worktreeCreate`, `cli:agentStart`, `cli:daemonCommand` |
| auth.md | `auth:` | `auth:login`, `auth:sessionRefresh`, `auth:permissionCheck`, `auth:audit` |
| fleet.md | `fleet:` | `fleet:healthCheck`, `fleet:sshProbe`, `fleet:sftpSync` |
| agent-ws.md | `agentWs:` (mở rộng, giữ `agentWs:lifecycle` sẵn có) | `agentWs:handshake`, `agentWs:tokenVerify`, `agentWs:tokenManage` |
| remote-integration.md | `remoteIntegration:` | `remoteIntegration:credentialDecrypt`, `remoteIntegration:ghExec`, `remoteIntegration:preflight` |
| profile.md | `profile:` | `profile:resolve`, `profile:mergeLayer` |
| ai-providers.md | `aiProvider:` | `aiProvider:encryptCred`, `aiProvider:quotaCheck`, `aiProvider:relayDispatch` |
| workflow-orchestration.md | `workflow:` | `workflow:planDag`, `workflow:runWave`, `workflow:stepExecute` |
| task-graph.md | `taskGraph:` | `taskGraph:plan`, `taskGraph:grantResolve`, `taskGraph:execute` |
| project-workspace.md | `projectWorkspace:` | `projectWorkspace:explorerBrowse`, `projectWorkspace:gitStatus`, `projectWorkspace:agentAttach` |

**Nguyên tắc:** 1 tracer = 1 sub-flow `BL-XXX-NN` (không gộp nhiều BL vào 1 tracer, không tách 1 BL thành nhiều tracer).

---

## 5. Nguyên tắc "khi nào `step()`" (chống over-instrumentation)

`span.step()` chỉ dùng khi bước đó:
1. **Băng qua process/host boundary** (RPC, relay.call, SSH exec, WS frame) — luôn đáng trace.
2. **Có khả năng chậm hoặc fail độc lập** (network call, external API, spawn process).
3. **Là điểm rẽ nhánh quan trọng** cho troubleshoot (vd: "cache hit" vs "cache miss", "local" vs "remote" path).

KHÔNG dùng `step()` cho: single-row SQLite SELECT/INSERT/UPDATE thuần tuý trong cùng process, hoặc biến đổi dữ liệu in-memory thuần tuý (map/filter). Những bước này có thể gộp field vào `ok()`/`fail()` cuối span thay vì step riêng.

---

## 6. Rollout Phases

| Phase | CRs | Lý do ưu tiên |
|-------|-----|----------------|
| Phase 0 | CR-TRACE-000 (CR này) | Core API + convention, không đổi behavior tracer hiện có |
| Phase 1 | CR-TRACE-001…004 (Worktree, Agent Orchestration, Terminal, Remote Dev/SSH) | Luồng core hàng ngày, nhiều support ticket nhất |
| Phase 2 | CR-TRACE-005…008 (Code Review, Project Integration, Mobile Companion, Automation) | Luồng tích hợp bên ngoài, dễ fail do network/API thứ 3 |
| Phase 3 | CR-TRACE-009…012 (Design & Browser, CLI & Headless, Auth, Fleet) | Luồng vận hành/admin |
| Phase 4 | CR-TRACE-013…016 (Agent WS Protocol, Remote Integration, Profile, AI Providers) | Luồng nền tảng ít user-facing hơn |
| Phase 5 | CR-TRACE-017…019 (Workflow Orchestration, Task Graph, Project Workspace) | Luồng phức tạp nhất (DAG/multi-step), cần Phase 1-4 ổn định trước |

---

## 7. Danh sách Change Requests

| CR | Domain | Flow doc | BL code | Priority |
|----|--------|----------|---------|----------|
| [CR-TRACE-001](./CR-TRACE-001-worktree-management.md) | Worktree Management | worktree-management.md | BL-WT-01→05 | P1 |
| [CR-TRACE-002](./CR-TRACE-002-agent-orchestration.md) | Agent Orchestration | agent-orchestration.md | BL-AG-01→05 | P1 |
| [CR-TRACE-003](./CR-TRACE-003-terminal-management.md) | Terminal Management | terminal-management.md | BL-TM-01→04 | P1 |
| [CR-TRACE-004](./CR-TRACE-004-remote-development.md) | Remote Development (SSH) | remote-development.md | BL-SSH-01→04 | P1 |
| [CR-TRACE-005](./CR-TRACE-005-code-review.md) | Code Review | code-review.md | BL-CR-01→05 | P1 |
| [CR-TRACE-006](./CR-TRACE-006-project-integration.md) | Project Integration | project-integration.md | BL-PI-01→04 | P2 |
| [CR-TRACE-007](./CR-TRACE-007-mobile-companion.md) | Mobile Companion | mobile-companion.md | BL-MB-01→04 | P2 |
| [CR-TRACE-008](./CR-TRACE-008-automation.md) | Automation | automation.md | BL-AT-01→04 | P2 |
| [CR-TRACE-009](./CR-TRACE-009-design-browser.md) | Design & Browser | design-browser.md | BL-DB-01→03 | P2 |
| [CR-TRACE-010](./CR-TRACE-010-cli-headless.md) | CLI & Headless | cli-headless.md | BL-CLI-01→03 | P2 |
| [CR-TRACE-011](./CR-TRACE-011-auth.md) | Auth & User Mgmt | auth.md | BL-AUTH-01→05 | P1 |
| [CR-TRACE-012](./CR-TRACE-012-fleet.md) | Fleet Management | fleet.md | BL-FLEET-01→04 | P2 |
| [CR-TRACE-013](./CR-TRACE-013-agent-ws.md) | Agent WebSocket | agent-ws.md | BL-AWS-01→03 | P2 |
| [CR-TRACE-014](./CR-TRACE-014-remote-integration.md) | Remote Integration | remote-integration.md | BL-INT-01→03 | P2 |
| [CR-TRACE-015](./CR-TRACE-015-profile.md) | Profile & Project | profile.md | BL-PRF-01→04 | P3 |
| [CR-TRACE-016](./CR-TRACE-016-ai-providers.md) | AI Provider Mgmt | ai-providers.md | BL-AIP-01→03 | P3 |
| [CR-TRACE-017](./CR-TRACE-017-workflow-orchestration.md) | Workflow Orchestration | workflow-orchestration.md | BL-WF-01→03 | P3 |
| [CR-TRACE-018](./CR-TRACE-018-task-graph.md) | Task Graph | task-graph.md | BL-TG-01→04 | P3 |
| [CR-TRACE-019](./CR-TRACE-019-project-workspace.md) | Project Workspace | project-workspace.md | BL-PW-01→04 | P3 |

---

## 8. Ghi chú triển khai — Doc/Code Drift phát hiện khi viết CR-TRACE-001…019

Khi viết CR cho từng domain, nhiều tên component trong `docs/flows/logic/*.md` (ví dụ `WorktreeManager`, `AgentManager`, `PairingManager`, `MobileDispatchHandler`, `FleetManager`, `SshManager`, `RelayManager`, `CliAuthProxy`, `PreflightService`, `TaskGraphBuilder`, `WorkflowServerResolver`, `TemplateRegistry`) **không tồn tại verbatim trong code** — chúng là tên kiến trúc mức HLD, còn implementation thực tế nằm ở các module/class khác (đã liệt kê cụ thể trong từng CR tương ứng, kèm file:line đã verify qua grep/Read).

Hệ quả cho việc triển khai:
- CR-TRACE-001…019 mô tả instrumentation dựa trên **code thực tế**, không phải theo tên trong flow doc — khi đọc một CR, ưu tiên bảng "File:function" trong đó hơn là tên component trong flow doc gốc.
- Một số sub-flow (`BL-XXX-NN`) được flow doc mô tả nhưng **chưa có implementation nào** (ví dụ: BL-CR-02/03 review-feedback injection, BL-AWS-03 admin token CRUD, BL-WF-03 workflow sharing, BL-PW-04 task-workspace integration, BL-DB-02/03 design-browser QA matrix). Các CR liên quan đã đánh dấu rõ "chưa xác định file cụ thể — cần điều tra thêm khi triển khai" thay vì đưa instrumentation giả định — **không nên bắt đầu implement tracing cho các sub-flow này cho đến khi feature gốc tồn tại**.
- Khuyến nghị riêng (ngoài phạm vi CR-TRACE series): cập nhật lại `docs/flows/logic/*.md` để phản ánh đúng kiến trúc hiện tại, tương tự cách `worktree-management.md` và `cli-headless.md` đã có sẵn ghi chú "⚠️ Kiến trúc thực tế" / "Khác biệt HLD vs Implementation".

---

## Acceptance Criteria (CR-TRACE-000)

- [ ] `Tracer.start(fields?, resume?)` triển khai, backward compatible với mọi call site hiện có
- [ ] Khi `resume.id` được truyền, span `id` == `resume.id` (không random mới)
- [ ] Quy ước `traceId` field cho cả 6 loại transport được document và tất cả CR-TRACE-001…019 tham chiếu về đây thay vì tự định nghĩa lại
- [ ] Naming convention (mục 4) không va chạm với tracer nội bộ hiện có (`devServer:*`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`)
- [ ] Nguyên tắc "khi nào step()" (mục 5) được áp dụng nhất quán khi review CR-TRACE-001…019
- [ ] `docs/features/F40-full-flow-tracing.md` cập nhật mục "Các flows được trace sẵn" trỏ về CR-TRACE series thay vì chỉ liệt kê 5 tracer cũ

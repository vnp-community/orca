# CR-TRACE-002 — Agent Orchestration Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-002 |
| **Tên** | Agent Orchestration — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/agent-orchestration.md`, `src/main/project/ProfileAwareAgentSpawner.ts`, `src/main/project/ProjectServerRouter.ts`, `src/main/profile/ProfileResolver.ts`, `src/main/dev-server/agent-ws-server.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`, `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-spawner.ts`, `src/relay/agent-session.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

`docs/flows/logic/agent-orchestration.md` mô tả `AgentManager`, `AgentConnectionManager`, `AgentHookParser` như các thành phần trung tâm điều phối agent. Không cái nào trong 3 class này tồn tại trong code (đã grep `class AgentManager`, `class AgentConnectionManager`, `class AgentHookParser`, `AgentHookParser` dạng bất kỳ trên toàn bộ `src/` — không có kết quả). Thành phần thật gần nhất với từng vai trò:

- **Spawn agent (BL-AG-01)**: `ProfileAwareAgentSpawner.spawn()` (`src/main/project/ProfileAwareAgentSpawner.ts:67-129`) — kết hợp `ProjectServerRouter.getProjectContext()` (resolve profile + provider), build env, rồi gọi **`relay.call('agent.exec', {...})`** — chú ý đây là `agent.exec`, **không phải** `agent.spawn` như flow doc ghi. Có comment trong code (`ProfileAwareAgentSpawner.ts:108-109`) xác nhận field mapping từng bị sai (`command/workdir` → `binary/args/cwd`) và đã fix — một dấu hiệu cụ thể rằng thiếu observability từng khiến lỗi integration này khó phát hiện.
- **Kết nối WS Dev Server → Orca**: `AgentWebSocketServer` (`src/main/dev-server/agent-ws-server.ts:41`) — đã có tracer `Tracers.agentWsFlow` (`agentWs:lifecycle`) cho bước handshake, nhưng **không có gì sau đó**: không trace việc lưu connection vào registry, không trace việc chọn connection nào khi spawn.
- **Dừng/kill agent (BL-AG-02), resume (BL-AG-03), switch account (BL-AG-04), OSC status parsing (BL-AG-05)**: không tìm thấy call site thật nào gọi `relay.call('agent.kill'|'agent.sendInput'|'agent.status')` từ phía Orca Server (chỉ có case handler phía Dev Server trong `agent-rpc-dispatch.ts`, không có caller). Cũng không có pattern-matching rate-limit hay OSC 133 agent-hook nào ở phía Orca Server tương ứng mô tả trong flow doc. Đây là gap triển khai thực sự, không chỉ gap tracing.

Hệ quả: khi agent spawn chậm hoặc lỗi hôm nay, **không thể phân biệt được** chậm ở đâu trong chuỗi `ProfileResolver.resolve()` (đọc profile) → `AIProviderResolver`-tương-đương (resolve provider) → `ProjectServerRouter.getRelayForProject()` (lấy/khởi tạo relay connection) → `relay.call('agent.exec')` (network + Dev Server side `node-pty.spawn`). `relay:agentCall` span hiện tại (GAP-3, CR-TRACE-000) chỉ log `{ devServerId, method: 'agent.exec' }` — không có `binary`, không có thời gian riêng cho từng bước resolve trước khi gọi relay, nên một agent chậm khởi động vì profile resolver treo (ví dụ DB lock) nhìn giống hệt một agent chậm vì network tới Dev Server chậm.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Renderer / Browser (agent card, prompt input) | UI | contextBridge (desktop) hoặc WebSocket RPC (web) | "WebSocket RPC (Browser ↔ Orca Server)" cho web mode |
| `ProfileAwareAgentSpawner.spawn()` (`ProfileAwareAgentSpawner.ts`) | Business Logic | in-process, gọi ra ngoài qua `relay.call` | — (điểm tạo span domain) |
| `ProjectServerRouter.getProjectContext()` / `getRelayForProject()` (`ProjectServerRouter.ts`) | Infrastructure | in-process | — |
| `ProfileResolver.resolve()` (`src/main/profile/ProfileResolver.ts:44`) | Business Logic | in-process (đọc DB/file profile) | — |
| `AgentWebSocketServer` (`agent-ws-server.ts:41`) | Infrastructure | Agent WS JSON-RPC 2.0 (Dev Server chủ động connect) | "Agent WS JSON-RPC 2.0" — đã có `agentWs:lifecycle` cho handshake |
| `DevServerRelayBridge.call()` (`dev-server-relay-bridge.ts:562-630`) | Transport | `relay.call()` (Orca Server ↔ Dev Server) | "`relay.call()` (Orca Server ↔ Dev Server)" — đã có `relay:agentCall` |
| `src/relay/agent-rpc-dispatch.ts` case `agent.exec`/`agent.spawn`/`agent.kill`/`agent.sendInput` | Runtime (Dev Server) | Agent WS JSON-RPC 2.0 | "Agent WS JSON-RPC 2.0" — nested `params._trace.id` |
| `src/relay/agent-spawner.ts`, `src/relay/agent-session.ts` | Runtime (Dev Server, node-pty spawn thực tế) | in-process trên Dev Server | — |
| Server DB (`orca_sessions`) | Persistence | in-process | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  agentOrchSpawn:      createTracer('agentOrch:spawn'),      // BL-AG-01
  agentOrchStop:       createTracer('agentOrch:stop'),       // BL-AG-02
  agentOrchResume:     createTracer('agentOrch:resume'),     // BL-AG-03
  agentOrchSwitch:     createTracer('agentOrch:switch'),     // BL-AG-04
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'), // BL-AG-05
}
```

> Đặt tên `agentOrch:*` (không phải `agent:*`) theo đúng quy ước CR-TRACE-000 §4, tránh va chạm với tracer hạ tầng `agent:rpc` (`agent-rpc-dispatch.ts:21`) vốn wrap generic mọi JSON-RPC method trên Dev Server, không riêng cho orchestration nghiệp vụ.

## 4. Instrumentation theo từng sub-flow

### BL-AG-01 — Khởi động AI Agent

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận request spawn | `start` | `{ projectId, agentType }` | `src/main/project/ProfileAwareAgentSpawner.ts:67` (`spawn()`) |
| Resolve project context (access + profile) | `step('resolve-context')` | `{ projectId }` | `ProfileAwareAgentSpawner.ts:71` → `ProjectServerRouter.getProjectContext()` (`ProjectServerRouter.ts:49`) |
| Resolve AI provider | `step('resolve-provider')` | `{ providerId }` hoặc rỗng nếu null | `ProfileAwareAgentSpawner.ts:96` (`providerService.resolveForProject`) |
| Lấy relay cho project | `step('get-relay')` | `{ devServerId: project.devServerId }` | `ProfileAwareAgentSpawner.ts:110` → `ProjectServerRouter.getRelayForProject()` |
| Gọi `agent.exec` qua relay | `step('relay-agent-exec')` | `{ binary, devServerId }` (KHÔNG log `env` — chứa path/secret-adjacent data) | `ProfileAwareAgentSpawner.ts:115-121` |
| Hoàn tất | `ok` / `fail` | `{ sessionId }` | `ProfileAwareAgentSpawner.ts:123-128` |

```typescript
// src/main/project/ProfileAwareAgentSpawner.ts — trong spawn()
async spawn(options: AgentSpawnOptions): Promise<AgentSpawnResult> {
  const { projectId, userId, command, extraEnv, workdir } = options
  const span = Tracers.agentOrchSpawn.start(
    { projectId, userId },
    options.traceId ? { id: options.traceId } : undefined
  )
  try {
    span.step('resolve-context', { projectId })
    const ctx = await this.router.getProjectContext(projectId, userId, this.profileResolver)
    // ... build profileEnv như hiện tại ...
    span.step('resolve-provider', { providerId: provider?.providerId ?? 'none' })
    const relay = await this.router.getRelayForProject(projectId, userId)
    span.step('relay-agent-exec', { binary, devServerId: ctx.project.devServerId })
    const result = await relay.call('agent.exec', { binary, args, cwd: workdir ?? ctx.project.repoPath, env: profileEnv, timeoutMs: 5 * 60 * 1000, traceId: span.id })
    const sessionId = (result as { sessionId?: string }).sessionId ?? randomId()
    span.ok({ sessionId })
    return { sessionId, provider: provider ? { providerId: provider.providerId, modelId: provider.modelId } : undefined }
  } catch (err) {
    span.fail(err, { projectId })
    throw err
  }
}
```

### BL-AG-02 — Dừng Agent

**Không tìm thấy call site thật gọi `relay.call('agent.sendInput'|'agent.kill')` từ Orca Server — chưa xác định file cụ thể, cần điều tra/implement khi triển khai.** `agent-rpc-dispatch.ts` (dòng 475, 488) chỉ có handler phía Dev Server nhận các lệnh này; caller phía Orca Server phải được xây trước khi gắn tracer.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận request stop | `start` | `{ sessionId, force }` | chưa xác định file cụ thể |
| Gửi `agent.sendInput` (Ctrl+C) hoặc `agent.kill` | `step('relay-agent-sendInput')` / `step('relay-agent-kill')` | `{ ptyId }` | tương ứng case `agent.sendInput`/`agent.kill` trong `src/relay/agent-rpc-dispatch.ts:475-501` (phía nhận) |
| Cập nhật trạng thái session | không cần `step()` (single-row UPDATE) | gộp vào `ok()` | chưa xác định file cụ thể |
| Hoàn tất | `ok`/`fail` | `{ sessionId, forced: boolean }` | — |

```typescript
// Khi call site 'agent.stop' được implement ở Orca Server
const span = Tracers.agentOrchStop.start({ sessionId, force })
try {
  if (!force) {
    span.step('relay-agent-sendInput', { ptyId })
    await relay.call('agent.sendInput', { ptyId, data: '\x03', traceId: span.id })
  } else {
    span.step('relay-agent-kill', { ptyId })
    await relay.call('agent.kill', { ptyId, signal: 'SIGKILL', traceId: span.id })
  }
  span.ok({ sessionId, forced: force })
} catch (err) {
  span.fail(err, { sessionId })
  throw err
}
```

### BL-AG-03 — Resume Agent Session

**Không tìm thấy call site thật cho `agent.resume` — chưa xác định file cụ thể.** Về mặt cơ chế, resume tái sử dụng đúng con đường `agent.exec`/`agent.spawn` của BL-AG-01 với `args` khác (`--resume <sessionId>`), nên tracer là span độc lập, KHÔNG resume `id` từ BL-AG-01 cũ (đây là một lượt thao tác người dùng mới, không phải tiếp nối cùng request).

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận request resume | `start` | `{ worktreeId }` | chưa xác định file cụ thể |
| SELECT session cũ | không `step()` (single-row SELECT) | gộp vào field của `start` hoặc `ok` | chưa xác định file cụ thể |
| Gọi lại `agent.exec` với resume args | `step('relay-agent-exec-resume')` | `{ devServerId, resumeSessionId }` | tái dùng cơ chế BL-AG-01 (`ProfileAwareAgentSpawner.spawn` hoặc tương đương) |
| Hoàn tất | `ok`/`fail` | `{ sessionId }` | — |

### BL-AG-04 — Switch Account / Provider

Sub-flow này **gộp BL-AG-02 (kill) + resolve provider mới + BL-AG-01 (spawn)** theo đúng mô tả flow doc. Vì gồm nhiều sub-flow con, dùng một span `agentOrch:switch` bao ngoài, và mỗi sub-flow con vẫn phát span riêng của chính nó (không nuốt vào span switch) — liên kết bằng field `parentTraceId`, giống pattern fan-out ở CR-TRACE-001 §4 BL-WT-02.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Phát hiện rate-limit, nhận lệnh switch | `start` (`agentOrch:switch`) | `{ sessionId, reason: 'rateLimited' }` | chưa xác định file cụ thể (không có rate-limit pattern matcher nào ở Orca Server hiện tại) |
| Kill agent cũ | `step` — trigger `agentOrch:stop` span con, `parentTraceId = switchSpan.id` | `{ sessionId }` | xem BL-AG-02 |
| Resolve provider mới | `step('resolve-new-provider')` | `{ providerId }` | tương tự `ProfileAwareAgentSpawner.ts:96` nhưng với account khác — chưa xác định call site chọn "account mới" cụ thể |
| Spawn agent mới | `step` — trigger `agentOrch:spawn` span con, `parentTraceId = switchSpan.id` | `{ sessionId }` | xem BL-AG-01 |
| Hoàn tất | `ok`/`fail` | `{ oldSessionId, newSessionId }` | — |

### BL-AG-05 — Monitor Trạng thái Agent Real-time

Đây là luồng **stream liên tục** (mỗi PTY output frame), không phải request/response — theo CR-TRACE-000 §5 nguyên tắc "khi nào step()", **không nên tạo 1 span mới cho mỗi `agent.output` event** (sẽ tạo hàng nghìn span/giây, noise cực lớn). Thay vào đó:

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận `agent.output` qua WS, forward tới OSC parser | KHÔNG span riêng — quá tần suất cao | — | `src/relay/agent-rpc-dispatch.ts` (phía gửi), chưa xác định file parser phía Orca Server |
| Phát hiện state-transition quan trọng (idle→running, rate-limited, completed) | `step()` trên **span đang mở của BL-AG-01/03** (nếu còn sống) hoặc field trong log thường (không tạo tracer riêng) | `{ sessionId, newStatus }` | chưa xác định file cụ thể (`AgentHookParser` không tồn tại) |

> **Khuyến nghị:** không tạo tracer `agentOrch:statusPoll` để trace từng frame; giữ tracer này (đã khai báo ở mục 3 cho đầy đủ theo BL code) nhưng chỉ dùng nếu về sau có một **polling loop rời rạc** (ví dụ mỗi N giây check status 1 lần qua `agent.status`) — không dùng cho stream `agent.output` liên tục. Đây là ví dụ cụ thể của nguyên tắc CR-TRACE-000 §5 mục "biến đổi dữ liệu in-memory/luồng tần suất cao không đáng `step()` riêng".

## 5. Lan truyền traceId qua transport của flow này

1. **Renderer/Browser → Orca Server**: với web mode, giống CR-TRACE-001 §5 mục 1 — `traceId` đi trong `params` của RPC request, không đụng `RpcRequest.id` (`src/main/runtime/rpc/core.ts:33-38`).
2. **`ProfileAwareAgentSpawner.spawn()` → `relay.call('agent.exec', ...)`**: thêm `traceId: span.id` vào params gửi xuống relay (xem code mẫu BL-AG-01 ở trên).
3. **`DevServerRelayBridge.call()` → Dev Server**: giống CR-TRACE-001, `relayCallTracer` (`relay:agentCall`) cần được sửa (theo CR-TRACE-000) để resume bằng `params.traceId` nhận được.
4. **Dev Server nhận request (`agent-rpc-dispatch.ts` case `agent.exec`/`agent.spawn`)**: đây là kênh **Agent WS JSON-RPC 2.0 thật sự** (khác với BL-WT vốn có thể đi qua SSH channel git) — theo CR-TRACE-000 §3.3, `traceId` phải nằm ở `params._trace.id`, không phải field `traceId` phẳng, để không đụng `id` JSON-RPC 2.0 chuẩn dùng match request/response. Cần cập nhật `ProfileAwareAgentSpawner.spawn()` mẫu code ở trên: gửi `_trace: { id: span.id }` thay vì `traceId: span.id` phẳng khi target là Agent WS JSON-RPC (khác với `relay.call('git.exec', ...)` ở CR-TRACE-001 vốn không nhất thiết là JSON-RPC 2.0 thuần).
5. **`agent-rpc-dispatch.ts` dispatch tới `agent-spawner.ts`/`agent-session.ts` (node-pty.spawn thật)**: in-process trên Dev Server, dùng `span.step()` của `agent:rpc` (hạ tầng có sẵn), không cần field wire riêng.

## Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` được thêm vào `tracers.ts` với tên chính xác `agentOrch:spawn|stop|resume|switch|statusPoll`
- [ ] `ProfileAwareAgentSpawner.spawn()` phát `agentOrch:spawn` span bao trọn `resolve-context` → `resolve-provider` → `relay-agent-exec`, và `ok()` chứa `sessionId`
- [ ] Trường `env`/credentials KHÔNG bao giờ xuất hiện trong field của bất kỳ span nào (chỉ `binary`, `devServerId`, `providerId`) — kiểm tra bằng review code, khớp với nguyên tắc "không log secret" đã áp dụng cho `ORCA_ACCOUNT_ID` thay vì raw credentials trong `ProfileAwareAgentSpawner.ts:100-104`
- [ ] Khi gửi qua Agent WS JSON-RPC 2.0, `traceId` nằm ở `params._trace.id`, không phải field phẳng `params.traceId` (khác quy ước dùng cho `relay.call('git.exec', ...)`)
- [ ] `agentOrch:switch` span có `parentTraceId` liên kết đúng tới `agentOrch:stop` và `agentOrch:spawn` span con khi BL-AG-04 được implement
- [ ] KHÔNG có span mới được tạo cho mỗi `agent.output` PTY stream frame (BL-AG-05) — chỉ log state-transition quan trọng
- [ ] Các sub-flow chưa có implementation thật (BL-AG-02/03/04, phần OSC parsing của BL-AG-05) được review lại field/tracer name này trước khi code thật được viết, để tránh đặt tên field lệch chuẩn

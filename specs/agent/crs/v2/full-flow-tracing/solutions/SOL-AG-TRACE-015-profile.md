# SOL-AG-TRACE-015: Profile & Project — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**TDD Ref:** TDD-AG-12 (ProfileAware Agent Spawner — AI Agent CLI Host)
**File(s):** `src/relay/agent-rpc-dispatch.ts` [MODIFY]
**Mức độ:** 🟢 đơn giản
**Thời gian ước tính:** 1h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-015 định nghĩa tracer `profile:agentSpawnRoute` cho `ProfileAwareAgentSpawner.spawn()` — nhưng class này nằm ở **`src/main/project/ProfileAwareAgentSpawner.ts`**, tức phía Orca Server (Main process), **không phải** Dev Server Agent. Đã đọc trực tiếp file này để xác nhận: toàn bộ 6 bước mà CR-TRACE-015 §4 mô tả (`getProjectContext` → compose env → `resolveProvider` → `getRelayForProject` → `relay.call('agent.exec', ...)`) chạy hết trong process Orca Server, trước khi request rời khỏi Main process. Việc thêm `profile:updateLayer`, `profile:resolve`, `profile:projectRoute`, `profile:agentSpawnRoute` là công việc của **một solution set khác** (backend), không thuộc phạm vi tài liệu này.

Có một điểm quan trọng cần làm rõ: TDD-AG-12 mô tả `ProfileAwareAgentSpawner` phiên bản agent-side (`src/relay/agent-spawner.ts`, hàm `buildAgentEnv()`) như là nơi "profile → env injection" xảy ra. Sau khi đọc `src/relay/agent-spawner.ts` thực tế, phát hiện đây là **false lead** đối với CR-TRACE-015 cụ thể:

- `buildAgentEnv()` (`agent-spawner.ts:183-243`) chỉ được gọi từ `handleAgentSpawn()` — handler của RPC method **`agent.spawn`** (interactive PTY, dùng bởi Task Graph / terminal tương tác — xem SOL-AG-TRACE-018).
- `ProfileAwareAgentSpawner.spawn()` (backend, BL-PRF-04) gọi **`relay.call('agent.exec', ...)`** (`ProfileAwareAgentSpawner.ts:115`) — một RPC method **khác**, non-interactive, xử lý bởi `case 'agent.exec'` trong `agent-rpc-dispatch.ts:502-557`. Handler này **không** gọi `buildAgentEnv()` — nó nhận `env` đã được backend build sẵn (`profileEnv` — merge `resolvedProfile.envVars` + `shell.envVars` + `extraEnv` + `ORCA_*` context, toàn bộ tại `ProfileAwareAgentSpawner.ts:75-105`) và chỉ merge đơn giản: `{ ...process.env, ...extraEnv }` (`agent-rpc-dispatch.ts:525`).

**Kết luận phạm vi:** Không có logic "build env từ resolved profile" nào cần instrument phía agent cho CR này — việc đó đã xảy ra 100% ở backend, ngoài phạm vi solution này. Điểm duy nhất còn lại phía agent là: **span quan sát chung `agent:rpc` bọc quanh `case 'agent.exec'` hiện đang mù (không trích field hữu ích) đối với chính request mà `ProfileAwareAgentSpawner.spawn()` gửi xuống.** Solution này thu hẹp lại đúng phạm vi đó, giữ tối giản thay vì thêm tracer/logic không cần thiết.

## 2. Gap hiện tại

`agent-rpc-dispatch.ts` đã có tracer `agent:rpc` (`rpcTracer`, dòng 21) bọc **mọi** RPC dispatch (dòng 126-148) — đây là tracer pre-existing, KHÔNG nằm trong danh sách "11 tracer đã tồn tại" mà CR-TRACE-000 GAP-3 liệt kê (drift chưa được CR-TRACE-000 phát hiện, xem thêm ghi chú ở SOL-AG-TRACE-016/017/018). Nó gọi `extractTraceFields(method, params)` để lấy field ngữ cảnh theo tiền tố method.

Với `method.startsWith('agent.')` (`agent-rpc-dispatch.ts:110-115`):

```typescript
if (method.startsWith('agent.')) {
  return {
    session: str(p['sessionId']),
    cmd:     truncCmd(p['cmd'] ?? p['command']),
  }
}
```

Nhưng `agent.exec` — method mà `ProfileAwareAgentSpawner.spawn()` gọi — có params shape hoàn toàn khác: `{ binary, args, cwd, env, timeoutMs, stdin? }` (xác nhận tại `agent-rpc-dispatch.ts:505-515`). Không có `sessionId` hay `cmd`/`command` nào trong request này → mọi span `agent:rpc` phát sinh từ BL-PRF-04 hiện log ra `session=undefined cmd=undefined`, đúng loại "1 exception duy nhất không có breakdown" mà CR-TRACE-015 §1 phàn nàn — chỉ khác là ở đây vấn đề nằm phía agent chứ không phải backend.

Thêm nữa, khi request thành công, `dispatch()` chỉ gọi `span.ok({ method: rpc.method })` (dòng 136) — bỏ qua `result.exitCode`/`result.timedOut` mà chính `case 'agent.exec'` đã tính toán xong (dòng 521-549) nhưng không đưa vào trace.

## 3. Full Implementation

### 3.1. Mở rộng `extractTraceFields()` cho bucket `agent.exec`

```typescript
// src/relay/agent-rpc-dispatch.ts

function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  const num = (v: unknown) => (typeof v === 'number' ? v : undefined)
  // ...existing truncPath/truncCmd unchanged...

  // ...existing fs./git./github.-gitlab./ai.provider. buckets unchanged...

  if (method === 'agent.exec') {
    // CR-TRACE-015 BL-PRF-04: agent.exec (non-interactive, dùng bởi
    // ProfileAwareAgentSpawner.spawn() ở backend) có params shape khác hẳn
    // agent.spawn (interactive PTY) — bucket 'agent.' cũ (session/cmd) không
    // khớp field nào của agent.exec, khiến span luôn hiện session=undefined
    // cmd=undefined. Bucket riêng này khớp đúng { binary, args, cwd, env, timeoutMs }.
    return {
      binary:         str(p['binary']),
      argsCount:      Array.isArray(p['args']) ? (p['args'] as unknown[]).length : undefined,
      hasEnvOverride: p['env'] !== undefined && p['env'] !== null,
      timeoutMs:      num(p['timeoutMs']),
    }
  }

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId']),
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}
```

**Lưu ý khi triển khai cùng SOL-AG-TRACE-017 / SOL-AG-TRACE-018:** cả hai solution đó cũng chỉnh sửa đúng bucket `method === 'agent.exec'` này (thêm `stepId`/`parentTraceId` — SOL-017; ghi chú prerequisite `taskId` — SOL-018). Ba thay đổi này **không xung đột** (khác field, cùng object literal) — nếu áp dụng nhiều solution cùng lúc, gộp field vào chung 1 object trả về thay vì 3 lần `if (method === 'agent.exec')` riêng lẻ.

### 3.2. Đưa `exitCode`/`timedOut` vào `span.ok()`

```typescript
// src/relay/agent-rpc-dispatch.ts — trong dispatch()

async dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void> {
  const ctxFields = extractTraceFields(rpc.method, rpc.params ?? {})
  const span = rpcTracer.start({ method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields })
  let response: JsonRpcResponse
  try {
    response = await route(rpc, tools, config, log, ws, state)
    if ('error' in response) {
      span.fail(response.error.message, { method: rpc.method, code: response.error.code })
    } else {
      // CR-TRACE-015 BL-PRF-04: surface result-level fields the handler already
      // computed (exitCode/timedOut for agent.exec) instead of only { method }.
      span.ok({ method: rpc.method, ...extractResultFields(rpc.method, response.result) })
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`RPC dispatch unhandled error method=${rpc.method}: ${msg}`)
    span.fail(msg, { method: rpc.method, phase: 'dispatch' })
    response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`)
  }

  if (ws.readyState === 1 /* WebSocket.OPEN */) {
    ws.send(encodeDataFrame(state, JSON.stringify(response)))
  }
}

// ─── Result field extraction (generic — extend as more methods need it) ───────

function extractResultFields(method: string, result: unknown): TraceFields {
  if (method === 'agent.exec' && result && typeof result === 'object') {
    const r = result as Record<string, unknown>
    return {
      exitCode: typeof r['exitCode'] === 'number'  ? r['exitCode']  : undefined,
      timedOut: typeof r['timedOut'] === 'boolean' ? r['timedOut'] : undefined,
    }
  }
  return {}
}
```

`JsonRpcSuccess.result` đã là `unknown` — không cần thay đổi type. `extractResultFields()` được viết generic theo `method` (không chỉ `agent.exec`) để các solution khác (nếu có method khác cần bung field từ result) có thể mở rộng cùng một chỗ thay vì rải rác nhiều `if` trong `dispatch()`.

## 4. Test Plan (Vitest)

File: `src/relay/__tests__/agent-rpc-dispatch.test.ts` (mở rộng file test hiện có — dùng lại `MockWs`/`lastResponseJson` đã có sẵn, thêm `registerTraceSink` từ `../../shared/trace` để bắt `TraceEvent[]` thay vì phải bật `ORCA_TRACE=1`, vì `emit()` luôn gọi sink bất kể cờ trace — xem `src/shared/trace/index.ts:137-140`).

```typescript
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

describe('agent.exec — extractTraceFields (CR-TRACE-015)', () => {
  it('start event includes binary/argsCount/hasEnvOverride/timeoutMs, not session/cmd', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))
    const dispatcher = createRpcDispatcher([], MOCK_CONFIG, MOCK_LOG)
    const ws = new MockWs()
    await dispatcher.dispatch(ws as unknown as WebSocket, createWireState(), {
      jsonrpc: '2.0', id: 1, method: 'agent.exec',
      params: { binary: 'echo', args: ['hi'], cwd: '/tmp', env: { FOO: 'bar' }, timeoutMs: 5000 },
    })
    unregister()
    const start = events.find(e => e.flow === 'agent:rpc' && e.level === 'start')!
    expect(start.fields.binary).toBe('echo')
    expect(start.fields.argsCount).toBe(1)
    expect(start.fields.hasEnvOverride).toBe(true)
    expect(start.fields.timeoutMs).toBe(5000)
    expect(start.fields.session).toBeUndefined()
  })

  it('ok event includes exitCode and timedOut from the agent.exec result', async () => { /* spawn echo, assert ok.fields.exitCode === 0, timedOut === false */ })

  it('agent.spawn (interactive) still uses the legacy session/cmd bucket, unaffected', async () => { /* method: agent.spawn — assert fields.session/cmd path, no binary/argsCount */ })

  it('agent.exec without env param → hasEnvOverride is false, not undefined-vs-false ambiguity', async () => { /* params without env key */ })
})
```

## 5. Acceptance Criteria

- [ ] Xác nhận và ghi chép rõ: KHÔNG có tracer `profile:*` nào được thêm phía agent — toàn bộ BL-PRF-01→03 và phần "compose env" của BL-PRF-04 là backend-only, verified qua đọc trực tiếp `src/main/project/ProfileAwareAgentSpawner.ts`
- [ ] `extractTraceFields()` có bucket riêng cho `method === 'agent.exec'`, tách khỏi bucket `agent.*` cũ (vốn chỉ đúng cho `agent.spawn`)
- [ ] Span `agent:rpc` cho `agent.exec` hiển thị `binary`, `argsCount`, `hasEnvOverride`, `timeoutMs` — không còn `session=undefined cmd=undefined`
- [ ] `span.ok()` của `agent.exec` bao gồm `exitCode`/`timedOut` lấy từ `response.result`, không chỉ `{ method }`
- [ ] `agent.spawn` (interactive PTY, đã có tracer `agent:spawn` riêng — xem `agent-spawner.ts:29`) không bị ảnh hưởng bởi thay đổi bucket `agent.exec`
- [ ] Không field nào trong bucket mới chứa giá trị của `env` (chỉ boolean `hasEnvOverride`, không log nội dung env vars — có thể chứa secret nếu backend lỡ nhét credential vào `extraEnv`)
- [ ] `extractResultFields()` được viết generic theo `method`, không hardcode riêng cho luồng Profile

# TASK-AG-015.1: Add dedicated agent.exec bucket to extractTraceFields + surface exitCode/timedOut in span.ok()

**Phase:** 3
**SOL Ref:** [SOL-AG-TRACE-015](../solutions/SOL-AG-TRACE-015-profile.md)
**CR Ref:** [CR-TRACE-015](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-015-profile.md)
**Precondition:** Phase 0 + [TASK-AG-002.2](./TASK-AG-002.2-agent-rpc-dispatch-resume-and-exec-span.md) (đảm bảo `extractTraceFields()`/`dispatch()` đã có cấu trúc base từ CR-002)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — Implemented as specified. Drift note: `agent.exec` already has its own inline handler in `route()` (from TASK-AG-002.2/002.3) using a separate `Tracers.agentOrchSpawn` span (`agentOrch:spawn` flow) for the subprocess-level detail — this task's bucket operates one layer up, on the outer `rpcTracer`/`agent:rpc` span in `extractTraceFields()`/`dispatch()`, which is complementary (not duplicate). `response.result` for `agent.exec` is exactly `{ stdout, stderr, exitCode, timedOut }`, matching `extractResultFields()` assumptions with no changes needed. `pnpm run typecheck:node` clean for this file; `gitnexus_impact` on both symbols returned LOW risk.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

`agent-rpc-dispatch.ts` là file high-fan-in bị nhiều task cùng mở rộng (002.2 đã tạo base, 017.1/018.2 sẽ mở rộng THÊM sau task này vào CÙNG bucket) — target đúng symbol, KHÔNG explore/impact cả file:

```bash
codegraph explore "extractTraceFields"
codegraph explore "createRpcDispatcher"
```

Cả 2 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis:

```
gitnexus_impact({ target: "extractTraceFields", direction: "upstream" })
gitnexus_impact({ target: "createRpcDispatcher", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`ProfileAwareAgentSpawner.spawn()` (backend, `src/main/project/ProfileAwareAgentSpawner.ts`, NGOÀI phạm vi — không sửa ở đây) gọi `relay.call('agent.exec', ...)`. Params shape của `agent.exec` là `{ binary, args, cwd, env, timeoutMs, stdin? }` — hoàn toàn khác `agent.spawn` (`{ sessionId, cmd/command }`). Nhánh `method.startsWith('agent.')` cũ trong `extractTraceFields()` không khớp field nào của `agent.exec`, khiến span luôn hiện `session=undefined cmd=undefined`.

**Xác nhận phạm vi:** KHÔNG có tracer `profile:*` nào được thêm phía agent trong task này — toàn bộ BL-PRF-01→03 và phần "compose env" của BL-PRF-04 là backend-only (đã verify qua đọc trực tiếp `ProfileAwareAgentSpawner.ts`). Phạm vi ở đây CHỈ là fix bucket field extraction cho `agent:rpc` (tracer hạ tầng chung, đã tồn tại).

## File: `src/relay/agent-rpc-dispatch.ts` [MODIFY]

### `extractTraceFields()` — bucket mới cho `agent.exec`

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

**Lưu ý điều phối với TASK-AG-017.1 / TASK-AG-018.2:** cả hai task đó cũng chỉnh sửa đúng bucket `method === 'agent.exec'` này (thêm `stepId`/`parentTraceId` — 017.1; thêm `taskId` — 018.2). Thực thi task này TRƯỚC — 017.1 và 018.2 sẽ MỞ RỘNG object literal trên (thêm field vào cùng object), không tạo `if` block riêng.

### `dispatch()` — đưa `exitCode`/`timedOut` vào `span.ok()`

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

`JsonRpcSuccess.result` đã là `unknown` — không cần thay đổi type. `extractResultFields()` viết generic theo `method` để các method khác có thể mở rộng cùng một chỗ, không rải rác nhiều `if` trong `dispatch()`.

**Đây là NGUỒN mà [TASK-AG-002.2](./TASK-AG-002.2-agent-rpc-dispatch-resume-and-exec-span.md) đã tạo `dispatch()` với `span.ok({ method: rpc.method })` đơn giản — task này SỬA lại dòng `span.ok(...)` đó để thêm `...extractResultFields(...)`, không viết lại toàn bộ `dispatch()` từ đầu.**

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-rpc-dispatch" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] Xác nhận và ghi chép rõ: KHÔNG có tracer `profile:*` nào được thêm phía agent trong task này
- [ ] `extractTraceFields()` có bucket riêng cho `method === 'agent.exec'`, tách khỏi bucket `agent.*` cũ (vốn chỉ đúng cho `agent.spawn`)
- [ ] Span `agent:rpc` cho `agent.exec` hiển thị `binary`, `argsCount`, `hasEnvOverride`, `timeoutMs` — không còn `session=undefined cmd=undefined`
- [ ] `span.ok()` của `agent.exec` bao gồm `exitCode`/`timedOut` lấy từ `response.result`, không chỉ `{ method }`
- [ ] `agent.spawn` (interactive PTY, tracer `agent:spawn` riêng) không bị ảnh hưởng bởi thay đổi bucket `agent.exec`
- [ ] Không field nào trong bucket mới chứa giá trị của `env` (chỉ boolean `hasEnvOverride`)
- [ ] `extractResultFields()` viết generic theo `method`, không hardcode riêng cho luồng Profile
- [ ] `pnpm run typecheck:node` pass

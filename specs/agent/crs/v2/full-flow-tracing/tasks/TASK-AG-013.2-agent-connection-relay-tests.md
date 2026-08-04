# TASK-AG-013.2: Extend agent-connection-relay.test.ts for tracing behavior

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-013](../solutions/SOL-AG-TRACE-013-agent-ws.md)
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Precondition:** Phase 0 + [TASK-AG-013.1](./TASK-AG-013.1-agent-connection-relay-tracer.md)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — implemented as specified. Drift: `agent-connection-direct.test.ts` referenced in spec does not exist in the repo (only `agent-session.test.ts` does) — ran vitest against the files that actually exist; both pass (32/32). Pre-existing TS2345 errors in this test file (unrelated `makeReq()` header-union typing, present before this task on the original 11 cases) now also appear on the 3 new call sites since they reuse the same helper — vitest execution is unaffected; not introduced by the tracing change.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "authenticate"
```

`authenticate` (trong `agent-connection-relay.ts`, vừa được TASK-AG-013.1 thêm tham số `span?`) là symbol MODIFY (đã tồn tại) — chạy thêm

```
gitnexus_impact({ target: "authenticate", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## File: `src/relay/__tests__/agent-connection-relay.test.ts` [MODIFY]

File test hiện tại **replicate `authenticate()` inline** (không import từ module thật) để test mà không cần khởi động `WebSocketServer` thật. Cập nhật bản replicate để khớp signature mới (`span?` param) và thêm test case cho hành vi trace.

```typescript
// src/relay/__tests__/agent-connection-relay.test.ts
// (bổ sung sau describe('authenticate() — URL query string') hiện có)

import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent, TraceSpan } from '../../shared/trace'

// Cập nhật bản replicate để nhận thêm `span?: TraceSpan`
function authenticate(
  ws: { close: (code: number, reason: string) => void },
  req: { url?: string; headers: Record<string, string>; socket: { remoteAddress?: string } },
  expectedToken: string,
  log: { warn: (msg: string) => void },
  span?: TraceSpan
): boolean {
  const rawUrl = req.url ?? ''
  let queryToken = ''
  try { queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? '' } catch {}
  const authHeader  = (req.headers['authorization'] ?? '')
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()
  const incoming    = queryToken || bearerToken
  const source: 'query' | 'header' | 'none' = queryToken ? 'query' : bearerToken ? 'header' : 'none'

  if (incoming !== expectedToken) {
    span?.fail('unauthorized', { source })
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }
  span?.step('tokenAccepted', { source })
  return true
}

describe('authenticate() — agent:connectionRelay tracing', () => {
  it('span.step("tokenAccepted", {source:"query"}) khi token hợp lệ qua query string', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(true)
    const step = events.find(e => e.level === 'step' && e.label === 'tokenAccepted')
    expect(step?.fields.source).toBe('query')
  })

  it('span.fail("unauthorized", {source:"none"}) khi thiếu token, KHÔNG có field nào chứa token thật', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(''), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(false)
    const fail = events.find(e => e.level === 'fail')
    expect(fail?.fields.source).toBe('none')
    expect(JSON.stringify(events)).not.toContain(TOKEN)
  })

  it('authenticate() vẫn hoạt động khi span không được truyền (backward-compat)', () => {
    expect(authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog)).toBe(true)
  })
})
```

`relayConnTracerForTest` là một `createTracer('agent:connectionRelay')` cục bộ trong file test (dùng cùng tên flow để so khớp `e.flow`), theo pattern replicate đã có trong file này.

`src/relay/__tests__/agent-connection-direct.test.ts` / `agent-session.test.ts` — **không cần sửa**, chỉ xác nhận qua CI hiện có rằng các test case `handshake-ok`/`close code=1000`/`close trước handshake` vẫn pass sau khi `agent-connection-relay.ts` thay đổi (không có phụ thuộc chéo giữa 2 file).

## Verification

```bash
pnpm vitest run src/relay/__tests__/agent-connection-relay.test.ts src/relay/__tests__/agent-connection-direct.test.ts src/relay/__tests__/agent-session.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent-connection-relay.test.ts` có thêm ≥ 3 test case theo trên, bao gồm 1 test khẳng định không có token thật lọt vào `TraceEvent`
- [ ] Bản replicate `authenticate()` trong file test cập nhật đúng signature mới (`span?` param cuối cùng)
- [ ] `agent-connection-direct.test.ts`/`agent-session.test.ts` KHÔNG sửa, vẫn pass nguyên trạng
- [ ] `pnpm vitest run src/relay/__tests__/agent-connection-relay.test.ts` pass

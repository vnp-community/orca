# TASK-FE-013.1: Badge read-only hiển thị trạng thái Agent WS trên `DevServerCard`

**Phase:** 2
**SOL Ref:** [SOL-FE-TRACE-013 (toàn bộ, minimal)](../solutions/SOL-FE-TRACE-013-agent-ws.md)
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001)
**Status:** ✅ Done (2026-08-04) — implemented as specified; fixed a `*/` JSDoc-closing bug in the task's own code sample (docstring literally contained `agentWs:*/agentToken:*`, which esbuild parsed as comment-end — reworded to avoid the sequence). No other drift: `TraceEvent`/`TraceFields`/`traceEvents: TraceEvent[]` (non-nullable, so `s.traceEvents` used directly without `?? []`) matched the doc's assumptions exactly; `DevServerCard` risk was LOW (1 direct caller: `DevServerList`) per `gitnexus_impact`. 8/8 new tests pass (5 lib + 3 component). Per this index's documented note, `pnpm tsc --noEmit` is a no-op (root tsconfig has `"files": []`) — ran the real check `npx tsc --noEmit -p config/tsconfig.tc.web.json` instead: zero errors reference `DevServerCard.tsx` or `agent-ws-trace-status.ts`; all reported errors are in unrelated files (pre-existing baseline or concurrent sibling-agent in-flight edits, e.g. `store/slices/repos.ts`). `detect_changes` shows heavy unrelated churn (~170 symbols/87 files) from concurrent sibling-agent work sharing this working directory; my own footprint is limited to `DevServerCard`/`handleConnect` (same-file diff proximity) — no new tracer added to `tracers.ts`.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

File `src/renderer/src/lib/agent-ws-trace-status.ts` là file MỚI (NEW) — không có symbol hiện có để chạy `gitnexus_impact`. Thay vào đó, chạy

```bash
codegraph explore "TraceEvent"
```

để hiểu type `TraceEvent` mà hàm `latestAgentWsStatusForDevServer()` mới sẽ đọc, trước khi viết code.

Đối với `DevServerCard.tsx` (đã mount, MODIFY case), chạy thêm:

```bash
codegraph explore "DevServerCard"
```

```
gitnexus_impact({ target: "DevServerCard", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

**Đây là task duy nhất cho CR-TRACE-013 — giữ tối giản theo đúng đề xuất của solution, KHÔNG mở rộng.** Đã xác nhận qua grep trực tiếp: **không có bất kỳ hành động browser nào** khởi tạo BL-AWS-01→03 (`POST /api/agent-token`, handshake, revoke token) — đây là traffic Orca-backend ↔ external Agent process, không phải RPC do browser gọi. `AgentTokenPanel.tsx` build sẵn (hiển thị command `ORCA_URL=... AGENT_TOKEN=... node agent.js`) nhưng **chưa mount** vào `AddDevServerDialog.tsx`, và ngay cả khi mount, bản thân nó không gọi RPC nào — chỉ render props.

Task này **KHÔNG thêm tracer/span mới nào** — chỉ thêm một view read-only **tiêu thụ** trace event đã có sẵn qua SSE bridge (`/api/trace-stream`), gắn vào `DevServerCard.tsx` (đã mount thật). Không bịa instrumentation cho một RPC call chưa tồn tại.

## File: `src/renderer/src/lib/agent-ws-trace-status.ts` [NEW]

```typescript
// src/renderer/src/lib/agent-ws-trace-status.ts
// Đọc lại (KHÔNG tạo) trace event agentWs:*/agentToken:* đã có trong store trace
// (nạp qua SSE /api/trace-stream) để hiển thị read-only trên DevServerCard.
// Không gọi RPC, không tạo span.
import type { TraceEvent } from '../../../shared/trace'

const AGENT_WS_FLOW_PREFIXES = ['agentWs:', 'agentToken:', 'agent:tokenManager'] as const

export type AgentWsCardStatus = {
  flow: string
  level: TraceEvent['level']
  ts: number
  reason?: string
}

/**
 * Trả về event agentWs:*/agentToken:* gần nhất khớp devServerId, nếu có.
 * traceEvents nên là mảng đã giới hạn kích thước (ring buffer) — không quét
 * toàn bộ lịch sử mỗi render.
 */
export function latestAgentWsStatusForDevServer(
  traceEvents: readonly TraceEvent[],
  devServerId: string
): AgentWsCardStatus | null {
  for (let i = traceEvents.length - 1; i >= 0; i -= 1) {
    const event = traceEvents[i]
    if (!AGENT_WS_FLOW_PREFIXES.some((prefix) => event.flow.startsWith(prefix))) continue
    if (event.fields.devServerId !== devServerId) continue
    return {
      flow: event.flow,
      level: event.level,
      ts: event.ts,
      reason: typeof event.fields.reason === 'string' ? event.fields.reason : undefined,
    }
  }
  return null
}
```

## File: `src/renderer/src/components/dev-server/DevServerCard.tsx` [MODIFY, additive]

```typescript
import { useAppStore } from '@/store'
import { latestAgentWsStatusForDevServer } from '@/lib/agent-ws-trace-status'

// Bên trong DevServerCard(), sau khi đã có `server`:
const agentWsStatus = useAppStore((s) =>
  latestAgentWsStatusForDevServer(s.traceEvents ?? [], server.id)
)

// ...existing JSX, thêm 1 badge nhỏ cạnh status badge hiện có:
{agentWsStatus && (
  <span
    className={agentWsStatus.level === 'fail' ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}
    title={`${agentWsStatus.flow} · ${new Date(agentWsStatus.ts).toLocaleTimeString()}`}
  >
    {agentWsStatus.level === 'ok' ? 'Agent WS: handshake ok' : null}
    {agentWsStatus.level === 'fail' ? `Agent WS: ${agentWsStatus.reason ?? 'handshake failed'}` : null}
  </span>
)}
```

> `s.traceEvents` giả định store `trace.ts` giữ ring-buffer `TraceEvent` gần nhất (dùng cho TracePanel) — nếu cấu trúc thật khác (Map theo `id`, giới hạn khác), điều chỉnh `latestAgentWsStatusForDevServer()` cho khớp; không thay đổi cấu trúc store hiện có chỉ để phục vụ task này.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/lib/__tests__/agent-ws-trace-status.test.ts
pnpm test --run src/renderer/src/components/dev-server/__tests__/DevServerCard.test.tsx
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [x] KHÔNG thêm tracer mới nào vào `src/shared/trace/tracers.ts` cho task này — xác nhận qua code review rằng không có `Tracers.xxx.start()` mới nào
- [x] KHÔNG thêm bất kỳ lời gọi RPC mới nào trong renderer cho agent-ws (không có `POST /api/agent-token` call site mới)
- [x] `latestAgentWsStatusForDevServer()` là hàm thuần (pure function), không side-effect, không tự subscribe SSE (SSE đã có sẵn qua `initBrowserTrace`)
- [x] Badge trên `DevServerCard` chỉ hiển thị khi có event khớp — không hiển thị placeholder gây hiểu lầm là "đang theo dõi" khi không có dữ liệu
- [x] `AgentTokenPanel.tsx` KHÔNG bị sửa trong task này (component hiển thị thuần, không có RPC call site để instrument)
- [x] Ghi rõ trong PR/commit: nếu một companion CR trong tương lai wire `AgentTokenPanel` vào `AddDevServerDialog.tsx` kèm nút "Generate token" gọi `POST /api/agent-token`, đó **sẽ** là điểm hợp lệ để thêm `Tracers.xxx.start()` thật — nhưng KHÔNG thêm trước khi trigger đó tồn tại
- [x] Test suite đạt ≥ 8 test case mới: 5 cho `agent-ws-trace-status.test.ts` (null khi không khớp prefix, null khi devServerId khác, event gần nhất, nhận diện cả 3 prefix, field `reason` đúng khi `level === 'fail'`), 3 cho `DevServerCard.test.tsx`

# SOL-FE-TRACE-013: Agent WebSocket — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**TDD Ref:** TDD-FE-10 (Fleet Management — `10-fleet-management.md`)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts` (SSE client tới `/api/trace-stream`), TracePanel.

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 Kết luận ngắn gọn: KHÔNG có hành động người dùng nào khởi tạo BL-AWS-01→03

CR-TRACE-013 (phía backend) đã ghi rõ luồng Agent WebSocket là traffic **Orca-backend ↔ external Agent process** — không phải RPC do browser gọi. Đã xác minh lại bằng grep trực tiếp trên `src/renderer/src`, không dựa vào giả định của CR gốc:

| Hành động backend cần trigger | Có UI browser nào gọi không? | Bằng chứng |
|---|---|---|
| `POST /api/agent-token` (BL-AWS-03, sinh token cho direct-websocket mode) | **KHÔNG** | Grep `agent-token`, `/api/agent-token` trên toàn bộ `src/renderer/src` (loại trừ test) — 0 kết quả ngoài comment trong `AgentTokenPanel.tsx` |
| `connectRelayWebSocket()` / handshake initiator (BL-AWS-01) | **KHÔNG trực tiếp** | Đây là hệ quả nội bộ của `devServer.connect` RPC (đã trace ở domain khác — Dev Server/Onboarding, ngoài phạm vi CR-TRACE-013) — user bấm "Connect" trên Dev Server, không phải "connect agent WS" tách biệt |
| Agent tự kết nối vào Orca (BL-AWS-02, direct-websocket) | **KHÔNG THỂ có UI** | Theo định nghĩa, đây là 1 process bên ngoài (`agent.js` chạy trên Dev Server) tự mở kết nối — không có trình duyệt nào tham gia vào round-trip này |
| Revoke/CRUD token qua Admin SPA | **KHÔNG TỒN TẠI** | Không tìm thấy UI admin token CRUD nào trong `src/renderer/src/components/admin/*` hay nơi khác — khớp phát hiện của CR-TRACE-013 rằng tính năng này chưa được build ở backend lẫn frontend |

### 1.2 Component build sẵn nhưng KHÔNG mount: `AgentTokenPanel.tsx`

`src/renderer/src/components/dev-server/AgentTokenPanel.tsx` — component thuần hiển thị (copyable command `ORCA_URL=... AGENT_TOKEN=... node agent.js`), theo comment đầu file "Shown in AddDevServerDialog after the backend generates an agentToken". Đã xác minh: `AddDevServerDialog.tsx` **không import** `AgentTokenPanel` (grep `AgentTokenPanel|agent-token|generateAgentToken` trong file này — 0 kết quả). Component tồn tại vì đã được xây trước cho luồng "direct-websocket" (dropdown `AddDevServerDialog.tsx:98` có option `direct-websocket`), nhưng dây nối token-generation → hiển thị panel này chưa được hoàn thiện.

Ngay cả khi `AgentTokenPanel` được mount trong tương lai, bản thân nó **không gọi RPC nào** — nó chỉ render props (`agentToken`, `orcaUrl`, `waiting`) do component cha truyền vào sau khi component cha tự gọi `POST /api/agent-token`. Việc thêm span vào đây bây giờ nghĩa là tạo span cho một RPC call chưa tồn tại — đúng loại việc mà bài toán yêu cầu tránh ("không bịa điểm instrument giả").

### 1.3 Đề xuất duy nhất: read-only monitoring view trong Dev Server Settings pane (đã mount)

Thay vì tạo instrumentation ảo, đề xuất một view **chỉ đọc** (không tạo span, chỉ tiêu thụ span đã có sẵn từ backend) gắn vào nơi đã mount thật: `src/renderer/src/components/settings/DevServerPane.tsx` → `DevServerList.tsx` → `DevServerCard.tsx` (đã hiển thị `connectionType` và `status` mỗi Dev Server, xác nhận tại `DevServerCard.tsx:64-67,92-166`).

Cơ chế: F40 đã có sẵn SSE bridge `/api/trace-stream` (`src/shared/trace/browser.ts:29-58`, `startSseClient()`) đẩy MỌI trace event (kể cả event phát sinh phía backend, bao gồm `agentWs:handshake`/`agentWs:tokenVerify`/`agentWs:lifecycle` mà CR-TRACE-013 backend companion sẽ thêm) vào cùng 1 dispatch callback — hiện tại callback này chỉ đổ vào Zustand store `trace.ts` cho TracePanel chung. Đề xuất: thêm 1 selector nhỏ đọc từ store trace đó, lọc theo `flow` bắt đầu bằng `agentWs:`/`agentToken:` và field `devServerId` khớp, hiển thị badge nhỏ trạng thái handshake gần nhất trên `DevServerCard`.

Đây **không phải** một điểm "user action → RPC → span.start()" như 2 CR còn lại — nó là một **consumer** của trace event đã tồn tại, đúng với đề xuất trong đề bài ("proposing only a read-only monitoring view... rather than fabricating a fake instrumentation point").

---

## 2. Full Implementation

### 2.1 KHÔNG thêm tracer mới vào `tracers.ts`

Không có hành động renderer nào cần `Tracers.xxx.start()` — `agentWs:handshake`/`agentWs:tokenVerify` (do CR-TRACE-013 backend định nghĩa) chỉ được **start ở phía backend** (`dev-server-relay-bridge.ts`, `agent-ws-server.ts`). Renderer chỉ đọc lại qua SSE, không tạo span.

### 2.2 Selector đọc trace event theo `devServerId` — file mới

```typescript
// src/renderer/src/lib/agent-ws-trace-status.ts
// Đọc lại (KHÔNG tạo) trace event agentWs:*/agentToken:* đã có trong store trace
// (nạp qua SSE /api/trace-stream — xem src/shared/trace/browser.ts) để hiển thị
// read-only trên DevServerCard. Không gọi RPC, không tạo span.
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
 * traceEvents nên là mảng đã giới hạn kích thước (vd. ring buffer 500 event
 * gần nhất trong store trace hiện có) — không quét toàn bộ lịch sử mỗi render.
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

### 2.3 `DevServerCard.tsx` — thêm badge read-only (additive)

```typescript
// src/renderer/src/components/dev-server/DevServerCard.tsx — EXTEND (additive)
import { useAppStore } from '@/store'
import { latestAgentWsStatusForDevServer } from '@/lib/agent-ws-trace-status'

// ...existing component body...
// Bên trong DevServerCard(), sau khi đã có `server`:
const agentWsStatus = useAppStore((s) =>
  latestAgentWsStatusForDevServer(s.traceEvents ?? [], server.id)
)

// ...existing JSX, thêm 1 badge nhỏ cạnh status badge hiện có (dòng ~92-102):
{agentWsStatus && (
  <span
    className={
      agentWsStatus.level === 'fail'
        ? 'text-xs text-destructive'
        : 'text-xs text-muted-foreground'
    }
    title={`${agentWsStatus.flow} · ${new Date(agentWsStatus.ts).toLocaleTimeString()}`}
  >
    {agentWsStatus.level === 'ok' ? 'Agent WS: handshake ok' : null}
    {agentWsStatus.level === 'fail' ? `Agent WS: ${agentWsStatus.reason ?? 'handshake failed'}` : null}
  </span>
)}
```

> `s.traceEvents` giả định store `trace.ts` (`src/renderer/src/store/slices/trace.ts`) đã giữ 1 mảng ring-buffer các `TraceEvent` gần nhất (dùng cho TracePanel) — nếu implementation thật của slice này lưu theo cấu trúc khác (Map theo `id`, hoặc giới hạn khác), điều chỉnh `latestAgentWsStatusForDevServer()` cho khớp; không thay đổi cấu trúc store hiện có chỉ để phục vụ CR này (additive-only).

---

## 3. Test Plan (Vitest)

```
src/renderer/src/lib/__tests__/agent-ws-trace-status.test.ts   (mới)
├── trả về null khi không có event nào khớp prefix agentWs:/agentToken:
├── trả về null khi có event khớp prefix nhưng devServerId khác
├── trả về event gần nhất (không phải đầu tiên) khi có nhiều event khớp cùng devServerId
├── nhận diện cả 3 prefix: agentWs:, agentToken:, agent:tokenManager
└── field reason được đọc đúng khi level === 'fail'

src/renderer/src/components/dev-server/__tests__/DevServerCard.test.tsx   (file đã tồn tại — thêm test case)
├── không hiển thị badge Agent WS khi store.traceEvents rỗng
├── hiển thị "Agent WS: handshake ok" khi có event level='ok' khớp devServerId
└── hiển thị reason khi event level='fail'
```

**Target:** ≥ 8 test case mới.

---

## 4. Acceptance Criteria

- [ ] KHÔNG thêm tracer mới nào vào `src/shared/trace/tracers.ts` cho CR này — xác nhận qua code review rằng không có `Tracers.xxx.start()` mới nào trong `src/renderer/src`
- [ ] KHÔNG thêm bất kỳ lời gọi RPC mới nào trong renderer cho agent-ws (không có `POST /api/agent-token` call site mới) — giữ nguyên phát hiện ở mục 1.1 rằng đây không phải luồng browser-initiated
- [ ] `latestAgentWsStatusForDevServer()` là hàm thuần (pure function), không side-effect, không tự subscribe SSE (SSE đã có sẵn qua `initBrowserTrace`, không tạo kết nối thứ hai)
- [ ] Badge trên `DevServerCard` chỉ hiển thị khi có event khớp — không hiển thị placeholder gây hiểu lầm là "đang theo dõi" khi thực chất không có dữ liệu
- [ ] `AgentTokenPanel.tsx` KHÔNG bị sửa trong CR này (component hiển thị thuần, không có RPC call site để instrument — xem mục 1.2)
- [ ] Solution note rõ trong review: nếu một companion CR trong tương lai wire `AgentTokenPanel` vào `AddDevServerDialog.tsx` kèm 1 nút "Generate token" gọi `POST /api/agent-token`, đó **sẽ** là điểm hợp lệ để thêm `Tracers.xxx.start()` thật — nhưng KHÔNG được thêm trước khi trigger đó tồn tại

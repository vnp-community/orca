# F40 — Full-Flow Tracing (Observability)

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F40 |
| **Tên** | Full-Flow Tracing (Observability) |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §4.2 Observability |
| **Tham chiếu URD** | UR-040 |
| **Tham chiếu SRS** | FR-9.1, FR-9.2, FR-9.3 |
| **ADR References** | — |
| **HLD References** | C3.1, C3.8 |

---

## Mô tả

Orca cung cấp hệ thống **structured tracing isomorphic** (chạy đồng nhất trên Node.js và browser) để theo dõi toàn bộ luồng xử lý từ Browser → RPC → IPC → Relay → Agent.

Mỗi "span" trace xuyên suốt toàn bộ stack: từ thao tác người dùng ở frontend, qua lớp RPC, qua IPC main process, xuống relay và agent — tất cả được gắn cùng một `id` duy nhất, với timestamp và elapsed time ở mỗi bước.

---

## Vấn đề cần giải quyết

Khi một thao tác fail hoặc chậm (ví dụ: browse directory mất 5 giây), không có cách nào biết lỗi xảy ra ở tầng nào: RPC timeout? IPC deadlock? Relay mất kết nối? Agent crash?
Hệ thống tracing giải quyết điều này bằng cách ghi lại **mọi bước trung gian** kèm thời gian thực thi.

---

## Kiến trúc tổng quan

```
┌─────────────────────────────────────────────────────────────────┐
│                        BROWSER (Frontend)                       │
│   ┌──────────────┐   ┌─────────────────┐   ┌────────────────┐  │
│   │  User Action │──▶│  Tracer.start() │──▶│  Zustand Store │  │
│   └──────────────┘   └────────┬────────┘   │  (TracePanel)  │  │
│                               │            └────────────────┘  │
│                    span.step('rpc') ◀── also receives SSE push  │
└───────────────────────────────┼─────────────────────────────────┘
                                │ RPC call (WebSocket / HTTP)
┌───────────────────────────────▼─────────────────────────────────┐
│                     MAIN PROCESS (Node.js)                      │
│   ┌──────────────────┐   ┌──────────────────────────────────┐   │
│   │  RPC Handler     │──▶│  Tracer.step('ipc') / step(..)   │   │
│   └──────────────────┘   └───────────────┬──────────────────┘   │
│                                          │                       │
│   ┌──────────────────────────────────────▼──────────────────┐   │
│   │  registerTraceSink → /api/trace-stream (SSE broadcast)  │   │
│   └─────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
                                │ IPC / Agent WS
┌───────────────────────────────▼─────────────────────────────────┐
│                      RELAY / AGENT                              │
│   ┌──────────────────┐   ┌──────────────────────────────────┐   │
│   │  Agent WS Server │──▶│  Tracer.step('agent') / ok/fail  │   │
│   └──────────────────┘   └──────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

---

## Luồng trace chi tiết: Browse Directory

```
[Browser]  browseDirFlow.start({ devServerId, path })        → id=abc123
[Browser]    span.step('rpc', { method: 'devServer.browseDir' })
[Main]       createTracer('devServer:browseDir').start(...)   → id=abc123
[Main]         span.step('ipc', { ipcChannel: 'browse-dir' })
[Main]         span.step('relay', { relayId: 'relay-01' })
[Relay]        span.step('agent', { agentId: 'dev-01' })
[Agent]          ... execute ...
[Relay]        span.ok({ entries: 12 })                      → durationMs=45
[Main]       span.ok()
[Browser]  span.ok({ count: 12 })
```

---

## Các flows được trace sẵn (Tracers registry)

| Tracer name | Flow | Mô tả |
|------------|------|--------|
| `Tracers.browseDirFlow` | `devServer:browseDir` | Browser → RPC → IPC → Relay → Agent: list directory |
| `Tracers.mkdirFlow` | `devServer:mkdir` | Browser → RPC → IPC → Relay → Agent: tạo thư mục |
| `Tracers.rmdirFlow` | `devServer:rmdir` | Browser → RPC → IPC → Relay → Agent: xóa thư mục |
| `Tracers.agentWsFlow` | `agentWs:lifecycle` | Agent WebSocket connect / disconnect lifecycle |
| `Tracers.ipcProxyFlow` | `ipc:devServerProxy` | IPC proxy call từ user-process sang main-process |

---

## Tính năng chi tiết

### Span Lifecycle (`start → step* → ok | fail`)

Mỗi span có 4 event types:

| Level | Khi nào | Console output |
|-------|---------|----------------|
| `start` | Bắt đầu operation | `[TRACE] devServer:browseDir id=abc123 devServerId=dev-01` |
| `step` | Mỗi bước trung gian | `[TRACE] devServer:browseDir id=abc123 step=relay` |
| `ok` | Hoàn thành thành công | `[TRACE] devServer:browseDir id=abc123 OK durationMs=45` |
| `fail` | Lỗi xảy ra | `[TRACE] devServer:browseDir id=abc123 FAIL err=timeout durationMs=3001` |

> **Lưu ý:** `fail` events **luôn luôn được log** bất kể flag `ORCA_TRACE` — đây là thiết kế cố ý để đảm bảo mọi lỗi đều có dấu vết.

### Enable / Disable tracing

**Node.js (server / main / relay):**
```bash
ORCA_TRACE=1 npm run dev       # enable
ORCA_TRACE=true npm run dev    # enable (alias)
# Unset = disable (chỉ fail events còn log)
```

**Browser (DevTools console):**
```js
localStorage.setItem('ORCA_TRACE', '1'); location.reload()  // enable
localStorage.removeItem('ORCA_TRACE')                       // disable
```

### Sink Architecture (Platform-agnostic)

Sink là callback nhận mọi `TraceEvent`. Có thể đăng ký nhiều sink cùng lúc:

```typescript
import { registerTraceSink } from 'src/shared/trace'

const unregister = registerTraceSink((event) => {
  // Gửi lên remote monitoring, lưu DB, dispatch vào store...
})

// Hủy đăng ký khi cleanup:
unregister()
```

Các sink mặc định:
- **Console sink**: tích hợp sẵn trong `emit()`, in `[TRACE]` log
- **Zustand store sink**: đăng ký bởi `initBrowserTrace()` ở frontend
- **SSE broadcast sink**: đăng ký bởi `trace-sse-routes.ts` khi có client kết nối

### Browser Adapter (`browser.ts`)

Gọi một lần duy nhất khi app khởi động:

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx
import { initBrowserTrace } from '../../../shared/trace/browser'

initBrowserTrace((event) => {
  useAppStore.getState().addTraceEvent(event)
})
```

`initBrowserTrace` thực hiện 3 việc:
1. Override `isTraceEnabled()` → check `localStorage.ORCA_TRACE`
2. Đăng ký sink → dispatch events vào Zustand store (hiển thị trên TracePanel UI)
3. Kết nối SSE stream `/api/trace-stream` → nhận backend events push về browser

### Backend SSE Stream (`/api/trace-stream`)

Endpoint trên HTTP server cho phép browser nhận real-time trace events từ server:

```
GET /api/trace-stream
Content-Type: text/event-stream
```

**Auth (theo thứ tự ưu tiên):**
1. `Authorization: Bearer <ORCA_AGENT_API_SECRET>` — CI/server use
2. Header `X-Orca-Admin: 1` — dev-only
3. Header `X-Orca-Trace-Client: 1` — browser trace panel (low-security, diagnostic only)
4. Không có secret cấu hình → cho phép mọi local connection

**Heartbeat:** Server gửi SSE comment `: heartbeat` mỗi 15 giây để giữ kết nối qua nginx/load-balancer timeout.

---

## Cách tạo tracer mới

```typescript
import { createTracer } from 'src/shared/trace'

// Tạo tracer cho một flow cụ thể
const tracer = createTracer('mySubsystem:myOperation')

// Sử dụng trong code
async function myOperation(params: MyParams) {
  const span = tracer.start({ paramA: params.a, paramB: params.b })
  try {
    span.step('fetchData', { source: 'db' })
    const data = await fetchFromDb()

    span.step('transform', { records: data.length })
    const result = transform(data)

    span.ok({ resultSize: result.length })
    return result
  } catch (err) {
    span.fail(err, { paramA: params.a })
    throw err
  }
}
```

**Naming convention:** `subsystem:operation` — ví dụ: `devServer:browseDir`, `agentWs:lifecycle`, `ipc:devServerProxy`

---

## Luồng người dùng (TracePanel UI)

```
1. Người dùng mở TracePanel trong Orca UI
2. Browser kết nối SSE /api/trace-stream
3. Người dùng thực hiện thao tác (ví dụ: browse directory)
4. Frontend emit start + step events → vào Zustand store
5. Backend emit step + ok events → push qua SSE → vào Zustand store
6. TracePanel hiển thị toàn bộ span với timeline và elapsed time
7. Nếu có fail: hiển thị lỗi với màu đỏ, luôn visible kể cả khi trace tắt
```

---

## Tiêu chí chấp nhận

- [ ] `createTracer()` hoạt động trong cả Node.js và browser environments
- [ ] Span `id` nhất quán xuyên suốt toàn bộ stack (không phải auto-generated ở mỗi layer)
- [ ] `fail` events luôn log, không phụ thuộc `ORCA_TRACE` flag
- [ ] `ok` events có `elapsedMs` chính xác (so với `startMs` của `start`)
- [ ] SSE client tự động reconnect khi mất kết nối (native EventSource behavior)
- [ ] Đăng ký và hủy đăng ký sink không gây memory leak
- [ ] `initBrowserTrace()` idempotent (safe to call nhiều lần)
- [ ] Heartbeat SSE mỗi 15 giây để tránh timeout

---

## Yêu cầu kỹ thuật

| Thành phần | File |
|-----------|------|
| **Core API** | `src/shared/trace/index.ts` |
| **Pre-built tracers** | `src/shared/trace/tracers.ts` |
| **Browser adapter** | `src/shared/trace/browser.ts` |
| **SSE endpoint** | `src/server/trace-sse-routes.ts` |
| **HTTP registration** | `src/server/http-server.ts` (line ~152) |
| **Frontend bootstrap** | `src/renderer/src/web/main-web-bootstrap.tsx` |
| **Usage: session** | `src/main/session/session-manager.ts` |
| **Usage: session WS** | `src/main/session/ws-session-router.ts` |
| **Usage: dev-server** | `src/main/dev-server/dev-server-relay-bridge.ts` |
| **Usage: dev-server mgr** | `src/main/dev-server/dev-server-manager.ts` |
| **Usage: RPC methods** | `src/main/runtime/rpc/methods/dev-server.ts` |
| **Usage: agent WS** | `src/main/dev-server/agent-ws-server.ts` |

---

## Exported API (`shared/trace/index.ts`)

| Symbol | Loại | Mô tả |
|--------|------|--------|
| `createTracer(flow)` | function | Tạo tracer cho một flow |
| `registerTraceSink(sink)` | function | Đăng ký sink, trả về cleanup fn |
| `setTraceEnabledPredicate(fn)` | function | Override cách check trace enabled |
| `isTraceEnabled()` | function | Kiểm tra trace có đang bật không |
| `TraceEvent` | interface | Cấu trúc event emit ra |
| `TraceSpan` | interface | Handle để gọi step/ok/fail |
| `Tracer` | interface | Factory để tạo span |
| `TraceFields` | type | `Record<string, string\|number\|boolean\|undefined>` |
| `TraceLevel` | type | `'start' \| 'step' \| 'ok' \| 'fail'` |

## Exported API (`shared/trace/browser.ts`)

| Symbol | Loại | Mô tả |
|--------|------|--------|
| `initBrowserTrace(dispatch)` | function | Khởi tạo browser tracing (idempotent) |
| `enableBrowserTrace()` | function | Set `localStorage.ORCA_TRACE = '1'` |
| `disableBrowserTrace()` | function | Remove `localStorage.ORCA_TRACE` |
| `isBrowserTraceEnabled()` | function | Check localStorage flag |
| `isBackendStreamConnected()` | function | Check SSE readyState === OPEN |
| `TraceDispatch` | type | `(event: TraceEvent) => void` |

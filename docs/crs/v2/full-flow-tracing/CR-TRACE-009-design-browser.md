# CR-TRACE-009 — Design & Browser Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-009 |
| **Tên** | Design & Browser — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/design-browser.md`, `src/main/runtime/rpc/methods/browser-core.ts`, `src/main/runtime/rpc/methods/browser-extras.ts`, `src/main/runtime/orca-runtime-browser.ts`, `src/main/browser/agent-browser-bridge.ts`, `src/main/browser/cdp-bridge.ts`, `src/main/browser/browser-manager.ts` |

---

## 1. Vấn đề

`docs/flows/logic/design-browser.md` mô tả các thành phần `DesignModeManager` và `BrowserCapture` — grep trực tiếp source **không tìm thấy hai class này**. Kiến trúc thật sự đã hội tụ về generic RPC method layer (`browser.snapshot`, `browser.screenshot`, `browser.viewport`, ...) dùng chung bởi cả agent tool-calling và (một phần) mobile client (`browser.viewport`, `browser.mouseClick`, `browser.screencast` đều nằm trong `MOBILE_RPC_METHOD_ALLOWLIST` — xem CR-TRACE-007). Mỗi RPC method (`src/main/runtime/rpc/methods/browser-core.ts`, `browser-extras.ts`) gọi thẳng vào `OrcaRuntime` (`src/main/runtime/orca-runtime-browser.ts`), lớp này lại delegate tiếp cho `AgentBrowserBridge` (`src/main/browser/agent-browser-bridge.ts`), rồi cuối cùng `AgentBrowserBridge` gọi `CdpBridge` (`src/main/browser/cdp-bridge.ts`) để thực thi lệnh CDP thật (`Page.captureScreenshot`, `DOM.*`, `Runtime.evaluate`, ...) trên `BrowserManager` (`src/main/browser/browser-manager.ts`, quản lý CDP debugger attach qua `setViewportOverride`).

Đây là một chuỗi 4 tầng in-process (RPC method → OrcaRuntime → AgentBrowserBridge → CdpBridge) trước khi chạm tới CDP thật — mỗi tầng đều có thể là nơi một screenshot bị treo (CDP debugger không attach được, `enqueueCommand`/`enqueueTargetedCommand` bị deadlock do một lệnh trước đó chưa resolve, hoặc `Page.captureScreenshot` timeout trên trang nặng). Không có tracing nghĩa là khi user báo "chụp màn hình bị treo" hoặc "đổi viewport không có tác dụng", không thể biết lỗi nằm ở CDP debugger attach (`browser-manager.ts`), ở hàng đợi lệnh tuần tự (`agent-browser-bridge.ts`), hay ở chính CDP protocol call (`cdp-bridge.ts`).

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thành phần thật (đã xác nhận qua grep) | Layer | Transport |
|---|---|---|---|
| Renderer (Design Mode UI) | Renderer gọi RPC qua kênh runtime chung (không phải `contextBridge.invoke` trực tiếp như flow doc mô tả — cùng cơ chế WS RPC/Unix socket dùng cho mọi RPC method khác) | UI | WS RPC hoặc Unix Socket (tuỳ chế độ desktop/web) — theo CR-TRACE-000 §3.3 hàng "WebSocket RPC (Browser ↔ Orca Server)" |
| DesignModeManager, BrowserCapture | `OrcaRuntime.browserSnapshot/browserScreenshot/browserSetViewport(...)` (`src/main/runtime/orca-runtime-browser.ts:365,474,852`) | Business Logic | In-process (gọi trực tiếp method trên object `runtime`, không băng qua wire) |
| — (không có tên trong flow doc) | `AgentBrowserBridge` (`src/main/browser/agent-browser-bridge.ts:513`) — hàng đợi lệnh theo worktree (`enqueueTargetedCommand`) | Runtime | In-process |
| CDP (Chrome DevTools Protocol) | `CdpBridge` (`src/main/browser/cdp-bridge.ts:91`) — gửi lệnh CDP thật (`Page.captureScreenshot`, `DOM.requestNode`, `Runtime.evaluate`, ...) | Debug Protocol | CDP over Electron debugger session (không phải network transport theo nghĩa CR-TRACE-000 — coi như "external call" cross-boundary vì tương tác với tiến trình renderer/webview khác) |
| Embedded Browser (Electron WebContentsView) | `BrowserManager` (`src/main/browser/browser-manager.ts:207`) quản lý attach/detach CDP debugger (`setViewportOverride`/`doSetViewportOverrideImpl`, dòng 1205-1330) | Runtime | CDP debugger attach |
| Daemon/PTY/Agent (nhận context) | **chưa xác định file cụ thể** — không tìm thấy pattern "inject UI context vào agent" tương ứng BL-DB-02 khi grep (`injectPrompt`, `writeToPty` không tồn tại trong source) | External | Cần điều tra thêm khi triển khai |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  designBrowserCaptureFlow:  createTracer('designBrowser:cdpConnect'),  // BL-DB-01 (capture UI element qua CDP)
  designBrowserInjectFlow:   createTracer('designBrowser:inspect'),     // BL-DB-02 (inject context to agent) — tên giữ theo bảng CR-TRACE-000 §4
  designBrowserViewportFlow: createTracer('designBrowser:screenshot'),  // BL-DB-03 (viewport + screenshot)
}
```

*Ghi chú đặt tên:* Bảng namespace ở CR-TRACE-000 §4 liệt kê ví dụ `designBrowser:cdpConnect`, `designBrowser:screenshot`, `designBrowser:inspect` nhưng không gán rõ ví dụ nào ứng với BL-DB nào. CR này gán: `cdpConnect` = BL-DB-01 (capture — vì bước đầu tiên và tốn kém nhất là attach/dùng CDP session), `inspect` = BL-DB-02 (đặt tên theo hành động "gửi context đã inspect cho agent", không phải theo API CDP), `screenshot` = BL-DB-03 (vì luồng test viewport luôn kết thúc bằng chụp ảnh so sánh baseline). Điều chỉnh tên nếu review không đồng ý.

## 4. Instrumentation theo từng sub-flow

### BL-DB-01 — Capture UI Element

| Bước | span event | fields | File:function |
|---|---|---|---|
| Nhận request từ Renderer | `start` | `worktreeId`/`browserPageId` | `src/main/runtime/rpc/methods/browser-core.ts` (`name: 'browser.snapshot'`, dòng 38-42) |
| Resolve target tab | `step('resolveTarget')` | `worktreeId`, `browserPageId` | `src/main/runtime/orca-runtime-browser.ts:365-368` (`browserSnapshot()`, gọi `resolveBrowserCommandTarget`) |
| Gọi vào hàng đợi lệnh theo worktree | `step('enqueue')` — cross-boundary tới CDP, có thể chờ lệnh trước đó | `sessionName` | `src/main/browser/agent-browser-bridge.ts:768-780` (`snapshot()` → `enqueueTargetedCommand`) |
| Thực thi CDP thật | `ok`/`fail` | `nodeCount` (nếu snapshot trả về được, tuỳ hình dạng response) | `src/main/browser/cdp-bridge.ts:148-174` (`snapshot()`) |

```typescript
// src/main/runtime/orca-runtime-browser.ts
async browserSnapshot(params: BrowserCommandTargetParams): Promise<BrowserSnapshotResult> {
  const span = Tracers.designBrowserCaptureFlow.start({ worktreeId: params.worktreeId })
  try {
    const target = await this.resolveBrowserCommandTarget(params)
    span.step('resolveTarget', { worktreeId: target.worktreeId, browserPageId: target.browserPageId })
    const result = await this.requireAgentBrowserBridge().snapshot(target.worktreeId, target.browserPageId)
    span.ok({})
    return result
  } catch (err) {
    span.fail(err, { worktreeId: params.worktreeId })
    throw err
  }
}
```

*Ghi chú:* flow doc mô tả các lệnh CDP chi tiết (`DOM.getOuterHTML`, `CSS.getComputedStyleForNode`, `Page.captureScreenshot` riêng lẻ cho từng thuộc tính) — thực tế `CdpBridge.snapshot()` là một lệnh tổng hợp (accessibility-tree-style snapshot dùng cho agent tool-calling), không phải 3 lệnh CDP tách rời như flow doc mô tả. Không cần `step()` cho từng sub-call CDP bên trong `snapshot()` (biến đổi/gộp dữ liệu thuần tuý theo CR-TRACE-000 §5) — chỉ 1 `step('enqueue')` ở biên hàng đợi là đủ.

### BL-DB-02 — Inject UI Context vào Agent

| Bước | span event | fields | File:function |
|---|---|---|---|
| Renderer gửi context đã capture | `start` | `worktreeId` (nếu có) | **chưa xác định file cụ thể** |
| Format message + gửi vào PTY/agent | `ok`/`fail` | — | **chưa xác định file cụ thể** — không tìm thấy `injectToAgent`, `writeToPty`, hay pattern tương đương khi grep `src/` |

*Không viết code snippet cho sub-flow này* — vì không xác nhận được implementation thật, viết snippet sẽ là fabrication. Khi triển khai CR này, bước đầu tiên bắt buộc là điều tra: (a) tính năng "Send to Agent" từ Design panel có thực sự tồn tại trong code hiện tại không (có thể đây là tính năng roadmap chưa ship), (b) nếu tồn tại dưới tên khác, xác định RPC method hoặc IPC channel thật trước khi thêm `designBrowser:inspect`. Nếu xác nhận tính năng chưa tồn tại, CR-TRACE-009 nên loại bỏ mục BL-DB-02 khỏi phạm vi thay vì trace một luồng không có code.

### BL-DB-03 — Viewport Testing

| Bước | span event | fields | File:function |
|---|---|---|---|
| Nhận request đổi viewport | `start` | `width`, `height`, `mobile` | `src/main/runtime/rpc/methods/browser-extras.ts:53-57` (`name: 'browser.viewport'`) |
| Resolve target + gọi bridge | `step('resolveTarget')` | `worktreeId`, `browserPageId` | `src/main/runtime/orca-runtime-browser.ts:852-869` (`browserSetViewport()`) |
| Set viewport qua CDP | `step('cdpSetViewport')` — cross-boundary, có thể fail nếu debugger chưa attach | `width`, `height`, `deviceScaleFactor` | `src/main/browser/agent-browser-bridge.ts` (`setViewport(...)`) → `src/main/browser/browser-manager.ts:1278-1330` (`doSetViewportOverrideImpl`, dùng `Emulation.setDeviceMetricsOverride` qua debugger CDP) |
| (QA run matrix) Chụp ảnh so sánh baseline | `step('screenshot')` mỗi viewport trong matrix | `viewportLabel` | `src/main/browser/cdp-bridge.ts:632-668` (`fullPageScreenshot()`, dùng `Page.captureScreenshot`) |
| Kết thúc | `ok` | `viewportCount` (nếu chạy matrix) | `orca-runtime-browser.ts` hoặc call site tổng hợp matrix (chưa xác định rõ nơi loop `[320,768,1280,1920]` được implement — flow doc mô tả nhưng chưa tìm thấy file cụ thể) |

```typescript
// src/main/runtime/orca-runtime-browser.ts
async browserSetViewport(
  params: { width: number; height: number; deviceScaleFactor?: number; mobile?: boolean } & BrowserCommandTargetParams
): Promise<BrowserViewportResult> {
  const span = Tracers.designBrowserViewportFlow.start({ width: params.width, height: params.height, mobile: params.mobile })
  try {
    const target = await this.resolveBrowserCommandTarget(params)
    span.step('resolveTarget', { worktreeId: target.worktreeId })
    const result = await this.requireAgentBrowserBridge().setViewport(
      params.width, params.height, params.deviceScaleFactor, params.mobile,
      target.worktreeId, target.browserPageId
    )
    span.step('cdpSetViewport', { width: params.width, height: params.height })
    span.ok({})
    return result
  } catch (err) {
    span.fail(err, { width: params.width, height: params.height })
    throw err
  }
}
```

*Ghi chú:* nhánh "QA run matrix" (loop qua nhiều viewport + so sánh baseline) mô tả trong flow doc **chưa xác định file cụ thể** — không tìm thấy vòng lặp `[320, 768, 1280, 1920]` hay logic so sánh baseline khi grep `src/`. Nếu tính năng này chưa tồn tại, `designBrowser:screenshot` chỉ cần bao phủ một lần gọi `browser.viewport` + `browser.screenshot` đơn lẻ; phần "step('screenshot') mỗi viewport trong matrix" ở bảng trên chỉ áp dụng nếu/khi tính năng matrix được implement.

## 5. Lan truyền traceId qua transport của flow này

Domain Design & Browser không băng qua network/process boundary thật (khác Mobile/Automation) — toàn bộ chuỗi RPC method → `OrcaRuntime` → `AgentBrowserBridge` → `CdpBridge` chạy **trong cùng Main process**. Vì vậy áp dụng CR-TRACE-000 §3.3 theo 2 cấp:

1. **Cấp ngoài cùng (Renderer → Main)**: giống mọi RPC method khác, `traceId` là sibling field cạnh `method`/`params` trong request envelope (WS RPC hoặc Unix Socket tuỳ chế độ). RPC method handler (`browser-core.ts`/`browser-extras.ts`) đọc `traceId` từ context RPC (nếu dispatcher truyền xuống qua `meta`/`ctx`) và gọi `Tracers.designBrowserXxxFlow.start(fields, traceId ? { id: traceId } : undefined)`.
2. **Cấp trong (OrcaRuntime → AgentBrowserBridge → CdpBridge)**: đây là lời gọi hàm trực tiếp trong cùng process — **không cần** một field `traceId` kiểu wire envelope; chỉ cần truyền `span.id` xuống dưới dạng tham số hàm (hoặc đóng span ngay ở `OrcaRuntime` layer và không cần các layer dưới biết `traceId`) vì `AgentBrowserBridge`/`CdpBridge` không tự tạo span riêng — chúng nằm bên trong `step('enqueue')`/`step('cdpSetViewport')` của cùng một span `designBrowser:*`, không phải một tracer độc lập kiểu `relay:agentCall`.
3. Nếu agent (không phải Renderer) là actor gọi các method này (tool-calling — `browser.snapshot`/`browser.click`/... đều expose cho agent qua Agent WS JSON-RPC), áp dụng đúng hàng "Agent WS JSON-RPC 2.0" của CR-TRACE-000 §3.3: `traceId` nested trong `params._trace.id`.

## Acceptance Criteria

- [ ] `Tracers.designBrowserCaptureFlow/designBrowserInjectFlow/designBrowserViewportFlow` thêm vào `tracers.ts` với tên `designBrowser:cdpConnect`/`designBrowser:inspect`/`designBrowser:screenshot`
- [ ] `designBrowser:cdpConnect` bao phủ `OrcaRuntime.browserSnapshot()` (`orca-runtime-browser.ts:365`) với `step('resolveTarget')` và `step('enqueue')`
- [ ] `designBrowser:screenshot` bao phủ `OrcaRuntime.browserSetViewport()` (`orca-runtime-browser.ts:852`) với `step('cdpSetViewport')`
- [ ] BL-DB-02 ("Send to Agent") được xác minh tồn tại trong code trước khi thêm `designBrowser:inspect`; nếu không tồn tại, mục này bị loại khỏi rollout thay vì trace một đường không chạy
- [ ] Nhánh "QA run matrix" (nhiều viewport + baseline compare) được xác minh tồn tại trước khi implement `step('screenshot')` lặp lại trong bảng BL-DB-03
- [ ] `span.fail()` được gọi khi `CdpBridge` throw `BrowserError` (`cdp-bridge.ts:54`) ở bất kỳ layer nào trong chuỗi
- [ ] Khi method được gọi từ Agent WS (tool-calling) thay vì Renderer, `traceId` được đọc từ `params._trace.id` đúng theo CR-TRACE-000 §3.3, không đọc nhầm từ sibling field dùng cho WS RPC

# BUG-FE-PW-005 — `AgentPanel` (tab Agent) gọi `window.api.agentOrchestration.*` — API này không tồn tại ở web runtime, nhiều khả năng làm cả tab Agent không dùng được trên `b15.openledger.vn`

## Mức độ: 🔴 HIGH

## Trạng thái: ✅ Đã fix + deploy (2026-09-15) — xem [SOL-FE-PW-004](./solutions/SOL-FE-PW-004-agent-panel-web-orchestration.md) cho chi tiết implementation (client mới `runtime-agent-orchestration-client.ts`, `AgentPanel.tsx` gọi `agent.*`/`aiProvider.resolve` thật qua wscompat, kèm fix phụ [BUG-022](../../backend-go/bugs/missing-v2/BUG-022-aiprovider-resolve-returns-raw-snake-case-proto.md)). Chờ user xác nhận lại trên `b15.openledger.vn`.

**Xác nhận thêm**: đọc chính xác `AgentPanel.tsx:71,88,94,112,129,147` — dòng 94 `result.sessionId` (với `result === undefined` trên web) throw `TypeError`, bị catch ở dòng 112, hiện toast `"Failed to start agent: Cannot read properties of undefined (reading 'sessionId')"`. Khớp 100% với suy luận ban đầu. **Tab Agent trong Project Workspace (Beta) hoàn toàn không dùng được trên web deployment** — mọi thao tác Start/Stop/Resume đều lỗi ngay lập tức.

## Tóm tắt

Phát hiện trong lúc tìm chỗ thêm UI cho `agent.switchAccount` (CR-PW-009): `AgentPanel.tsx` (component tab "Agent" trong Project Workspace Beta) gọi trực tiếp `window.api.agentOrchestration.start/stop/resume(...)` và `window.api.agentOrchestration.onStatusChanged(...)` — đây là API kiểu **Electron preload bridge** (`window.api`), không phải `callRuntimeRpc`/wscompat pattern mà mọi nơi khác trong Project Workspace đã verify hoạt động đúng trên web (`ProjectSettings.tsx`, `CreateProjectDialog.tsx`, `WorkspaceTerminalPanel.tsx`...).

## Root Cause — suy luận từ source (chưa click-test thật)

`frontend/src/renderer/src/web/web-preload-api.ts`:
```ts
window.api = withFallback(createWebPreloadApi(), []) as PreloadApi
```
`createWebPreloadApi()` — grep xác nhận **không có key `agentOrchestration`** trong toàn bộ file. `withFallback` dùng `Proxy`: truy cập property không tồn tại trả về `createFallbackProxy` — 1 hàm mà gọi ra luôn `return undefined`, **không throw**.

Hệ quả suy ra: `window.api.agentOrchestration.start({...})` ở web mode trả về `undefined` (không phải Promise thật) → `AgentPanel.tsx`:
```ts
const result = await window.api.agentOrchestration.start({...})
span.step('ipc-invoke-resolved', { sessionId: result.sessionId, status: result.status })  // ← result là undefined
```
→ `TypeError: Cannot read properties of undefined (reading 'sessionId')`, bị `catch` nuốt thành toast "Failed to start agent: Cannot read properties of undefined...". Tương tự cho `stop`/`resume`. `onStatusChanged` cũng trả fallback proxy — subscribe không bao giờ nhận được event thật.

**Nếu đúng như suy luận này: toàn bộ tab Agent trong Project Workspace (Beta) không dùng được trên web deployment (`b15.openledger.vn`)** — nút Start/Resume/Stop luôn lỗi ngay khi bấm.

## Cần làm để xác nhận (chưa làm — ngoài khả năng của phiên này, cần click thật trên UI)

- Vào Project Workspace (Beta) → 1 worktree → tab Agent → bấm "Start Agent" → xem có đúng lỗi `Cannot read properties of undefined` hiện lên hay không.

## Hướng fix — ĐÃ VIẾT SPEC CHI TIẾT: [SOL-FE-PW-004](./solutions/SOL-FE-PW-004-agent-panel-web-orchestration.md)

**Cập nhật quan trọng (2026-09-15)**: backend-go **đã có sẵn** đầy đủ `agent.start`/`agent.stop`/`agent.kill`/`agent.resume`/`agent.switchAccount`/`agent.subscribeStatus` thật trong `channels_agent.go` — đây KHÔNG phải gap thiếu backend như CR-TSRC-001, chỉ là chưa nối phía frontend. Tuy nhiên contract thật khác khá nhiều so với những gì `AgentPanel.tsx` đang gọi (thiếu `connectionId`/`modelId`/`accountId`, `resume` phải theo `worktreeId` chứ không phải `sessionId`, response field tên khác, có thêm pty output stream chưa từng xử lý) — xem SOL-FE-PW-004 để biết đầy đủ chi tiết + việc cần làm trước khi implement.

## Ảnh hưởng tới CR-PW-009

Việc thêm UI `agent.switchAccount` (đề xuất trong CR-PW-009) **nên hoãn lại** cho tới khi bug này được xác nhận và fix — xây thêm 1 nút trên 1 panel có khả năng đã hỏng toàn bộ không có giá trị.

## Liên quan

- [CR-PW-009](../../../../docs/crs/v3/project-workspace/CR-PW-009-close-backend-only-ui-gaps-in-project-workspace.md)
- `agent-orchestration.ts` (desktop, `desktop/src/main/ipc/agent-orchestration.ts`) — bản Electron thật, hoạt động đúng trên desktop, không phải trên web

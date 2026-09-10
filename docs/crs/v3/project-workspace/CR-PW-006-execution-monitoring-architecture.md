# CR-PW-006 — Monitor workflow step execution trên dev-server qua agent + backend-go (proxy)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-006 |
| **Tên** | Kiến trúc theo dõi live execution end-to-end: agent (dev-server) → backend-go (proxy) → frontend |
| **Loại** | Architecture / Feature |
| **Priority** | 🟡 P1 (Phase A) / 🔵 P2-P3 (Phase B-E, thiết kế, chưa triển khai đầy đủ) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-06 |
| **Trạng thái** | 🟡 Partially Implemented — Phase A (interim polling) ✅ Implemented; Phase B/C/D/E 🔲 Designed, không triển khai (xem "Trạng thái triển khai") |
| **Tác giả** | Yêu cầu user: "monitor quá trình thực thi trên các dev-server thông qua agent và backend-go (đóng vai trò proxy)" |
| **Tác động HLD** | F38 — Project Workspace |
| **Tác động Features** | Workflows tab, `ExecutionMonitor`, `agent/`, `backend-go/services/workflow-service`, `backend-go/services/infra-fleet-service` |

---

## Bối cảnh & Vấn đề gốc

Có **2 workflow engine song song, không tương thích** trong codebase:

1. **Legacy TS** (`backend/src/main/workflow/`, `desktop/src/main/workflow/`):
   `WorkflowOrchestrator.ts` + `StepExecutors.ts`, model DAG/wave/step đầy đủ.
   `WorkflowStep.serverSpec: 'project:<projectId>' | 'server:<devServerId>'` — nhánh
   `'server:<devServerId>'` **chưa được implement**, ném lỗi `SERVER_SPEC_NOT_SUPPORTED` (xác
   nhận trong `StepExecutors.ts` ở cả `backend/` và `desktop/`, cùng message). Đây là engine mà
   Electron/local mode dùng — `ExecutionMonitor.tsx`/`WorkflowMonitor.tsx` (CR-PW-003) được viết
   theo shape đầy đủ này (`execution.definition.name`, `.steps`, `.startedAt`, `.triggeredBy`).

2. **Go mới** (`backend-go/services/workflow-service`): model phẳng —
   `WorkflowExecution{id, template_id, status, root_trace_id, project_id}` — KHÔNG có
   `definition`/`steps`/`startedAt`/`triggeredBy`. `GetExecutionResponse` chỉ trả về đúng 5 field
   này. Đây là engine mà Web mode (qua `api-gateway`'s wscompat) dùng.

Hệ quả: **frontend UI được viết cho model #1, nhưng Web mode chạy trên model #2** — trước
CR-PW-005, Web mode còn không gọi được phần lớn RPC của model #2; sau CR-PW-005, gọi được nhưng
response shape vẫn thiếu hẳn phần "steps" mà `ExecutionMonitor.tsx` cần để vẽ wave/step. Ngay cả
khi Web mode gọi đúng RPC, `execution.definition` sẽ là `undefined` → `ExecutionMonitor.tsx`'s
`execution.definition.steps` sẽ crash (`groupStepsByWave(execution.definition.steps ?? [], ...)`
— may mắn có `?? []` guard cho `steps`, nhưng `execution.definition.name`ở dòng render header thì
không có guard, sẽ throw `Cannot read properties of undefined`). **Đây là 1 gap riêng, chưa có CR
số nào che phủ nó** — ghi nhận ở đây, không sửa trong phiên này (ngoài phạm vi an toàn, xem "Trạng
thái triển khai").

Thêm vào đó: **không có transport push nào** giữa workflow-service (backend-go) và trình duyệt
(browser) — `useWorkflowExecution.ts`'s `window.api.on(...)` là bridge Electron-only, và trong Web
mode nó fail **âm thầm** (xem chi tiết kỹ thuật ở mục "Root cause: window.api.on trong Web mode"
bên dưới) — không lỗi, không cảnh báo, chỉ đơn giản là không bao giờ nhận được step-status/output/
complete event.

Cuối cùng: **`agent/` không có code thực thi workflow nào** — chỉ có
`agent/src/shared/workflow-types.ts` (type definitions thuần, mirror shared types, không có
dispatcher/handler nào dùng nó). 1 step loại `'agent'` nhắm vào dev-server hôm nay đi qua
`backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go` →
gọi **unary** (không streaming) `infrafleetv1.InfraFleetServiceClient.Relay(ctx, {ConnectionId,
Method: "agent.exec", ParamsJson})` — xác nhận đọc trực tiếp file này: executor gọi `Relay` 1 lần,
đợi 1 response duy nhất, KHÔNG có channel progress/output nào chảy ngược lại trong lúc step đang
chạy.

Về nghi vấn `"agent.exec"` vs `"agent.execPrompt"`: đọc trực tiếp `agent_step_executor.go` cho
thấy đây **đã là 1 gap tự ghi nhận sẵn trong code** (doc comment của hằng số `agentExecMethod`,
trích dẫn `specs/agent/api/gaps-and-findings.md`'s "TS Gap 4") — CR này chỉ xác nhận lại nó còn
nguyên vẹn, không phải phát hiện mới. Cross-check `agent/src/`: có tồn tại `case 'agent.exec':`
thật trong `agent-rpc-dispatch-agent-exec.ts` — NHƯNG đây là 1 RPC generic process-exec
(`{binary, args, cwd, stdin, env, timeoutMs}`), khác hẳn shape mà `agentExecMethod`'s caller gửi
(`agentExecParams{Prompt, WorktreePath, TrustPreset}`). Legacy TS's `StepExecutors.executeAgent()`
đã từng gặp đúng bug này và chuyển sang gọi `"agent.execPrompt"` (param shape đúng: prompt/model/
trustPreset) — nhưng backend-go's `AgentExecutor` **chưa được cập nhật theo**, vẫn gửi
`"agent.exec"` kèm params sai shape. Kết quả thực tế nhiều khả năng: khi step loại `'agent'` chạy
qua backend-go's workflow-service nhắm 1 dev-server thật, agent nhận đúng method (`agent.exec`
tồn tại) nhưng payload không khớp field nó mong đợi — hành vi cụ thể (lỗi rõ ràng hay silently
sai) tùy vào validation phía `agent.exec`'s handler, chưa verify sâu hơn trong CR này. Đây là 1
**param-shape mismatch cross-stack đã biết, chưa được fix** — ghi nhận, không sửa trong phiên này
(xem "Không thuộc phạm vi CR này").

### Root cause: `window.api.on` trong Web mode

`useWorkflowExecution.ts` (trước khi sửa) gọi
`(window as any).api.on('workflow:stepStatus', cb)`, guard bởi
`if (!(window as any).api?.on) {return}`. Trong Web mode, `window.api` được build bởi
`createWebPreloadApi()` (`frontend/src/renderer/src/web/web-preload-api.ts`) rồi bọc qua
`withFallback(createWebPreloadApi(), [])` — 1 `Proxy`. `withFallback`'s `get` trap:

```ts
get(current, property, receiver) {
  if (property in current) { ... }
  return createFallbackProxy([...path, String(property)])
}
```

`'on'` **không phải 1 key thật** ở top-level của object `createWebPreloadApi()` trả về (chỉ có
các key `on<Something>` lồng bên trong từng namespace con, ví dụ `terminal.onData`) — nên
`'on' in current` là `false`, và truy cập `window.api.on` trả về `createFallbackProxy(['on'])`:
1 hàm callable (`Proxy` trên 1 `function`), **truthy**. Guard `if (!(window as any).api?.on)`
**không bao giờ true** trong Web mode → code tiếp tục gọi `window.api.on(...)`, và lệnh gọi này
hit `getFallbackResult(['on'], args)`:

```ts
function getFallbackResult(path: string[], args: unknown[]): unknown {
  const name = path.at(-1) ?? ''
  if (name.startsWith('on')) {
    return noopUnsubscribe
  }
  ...
```

→ trả về `noopUnsubscribe` **ngay lập tức**, không bao giờ đăng ký callback thật. Đây là lý do live
step-status/output/complete event bị **rớt âm thầm mãi mãi** ở Web mode — tệ hơn "code path bị
skip": nhìn code thì tưởng đã wire xong (không lỗi, không warning), nhưng không bao giờ deliver gì
cả. Đã confirm bằng cách đọc trực tiếp `withFallback`/`createFallbackProxy`/`getFallbackResult`
trong `web-preload-api.ts`, không suy đoán.

### Không có transport push JSON chung nào để tái dùng

Đã tìm `remote-runtime-terminal-multiplexer.ts` (`subscribeTerminal`) và `web-runtime-client.ts`'s
`WebRuntimeClient` — đây là 1 giao thức binary frame chuyên cho PTY streaming
(`TerminalStreamOpcode`, `subscribeTerminal`), KHÔNG phải 1 cơ chế "on(channel, handler)" tổng quát
cho JSON event. Grep toàn bộ `frontend/src/renderer/src/` cho `onPushEvent`/`pushChannel`/
`subscribeChannel`/`RegisterStreamChannel` phía client: **0 kết quả** — không có pattern
client-side generic nào để tái dùng hôm nay ngoài cơ chế PTY chuyên biệt này. Điều này xác nhận:
xây transport JSON push generic mới (Phase D/E) là việc thật, không phải "chỉ cần gọi lại 1 hàm có
sẵn" — và do đó nằm ngoài phạm vi an toàn của phiên làm việc này.

## Kiến trúc mục tiêu theo phase

```
Phase A (✅ Implemented, phiên này)
  frontend polls workflow.getExecution mỗi 4s khi status='running'
  — dùng RPC đã nối dây ở CR-PW-005, không cần push transport nào

Phase B (🔲 Designed, chưa implement — cần đổi .proto)
  workflow-service: thêm RPC ListExecutions (paginated, filter project_id/status,
  mirror ListTemplates) + ListStepExecutions (wire vào repository method đã có sẵn)
  → api-gateway wscompat expose workflow.listExecutions/workflow.listStepExecutions

Phase C (🔲 Designed, chưa implement — cần migration nhỏ)
  workflow.step_executions thêm cột started_at/completed_at (hiện KHÔNG có)

Phase D (🔲 Designed, chưa implement — cross-repo agent + backend-go)
  agent's "agent.exec"/"agent.execPrompt" handler emit progress event NGƯỢC LẠI
  qua CHÍNH connection relay đã có sẵn tới infra-fleet-service (không tạo connection thứ 2)
  → infra-fleet-service hoặc workflow-service persist các event này vào step_executions
    khi chúng đến, VÀ expose qua 1 server-streaming gRPC method mới
  → api-gateway's wscompat expose route đó qua RegisterStreamChannel mới:
    "workflow.execution.subscribe" (mirror đúng shape "terminal.create"'s
    drainAttachPtyOutput)

Phase E (🔲 Designed, chưa implement — phụ thuộc Phase D)
  frontend: thay polling (Phase A) bằng subscribe "workflow.execution.subscribe"
  qua client pattern tương tự terminal multiplexer's push-consuming side
  (khi Phase D tồn tại thật — hiện KHÔNG có pattern JSON push nào để tái dùng,
  xem mục root cause ở trên)
```

### Rủi ro / độ phức tạp / phụ thuộc từng phase

| Phase | Đội sở hữu | Cần đổi proto? | Reversible? | Rủi ro chính |
|---|---|---|---|---|
| A | Frontend | Không | Có (chỉ đổi 1 hook) | Thấp — polling tăng nhẹ tải RPC khi có execution đang chạy; đã giới hạn 4s/lần, dừng khi status khác `'running'` |
| B | backend-go (workflow-service + api-gateway) | **Có** — `ListExecutions`/`ListStepExecutions` mới | Có (RPC mới, thuần additive) | Trung bình — `buf generate` phải chạy sạch, không phá generated code của service khác dùng chung `proto/gen/go` |
| C | backend-go (workflow-service) | Không (chỉ SQL migration) | Có (`ALTER TABLE ADD COLUMN`, có down migration) | Thấp — additive, không đổi cột hiện có |
| D | agent + infra-fleet-service + workflow-service + api-gateway | **Có** — RPC streaming mới + có thể đổi `agent.exec` payload | Khó — đụng tới connection relay đang chạy cho PTY, cần test kỹ không phá luồng đó; đổi giao thức giữa 4 service/repo cùng lúc, cần điều phối deploy | **Cao** — đây là lý do CR này KHÔNG implement Phase D trong phiên này |
| E | Frontend | Không (client-side only, sau khi D tồn tại) | Có | Thấp, nhưng phụ thuộc cứng vào D |

## Trạng thái triển khai (phiên làm việc 2026-09-06)

- **Phase A — ✅ Implemented.** `useWorkflowExecution.ts` không còn gọi `window.api.on(...)` —
  thay bằng polling `workflow.getExecution` mỗi 4s khi `execution.status === 'running'`, dừng khi
  status là terminal (`completed`/`failed`/`cancelled`) hoặc không xác định. Đây là cross-platform
  thật (Electron + Web) vì đi qua `callRuntimeRpc` — cùng cơ chế `WorkflowMonitor.tsx` (CR-PW-003)
  đã dùng cho `workflow.listExecutions`. **Chỉ báo cáo được `status` ở cấp execution, KHÔNG có
  per-step detail** cho tới khi Phase B/C được implement — đây là hạn chế đã biết trước, ghi rõ
  trong code comment của hook. Verification: `vitest run` — 10/10 pass (5 test cũ + 5 test polling
  mới, dùng `vi.useFakeTimers()`), xem [FE-SOL-005](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-005-execution-status-polling.md).
- **Phase B — 🔲 Designed, KHÔNG implement.** Quyết định KHÔNG chạm `.proto` trong phiên này dù
  tooling có sẵn (`buf`, `protoc`) — lý do: `ListExecutions`/`ListStepExecutions` là RPC hoàn toàn
  mới (không phải nối dây RPC có sẵn như CR-PW-005), và message shape phù hợp cho
  `ListStepExecutions` cần khớp với `step_executions` domain hiện tại (chưa có `started_at`/
  `completed_at` — Phase C) — làm cả 2 cùng lúc trong 1 phiên tăng rủi ro không cần thiết cho 1
  service dùng chung `proto/gen/go` với các service khác. Thiết kế đầy đủ nằm ở mục "Kiến trúc mục
  tiêu" trên; chưa có code, chưa có test.
- **Phase C — 🔲 Designed, KHÔNG implement.** Phụ thuộc Phase B tồn tại trước (không có ý nghĩa gì
  nếu thêm cột mà không có RPC nào đọc nó).
- **Phase D — 🔲 Designed, KHÔNG implement.** Rủi ro CAO nhất (xem bảng trên) — cross-repo
  (agent/backend-go), đụng connection relay đang phục vụ PTY streaming thật, và còn 1 nghi vấn
  chưa giải quyết (`"agent.exec"` vs `"agent.execPrompt"` mismatch) cần điều tra riêng trước khi
  thiết kế event-emission chi tiết hơn. Xem [SOL-AG-PW-001](../../../../../specs/agent/crs/v3/project-workspace/solutions/SOL-AG-PW-001-execution-progress-reporting-design.md)
  (agent-side design, trạng thái "🔲 Designed — not implemented").
- **Phase E — 🔲 Designed, KHÔNG implement.** Phụ thuộc cứng Phase D.

## Không thuộc phạm vi CR này

- Sửa param-shape mismatch `"agent.exec"` vs `"agent.execPrompt"` trong `agent_step_executor.go`
  (đổi `agentExecMethod`/`agentExecParams` sang `"agent.execPrompt"` + shape đúng) — đã xác nhận
  đây là bug thật (không phải alias ẩn: `agent.exec` tồn tại thật phía agent nhưng là 1 RPC khác,
  process-exec thuần, không nhận `prompt`/`worktreePath`/`trustPreset`), nhưng đây là 1 fix cho
  con đường "step 'agent' chạy trên dev-server thật qua backend-go", tách biệt khỏi trọng tâm
  "monitor execution" của CR này — cần 1 CR/fix riêng.
- Implement `serverSpec: 'server:<devServerId>'` cho legacy TS engine (`StepExecutors.ts` —
  `SERVER_SPEC_NOT_SUPPORTED`) — engine legacy TS không phải trọng tâm của CR này (CR này tập
  trung vào backend-go + agent, engine mà Web mode/hosted backend thật sự dùng).
- Sửa `ExecutionMonitor.tsx`'s `execution.definition.name`/`.steps` crash khi chạy trên model Go
  mỏng (thiếu `definition`) — đây là 1 gap riêng phát hiện trong lúc viết CR này, chưa có CR nào
  che phủ; ghi nhận, không sửa (sửa đúng cần quyết định UI nên hiển thị gì khi thiếu `definition`,
  ngoài phạm vi "monitor execution trên dev-server" của CR này).
- Bất kỳ thay đổi proto/schema/agent-side code nào cho Phase B/C/D — xem "Trạng thái triển khai".

## Liên quan

- [CR-PW-003](./CR-PW-003-workflows-tab-wiring.md), [CR-PW-004](./CR-PW-004-step-status-badge-cancelled-crash.md), [CR-PW-005](./CR-PW-005-wscompat-missing-workflow-rpcs.md)
- `frontend/src/renderer/src/hooks/useWorkflowExecution.ts`
- `frontend/src/renderer/src/web/web-preload-api.ts` (`withFallback`/`createFallbackProxy`/`getFallbackResult`)
- `frontend/src/renderer/src/runtime/remote-runtime-terminal-multiplexer.ts` (push pattern tham chiếu, KHÔNG tái dùng trực tiếp được)
- `backend-go/services/workflow-service/internal/domain/step_execution.go`, `internal/adapter/postgres/repository.go`, migration `0004_step_executions.up.sql`
- `backend-go/services/workflow-service/internal/adapter/infrafleetclient/agent_step_executor.go`, `relay_client.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go` (`drainAttachPtyOutput` — mẫu tham chiếu cho Phase D's `RegisterStreamChannel`), `push_bridge.go`
- `backend/src/main/workflow/StepExecutors.ts`, `desktop/src/main/workflow/StepExecutors.ts` (`SERVER_SPEC_NOT_SUPPORTED`)
- `agent/src/shared/workflow-types.ts`
- [FE-SOL-005](../../../../../specs/frontend/crs/v3/project-workspace/solutions/FE-SOL-005-execution-status-polling.md) (Phase A)
- [BE-SOL-001](../../../../../specs/backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md) (Phase A dependency, CR-PW-005)
- [SOL-AG-PW-001](../../../../../specs/agent/crs/v3/project-workspace/solutions/SOL-AG-PW-001-execution-progress-reporting-design.md) (Phase D design-only)

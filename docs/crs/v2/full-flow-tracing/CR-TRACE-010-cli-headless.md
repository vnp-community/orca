# CR-TRACE-010 — CLI & Headless Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-010 |
| **Tên** | CLI & Headless — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/cli-headless.md`, `src/cli/handlers/worktree.ts`, `src/cli/handlers/orchestration.ts`, `src/cli/handlers/automations.ts`, `src/cli/handlers/core.ts`, `src/cli/runtime/client.ts`, `src/cli/runtime/transport.ts`, `src/cli/runtime/websocket-transport.ts`, `src/cli/runtime/launch.ts`, `src/shared/remote-runtime-client.ts`, `src/shared/runtime-rpc-envelope.ts`, `src/main/runtime/runtime-rpc.ts`, `src/main/runtime/rpc/unix-socket-transport.ts`, `src/main/runtime/rpc/methods/worktree.ts`, `src/main/runtime/rpc/methods/orchestration.ts`, `src/main/runtime/rpc/methods/orchestration-gates.ts`, `src/main/runtime/rpc/methods/automations.ts`, `src/main/automations/service.ts` |

---

## 1. Vấn đề

`cli-headless.md` tự ghi nhận rằng HLD ("Daemon Process qua Unix Socket") không khớp thực tế. Khi điều tra source thật cho CR này, phát hiện thêm là **chính mô tả "thực tế" trong flow doc (`CLI → HTTP/WS :6768 → Electron Main`) cũng chỉ đúng một phần**:

- Khi CLI chạy **local** (trường hợp phổ biến nhất — `orca worktree create`, `orca agent ...` trên máy đang chạy Orca desktop), `RuntimeClient.call()` (`src/cli/runtime/client.ts:50-84`) không gọi HTTP/WS :6768 — nó mở kết nối **Unix domain socket / Windows named pipe** qua `sendRequest()` (`src/cli/runtime/transport.ts:7-24`, `socket.write` tại dòng 174) tới `UnixSocketTransport` phía main process (`src/main/runtime/rpc/unix-socket-transport.ts`), dùng newline-delimited JSON, rồi `OrcaRuntimeRpcServer.handleMessage()` (`src/main/runtime/runtime-rpc.ts:922`) dispatch tới method handler.
- **:6768** chỉ xuất hiện khi CLI dùng **remote pairing** (`orca <cmd> --pairing-code ...` / `--environment ...`, hoặc `orca serve --port 6768 ...`): `RuntimeClient.call()` rẽ sang `sendWebSocketRequest()` (`src/cli/runtime/websocket-transport.ts`) → `sendRemoteRuntimeRequest()` (`src/shared/remote-runtime-client.ts`), một kênh WebSocket **mã hoá end-to-end** (payload encrypt/decrypt qua `e2ee-crypto.ts`), tương tự cơ chế Mobile Companion.

Hiện tại **không có span nào** trên toàn bộ chuỗi này. Hậu quả cụ thể:
- `orca worktree create` treo/timeout — không biết đang kẹt ở bước kết nối socket (Unix socket không tồn tại vì Orca chưa chạy?), ở `worktree.create` handler (Git CLI chậm?), hay ở `relay.call('git.worktree.add')` xuống Dev Server remote.
- `orca agent start`/`orca run --automation` (thực chất là `orchestration.run` / `orchestration.dispatch` / `automation.runNow`) fail — không phân biệt được lỗi xảy ra ở tầng CLI-transport, ở `AutomationService` (precheck vs dispatch vs usage collection), hay ở remote agent spawn qua relay.
- `orca serve` (headless mode thật, không phải daemon riêng — xem `serveOrcaApp()` tại `src/cli/runtime/launch.ts:66`) không có cách nào trace end-to-end một request tới khi hoàn tất vì đây là process con (`spawnProcess`) độc lập với CLI gọi nó.

Vì §3.3 của CR-TRACE-000 mô tả transport ":6768" theo quy ước chung ("CLI tự tạo `traceId`... giống WS RPC"), CR này áp dụng đúng quy ước đó cho *cả hai* transport thật (Unix socket/named-pipe local, và WS+E2EE remote), vì cả hai đều nằm trong nhóm "CLI ↔ Electron Main" mà CR-TRACE-000 gộp chung.

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 áp dụng |
|------------|-------|-----------|----------------------------|
| CLI command handlers (`src/cli/handlers/worktree.ts`, `orchestration.ts`, `automations.ts`) | Client | — (gọi `client.call()`) | — |
| `RuntimeClient.call()` (`src/cli/runtime/client.ts:50`) | Client transport router | chọn Unix socket hoặc WS remote | — |
| `sendRequest()` / `UnixSocketTransport` (`src/cli/runtime/transport.ts`, `src/main/runtime/rpc/unix-socket-transport.ts`) | Local IPC | Unix domain socket / named pipe, newline-delimited JSON | Áp dụng hàng "HTTP/WS :6768 (CLI ↔ Electron Main)" — cùng nhóm CLI↔Main, khác cơ chế truyền tải vật lý |
| `sendWebSocketRequest()` / `sendRemoteRuntimeRequest()` (`src/cli/runtime/websocket-transport.ts`, `src/shared/remote-runtime-client.ts`) | Remote pairing | WebSocket + E2EE box (payload mã hoá trước khi gửi) | Áp dụng như hàng "WebSocket + TweetNaCl box (Mobile ↔ Main)" — `traceId` phải nằm **trong** payload JSON trước khi encrypt, không đặt ngoài envelope |
| `OrcaRuntimeRpcServer.handleMessage()` (`src/main/runtime/runtime-rpc.ts:922`) | Main process dispatcher | in-process | — |
| `WORKTREE_METHODS['worktree.create']` (`src/main/runtime/rpc/methods/worktree.ts:71`) | Business logic | — | — |
| `orchestration.run` / `orchestration.runStop` (`src/main/runtime/rpc/methods/orchestration-gates.ts:44,85`), `orchestration.dispatch` / `orchestration.send` (`src/main/runtime/rpc/methods/orchestration.ts:475,207`) | Business logic | — | — |
| `automation.runNow` (`src/main/runtime/rpc/methods/automations.ts:179`) → `AutomationService` (`src/main/automations/service.ts:32`) | Business logic | — | — |
| `relay.call('git.worktree.add' \| 'agent.spawn' \| ...)` → Dev Server | Remote execution | `relay.call()` | Hàng "`relay.call()` (Orca Server ↔ Dev Server)" — resume bằng `traceId` nhận từ RPC method layer |
| `serveOrcaApp()` (`src/cli/runtime/launch.ts:66`) | Process spawn | `child_process.spawn` (stdio inherit) | Không lan truyền `traceId` — process con độc lập, tự có chu kỳ trace riêng khi client kết nối vào nó |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  cliWorktreeCreateFlow: createTracer('cli:worktreeCreate'), // BL-CLI-01
  cliAgentStartFlow:     createTracer('cli:agentStart'),     // BL-CLI-02
  cliDaemonCommandFlow:  createTracer('cli:daemonCommand'),  // BL-CLI-03
}
```

## 4. Instrumentation theo từng sub-flow

### BL-CLI-01 — Tạo Worktree qua CLI

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| CLI gửi request | `start` | `{ repo, name, remote: boolean }` | `src/cli/handlers/worktree.ts` — `'worktree create'` handler (dòng 198) |
| Transport kết nối | `step('transport-connect')` | `{ kind: 'unix' \| 'named-pipe' \| 'ws-remote' }` | `src/cli/runtime/client.ts:50` — `RuntimeClient.call()` |
| RPC gửi đi, đính `traceId` | `step('rpc-send')` | `{ method: 'worktree.create', traceId: span.id }` | `src/cli/runtime/transport.ts:174` (frame `{ id, authToken, method, params }`) hoặc `websocket-transport.ts` |
| Server nhận & xử lý | *(resumed bởi tracer `worktree:create` riêng — ngoài phạm vi CR này, xem CR-TRACE-001)* | `traceId` đọc từ `params.traceId` | `src/main/runtime/rpc/methods/worktree.ts:71` |
| CLI nhận kết quả | `ok` / `fail` | `{ worktreeId, path }` / `err` | `src/cli/handlers/worktree.ts` sau `client.call(...)` |

```typescript
// src/cli/handlers/worktree.ts — 'worktree create'
'worktree create': async ({ flags, client, cwd, json }) => {
  const span = Tracers.cliWorktreeCreateFlow.start({ remote: client.isRemote })
  span.step('transport-connect', { kind: client.isRemote ? 'ws-remote' : 'unix' })
  try {
    const result = await client.call<RuntimeWorktreeCreateResult>('worktree.create', {
      repo: await getCreateRepoSelector(flags, cwdParentWorktree, client),
      name: getRequiredStringFlag(flags, 'name'),
      // ...existing params...
      traceId: span.id // sibling field per CR-TRACE-000 §3.2/§3.3
    })
    span.ok({ worktreeId: result.result.worktree.id })
    printResult(result, json, formatWorktreeShow)
  } catch (err) {
    span.fail(err)
    throw err
  }
}
```

### BL-CLI-02 — Quản lý Agent qua CLI

Lưu ý: flow doc dùng tên minh hoạ `agent.start`/`agent.stop`, nhưng RPC method thật cho vòng đời agent qua CLI là `orchestration.run` / `orchestration.runStop` / `orchestration.dispatch` / `orchestration.send` (xác nhận qua `src/cli/handlers/orchestration.ts`). Một span `cli:agentStart` bao trùm một lệnh CLI đơn (ví dụ `orca orchestration run`), không phân biệt theo tên method cụ thể.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| CLI gửi lệnh | `start` | `{ command: 'orchestration.run' \| 'orchestration.dispatch' \| ..., remote }` | `src/cli/handlers/orchestration.ts` (dòng 725 `'orchestration run'`, dòng 475/207 dispatch/send) |
| Gửi RPC kèm traceId | `step('rpc-send')` | `{ method, traceId: span.id }` | như trên |
| Local spawn hoặc remote relay | `step('spawn-target')` | `{ target: 'local-pty' \| 'remote-relay' }` | Server-side, ngoài phạm vi CR này (agent-orchestration.md / CR-TRACE-002) |
| CLI nhận kết quả cuối / lỗi | `ok` / `fail` | `{ runId, status }` / `err` | `src/cli/handlers/orchestration.ts` |

```typescript
'orchestration run': async ({ flags, client, cwd, json }) => {
  const from = await resolveCoordinatorTerminalHandle(flags, cwd, client)
  const span = Tracers.cliAgentStartFlow.start({ command: 'orchestration.run', remote: client.isRemote })
  try {
    const result = await client.call<{ runId: string; status: string }>('orchestration.run', {
      spec: getRequiredStringFlag(flags, 'spec'),
      from,
      worktree: getOptionalStringFlag(flags, 'worktree'),
      traceId: span.id
    })
    span.ok({ runId: result.result.runId, status: result.result.status })
    printResult(result, json, (r) => `Run ${r.runId} started (${r.status})`)
  } catch (err) {
    span.fail(err)
    throw err
  }
}
```

### BL-CLI-03 — Chạy Orca Headless Mode

Ghi chú kiến trúc: "headless daemon" = `orca serve` khởi động lại chính Orca app dưới dạng process con (`serveOrcaApp()`, `src/cli/runtime/launch.ts:66-110`, gọi `spawnProcess(executable, [...appArgs, '--serve', ...])`). Đây **không phải** một process CLI gọi RPC vào — nó *là* runtime, nên không có traceId "vào" để resume; span chỉ bao bọc việc launch tiến trình con từ CLI. Các lệnh vận hành sau đó (`orca status`, `orca automations run`) là các `client.call()` RPC bình thường như BL-CLI-01/02.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| CLI spawn headless process | `start` → `step('spawn')` → `ok`/`fail` | `{ port, pairingAddress, mobilePairing }` | `src/cli/runtime/launch.ts:66` — `serveOrcaApp()` |
| `orca automations run` gửi lệnh | `start` | `{ automationId, remote }` | `src/cli/handlers/automations.ts:494` — `'automations run'` |
| RPC tới `automation.runNow` | `step('rpc-send')` | `{ method: 'automation.runNow', traceId: span.id }` | `src/main/runtime/rpc/methods/automations.ts:179` |
| `AutomationService` chạy precheck + dispatch | *(resumed bởi tracer riêng của automation.md — xem CR-TRACE-008)* | `traceId` từ `params.traceId` | `src/main/automations/service.ts:32` |
| CLI nhận kết quả / exit code | `ok` / `fail` | `{ runStatus, exitCode }` | `src/cli/handlers/automations.ts` |

```typescript
export function serveOrcaApp(args: { port?: string | null; ... } = {}): Promise<number> {
  const span = Tracers.cliDaemonCommandFlow.start({ port: args.port ?? undefined })
  const executable = resolveForegroundOrcaExecutable()
  span.step('spawn', { executable })
  const child = spawnProcess(executable, childArgs, { /* ...existing... */ })
  // ok()/fail() gắn vào child 'exit' listener hiện có (không phải trong scope hiện tại của hàm)
  return exitCode
}
```

## 5. Lan truyền traceId qua transport của flow này

1. **Local (Unix socket / named pipe):** `RuntimeClient.call()` tạo/nhận `traceId` từ `Tracers.cli*Flow.start().id`, đưa vào `params.traceId` — request frame ở `sendRequest()` (`src/cli/runtime/transport.ts:174`) đã có shape `{ id, authToken, method, params }`; `traceId` đi trong `params`, không cần đổi frame top-level. Phía server, `OrcaRuntimeRpcServer.handleMessage()` (`runtime-rpc.ts:922`) forward `params` nguyên vẹn tới method handler, method handler đọc `params.traceId` và dùng `resume: params.traceId ? { id: params.traceId } : undefined` khi `.start()` tracer riêng của domain đó (vd `worktree:create`).
2. **Remote pairing (WS + E2EE):** `traceId` phải nằm **trong** payload JSON trước khi `encrypt()` (`e2ee-crypto.ts`) — đặt cùng cấp `method`/`params` giống local, vì toàn bộ payload post-encrypt là ciphertext opaque (đúng nguyên tắc CR-TRACE-000 §3.3 hàng Mobile).
3. **Response echo:** `RuntimeRpcEnvelopeSchema` (`src/shared/runtime-rpc-envelope.ts`) hiện dùng `.strip()` trên `_meta`, nghĩa là nếu server thêm `traceId` vào `_meta` mà không cập nhật `MetaSuccess`/`MetaFailure` schema, Zod sẽ **âm thầm loại bỏ** field đó ở phía CLI. Đây là điểm cần sửa cùng lúc: thêm `traceId: z.string().optional()` vào cả `MetaSuccess` và `MetaFailure` trước khi triển khai bất kỳ code echo traceId nào.
4. **relay.call() xuống Dev Server** (khi worktree/agent target là remote): method handler forward `traceId: span.id` (của tracer domain, đã resume từ CLI) xuống `relay.call(method, { ...params, traceId })`, theo đúng CR-TRACE-000 §3.3 hàng `relay.call()`.
5. **`orca serve` process con:** không nhận `traceId` từ CLI gọi launch nó — mỗi request tới runtime đó sau khi serve xong lại đi qua đường WS remote pairing ở mục 2.

## Acceptance Criteria

- [ ] `Tracers.cliWorktreeCreateFlow`, `cliAgentStartFlow`, `cliDaemonCommandFlow` được thêm vào `tracers.ts` theo đúng tên ở mục 3
- [ ] `MetaSuccess`/`MetaFailure` trong `runtime-rpc-envelope.ts` có field `traceId` optional (không bị `.strip()` loại bỏ)
- [ ] `'worktree create'` CLI handler gửi `traceId` trong params và log `ok`/`fail` khớp với response thật
- [ ] `'orchestration run'`, `'orchestration dispatch'`, `'orchestration run-stop'` đều bọc trong `cli:agentStart` span với field `command` phân biệt method
- [ ] `serveOrcaApp()` có span bao quanh việc spawn process con, fail nếu spawn lỗi trước khi `child.once('error')`
- [ ] `'automations run'` CLI handler gửi `traceId`, span `ok` chứa `runStatus`/exit code thật
- [ ] Test thủ công: set `ORCA_TRACE=1`, chạy `orca worktree create` trỏ tới Orca chưa khởi động — xác nhận span `fail` xuất hiện ở bước `transport-connect`, không phải một lỗi chung chung
- [ ] Test thủ công: chạy CLI qua `--pairing-code` (remote) và xác nhận `traceId` vẫn xuất hiện trong log dù payload được mã hoá

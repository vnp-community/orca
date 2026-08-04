# CR-TRACE-004 — Remote Development (SSH) Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-004 |
| **Tên** | Remote Development (SSH) — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/remote-development.md`, `src/main/ssh/ssh-connection.ts`, `src/main/ssh/ssh-connection-manager.ts`, `src/main/ssh/ssh-relay-session.ts`, `src/main/ssh/ssh-relay-deploy.ts`, `src/main/ssh/ssh-port-forward.ts`, `src/main/ssh/ssh-port-scanner.ts`, `src/main/runtime/rpc/methods/ssh.ts`, `src/main/ipc/ssh.ts` |

---

## 1. Vấn đề

Luồng Remote Development (BL-SSH-01→04) có nhiều bước có khả năng chậm/fail độc lập — TCP handshake, SSH auth, SFTP upload binary, exec verify, tunnel setup — nhưng **không có tracer nào** hiện nay. Khi user báo "connect SSH bị treo" hoặc "deploy relay fail", hiện tại chỉ có `console.log`/`log.info` rải rác (vd. `agent-spawner.ts:479`) chứ không có timeline thống nhất theo từng bước.

Cụ thể, các "điểm mù" khi troubleshoot hôm nay:

- **BL-SSH-01 (Kết nối SSH)**: `SshConnection.attemptConnect()` (`src/main/ssh/ssh-connection.ts:515`) chạy retry loop nội bộ (`INITIAL_RETRY_ATTEMPTS`, dòng 487) — không biết attempt nào fail vì lý do gì (auth reject vs TCP timeout vs host key mismatch) nếu không đọc log thô.
- **BL-SSH-02 (Deploy Relay)**: `deployAndLaunchRelay()` (`src/main/ssh/ssh-relay-deploy.ts:104`) là một chuỗi dài: SFTP upload → chmod → verify version → native deps check/repair → launch → connect. Nếu quá trình treo, không biết đang kẹt ở sub-bước nào (upload chậm do mạng? native deps đang compile? relay process không bind port?).
- **BL-SSH-03 (Auto-Reconnect)**: `SshConnection.scheduleReconnect()` (dòng 1055) và `SshConnectionManager.reconnect()` (`ssh-connection-manager.ts:70`) chạy backoff nhưng không log rõ attempt number / elapsed backoff — khó phân biệt "đang retry" với "đã bỏ cuộc".
- **BL-SSH-04 (Port Forward)**: `PortScanner` (`ssh-port-scanner.ts:37`) polling định kỳ (`SSH_PORT_SCAN_BASE_INTERVAL_MS`) phát hiện port mới rồi gọi `SshPortForwardManager.addForward()` (`ssh-port-forward.ts:39`) — không có cách nào biết độ trễ từ "port mở trên remote" đến "forward khả dụng ở local".

Theo CR-TRACE-000 §3.3, đây là flow đặc biệt: **chỉ trace phía Main process** (không lan truyền `traceId` vào remote shell/relay binary vì đó không phải code Orca chạy trong process được instrument).

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Renderer (React UI) | UI | IPC (`contextBridge.invoke`) | WebSocket RPC / IPC row — Renderer không tự tạo `traceId`, span bắt đầu ở Main |
| Main Process — `SshConnectionManager`, `SshConnection` | Business Logic | — | Span gốc, không có layer trước để resume |
| Main Process — `SshRelaySession`, `deployAndLaunchRelay()` | Business Logic | SFTP + SSH exec | Không băng qua transport có `traceId` — vẫn là in-process Main, chỉ resume nếu gọi từ RPC layer có `traceId` |
| ssh2 Library | Transport | TCP/SSH | Không nhận `traceId` (thư viện ngoài) |
| Orca Relay Binary (remote) | Remote | SSH exec / WS `:6799` qua tunnel | **Không lan truyền** — theo CR-TRACE-000 §3.3 hàng "SSH exec / SshChannelMultiplexer": remote shell không chạy code Orca nên không nhận span. Sau khi relay WS kết nối, các RPC tiếp theo qua relay (`relay.call()`) mới dùng lại quy ước `relay:agentCall` đã có sẵn — nằm ngoài phạm vi CR này |
| Remote Host | Infrastructure | — | Không có instrumentation (ngoài boundary Orca) |
| SQLite Database | Persistence | in-process | Không cần `step()` riêng theo mục 5 CR-TRACE-000 (single-row INSERT/UPDATE) — gộp field vào `ok()` |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  sshConnectFlow:     createTracer('ssh:connect'),
  sshDeployRelayFlow: createTracer('ssh:deployRelay'),
  sshReconnectFlow:   createTracer('ssh:reconnect'),
  sshPortForwardFlow: createTracer('ssh:portForward'),
}
```

## 4. Instrumentation theo từng sub-flow

### BL-SSH-01 — Kết nối SSH Host

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu connect | `start` | `targetId`, `authMethod` | `src/main/runtime/rpc/methods/ssh.ts:29` (`ssh.connect` handler) |
| Mỗi lần thử kết nối | `step('attemptConnect')` | `attempt`, `phase: 'tcp'\|'auth'\|'verify'` | `src/main/ssh/ssh-connection.ts:515` (`attemptConnect()`) |
| Test connection (`echo orca-ok`) | `step('verify-exec')` | `targetId` | `src/main/ssh/ssh-connection.ts` (trong `attemptConnect()`, sau khi handshake xong — chưa xác định số dòng chính xác của lệnh verify exec) |
| Thành công | `ok` | `targetId`, `retries` | `src/main/ssh/ssh-connection-manager.ts:28` (`connect()`) |
| Thất bại (hết `INITIAL_RETRY_ATTEMPTS`) | `fail` | `targetId`, `lastAttempt` | `src/main/ssh/ssh-connection.ts:487-515` |

```typescript
// src/main/runtime/rpc/methods/ssh.ts — ssh.connect handler
handler: async (params, ctx) => {
  const span = Tracers.sshConnectFlow.start({ targetId: params.targetId, authMethod: params.authMethod })
  try {
    const conn = await ctx.sshConnectionManager.connect(target)
    span.ok({ targetId: params.targetId })
    return { connectionId: conn.id }
  } catch (err) {
    span.fail(err, { targetId: params.targetId })
    throw err
  }
}
```

```typescript
// src/main/ssh/ssh-connection.ts — attemptConnect() retry loop (dòng ~487)
for (let attempt = 0; attempt < INITIAL_RETRY_ATTEMPTS; attempt++) {
  try {
    span.step('attemptConnect', { attempt, phase: 'tcp' })
    await this.attemptConnect()
    break
  } catch (err) {
    if (attempt < INITIAL_RETRY_ATTEMPTS - 1) continue
    throw err
  }
}
```

### BL-SSH-02 — Deploy Orca Relay Binary

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu deploy | `start` | `targetId` | `src/main/ssh/ssh-relay-session.ts:326` (`SshRelaySession.establish()`) |
| SFTP upload binary | `step('upload')` | `localPath`, `remotePath` | `src/main/ssh/ssh-relay-deploy.ts:350` (`uploadRelay()`) |
| Kiểm tra/cài native deps | `step('nativeDeps')` | `needsInstall` | `src/main/ssh/ssh-relay-deploy.ts:441` (`hasRequiredNativeDeps()`) / `:505` (`installNativeDeps()`) |
| Launch relay process | `step('launch')` | `wsPort` | `src/main/ssh/ssh-relay-deploy.ts:660` (`launchRelay()`) |
| Kết nối WS tới relay | `step('wsConnect')` | `tunnelPort` | `src/main/ssh/ssh-relay-deploy.ts:104` (`deployAndLaunchRelay()`, đoạn kết nối cuối) |
| Hoàn tất | `ok` | `targetId`, `wsPort` | `src/main/ssh/ssh-relay-session.ts:326` (`establish()`, sau khi `deployAndLaunchRelay()` resolve) |
| Lỗi ở bất kỳ sub-bước | `fail` | `targetId`, `stage` | `src/main/ssh/ssh-relay-deploy.ts:240` (`deployAndLaunchRelayInner()`) |

```typescript
// src/main/ssh/ssh-relay-session.ts — establish()
async establish(conn: SshConnection, graceTimeSeconds?: number): Promise<void> {
  const span = Tracers.sshDeployRelayFlow.start({ targetId: this.targetId })
  try {
    await deployAndLaunchRelay(conn, undefined, graceTimeSeconds, this.targetId)
    span.ok({ targetId: this.targetId })
  } catch (err) {
    span.fail(err, { targetId: this.targetId })
    throw err
  }
}
```

> Ghi chú: `deployAndLaunchRelay()` nhận nhiều tham số optional (grace time, targetId) — CR triển khai cần truyền `span` (hoặc chỉ `span.step`/`span.id`) xuống các hàm con `uploadRelay()`/`installNativeDeps()`/`launchRelay()` bằng cách thêm optional callback param, tương tự cách `browseDirFlow` truyền `span.step` trực tiếp trong `dev-server.ts`. Chi tiết signature thay đổi cần review khi triển khai.

### BL-SSH-03 — SSH Auto-Reconnect

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Phát hiện disconnect | `start` (resume nếu có `sshConnectFlow` id trước đó — xem ghi chú) | `targetId`, `reason` | `src/main/ssh/ssh-connection.ts:1041,1051` (nơi gọi `scheduleReconnect()`) |
| Mỗi lần backoff | `step('backoffWait')` | `attempt`, `delayMs` | `src/main/ssh/ssh-connection.ts:1055` (`scheduleReconnect()`) |
| Thử reconnect | `step('reconnectAttempt')` | `attempt` | `src/main/ssh/ssh-connection.ts:668` (`reconnect()`) |
| Re-establish relay session | `step('relayResume')` | `targetId` | `src/main/ssh/ssh-relay-session.ts:438` (`reconnect()`) |
| Thành công | `ok` | `targetId`, `totalAttempts` | `src/main/ssh/ssh-connection-manager.ts:70` (`reconnect()`) |
| Fail sau max attempts | `fail` | `targetId`, `totalAttempts` | `src/main/ssh/ssh-connection.ts` (nhánh `ssh:unreachable`, chưa xác định số dòng chính xác) |

```typescript
// src/main/ssh/ssh-connection.ts — scheduleReconnect()
private scheduleReconnect(): void {
  if (this.disposed || this.reconnectTimer) return
  this.state.reconnectAttempt++
  const span = this.reconnectSpan ??= Tracers.sshReconnectFlow.start({ targetId: this.targetId })
  span.step('backoffWait', { attempt: this.state.reconnectAttempt, delayMs: this.currentBackoffMs })
  this.reconnectTimer = setTimeout(() => this.reconnect(), this.currentBackoffMs)
}
```

Ghi chú: vì đây là quá trình nhiều lần retry kéo dài (có thể tới 60s backoff x 5 lần), span nên được giữ ở instance field (`this.reconnectSpan`) thay vì tạo mới mỗi lần `scheduleReconnect()` được gọi, và chỉ `ok()`/`fail()` một lần khi vòng lặp kết thúc — tránh tạo nhiều span rời rạc cho cùng một sự cố mất kết nối.

### BL-SSH-04 — Auto Port Forwarding

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Phát hiện port mới | `start` | `targetId`, `remotePort` | `src/main/ssh/ssh-port-scanner.ts:37` (`PortScanner`, nơi phát ra sự kiện port mới — chưa xác định tên method chính xác) |
| Tạo SSH local forward | `step('addForward')` | `remotePort`, `localPort` | `src/main/ssh/ssh-port-forward.ts:39` (`SshPortForwardManager.addForward()`) |
| Hoàn tất | `ok` | `localUrl`, `remotePort` | `src/main/ssh/ssh-port-forward.ts:39` (`addForward()`, sau khi resolve) |
| Lỗi tạo forward | `fail` | `remotePort` | `src/main/ssh/ssh-port-forward.ts:39` |

```typescript
// src/main/ssh/ssh-port-forward.ts — addForward()
async addForward(targetId: string, remotePort: number): Promise<PortForwardEntry> {
  const span = Tracers.sshPortForwardFlow.start({ targetId, remotePort })
  try {
    const entry = await this.createForward(targetId, remotePort)
    span.ok({ localUrl: `http://localhost:${entry.localPort}`, remotePort })
    return entry
  } catch (err) {
    span.fail(err, { remotePort })
    throw err
  }
}
```

## 5. Lan truyền traceId qua transport của flow này

Theo CR-TRACE-000 §3.3, hàng "SSH exec / `SshChannelMultiplexer`": **không lan truyền `traceId` vào remote shell process**. Áp dụng cụ thể cho flow này:

- **Renderer → Main (IPC/RPC)**: `ssh.connect` (`src/main/runtime/rpc/methods/ssh.ts:29`) và `ipcMain.handle('ssh:connect', ...)` (`src/main/ipc/ssh.ts:1037`) là điểm bắt đầu span — nếu Renderer trong tương lai có tracer riêng và gửi `traceId`, các handler này đọc `params.traceId` và `resume` như quy ước WS RPC (`Tracers.sshConnectFlow.start(fields, params.traceId ? { id: params.traceId } : undefined)`). Hiện tại Renderer chưa có tracer cho hành động này nên `resume` sẽ luôn `undefined` — đây là việc của CR-TRACE-000 Phase 1 nói chung, không phải riêng CR này.
- **Main → ssh2 (TCP/SSH)**: không có `traceId` — thư viện `ssh2` không phải code Orca.
- **Main → SFTP upload → Remote filesystem**: không có `traceId` — SFTP write không mang metadata ứng dụng.
- **Main → SSH exec (`chmod`, `--version`, `start`)**: không có `traceId` — đúng theo ghi chú CR-TRACE-000 "remote shell không chạy code Orca nên không thể nhận span".
- **Main → SSH tunnel → Relay WS `:6799`**: sau khi tunnel dựng xong và relay WS kết nối, các lệnh tiếp theo đi qua `relay.call()` — tại điểm đó áp dụng lại quy ước `relay:agentCall` đã tồn tại (`dev-server-relay-bridge.ts`), nằm ngoài phạm vi 4 tracer mới của CR này. CR này chỉ chịu trách nhiệm span cho phần orchestration ở Main trước khi relay sẵn sàng.

## Acceptance Criteria

- [ ] `Tracers.sshConnectFlow` bọc toàn bộ `SshConnectionManager.connect()`, log được số lần retry thực tế trước khi `ok`/`fail`
- [ ] `Tracers.sshDeployRelayFlow` có `step()` riêng cho `upload`, `nativeDeps`, `launch`, `wsConnect` — đủ để xác định deploy relay đang kẹt ở sub-bước nào
- [ ] `Tracers.sshReconnectFlow` gộp toàn bộ vòng backoff (nhiều `step('backoffWait')`) vào MỘT span duy nhất cho mỗi sự cố mất kết nối, không tạo span mới mỗi lần retry
- [ ] `Tracers.sshPortForwardFlow` đo được độ trễ từ "port phát hiện trên remote" đến "forward khả dụng ở local" qua `elapsedMs` của `ok()`
- [ ] Không có `traceId` nào bị gửi vào SSH exec command hoặc SFTP payload (đúng theo CR-TRACE-000 §3.3 — remote shell không nhận span)
- [ ] `ORCA_TRACE=1` bật lên cho thấy đầy đủ 4 tracer mới trong console log khi thực hiện end-to-end: connect → deploy → (giả lập drop) reconnect → mở port trên remote
- [ ] Không có tracer nào trong CR này trùng tên với `devServer:*`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc` (theo GAP-3 của CR-TRACE-000)

# CR-TRACE-012 — Fleet Management Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-012 |
| **Tên** | Fleet Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/fleet.md`, `src/shared/fleet-config-parser.ts`, `src/main/ssh/fleet-health-monitor.ts`, `src/main/ssh/fleet-status-service.ts`, `src/main/ssh/fleet-bootstrap-service.ts`, `src/main/ssh/fleet-remote-commands.ts`, `src/main/ssh/fleet-health-store.ts`, `src/main/ssh/dev-server-provisioner.ts`, `src/main/ssh/ssh-relay-deploy.ts`, `src/main/runtime/rpc/methods/ssh.ts`, `src/main/runtime/rpc/fleet-metrics-handler.ts`, `src/cli/handlers/fleet.ts` |

---

## 1. Vấn đề

Kiến trúc thật khác đáng kể so với mô tả trong `fleet.md`: **không tồn tại class `FleetManager`/`FleetProvisioner`** trong codebase (grep xác nhận không có `class FleetManager`, không có `class FleetProvisioner`). Thay vào đó, logic được chia thành các module hàm độc lập, cố ý tách khỏi `orca-runtime.ts` (26k dòng) theo ghi chú trong chính source:

- `src/main/ssh/fleet-health-monitor.ts` — `class FleetHealthMonitor` (khớp flow doc)
- `src/main/ssh/fleet-status-service.ts` — hàm `getFleetStatus()`, không phải class
- `src/main/ssh/fleet-bootstrap-service.ts` — hàm `bootstrapServer()`, thực hiện cài Node.js/Git/clone repo/chạy setup script — **khác** với mô tả BL-FLEET-02 trong flow doc (upload relay binary qua SFTP + systemd). Việc deploy relay binary thật sự nằm ở `src/main/ssh/ssh-relay-deploy.ts` (`deployAndLaunchRelay()`), dùng chung cho mọi kết nối SSH (không riêng cho "Fleet provisioning").
- `src/main/ssh/dev-server-provisioner.ts` — `class DevServerProvisioner`, tạo unix account per-user trên dev server — một nghiệp vụ *không được nhắc tới* trong `fleet.md` nhưng có liên quan tới BL-FLEET-04 (onboarding).

Vì kiến trúc thật phân mảnh hơn flow doc mô tả, việc thiếu tracing càng nghiêm trọng hơn: một lần "Provision All" (BL-FLEET-02) hoặc "Add Server wizard" (BL-FLEET-04) đi qua ít nhất 4 module riêng biệt (`fleet-bootstrap-service`, `fleet-remote-commands`, `ssh-relay-deploy`, `dev-server-provisioner`) với SSH exec tuần tự — khi một dev server "kẹt" ở giữa quá trình provision, không có cách nào biết đang ở bước cài Node.js, clone repo, deploy relay, hay tạo unix account. Tương tự, `FleetHealthMonitor.runHealthCheck()` (`fleet-health-monitor.ts:52`) chạy cron mỗi 60s (không phải 30s như flow doc ghi — xác nhận `DEFAULT_PING_INTERVAL_MS = 60_000` tại dòng 8) qua nhiều target — nếu một target bị treo, vòng lặp `for` tuần tự có thể làm chậm toàn bộ chu kỳ health check cho các target khác mà không lộ ra dấu hiệu nào ngoài "health dashboard cập nhật chậm".

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 áp dụng |
|------------|-------|-----------|----------------------------|
| Admin Browser (Fleet panel, Health dashboard, Wizard) | UI | HTTP/WS | — |
| `ssh.ts` RPC methods: `ssh.connect`, `ssh.filterTargets`, `ssh.getAllConnectionStates`, `ssh.bootstrapServer`, `ssh.getFleetStatus` (`src/main/runtime/rpc/methods/ssh.ts:29,56,67,73,108`) | Backend RPC | WebSocket RPC (Browser ↔ Orca Server) | Hàng "WebSocket RPC (Browser ↔ Orca Server)" |
| `FleetHealthMonitor` (`src/main/ssh/fleet-health-monitor.ts:18`) | Business logic (cron) | in-process, gọi SSH connection state | — |
| `getFleetStatus()` (`src/main/ssh/fleet-status-service.ts:21`) | Business logic | in-process, đọc `SshConnectionStore`/`SshConnectionManager`/`fleetHealthStore` | — |
| `bootstrapServer()` (`src/main/ssh/fleet-bootstrap-service.ts:50`) + hàm trong `fleet-remote-commands.ts` (`installNodeJs`, `ensureGitInstalled`, `cloneOrUpdateRepo`, `installPackages`, `runRemoteScript`) | Business logic → Remote exec | SSH exec (`ssh2` qua `SshConnection`) | Không có hàng riêng trong CR-TRACE-000 cho "SSH exec do Main chủ động chạy nhiều lệnh tuần tự" — áp dụng như hàng SSH exec: chỉ trace các bước phía Main, không lan truyền vào remote shell |
| `deployAndLaunchRelay()` (`src/main/ssh/ssh-relay-deploy.ts:104`), dùng `uploadDirectory()` (`ssh-relay-deploy-helpers.ts`) | Remote deploy | SFTP (qua SSH) | Tương tự trên |
| `DevServerProvisioner` (`src/main/ssh/dev-server-provisioner.ts:14`) | Remote provisioning | SSH exec | Tương tự trên |
| `parseFleetConfig()` (`src/shared/fleet-config-parser.ts`) | Config parsing | in-process, đọc file YAML | — |
| `createFleetMetricsHandler()` (`src/main/runtime/rpc/fleet-metrics-handler.ts:21`) | Backend | HTTP GET `/metrics` (Prometheus) | Không băng qua traceId — endpoint pull-based, không có request context của user |
| `fleetHealthStore` (`src/main/ssh/fleet-health-store.ts`) | Persistence | in-process | — |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  fleetInventoryLoadFlow: createTracer('fleet:inventoryLoad'), // BL-FLEET-01
  fleetProvisionFlow:     createTracer('fleet:provision'),     // BL-FLEET-02
  fleetHealthCheckFlow:   createTracer('fleet:healthCheck'),   // BL-FLEET-03
  fleetOnboardFlow:       createTracer('fleet:onboard'),       // BL-FLEET-04
}
```

## 4. Instrumentation theo từng sub-flow

### BL-FLEET-01 — Fleet Inventory Config (YAML)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Parse YAML | `start` | `{ source: 'yaml-file' \| 'admin-ui' }` | `src/shared/fleet-config-parser.ts` — `parseFleetConfig()` (gọi từ `fleet-bootstrap-service.ts:65`) |
| Validate schema fail | `fail` | `{ reason: 'schema_invalid' }` | như trên |
| Upsert DB | (gộp vào `ok`, single UPSERT — theo nguyên tắc CR-TRACE-000 §5) | `{ serverCount }` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai (không tìm thấy hàm UPSERT `orca_dev_servers` riêng trong `fleet-*.ts`; có thể nằm trong `SshConnectionStore`, xác nhận khi triển khai) |
| Test connection (Admin UI path) | `step('test-connect')` | `{ targetId }` | `src/main/runtime/rpc/methods/ssh.ts:29` — `ssh.connect` |

```typescript
// src/main/ssh/fleet-bootstrap-service.ts (điểm gọi parseFleetConfig)
if (options.fleetConfigPath) {
  const span = Tracers.fleetInventoryLoadFlow.start({ source: 'yaml-file' })
  try {
    fleetConfig = await parseFleetConfig(options.fleetConfigPath)
    span.ok({ serverCount: fleetConfig.servers.length })
  } catch (err) {
    span.fail(err, { path: options.fleetConfigPath })
    throw err
  }
}
```

### BL-FLEET-02 — Bulk Server Provisioning

Lưu ý: real code KHÔNG làm đúng như flow doc mô tả (SFTP relay binary + systemd trực tiếp trong một `FleetProvisioner.provision()`). Thực tế là chuỗi các bước độc lập trong `bootstrapServer()`, mỗi bước có thể fail riêng.

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu bootstrap 1 server | `start` | `{ targetId }` | `src/main/ssh/fleet-bootstrap-service.ts:50` — `bootstrapServer()` |
| Cài Node.js | `step('install-node')` | `{ targetId, skipped: boolean }` | `src/main/ssh/fleet-remote-commands.ts:83` — `installNodeJs()` |
| Cài Git | `step('install-git')` | `{ targetId, skipped: boolean }` | `fleet-remote-commands.ts:127` — `ensureGitInstalled()` |
| Clone/update repo | `step('clone-repo')` | `{ targetId, action: 'cloned' \| 'updated' }` | `fleet-remote-commands.ts:167` — `cloneOrUpdateRepo()` |
| Cài package + chạy setup script | `step('setup-script')` | `{ targetId, skipped: boolean }` | `fleet-remote-commands.ts:206,245` — `installPackages()`, `runRemoteScript()` |
| Deploy + khởi động relay | `step('deploy-relay')` | `{ targetId }` | `src/main/ssh/ssh-relay-deploy.ts:104` — `deployAndLaunchRelay()` |
| Kết quả | `ok` / `fail` | `{ targetId, stepsCompleted }` / `{ targetId, failedAt }` | `fleet-bootstrap-service.ts:50` (cuối `bootstrapServer()`) |

```typescript
export async function bootstrapServer(
  targetId: string,
  options: BootstrapOptions = {}
): Promise<BootstrapResult> {
  const span = Tracers.fleetProvisionFlow.start({ targetId })
  const report: BootstrapStep[] = []
  const notify = (step: BootstrapStep): void => {
    report.push(step)
    span.step(step.name, { targetId })
    options.onProgress?.(step)
  }
  try {
    // ...existing installNodeJs / ensureGitInstalled / cloneOrUpdateRepo / installPackages...
    span.ok({ targetId, stepsCompleted: report.length })
    return { success: true, steps: report }
  } catch (err) {
    span.fail(err, { targetId, failedAt: report.at(-1)?.name })
    throw err
  }
}
```

`ssh.bootstrapServer` RPC method (`src/main/runtime/rpc/methods/ssh.ts:73`) gọi song song nhiều `bootstrapServer()` — vì `fleet:provision` là 1 span/1 server, N server chạy song song sẽ tạo N span độc lập với N `traceId` khác nhau (không có "master span" cho cả batch trong scope P2 này; nếu cần theo dõi cả batch, cân nhắc bổ sung ở bản sau bằng cách CLI/Admin UI tự tạo một `traceId` batch và forward vào từng `bootstrapServer()` qua `resume`).

### BL-FLEET-03 — Fleet Health Monitoring

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu 1 chu kỳ cron | `start` | `{ targetCount }` | `src/main/ssh/fleet-health-monitor.ts:52` — `runHealthCheck()` |
| Ping 1 target | `step('ping')` | `{ targetId, status }` | vòng `for` trong `runHealthCheck()`, dòng 56-85 |
| Ghi health snapshot | (gộp vào step `ping`, single INSERT — nguyên tắc CR-TRACE-000 §5) | `{ targetId }` | `fleetHealthStore.recordConnectionState()` (`fleet-health-store.ts`), gọi tại dòng 61 |
| Trạng thái đổi → alert | `step('alert-transition')` | `{ targetId, oldStatus, newStatus }` | `runHealthCheck()` dòng 73-81, `emitAlert()` dòng 88 |
| Webhook gửi | `step('webhook')` | `{ targetId, webhookConfigured: boolean }` | `emitAlert()` dòng 96-100 — `sendWebhookAlert()` |
| Kết thúc chu kỳ | `ok` | `{ targetCount, errorCount }` | cuối `runHealthCheck()` |

```typescript
// src/main/ssh/fleet-health-monitor.ts
async runHealthCheck(): Promise<void> {
  if (!this.getSshTargets || !this.getConnectionState) return
  const targets = await this.getSshTargets()
  const span = Tracers.fleetHealthCheckFlow.start({ targetCount: targets.length })
  let errorCount = 0
  for (const target of targets) {
    const state = this.getConnectionState(target.id)
    const status = (state?.status ?? 'disconnected')
    span.step('ping', { targetId: target.id, status })
    fleetHealthStore.recordConnectionState({ targetId: target.id, status, /* ... */ })
    if (status === 'error' || status === 'reconnection-failed' || status === 'auth-failed') {
      errorCount++
      // ...existing alert-transition logic calls this.emitAlert(...)...
    }
  }
  span.ok({ targetCount: targets.length, errorCount })
}
```

Theo nguyên tắc CR-TRACE-000 §5, **không** tạo 1 span riêng cho mỗi target (single ping/network call có thể fail độc lập nên đủ điều kiện `step()`, nhưng KHÔNG đủ để thành sub-span riêng vì đây vẫn là 1 nghiệp vụ "1 chu kỳ health check" theo BL-FLEET-03) — dùng `step('ping', ...)` lặp lại trong cùng 1 span thay vì N span.

### BL-FLEET-04 — Dev Server Onboarding Wizard

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Test connection (step 3) | `start` → `ok`/`fail` | `{ host }` | `src/main/runtime/rpc/methods/ssh.ts:29` — `ssh.connect`, hoặc trực tiếp `FleetManager.testConnection()` — **chưa xác định file cụ thể**: flow doc gọi `FleetManager.testConnection()` nhưng class này không tồn tại; hành vi tương đương thật là `ssh.connect` RPC + `detectRemotePlatform()` (`fleet-remote-commands.ts:26`) |
| Deploy relay (step 4) | *(resume `fleet:provision` từ BL-FLEET-02)* | `{ targetId, single: true }` | `ssh-relay-deploy.ts:104` |
| Tạo unix account (không có trong flow doc nhưng có trong code) | `step('provision-user-account')` | `{ userId, linuxUser }` — không log private key | `src/main/ssh/dev-server-provisioner.ts:26` — `DevServerProvisioner.ensureUserAccount()` |
| Finish, cập nhật metadata | `ok` | `{ targetId, tags, region }` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai (PATCH `/admin/api/fleet/servers/:id` handler không được tìm thấy qua grep trong phạm vi điều tra này) |

```typescript
// src/main/ssh/dev-server-provisioner.ts
async ensureUserAccount(conn: SshConnection, userId: string, userEmail: string): Promise<string> {
  const span = Tracers.fleetOnboardFlow.start({ userId }, /* resume nếu wizard đã có traceId từ step trước */ undefined)
  const linuxUser = toLinuxUsername(userEmail, userId)
  span.step('check-user-exists', { linuxUser })
  const exists = await this.checkUserExists(conn, linuxUser)
  if (!exists) {
    span.step('create-user', { linuxUser })
    await this.createUser(conn, linuxUser)
  }
  await this.authorizeKey(conn, linuxUser, this.orcaPublicKey)
  span.ok({ linuxUser })
  return linuxUser
}
```

## 5. Lan truyền traceId qua flow này

Toàn bộ Fleet Management chạy trên **Admin Browser → WebSocket RPC → Orca Web Server → SSH exec/SFTP tới Dev Server**. Theo CR-TRACE-000 §3.3:

1. **Browser → `ssh.*` RPC methods** (`src/main/runtime/rpc/methods/ssh.ts`): áp dụng hàng "WebSocket RPC (Browser ↔ Orca Server)" — Admin SPA tạo `traceId` bằng tracer riêng phía renderer trước khi gọi `ssh.bootstrapServer`/`ssh.connect`, gửi kèm `params.traceId`; RPC method đọc `params.traceId` và `resume` vào `Tracers.fleetProvisionFlow`/`fleetOnboardFlow`.
2. **SSH exec / SFTP (Main ↔ Dev Server)**: theo đúng hàng cuối bảng CR-TRACE-000 §3.3 ("SSH exec ... không lan truyền vào remote shell process — chỉ trace các bước phía Main") — mọi `step()` trong `bootstrapServer()`, `fleet-remote-commands.ts`, `ssh-relay-deploy.ts`, `dev-server-provisioner.ts` chỉ ghi nhận phía Main (thời điểm gửi lệnh, thời điểm nhận kết quả exec), KHÔNG cố gắng lan truyền `traceId` vào bên trong remote shell process — dev server chưa chạy code Orca ở giai đoạn bootstrap nên không có nơi nhận span.
3. **`FleetHealthMonitor` cron (BL-FLEET-03)**: không có "caller" nào để resume `traceId` từ — cron tự khởi tạo (`Tracers.fleetHealthCheckFlow.start()` không có `resume`), là điểm bắt đầu (root span) của chuỗi trace này, khác với các flow do user-action khởi phát.
4. **`/metrics` Prometheus endpoint**: không tham gia lan truyền `traceId` — đây là pull-based scraping của công cụ ngoài (Prometheus), không có request context của một user action cụ thể.

## Acceptance Criteria

- [ ] `Tracers.fleetInventoryLoadFlow`, `fleetProvisionFlow`, `fleetHealthCheckFlow`, `fleetOnboardFlow` thêm vào `tracers.ts`
- [ ] `bootstrapServer()` phát ra đúng 5 step (`install-node`, `install-git`, `clone-repo`, `setup-script`, `deploy-relay`) theo thứ tự thật, `fail()` chứa `failedAt` đúng bước đang chạy khi lỗi
- [ ] `FleetHealthMonitor.runHealthCheck()` có 1 span bao trọn 1 chu kỳ cron, KHÔNG tạo span riêng cho từng target (đúng nguyên tắc CR-TRACE-000 §5)
- [ ] `ssh.bootstrapServer` RPC method truyền `traceId` từ `params` vào `resume` của `fleetProvisionFlow`
- [ ] `DevServerProvisioner.ensureUserAccount()` không log private key hay nội dung `orcaPublicKey` vào bất kỳ trace field nào
- [ ] Test thủ công: giả lập một dev server không phản hồi trong `installNodeJs()` — xác nhận span `fail` chỉ đúng ở step `install-node`, không lan tới `clone-repo`/`deploy-relay`
- [ ] CR-TRACE-000 hoặc tài liệu liên quan được ghi chú: `fleet.md` mô tả `FleetManager`/`FleetProvisioner` không khớp code thật (module hàm rời rạc) — khuyến nghị cập nhật flow doc này ở một CR tài liệu riêng (ngoài phạm vi CR-TRACE-012)

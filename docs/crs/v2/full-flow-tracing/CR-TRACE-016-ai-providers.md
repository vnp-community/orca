# CR-TRACE-016 — AI Provider Management Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-016 |
| **Tên** | AI Provider Management — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P3 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/ai-providers.md`, `src/main/ai-providers/AIProviderService.ts`, `src/main/ai-providers/ProviderResolver.ts`, `src/main/ai-providers/ProviderHealthChecker.ts`, `src/main/ai-providers/ai-provider-rpc-handler.ts`, `src/main/dev-server/relay-connection-pool.ts`, `src/shared/trace/tracers.ts` |

---

## 1. Vấn đề

Ba sub-flow BL-AIP-01→03 hiện **không có bất kỳ tracer nào**. Ba điểm không quan sát được cụ thể:

1. **BL-AIP-01 (ghi credential)**: `AIProviderService.writeCredentialToDevServer()` (`AIProviderService.ts:227`) đi qua 3 bước có thể fail độc lập — lookup account, `relayPool.getOrConnect()` (có thể timeout nếu Dev Server WS chưa mở), rồi `relay.call('ai.provider.writeCredential', ...)` (Dev Server tự AES-256-GCM encrypt + ghi file). Nếu admin báo "Add Provider Account bị treo/lỗi", hiện không thể phân biệt: relay chưa kết nối, hay Dev Server ghi file lỗi (disk full, permission), hay timeout JSON-RPC.
2. **BL-AIP-02 (resolution cascade)**: `ProviderResolver.resolve()` (`ProviderResolver.ts:39`) chạy qua nhiều filter (active status → quota check song song → priority scope user/project/server → modelHint) trước khi throw `NO_PROVIDER_AVAILABLE`. Khi agent/workflow spawn fail với lỗi này, không biết filter nào loại hết candidates (hết quota? không active? sai scope?).
3. **BL-AIP-03 (health check cron)**: `ProviderHealthChecker` (`ProviderHealthChecker.ts:36`, timer tại dòng 55) chạy định kỳ gọi `AIProviderService.testConnection()` (dòng 251) cho N account song song. Không có cách biết account nào chậm/fail trong 1 cycle mà không đọc log thô.

**Ràng buộc bảo mật bắt buộc:** KHÔNG được đưa `apiKey` (plaintext hay decrypted), `encryptedBlob`, hay `iv` vào bất kỳ `TraceFields` nào ở bất kỳ span nào trong CR này — kể cả field debug tạm thời. Chỉ trace `accountId`, `provider`, `devServerId`, `scope`, độ dài blob (`blobLength`), kết quả boolean (`ok`), latency. Vi phạm điều này là bug bảo mật nghiêm trọng vì trace event có thể được ship tới console log hoặc SSE stream tới browser khác.

## 2. Thành phần & Transport liên quan

> **Lưu ý khác biệt flow doc vs code:** `ai-providers.md` mô tả BL-AIP-01 như `POST /api/ai-providers/accounts` (REST). Trên thực tế, toàn bộ CRUD + credential write được implement dưới dạng **WS RPC method** (`defineMethod`) trong `ai-provider-rpc-handler.ts` với namespace `aiProvider.*` (`aiProvider.create`, `aiProvider.writeCredential`, `aiProvider.testConnection`, `aiProvider.resolve`, ...) — cùng framework RPC với `devServer.browseDir` (`dev-server.ts`). Không có route REST riêng. Component "ProviderCredentialWriter" trong bảng dưới đây tương ứng thực tế với method `AIProviderService.writeCredentialToDevServer()`, không phải một class riêng.

| Thành phần (flow doc) | Thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row |
|---|---|---|---|---|
| Admin/Lead Browser + SubtleCrypto | Browser-side encrypt, không có file server xác nhận | UI | — (client-side crypto, không hop qua trace) | n/a |
| Orca Web Server / AIProviderService | `AIProviderService.ts` (class `AIProviderService`, `ai-provider-rpc-handler.ts`) | Backend | WebSocket RPC (`aiProvider.create`, `aiProvider.writeCredential`) | Row 1 — WS RPC (Browser ↔ Orca Server) |
| ProviderCredentialWriter | `AIProviderService.writeCredentialToDevServer()` (`AIProviderService.ts:227`) | Security | `DevServerRelayBridge.call()` qua `RelayConnectionPool.getOrConnect()` (`relay-connection-pool.ts:39`) | Row 2 — `relay.call()` (Orca Server ↔ Dev Server) |
| AgentConnectionManager | chưa xác định file cụ thể — cần điều tra thêm khi triển khai (code hiện dùng `RelayConnectionPool` cho việc pool connection, không có class riêng tên `AgentConnectionManager`) | Backend | — | — |
| Dev Server Agent | ngoài phạm vi source repo (chạy trên máy remote) | Remote | JSON-RPC nhận qua reverse WS đã mở | Row 3 — Agent WS JSON-RPC (`params._trace.id`) |
| AIProviderResolver | `ProviderResolver.ts` (class `ProviderResolver`, method `resolve()` dòng 39) — cũng có `AIProviderService.resolveForProject()` (dòng 320) là bản resolution cũ hơn không quota-aware, dùng ở chỗ khác | Business Logic | in-process (được gọi bởi `agent.spawn`/`WorkflowOrchestrator`) | n/a — không băng qua boundary |
| ProviderHealthChecker | `ProviderHealthChecker.ts` (class `ProviderHealthChecker`, dòng 36) | Background | `relay.call()` qua `AIProviderService.testConnection()` (dòng 251→261) | Row 2 — `relay.call()` |
| Server Database | SQLite qua `this.pool.withConnection()` trong `AIProviderService.ts` | Persistence | in-process, không cần step riêng (theo §5 CR-TRACE-000) | n/a |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  aiProviderWriteCredFlow: createTracer('aiProvider:writeCredential'), // BL-AIP-01
  aiProviderResolveFlow:   createTracer('aiProvider:resolve'),         // BL-AIP-02
  aiProviderHealthFlow:    createTracer('aiProvider:healthCheck'),     // BL-AIP-03
}
```

## 4. Instrumentation theo từng sub-flow

### BL-AIP-01 — Đăng ký AI Provider Account trên Dev Server

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Nhận RPC `aiProvider.writeCredential` | `start` | `accountId`, `blobLength: encryptedBlob.length` | `ai-provider-rpc-handler.ts:141-149` |
| Lookup account + dev server | `step('lookup-account')` | `accountId`, `devServerId` | `AIProviderService.ts:232-236` |
| Lấy/mở relay connection | `step('relay-connect')` | `devServerId`, `poolHit: boolean` | `relay-connection-pool.ts:39` (`RelayConnectionPool.getOrConnect`) |
| Gửi JSON-RPC ghi credential | `step('agent-call')` | `method: 'ai.provider.writeCredential'`, `accountId` (KHÔNG kèm blob/iv) | `AIProviderService.ts:239` |
| Cập nhật status → active | `ok` / `fail(err)` | `accountId`, `status` | `AIProviderService.ts:244` |

```typescript
// AIProviderService.ts — writeCredentialToDevServer()
async writeCredentialToDevServer(accountId: string, encryptedBlob: string, iv: string): Promise<void> {
  const span = Tracers.aiProviderWriteCredFlow.start({ accountId, blobLength: encryptedBlob.length })
  const account = await this.getAccount(accountId)
  if (!account) { span.fail('ACCOUNT_NOT_FOUND'); throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`) }

  const server = this.devServerManager.get(account.devServerId)
  if (!server) { span.fail('DEV_SERVER_NOT_FOUND', { devServerId: account.devServerId }); throw new Error(/* ... */) }

  span.step('relay-connect', { devServerId: account.devServerId })
  const relay = await this.relayPool.getOrConnect(account.devServerId, server)

  span.step('agent-call', { method: 'ai.provider.writeCredential', accountId })
  // NOTE: never pass encryptedBlob/iv into span fields — security constraint §1
  await relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv })

  await this.updateAccount(accountId, { status: 'active' })
  span.ok({ accountId, status: 'active' })
}
```

### BL-AIP-02 — Provider Account Resolution cho Agent/Workflow

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu resolve | `start` | `devServerId`, `projectId`, `userId`, `modelHint` | `ProviderResolver.ts:39` |
| Quota filter xong | `step('quota-filter')` | `totalAccounts`, `overQuotaCount` | `ProviderResolver.ts:56-61` |
| Chọn theo scope (rẽ nhánh quan trọng — biết resolve theo user/project/server) | `step('scope-match')` | `matchedScope`, `usedModelHint: boolean` | `ProviderResolver.ts:76-86` |
| Kết quả | `ok({ accountId, scope })` hoặc `fail('NO_PROVIDER_AVAILABLE')` | `accountId?`, `scope?` | `ProviderResolver.ts:64,88` |

```typescript
// ProviderResolver.ts — resolve()
async resolve(options: ResolveOptions): Promise<AIProviderAccount> {
  const span = Tracers.aiProviderResolveFlow.start({
    devServerId: options.devServerId, projectId: options.projectId,
    userId: options.userId, modelHint: options.modelHint
  })
  const all = await this.service.listAccounts(options.devServerId)
  const active = all.filter(a => a.status === 'active')
  // ...quota check...
  span.step('quota-filter', { totalAccounts: all.length, overQuotaCount: overQuotaIds.size })

  if (available.length === 0) {
    span.fail('NO_PROVIDER_AVAILABLE', { reason: 'quota-or-inactive' })
    throw new Error('NO_PROVIDER_AVAILABLE: no active AI provider accounts within quota')
  }
  // ...scope matching loop...
  if (match) {
    span.step('scope-match', { matchedScope: scope, usedModelHint: !!modelHint })
    span.ok({ accountId: match.id, scope })
    return match
  }
  span.fail('NO_PROVIDER_AVAILABLE', { reason: 'no-scope-match' })
  throw new Error('NO_PROVIDER_AVAILABLE: no matching AI provider account found')
}
```

### BL-AIP-03 — Provider Health Check & Quota Management

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu 1 cron cycle | `start` | `accountCount` | `ProviderHealthChecker.ts` (cycle handler gọi bởi `setInterval`, dòng 55) |
| Ping từng account (song song) — mỗi account có span riêng resume theo cycle id nếu cần group, hoặc step trong span cha | `step('ping-account')` | `accountId`, `provider` | `AIProviderService.ts:251` (`testConnection`) |
| Kết quả ping | `step('ping-result')` | `accountId`, `ok`, `latencyMs` | `AIProviderService.ts:261-269` |
| Kết thúc cycle | `ok({ healthyCount, degradedCount, unreachableCount })` | | `ProviderHealthChecker.ts` |

```typescript
// ProviderHealthChecker.ts — cycle handler (bên trong setInterval callback)
const span = Tracers.aiProviderHealthFlow.start({ accountCount: accounts.length })
let healthy = 0, degraded = 0
for (const account of accounts) {
  span.step('ping-account', { accountId: account.id, provider: account.provider })
  const result = await this.service.testConnection(account.id)
  span.step('ping-result', { accountId: account.id, ok: result.ok, latencyMs: result.latencyMs })
  result.ok ? healthy++ : degraded++
}
span.ok({ healthyCount: healthy, degradedCount: degraded })
```

**Lưu ý:** `testConnection()` (`AIProviderService.ts:251`) đã có logic try/catch riêng và không throw — vì vậy `span.fail()` ở cấp `AIProviderService` không áp dụng cho lỗi từng account (đã được nuốt thành `{ ok: false }`); span cha (`ProviderHealthChecker`) chỉ `ok()` với đếm số lượng, không `fail()` toàn cycle trừ khi vòng lặp tự nó throw (bug logic, không phải account-level failure).

## 5. Lan truyền traceId qua transport của flow này

- **Hop 1 (Browser → Orca Server, WS RPC `aiProvider.writeCredential`)**: theo CR-TRACE-000 §3.3 hàng 1 — browser tạo `traceId` bằng tracer riêng của nó trước khi gọi RPC, gửi kèm field `traceId` cạnh `params`. `ai-provider-rpc-handler.ts:143-149` đọc `params.traceId` (nếu FE gửi) và truyền `resume: params.traceId ? { id: params.traceId } : undefined` vào `Tracers.aiProviderWriteCredFlow.start()`.
- **Hop 2 (Orca Server → Dev Server, `AIProviderService.writeCredentialToDevServer()` → `relay.call()`)**: theo CR-TRACE-000 §3.3 hàng 2 — đính `traceId: span.id` vào params envelope của `relay.call('ai.provider.writeCredential', { accountId, encryptedBlob, iv, traceId: span.id })`. Vì topology thực tế là Dev Server chủ động mở WS vào Orca Server (không phải SSH), request JSON-RPC gửi ngược qua kết nối đó vẫn đi qua cùng `DevServerRelayBridge.call()` abstraction — nên áp dụng đúng quy ước hàng 2 (params envelope), KHÔNG phải hàng 3 (`params._trace.id`) vì code không dùng JSON-RPC 2.0 thô ở lớp này mà dùng wrapper `relay.call()` giống các flow khác (`devServer:browseDir`).
- **BL-AIP-02/03 không băng qua boundary về phía traceId nhận từ ngoài** — `ProviderResolver.resolve()` được gọi in-process bởi caller (`agent.spawn`, `WorkflowOrchestrator`); nếu caller đã có `traceId` của flow lớn hơn (ví dụ workflow run), nó có thể truyền `resume: { id: parentTraceId }` khi gọi `aiProviderResolveFlow.start()` để span này nối vào cùng trace — nhưng đây là optional, không bắt buộc vì BL-AIP-02 tự nó không phải network hop.
- **BL-AIP-03 (cron)** không có "caller" bên ngoài — `traceId` luôn tự sinh mới mỗi cycle (`resume` không dùng), vì đây là entry point, không phải continuation.

## Acceptance Criteria

- [ ] `Tracers.aiProviderWriteCredFlow` bao phủ toàn bộ `writeCredentialToDevServer()` — start/relay-connect/agent-call/ok hoặc fail
- [ ] Không có bất kỳ trace event nào (start/step/ok/fail) chứa field `apiKey`, `encryptedBlob`, hoặc `iv` — verify bằng cách grep `fields` object literals trong diff PR
- [ ] `Tracers.aiProviderResolveFlow` phân biệt được rõ 2 lý do fail: `quota-or-inactive` vs `no-scope-match` qua field `reason`
- [ ] `Tracers.aiProviderHealthFlow` cho biết số lượng account healthy/degraded mỗi cycle mà không cần đọc console log thô
- [ ] `traceId` từ RPC request `aiProvider.writeCredential` (nếu FE gửi) resume đúng vào span thay vì tạo id mới
- [ ] `traceId` được forward vào `relay.call()` params envelope theo đúng CR-TRACE-000 §3.3 hàng 2
- [ ] TracePanel hiển thị đúng 3 tracer mới dưới namespace `aiProvider:*`, không đụng namespace `devServer:*`/`agent:*` sẵn có
- [ ] Review code không thêm `span.step()` cho các SQLite SELECT/UPDATE đơn (`listAccounts`, `updateAccount`) theo nguyên tắc §5 CR-TRACE-000

# SOL-INT-006 — `Orca` (Node.js/TypeScript): OrcaCallbackPublisher

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-INT-006](../../../../../../docs/crs/v3/orca/CR-ORCA-INT-006-orca-callback-publisher.md) |
| **Title** | Orca-side Callback Publisher — gọi webhook về vnp-workspace khi task hoàn thành |
| **Service** | Orca codebase (`/Users/binhnt/Work/blockchain/vnp-blc/orca/`) — TypeScript/Node.js |
| **Priority** | P0 — song song với SOL-INT-005, nền tảng cho callback flow |
| **Risk** | medium — cần xác nhận event name thật trong Orca EventBus |
| **Status** | 📐 PROPOSED — Pending Orca TS implementation |
| **Depends on** | — (independent — không phụ thuộc bất kỳ CR Go nào) |
| **TDD refs** | Orca F37 (Task Graph), Orca §15.3 (Notification Relay) |
| **Scope** | ⚠️ **Orca codebase (TypeScript)** — KHÔNG phải Go monorepo vnp-workspace |
| **Effort** | ~0.5 ngày (2 code files + 1 test, TypeScript) |

---

## 1. Tóm tắt & Mục tiêu

Thêm `OrcaCallbackPublisher` vào Orca Web Server — **thay đổi duy nhất cần làm trong Orca codebase**. Publisher lắng nghe `task.statusChanged` EventBus event và gọi webhook về vnp-workspace khi coding task hoàn thành/thất bại.

**Nguyên tắc:** Fire-and-forget, retry 3 lần với exponential backoff, không crash Orca server nếu webhook lỗi.

---

## 2. Files cần tạo/sửa trong Orca repo

### 2.1 OrcaCallbackPublisher

**File mới:** `src/main/integrations/workspace/orca-callback-publisher.ts`

```typescript
import * as crypto from 'crypto'
import { EventBus } from '../../shared/event-bus' // xác nhận import path thật
import type { OrcaTask } from '../../shared/task-types'

export interface OrcaCallbackConfig {
  workspaceCallbackUrl: string
  workspaceCallbackSecret: string
  workspaceCallbackEnabled?: boolean
}

// Callback payload — phải khớp CHÍNH XÁC với CR-ORCA-INT-003 receiver schema
export interface WorkspaceCallbackPayload {
  event: 'orca.task.completed' | 'orca.task.failed'
  task_id: string            // Orca task ID (string)
  workspace_id: string       // externalWorkspaceId từ OrcaTask metadata
  user_id: string            // externalUserId từ OrcaTask metadata
  timestamp: number          // Unix timestamp (seconds)
  result: {
    summary: string
    files_changed: string[]
    pr_url?: string
    commit_hash?: string
  } | null
  error?: string
}

export class OrcaCallbackPublisher {
  constructor(private readonly config: OrcaCallbackConfig) {}

  // start đăng ký listener cho EventBus 'task.statusChanged' event
  // Chỉ process status 'done' | 'failed' — bỏ qua status khác
  start(eventBus: EventBus): void

  // publishTaskCompletion — private, gọi bởi event listener
  private async publishTaskCompletion(
    task: OrcaTask,
    status: 'done' | 'failed'
  ): Promise<void>

  // buildPayload — tạo WorkspaceCallbackPayload từ OrcaTask
  private buildPayload(task: OrcaTask, status: 'done' | 'failed'): WorkspaceCallbackPayload

  // signPayload — HMAC-SHA256(raw_body, secret) → "sha256=<hex>"
  private signPayload(body: string): string

  // deliverWithRetry — 3 lần retry, exponential backoff 1s→2s→4s, timeout 10s mỗi attempt
  private async deliverWithRetry(payload: WorkspaceCallbackPayload): Promise<void>
}
```

**Logic signPayload:**
```typescript
private signPayload(body: string): string {
  const hmac = crypto.createHmac('sha256', this.config.workspaceCallbackSecret)
  hmac.update(body)
  return `sha256=${hmac.digest('hex')}`
}
```

**Logic deliverWithRetry:**
```typescript
// 3 attempts: delays [1000, 2000, 4000] ms
// Mỗi attempt: fetch(url, { method: 'POST', headers: {...}, body, signal: AbortSignal.timeout(10000) })
// Nếu tất cả fail: logger.error('[OrcaCallbackPublisher] delivery failed after 3 retries', ...)
// Không throw — caller (start) không bị crash
```

### 2.2 Wire trong Bootstrap

**File sửa:** `src/server/index.ts` (hoặc `src/main/index.ts` — xác nhận entry point thật)

```typescript
// Sau khi TaskService và EventBus init:
if (process.env.WORKSPACE_CALLBACK_ENABLED === 'true') {
  const callbackPublisher = new OrcaCallbackPublisher({
    workspaceCallbackUrl: process.env.WORKSPACE_CALLBACK_URL ?? '',
    workspaceCallbackSecret: process.env.WORKSPACE_CALLBACK_SECRET ?? '',
    workspaceCallbackEnabled: true,
  })
  callbackPublisher.start(eventBus)
  logger.info('[OrcaCallbackPublisher] started →', process.env.WORKSPACE_CALLBACK_URL)
}
```

### 2.3 OrcaTask Metadata Extension

**File sửa:** `src/shared/task-types.ts`

```typescript
interface OrcaTask {
  // ... existing fields ...
  
  // Integration metadata (optional — chỉ set khi task từ external system)
  integrationSource?: 'vnp-workspace' | string
  externalWorkspaceId?: string   // workspace_id từ vnp-workspace
  externalUserId?: string        // user_id từ vnp-workspace
}
```

**File sửa:** Task CREATE handler (`src/main/runtime/rpc/methods/tasks.ts` hoặc REST handler thật)

```typescript
// Nhận thêm optional fields trong request body:
// integration_source, external_workspace_id, external_user_id
// Lưu vào OrcaTask nếu có (không required — backward compatible)
```

> ⚠️ **Open Task:** Cần xác nhận tên field thật trong OrcaTask DB schema (SQLite migration) trước khi thêm. Nếu migration `0010_task_graph.ts` chưa có columns → thêm vào migration mới `0011_task_integration_metadata.ts`.

### 2.4 Unit Test

**File mới:** `src/main/integrations/workspace/orca-callback-publisher.test.ts`

Test cases (dùng `vitest` + `vi.fn()` để mock `fetch`):
- `delivers_on_task_completed` — task.statusChanged('done') → POST gọi đúng URL, body đúng schema, header `X-VNP-Orca-Signature` có
- `delivers_on_task_failed` — event='orca.task.failed', result=null
- `disabled_when_not_enabled` — `workspaceCallbackEnabled=false` → fetch KHÔNG được gọi
- `hmac_signature_valid` — tự verify signature trong test (parse header → re-compute HMAC → so sánh)
- `retry_3_times_on_failure` — mock fetch throw lỗi 3 lần → log error, không crash
- `workspace_id_user_id_in_payload` — confirm `workspace_id` = `task.externalWorkspaceId`, `user_id` = `task.externalUserId`

---

## 3. Config & Env Vars (Orca-side)

| Variable | Description | Default |
|----------|-------------|---------|
| `WORKSPACE_CALLBACK_ENABLED` | Bật callback (ship dark — mặc định false) | `"false"` |
| `WORKSPACE_CALLBACK_URL` | URL của vnp-workspace: `https://api.vnp-workspace.internal/api/v1/orca-callbacks` | `""` |
| `WORKSPACE_CALLBACK_SECRET` | Shared secret — phải khớp với `ORCA_CALLBACK_SECRET` bên vnp-workspace | `""` |

---

## 4. Checklist Thực Hiện

- [ ] **T1** Xác nhận event name thật trong Orca EventBus: `grep -rn "statusChanged\|task:completed\|task.done" src/main/task/` → ghi lại event string chính xác
- [ ] **T2** Xác nhận entry point bootstrap: `cat src/server/index.ts | head -30` hoặc `src/main/index.ts`
- [ ] **T3** Xác nhận TaskService expose EventBus hoặc đăng ký listener theo cách khác
- [ ] **T4** Tạo `src/main/integrations/workspace/orca-callback-publisher.ts`
- [ ] **T5** Tạo `src/main/integrations/workspace/orca-callback-publisher.test.ts`
- [ ] **T6** Thêm integration metadata fields vào `src/shared/task-types.ts`
- [ ] **T7** Thêm field nhận vào Task CREATE handler
- [ ] **T8** Nếu cần migration SQLite: tạo `src/db/migrations/0011_task_integration_metadata.ts`
- [ ] **T9** Wire `OrcaCallbackPublisher` trong bootstrap file
- [ ] **T10** `npx tsc --noEmit` → clean (hoặc `pnpm tsc --noEmit`)
- [ ] **T11** `vitest run src/main/integrations/workspace/orca-callback-publisher.test.ts` → 6/6 pass

---

## 5. Acceptance Criteria (từ CR-ORCA-INT-006)

- [ ] `OrcaCallbackPublisher` compile clean (TypeScript strict)
- [ ] `vitest run orca-callback-publisher.test.ts` pass
- [ ] task.completed → HTTP POST đúng URL, đúng HMAC header, đúng payload shape
- [ ] task.failed → `payload.event = 'orca.task.failed'`, `result = null`
- [ ] `WORKSPACE_CALLBACK_ENABLED=false` → không gọi HTTP POST
- [ ] HMAC-SHA256 signature hợp lệ (self-verify trong test)
- [ ] Retry: lỗi 3 lần → log error + bỏ qua (không crash Orca server)
- [ ] Timeout: mỗi attempt timeout 10s
- [ ] `workspace_id` và `user_id` được lưu vào OrcaTask khi tạo từ external source
- [ ] Callback payload field names khớp chính xác với CR-ORCA-INT-003 schema

---

## 6. Rủi ro & Open Tasks

| Rủi ro | Xác suất | Giảm thiểu |
|--------|----------|------------|
| Event name sai (task.statusChanged vs task:done) | Trung bình | T1 — grep trước khi code |
| OrcaTask không có externalWorkspaceId field | Trung bình | T8 — thêm migration nếu cần |
| fetch API không có AbortSignal.timeout ở Node.js version hiện tại | Thấp | Dùng `AbortController` + `setTimeout` thủ công nếu cần |
| Circular import giữa integrations và task-types | Thấp | Chỉ import types (interface, không class) |

---

*SOL-INT-006 · Orca (TypeScript) · Phase 1 · [CR-ORCA-INT-006](../../../../../../docs/crs/v3/orca/CR-ORCA-INT-006-orca-callback-publisher.md)*

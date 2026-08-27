## Status: 🔲 NOT STARTED

# TASK-INT-006-01 — Orca (TypeScript): OrcaCallbackPublisher

**Phase:** 1 — Nền tảng (song song với TASK-INT-005-*)
**Scope:** ⚠️ **Orca codebase (TypeScript)** — KHÔNG phải Go monorepo vnp-workspace
**Source:** [SOL-INT-006](../solutions/SOL-INT-006-orca-callback-publisher.md)
**Depends On:** — (independent)
**Estimated Files:** 3 files (2 code + 1 test)
**Working Dir:** Orca repo root (TypeScript/Node.js)

---

## Bối cảnh

Thêm `OrcaCallbackPublisher` vào Orca Web Server — đây là **thay đổi duy nhất cần làm trong Orca codebase**. Publisher lắng nghe `task.statusChanged` EventBus event và POST callback về vnp-workspace khi coding task hoàn thành/thất bại.

**Nguyên tắc:** Fire-and-forget, retry 3 lần với exponential backoff, không crash Orca server nếu webhook lỗi.

---

## Pre-conditions (xác nhận trước khi code)

```bash
# T1: Tìm event name thật trong Orca EventBus
grep -rn "statusChanged\|task:completed\|task.done\|emit.*task\|task.*emit" src/main/task/ src/shared/

# T2: Xác nhận entry point bootstrap
head -30 src/server/index.ts   # hoặc src/main/index.ts

# T3: Xác nhận TaskService/EventBus API
grep -rn "eventBus\|EventBus\|new EventBus\|EventEmitter" src/ --include="*.ts" | head -20

# T4: Xác nhận OrcaTask type (có externalWorkspaceId chưa?)
grep -rn "externalWorkspaceId\|integration_source" src/ --include="*.ts"
```

---

## Acceptance Criteria

- [ ] `OrcaCallbackPublisher` compile clean (`npx tsc --noEmit`)
- [ ] `vitest run src/main/integrations/workspace/orca-callback-publisher.test.ts` pass
- [ ] task.completed → HTTP POST đúng URL, HMAC header `X-VNP-Orca-Signature: sha256=<hex>`, đúng payload shape
- [ ] task.failed → `payload.event = 'orca.task.failed'`, `result = null`
- [ ] `WORKSPACE_CALLBACK_ENABLED=false` → không gọi HTTP POST
- [ ] HMAC-SHA256 signature hợp lệ (self-verify trong test)
- [ ] Retry 3 lần khi lỗi → log error + bỏ qua (không crash)
- [ ] Timeout: mỗi attempt timeout 10s

---

## File 1: `src/main/integrations/workspace/orca-callback-publisher.ts`

```typescript
import * as crypto from 'crypto'

// Xác nhận import paths thật từ T1-T3 pre-conditions
// import { EventBus } from '../../shared/event-bus'   // ← xác nhận path thật
// import type { OrcaTask } from '../../shared/task-types'

export interface OrcaCallbackConfig {
  workspaceCallbackUrl: string
  workspaceCallbackSecret: string
  workspaceCallbackEnabled?: boolean
}

/** Callback payload — phải khớp với vnp-workspace CR-ORCA-INT-003 receiver schema */
export interface WorkspaceCallbackPayload {
  event: 'orca.task.completed' | 'orca.task.failed'
  task_id: string          // Orca task ID
  workspace_id: string     // từ task.externalWorkspaceId
  user_id: string          // từ task.externalUserId
  timestamp: number        // Unix timestamp seconds
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

  /**
   * Đăng ký listener cho EventBus 'task.statusChanged' event.
   * Chỉ process status 'done' | 'failed'.
   */
  start(eventBus: any /* EventBus — xác nhận type thật */): void {
    if (!this.config.workspaceCallbackEnabled) {
      return // ship dark — không active khi disabled
    }
    // Xác nhận event name thật từ T1
    eventBus.on('task.statusChanged', (task: any, status: string) => {
      if (status === 'done' || status === 'failed') {
        this.publishTaskCompletion(task, status as 'done' | 'failed').catch(err => {
          console.error('[OrcaCallbackPublisher] unexpected error in publishTaskCompletion', err)
        })
      }
    })
  }

  private async publishTaskCompletion(task: any, status: 'done' | 'failed'): Promise<void> {
    const payload = this.buildPayload(task, status)
    await this.deliverWithRetry(payload)
  }

  private buildPayload(task: any, status: 'done' | 'failed'): WorkspaceCallbackPayload {
    const isCompleted = status === 'done'
    return {
      event: isCompleted ? 'orca.task.completed' : 'orca.task.failed',
      task_id: String(task.id ?? task.taskId ?? ''),
      workspace_id: String(task.externalWorkspaceId ?? ''),
      user_id: String(task.externalUserId ?? ''),
      timestamp: Math.floor(Date.now() / 1000),
      result: isCompleted && task.result ? {
        summary: task.result.summary ?? '',
        files_changed: task.result.filesChanged ?? [],
        pr_url: task.result.prUrl ?? undefined,
        commit_hash: task.result.commitHash ?? undefined,
      } : null,
      error: !isCompleted ? (task.error ?? 'Agent execution failed') : undefined,
    }
  }

  private signPayload(body: string): string {
    const hmac = crypto.createHmac('sha256', this.config.workspaceCallbackSecret)
    hmac.update(body)
    return `sha256=${hmac.digest('hex')}`
  }

  private async deliverWithRetry(payload: WorkspaceCallbackPayload): Promise<void> {
    const body = JSON.stringify(payload)
    const signature = this.signPayload(body)
    const delays = [1000, 2000, 4000] // exponential backoff

    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const response = await fetch(this.config.workspaceCallbackUrl, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-VNP-Orca-Signature': signature,
          },
          body,
          signal: AbortSignal.timeout(10_000), // 10s timeout per attempt
        })
        if (response.ok) {
          return // success
        }
        console.warn(`[OrcaCallbackPublisher] delivery attempt ${attempt + 1} failed: HTTP ${response.status}`)
      } catch (err) {
        console.warn(`[OrcaCallbackPublisher] delivery attempt ${attempt + 1} error:`, err)
      }

      if (attempt < 2) {
        await new Promise(resolve => setTimeout(resolve, delays[attempt]))
      }
    }

    console.error(
      '[OrcaCallbackPublisher] delivery failed after 3 retries',
      { task_id: payload.task_id, event: payload.event }
    )
    // Không throw — không crash Orca server
  }
}
```

---

## File 2: Wire trong bootstrap (xác nhận entry point từ T2)

```typescript
// src/server/index.ts (hoặc src/main/index.ts — xác nhận từ T2)
import { OrcaCallbackPublisher } from '../main/integrations/workspace/orca-callback-publisher'

// Sau khi TaskService và EventBus init:
if (process.env.WORKSPACE_CALLBACK_ENABLED === 'true') {
  const callbackPublisher = new OrcaCallbackPublisher({
    workspaceCallbackUrl: process.env.WORKSPACE_CALLBACK_URL ?? '',
    workspaceCallbackSecret: process.env.WORKSPACE_CALLBACK_SECRET ?? '',
    workspaceCallbackEnabled: true,
  })
  callbackPublisher.start(eventBus) // inject eventBus thật
  console.info('[OrcaCallbackPublisher] started →', process.env.WORKSPACE_CALLBACK_URL)
}
```

---

## File 3: `src/main/integrations/workspace/orca-callback-publisher.test.ts`

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { OrcaCallbackPublisher } from './orca-callback-publisher'
import * as crypto from 'crypto'

// Mock fetch globally
const mockFetch = vi.fn()
global.fetch = mockFetch

function makeTask(overrides = {}): any {
  return {
    id: 'orca-task-abc',
    externalWorkspaceId: 'wks-1',
    externalUserId: 'usr-1',
    result: { summary: 'done', filesChanged: ['main.go'] },
    ...overrides,
  }
}

describe('OrcaCallbackPublisher', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockResolvedValue({ ok: true })
  })

  const config = {
    workspaceCallbackUrl: 'http://workspace/api/v1/orca-callbacks',
    workspaceCallbackSecret: 'test-secret',
    workspaceCallbackEnabled: true,
  }

  it('delivers on task completed', async () => {
    const publisher = new OrcaCallbackPublisher(config)
    // Access private method via type cast
    await (publisher as any).deliverWithRetry(
      (publisher as any).buildPayload(makeTask(), 'done')
    )

    expect(mockFetch).toHaveBeenCalledOnce()
    const [url, opts] = mockFetch.mock.calls[0]
    expect(url).toBe(config.workspaceCallbackUrl)
    expect(opts.method).toBe('POST')
    expect(opts.headers['X-VNP-Orca-Signature']).toMatch(/^sha256=/)

    const body = JSON.parse(opts.body)
    expect(body.event).toBe('orca.task.completed')
    expect(body.workspace_id).toBe('wks-1')
    expect(body.result).not.toBeNull()
  })

  it('delivers on task failed', async () => {
    const publisher = new OrcaCallbackPublisher(config)
    const payload = (publisher as any).buildPayload(makeTask({ error: 'timeout' }), 'failed')

    expect(payload.event).toBe('orca.task.failed')
    expect(payload.result).toBeNull()
    expect(payload.error).toBe('timeout')
  })

  it('does not deliver when disabled', async () => {
    const publisher = new OrcaCallbackPublisher({ ...config, workspaceCallbackEnabled: false })
    const mockEventBus = { on: vi.fn() }

    publisher.start(mockEventBus)
    expect(mockEventBus.on).not.toHaveBeenCalled()
  })

  it('verifies hmac signature', () => {
    const publisher = new OrcaCallbackPublisher(config)
    const body = JSON.stringify({ event: 'orca.task.completed' })
    const sig = (publisher as any).signPayload(body)

    const expected = 'sha256=' + crypto.createHmac('sha256', config.workspaceCallbackSecret)
      .update(body).digest('hex')
    expect(sig).toBe(expected)
  })

  it('retries 3 times on failure then logs error without throw', async () => {
    mockFetch.mockRejectedValue(new Error('network error'))
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

    const publisher = new OrcaCallbackPublisher(config)
    // Should not throw
    await expect(
      (publisher as any).deliverWithRetry((publisher as any).buildPayload(makeTask(), 'done'))
    ).resolves.toBeUndefined()

    expect(mockFetch).toHaveBeenCalledTimes(3)
    expect(consoleSpy).toHaveBeenCalledWith(
      expect.stringContaining('failed after 3 retries'),
      expect.any(Object)
    )
    consoleSpy.mockRestore()
  })

  it('includes workspace_id and user_id in payload', () => {
    const publisher = new OrcaCallbackPublisher(config)
    const payload = (publisher as any).buildPayload(makeTask(), 'done')
    expect(payload.workspace_id).toBe('wks-1')
    expect(payload.user_id).toBe('usr-1')
  })
})
```

---

## Env Vars (Orca-side)

```
WORKSPACE_CALLBACK_ENABLED=false   # Ship dark — phải opt-in
WORKSPACE_CALLBACK_URL=            # https://api.vnp-workspace.internal/api/v1/orca-callbacks
WORKSPACE_CALLBACK_SECRET=         # Shared secret với vnp-workspace ORCA_CALLBACK_SECRET
```

---

## Checklist

- [ ] **Pre** T1: `grep -rn "statusChanged\|task:completed" src/main/task/` → ghi lại event name thật
- [ ] **Pre** T2: Xác nhận entry point bootstrap file
- [ ] **Pre** T3: Xác nhận EventBus type và method (`.on()` / `.subscribe()` / `.emit()`)
- [ ] **Pre** T4: Check nếu `OrcaTask` đã có `externalWorkspaceId` field hay cần thêm migration
- [ ] Tạo `src/main/integrations/workspace/orca-callback-publisher.ts` (sửa import paths cho đúng)
- [ ] Wire trong bootstrap file (với đúng entry point)
- [ ] Tạo `src/main/integrations/workspace/orca-callback-publisher.test.ts`
- [ ] `npx tsc --noEmit` → clean
- [ ] `vitest run src/main/integrations/workspace/orca-callback-publisher.test.ts` → 6/6 pass

---

*TASK-INT-006-01 · Orca (TypeScript) · Phase 1 · [SOL-INT-006](../solutions/SOL-INT-006-orca-callback-publisher.md)*

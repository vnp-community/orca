> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-004-15 — Orca: `PlannerResultCollector` + `PlannerCallbackPublisher`

**Phase:** 2 — song song, không chặn code Go
**Scope:** 🟠 **Orca TypeScript CONTRACT — KHÔNG thực thi trong repo `vnp-workplace`.** Code mới cần thêm vào repo `orca` (`/opt/repos/orca`): hook mới trong `backend/src/main/task/TaskAgentExecutor.ts`, 2 service mới (`PlannerResultCollector`, `PlannerCallbackPublisher`).
**Source:** [SOL-ORCA-004 §3.1–§3.2, §9](../solutions/SOL-ORCA-004-orca-result-reporter.md#31-luồng-end-to-end)
**Depends On:** [TASK-ORCA-001-13](./TASK-ORCA-001-13-orca-planner-task-routes.md) (cần `callback_url` lưu trong `PlannerTaskRecord`)
**Người thực thi:** Orca team
**Ghi chú re-scope (2026-08-10):** Nội dung task này (Orca-side, TypeScript) **không đổi về mặt kỹ thuật** so với bản gốc — chỉ cập nhật thuật ngữ phía nhận (`plan-svc` → `planner-service`, `vnp-planner` → `vnp-workplace`) theo re-scope kiến trúc. Xem `docs/crs/v3/orca/README.md` §Ghi chú Re-scope.

---

## Vì sao task này tồn tại

Đích đến của callback push từ Orca là `planner-service` (`vnp-workplace`, `:3013`), nhận tại `POST /api/v1/planner/orca-callback` qua `api-gateway` reverse-proxy — route nhận đã sẵn sàng ở `planner-service` (TASK-ORCA-004-07). Việc điều phối/chờ kết quả ở mức workflow (`OrcaTaskDispatchWorkflow`, `temporal-worker`) **vẫn thuộc `vnp-planner`** (CR-ORCA-002, không nằm trong phạm vi re-scope 2026-08-10, không đổi) và độc lập với route callback này — xem SOL-ORCA-004 §3.5 (Open Decision, cơ chế nối 2 phía chưa chốt). Phía Orca **hoàn toàn chưa có** cơ chế thu thập kết quả (`git diff`/`git log`/test output) lẫn cơ chế publish callback — `grep -rn "orca-callback\|orca_callback\|callback_url" /opt/repos/orca` → 0 kết quả (khảo sát 2026-08-10).

---

## Vị trí tích hợp thật (đã khảo sát)

Trong try/catch của `TaskAgentExecutor.executeTask()` (`backend/src/main/task/TaskAgentExecutor.ts:76-106`):
- Nhánh **thành công**: ngay sau `await this.taskService.update(taskId, { status: 'review' })` (dòng 88).
- Nhánh **lỗi**: ngay sau `await this.taskService.update(taskId, { status: 'blocked' }).catch(...)` (dòng 98).

Cả hai chỉ nên chạy khi task có nhãn `planner:*` (đánh dấu nguồn gốc từ hệ thống planner — `planner-service`/`vnp-workplace`, thay `vnp-planner` theo re-scope 2026-08-10 — cùng field mới ở TASK-ORCA-003-14).

---

## Acceptance Criteria

- [ ] `PlannerResultCollector.collect(worktreePath)` trả `{filesCreated[], filesModified[], commitHash, testOutput}` bằng `git status --porcelain` + `git log -1 --format=%H` + đọc output test gần nhất (nguồn test output tuỳ theo cách agent chạy `go test`/`npm test` trong worktree — Orca team quyết định cách capture, ví dụ redirect stdout của tiến trình PTY agent vào buffer)
- [ ] `agent_output`/`test_output` giới hạn ~5000 ký tự cuối (`slice(-5000)`) trước khi gửi — tránh payload quá lớn
- [ ] `PlannerCallbackPublisher.publish(callbackUrl, payload)` gửi `POST {callback_url}` với header `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` + `X-Orca-Source: orca-planner-callback`
- [ ] `PlannerCallbackPublisher.publish` **KHÔNG retry** khi gửi thất bại (đúng theo thiết kế đã chốt — phía Go luôn có poll fallback độc lập, xem SOL-ORCA-002 §3.7) — chỉ log lỗi
- [ ] Timeout gửi callback = `ORCA_PLANNER_CALLBACK_TIMEOUT_MS` (mặc định 10000ms, cấu hình được — xem TASK-ORCA-006-16)
- [ ] Chỉ chạy cho task có nhãn `planner:*` — task thường (không từ planner) không bị ảnh hưởng
- [ ] `PlannerTaskRecord.result` (TASK-ORCA-001-13) được set đúng khi collector chạy xong, để `GET /api/planner-tasks/{id}` trả `result` không còn `null`

---

## Code mẫu tham khảo

### `backend/src/main/task/PlannerResultCollector.ts` [NEW]

```ts
/**
 * PlannerResultCollector — gathers git/test evidence from a completed
 * planner:* task's worktree (CR-ORCA-004 / SOL-ORCA-004 §3.1).
 *
 * Contract source of truth: backend/specs/crs/v3/orca/solutions/
 *   SOL-ORCA-004-orca-result-reporter.md §3.2 (vnp-workplace repo, planner-service payload shape)
 */
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const OUTPUT_TAIL_CHARS = 5000

export type PlannerCollectedResult = {
  filesCreated: string[]
  filesModified: string[]
  commitHash: string | null
  testOutput: string | null
}

export class PlannerResultCollector {
  async collect(worktreePath: string, lastTestOutput: string | null): Promise<PlannerCollectedResult> {
    const [statusOut, logOut] = await Promise.all([
      this.safeGit(worktreePath, ['status', '--porcelain']),
      this.safeGit(worktreePath, ['log', '-1', '--format=%H'])
    ])

    const filesCreated: string[] = []
    const filesModified: string[] = []
    for (const line of statusOut.split('\n').filter(Boolean)) {
      const status = line.slice(0, 2).trim()
      const file = line.slice(3)
      if (status === 'A' || status === '??') filesCreated.push(file)
      else if (status === 'M') filesModified.push(file)
    }

    return {
      filesCreated,
      filesModified,
      commitHash: logOut.trim() || null,
      testOutput: lastTestOutput ? lastTestOutput.slice(-OUTPUT_TAIL_CHARS) : null
    }
  }

  private async safeGit(cwd: string, args: string[]): Promise<string> {
    try {
      const { stdout } = await execFileAsync('git', args, { cwd })
      return stdout
    } catch (err) {
      console.warn(`[PlannerResultCollector] git ${args.join(' ')} failed:`, (err as Error).message)
      return ''
    }
  }
}
```

### `backend/src/main/task/PlannerCallbackPublisher.ts` [NEW]

```ts
/**
 * PlannerCallbackPublisher — POSTs the collected result to the planner system's
 * callback_url. As of the 2026-08-10 re-scope, callback_url resolves to
 * planner-service (vnp-workplace, :3013) behind api-gateway — see SOL-ORCA-004
 * §3.2/§3.4 (CR-ORCA-004).
 *
 * IMPORTANT: does NOT retry on failure. vnp-planner's OrcaTaskDispatchWorkflow
 * (temporal-worker, CR-ORCA-002 — unchanged by the 2026-08-10 re-scope, still
 * in vnp-planner) has an independent poll fallback calling Orca directly
 * (GET /api/planner-tasks/{id} every 5 min) — see SOL-ORCA-002 §3.7. Adding
 * retry here would duplicate that responsibility without a shared source of
 * truth for how many attempts have been made. Note: this poll fallback is a
 * property of temporal-worker/vnp-planner, independent of how planner-service
 * (vnp-workplace) itself relays completion internally — see SOL-ORCA-004 §3.5
 * (Open Decision, not yet chốt).
 */

export type PlannerCallbackPayload = {
  orca_task_id: string
  planner_task_id: string
  planner_job_id?: string
  success: boolean
  files_created: string[]
  files_modified: string[]
  commit_hash: string | null
  test_output: string | null
  error_message: string | null
  agent_output: string | null
}

export class PlannerCallbackPublisher {
  constructor(
    private readonly apiSecret: string,
    private readonly timeoutMs = Number(process.env['ORCA_PLANNER_CALLBACK_TIMEOUT_MS'] ?? 10000)
  ) {}

  async publish(callbackUrl: string, payload: PlannerCallbackPayload): Promise<void> {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeoutMs)
    try {
      const res = await fetch(callbackUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${this.apiSecret}`,
          'X-Orca-Source': 'orca-planner-callback'
        },
        body: JSON.stringify(payload),
        signal: controller.signal
      })
      if (!res.ok) {
        console.error(`[PlannerCallbackPublisher] callback returned ${res.status} for ${payload.planner_task_id}`)
      }
    } catch (err) {
      // No retry — see class doc-comment. Log and move on; the poll fallback
      // on vnp-planner's side will pick this up.
      console.error(
        `[PlannerCallbackPublisher] failed to deliver callback for ${payload.planner_task_id}:`,
        (err as Error).message
      )
    } finally {
      clearTimeout(timer)
    }
  }
}
```

### Hook vào `TaskAgentExecutor.executeTask()` [MODIFY]

```ts
// backend/src/main/task/TaskAgentExecutor.ts — extend the try/catch in executeTask()
import { PlannerResultCollector } from './PlannerResultCollector'
import { PlannerCallbackPublisher } from './PlannerCallbackPublisher'

const resultCollector = new PlannerResultCollector()
const callbackPublisher = new PlannerCallbackPublisher(process.env['ORCA_PLANNER_API_SECRET'] ?? '')

// ... inside executeTask(), after resolving `task` and confirming it's a
// planner:* task (task.labels?.some(l => l.startsWith('planner:'))) ...

try {
  // ... existing spawn logic (step 5) ...
  await this.taskService.update(taskId, { status: 'review' })
  await this.taskService.addComment(taskId, userId, `Agent execution completed successfully`, 'activity')

  if (isPlannerTask && task.plannerCallbackUrl) {
    const collected = await resultCollector.collect(worktreePath, latestTestOutputBuffer)
    const payload = {
      orca_task_id: taskId,
      planner_task_id: task.plannerTaskId!, // set at task creation, TASK-ORCA-001-13
      planner_job_id: task.plannerJobId,
      success: true,
      files_created: collected.filesCreated,
      files_modified: collected.filesModified,
      commit_hash: collected.commitHash,
      test_output: collected.testOutput,
      error_message: null,
      agent_output: agentOutputBuffer.slice(-5000)
    }
    void callbackPublisher.publish(task.plannerCallbackUrl, payload) // fire-and-forget, no await blocking the executor's own flow
  }
  span.ok({ status: 'review' })
} catch (err) {
  const errMsg = err instanceof Error ? err.message : String(err)
  await this.taskService.update(taskId, { status: 'blocked' }).catch(() => {})
  await this.taskService.addComment(taskId, userId, `Agent execution failed: ${errMsg}`, 'activity').catch(() => {})

  if (isPlannerTask && task.plannerCallbackUrl) {
    const collected = await resultCollector.collect(worktreePath, latestTestOutputBuffer).catch(() => null)
    void callbackPublisher.publish(task.plannerCallbackUrl, {
      orca_task_id: taskId,
      planner_task_id: task.plannerTaskId!,
      planner_job_id: task.plannerJobId,
      success: false,
      files_created: collected?.filesCreated ?? [],
      files_modified: collected?.filesModified ?? [],
      commit_hash: collected?.commitHash ?? null,
      test_output: collected?.testOutput ?? null,
      error_message: errMsg,
      agent_output: agentOutputBuffer.slice(-5000)
    })
  }
  span.fail(err, { status: 'blocked' })
  throw err
}
```

> `latestTestOutputBuffer`/`agentOutputBuffer` — Orca team quyết định cơ chế capture thật (ví dụ buffer gắn vào `node-pty` stdout stream của tiến trình agent) — không thuộc phạm vi đặc tả chi tiết của task này, chỉ cần đảm bảo 2 biến đó tồn tại và được truncate đúng trước khi gửi callback.

---

## Verification (phía Orca team)

```bash
cd /opt/repos/orca
npm run build
npm test -- PlannerResultCollector
npm test -- PlannerCallbackPublisher

# Integration smoke test (cần api-gateway + planner-service chạy — TASK-ORCA-004-07 đã sẵn sàng nhận):
# 1. Submit 1 planner:* task với callback_url trỏ tới api-gateway thật/dev
#    (POST /api/v1/planner/orca-callback, reverse-proxy sang planner-service :3013)
# 2. Xác nhận agent hoàn tất → planner-service log nhận được POST /api/v1/planner/orca-callback
#    với payload đúng schema, status 200
```

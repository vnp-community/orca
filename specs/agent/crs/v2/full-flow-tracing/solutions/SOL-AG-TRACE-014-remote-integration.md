# SOL-AG-TRACE-014: Remote Integration (GitHub/GitLab CLI) — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**TDD Ref:** TDD-AG-13 (External API Connectors — GitHub & GitLab)
**File(s):**
- `src/relay/external-api-connector.ts` [MODIFY]
- `src/relay/fs-agent-extensions.ts` [MODIFY]
**Mức độ:** 🟢 Đơn giản
**Thời gian ước tính:** 2h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-014 định nghĩa 3 tracer mới trong `tracers.ts`: `remoteIntegration:credentialDecrypt`, `remoteIntegration:credentialStore` (cả hai sống ở **Main process** — `GitProviderCredentialService.ts`, `web-credential-store.ts`, `credentials.ts`, đã xác nhận cả 3 file này nằm trong `src/main/`, **ngoài phạm vi solution này**), và `remoteIntegration:preflight` (Main process, `src/main/runtime/rpc/methods/preflight.ts`, cũng ngoài phạm vi). Solution này **chỉ** bao phủ phần thực thi CLI `gh`/`glab` thật sự chạy trên Dev Server Agent (`src/relay/external-api-connector.ts`) — tương ứng với tracer `remoteIntegration:ghExec` mà CR-TRACE-014 §3 mô tả là "layer riêng, đo latency riêng" so với `credentialDecrypt` (đúng nguyên tắc CR-TRACE-000 §3.1: mỗi layer 1 tracer).

**Điểm quan trọng cần nêu rõ (đúng yêu cầu CR-TRACE-014 §1 BL-INT-01):** không có transport nối `remoteIntegrationCredentialDecryptFlow` (Main) với tracer agent-side trong solution này — CR-TRACE-014 §5 mục 3 đã xác nhận không tồn tại `CliAuthProxy`/`credential.request` bridge nào giữa 2 process cho việc này. Do đó agent-side tracer trong solution này **độc lập hoàn toàn**, không `resume` từ span nào phía Main.

Ngoài `external-api-connector.ts` (phạm vi chính CR-014 giao), điều tra thực tế phát hiện thêm **một implementation preflight thứ ba** chưa được CR-TRACE-014 đề cập — `handlePreflightCheck()` trong `src/relay/fs-agent-extensions.ts:254`, được gọi qua Agent WS JSON-RPC method `preflight.check` (case tại `agent-rpc-dispatch.ts`). Đây khác với `PreflightHandler.checkFullPreflight()` (`src/relay/preflight-handler.ts`, dùng cho mode `relay-ssh` qua `RelayDispatcher`, đã được CR-TRACE-014 §4 BL-INT-03 đánh dấu rõ "KHÔNG bắt buộc — out of scope, relay process riêng"). Vì `handlePreflightCheck()` chạy trên **cùng tiến trình/kênh Agent WS JSON-RPC** như `external-api-connector.ts` (không phải "relay process riêng" mà CR-014 loại trừ), solution này đưa nó vào phạm vi luôn — xem mục 3.3.

## 2. Gap hiện tại

| # | File:function | Trạng thái | Hành động |
|---|----------------|-----------|-----------|
| 1 | `external-api-connector.ts:130 handleGitHubPrCreate`, `:195 handleGitHubPrMerge` | **Đã có** `apiTracer` (`agent:ext-api`) — `start`/`ok`/`fail` đầy đủ, kể cả nhánh idempotency (`alreadyExisted`) | Không sửa — document |
| 2 | `external-api-connector.ts:301 handleGitHubAuthStatus` | **Không có tracer** — đúng như CR-TRACE-014 §4 BL-INT-01 chỉ ra (dòng 301 khớp chính xác với thực tế) | **Thêm** span dùng lại `apiTracer` |
| 3 | `external-api-connector.ts:428 handleGitLabAuthStatus` | **Không có tracer** (dòng 428 khớp CR-TRACE-014) | **Thêm** span dùng lại `apiTracer` |
| 4 | `external-api-connector.ts:234 handleGitHubIssueList`, `:264 handleGitHubIssueCreate`, `:325 handleGitLabMrCreate`, `:370 handleGitLabMrList`, `:399 handleGitLabPipelineStatus` | **Không có tracer** — cùng file, cùng lớp thao tác (`execFileCaptured('gh'/'glab', ...)`) như 2 method đã traced ở #1 | **Thêm** span dùng lại `apiTracer` (nhất quán trong cùng file) |
| 5 | `fs-agent-extensions.ts:254 handlePreflightCheck` | **Không có tracer** — chỉ check `github-cli`/`ripgrep`/`docker`/`claude` binary có mặt hay không, không check `gh auth status` thật sự | **Thêm mới** tracer `agent:preflight` |

## 3. Full Implementation

### 3.1 `external-api-connector.ts` — bổ sung span cho `handleGitHubAuthStatus` / `handleGitLabAuthStatus`

Đây là 2 method CR-TRACE-014 §4 BL-INT-01 gọi đích danh ("Dev Server: Kiểm tra auth status trước khi exec"). `apiTracer` đã tồn tại đầu file (dòng 20) — dùng lại, không tạo tracer mới.

```typescript
// src/relay/external-api-connector.ts — handleGitHubAuthStatus() (dòng 301)

export async function handleGitHubAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGhEnv(userId, config.toolEnv)
  const span = apiTracer.start({ method: 'github.auth.status', cli: 'gh' })

  span.step('exec', { cli: 'gh' })
  try {
    const result = await execFileCaptured('gh', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`github.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'gh', authenticated: ok })
    } else {
      span.fail(result.stderr || 'gh auth status non-zero exit', { cli: 'gh', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'gh' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

```typescript
// src/relay/external-api-connector.ts — handleGitLabAuthStatus() (dòng 428)

export async function handleGitLabAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGlabEnv(userId, config.toolEnv)
  const span = apiTracer.start({ method: 'gitlab.auth.status', cli: 'glab' })

  span.step('exec', { cli: 'glab' })
  try {
    const result = await execFileCaptured('glab', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`gitlab.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'glab', authenticated: ok })
    } else {
      span.fail(result.stderr || 'glab auth status non-zero exit', { cli: 'glab', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'glab' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

**Ràng buộc bảo mật (bắt buộc — nhắc lại từ CR-TRACE-014 §4 và §Acceptance Criteria):** không field nào trong `agent:ext-api` được chứa giá trị token/PAT — `userId` là ID nội bộ (không phải secret), `result.stdout`/`result.stderr` của `gh auth status`/`glab auth status` **không** được đưa vào trace fields (chỉ đưa vào JSON-RPC `result` trả về client — khác kênh với trace). Chỉ field cho phép: `method`/`cli`/`exitCode`/`authenticated`.

### 3.2 `external-api-connector.ts` — bổ sung span cho 5 handler còn lại (nhất quán trong file)

Áp dụng cùng pattern (`apiTracer.start({method, ...})` → `ok`/`fail`) cho `handleGitHubIssueList`, `handleGitHubIssueCreate`, `handleGitLabMrCreate`, `handleGitLabMrList`, `handleGitLabPipelineStatus` — không nằm trong bảng CR-TRACE-014 §4 (CR chỉ liệt kê auth-status), nhưng thuộc cùng "layer đo latency `gh`/`glab` CLI exec" mà `remoteIntegration:ghExec`/`agent:ext-api` đại diện, và cùng file/tracer đã dùng cho `pr.create`/`pr.merge`. Ví dụ đại diện (`handleGitLabMrCreate`, các handler còn lại theo đúng khuôn mẫu này):

```typescript
// src/relay/external-api-connector.ts — handleGitLabMrCreate() (dòng 325)

export async function handleGitLabMrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title        = typeof params.title        === 'string' ? params.title.trim()        : ''
  const description  = typeof params.description  === 'string' ? params.description          : ''
  const targetBranch = typeof params.targetBranch === 'string' ? params.targetBranch.trim()  : 'main'
  const cwd          = typeof params.cwd          === 'string' && params.cwd ? params.cwd : config.workDir
  const userId       = typeof params.userId       === 'string' ? params.userId               : ''
  const span = apiTracer.start({ method: 'gitlab.mr.create', title: title.slice(0, 40), targetBranch })

  if (!title) {
    span.fail('missing title', { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(targetBranch)) {
    span.fail('unsafe characters in params', { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in MR params' } }
  }

  const glabArgs = ['mr', 'create', '--title', title, '--description', description, '--target-branch', targetBranch, '--yes']
  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
    const url = result.stdout.trim().split('\n').find((l: string) => l.startsWith('https://')) ?? result.stdout.trim()
    log.info(`gitlab.mr.create: MR → ${url}`)
    span.ok({ url })
    return { jsonrpc: '2.0', id, result: { url, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

Áp dụng tương tự cho `handleGitHubIssueList`/`handleGitHubIssueCreate` (`span = apiTracer.start({method:'github.issue.list'|'github.issue.create', ...})`) và `handleGitLabMrList`/`handleGitLabPipelineStatus` — mỗi handler: `span.ok({...kết quả không nhạy cảm...})` khi `exitCode === 0`, `span.fail(result.stderr, {exitCode})` khi khác 0, `span.fail(err)` khi catch exception.

### 3.3 `fs-agent-extensions.ts` — tracer mới `agent:preflight` cho `handlePreflightCheck`

```typescript
// src/relay/fs-agent-extensions.ts
// (thêm tracer mới cạnh fsTracer đã có ở đầu file)

const preflightTracer = createTracer('agent:preflight')

export async function handlePreflightCheck(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const services = Array.isArray(params.services) ? params.services.map(String) : []
  const results: Record<string, boolean> = {}
  const span = preflightTracer.start({ services: services.join(',') || '(empty)' })

  await Promise.all(services.map(async (service) => {
    try {
      switch (service) {
        case 'github-cli':
          results[service] = await checkBinaryAvailable('gh', config)
          break
        case 'ripgrep':
          results[service] = await checkRgAvailable()
          break
        case 'docker':
          results[service] = await checkBinaryAvailable('docker', config)
          break
        case 'claude':
          results[service] = await checkBinaryAvailable('claude', config)
          break
        default:
          results[service] = false
      }
    } catch {
      results[service] = false
    }
  }))

  const failedServices = Object.entries(results).filter(([, ok]) => !ok).map(([svc]) => svc)
  if (failedServices.length > 0) {
    span.fail(`unavailable: ${failedServices.join(',')}`, { failedCount: failedServices.length })
  } else {
    span.ok({ checkedCount: services.length })
  }
  return { jsonrpc: '2.0', id, result: results }
}
```

Đặt tên `agent:preflight` (không dùng `remoteIntegration:preflight` như CR-TRACE-014 §3 đề xuất cho backend) vì lý do tương tự mục 1 của SOL-AG-TRACE-013 — namespace `remoteIntegration:` thuộc backend/Main process; agent-side theo convention `agent:xxx` cục bộ đã thiết lập trong `src/relay/`. Lưu ý: `span.fail()` ở đây dùng cho "có service không khả dụng" (business-level fail, không phải exception) — nhất quán với CR-TRACE-014 §Acceptance Criteria yêu cầu `remoteIntegration:preflight` phải phân biệt rõ trạng thái fail thay vì chỉ dựa vào exception.

## 4. Test Plan (Vitest)

### 4.1 Mở rộng `src/relay/__tests__/external-api-connector.test.ts` (đã tồn tại)

Thêm sau `describe('buildGlabEnv', ...)` hiện có:

```typescript
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'
import { handleGitHubAuthStatus, handleGitLabAuthStatus } from '../external-api-connector'

describe('handleGitHubAuthStatus — agent:ext-api tracing', () => {
  it('span.ok({cli:"gh", authenticated:true}) khi gh auth status exit 0', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    // mock execFileCaptured indirectly via spawn — xem setup có sẵn trong file test hiện tại
    const res = await handleGitHubAuthStatus(1, { userId: 'u1' }, config, log) as { result: { ok: boolean } }
    unregister()
    const ok = events.find(e => e.flow === 'agent:ext-api' && e.level === 'ok')
    expect(ok?.fields.cli).toBe('gh')
  })

  it('KHÔNG có field nào trong agent:ext-api chứa nội dung stdout/stderr của gh auth status', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitHubAuthStatus(1, { userId: 'u1' }, config, log)
    unregister()
    const fields = events.filter(e => e.flow === 'agent:ext-api').flatMap(e => Object.keys(e.fields))
    expect(fields).not.toContain('stdout')
    expect(fields).not.toContain('stderr')
  })
})

describe('handleGitLabAuthStatus — agent:ext-api tracing', () => {
  it('span.fail(..., {cli:"glab", exitCode}) khi glab auth status exit != 0', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitLabAuthStatus(1, { userId: 'u1' }, config, log)
    unregister()
    const fail = events.find(e => e.flow === 'agent:ext-api' && e.level === 'fail')
    expect(fail?.fields.cli).toBe('glab')
  })
})
```

### 4.2 `src/relay/__tests__/fs-agent-extensions.test.ts` [kiểm tra xem đã tồn tại — nếu chưa, tạo mới]

```typescript
import { describe, it, expect, vi } from 'vitest'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'
import { handlePreflightCheck } from '../fs-agent-extensions'

describe('handlePreflightCheck — agent:preflight tracing', () => {
  it('span.ok({checkedCount}) khi tất cả services khả dụng', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['ripgrep'] }, config)
    unregister()
    expect(events.some(e => e.flow === 'agent:preflight' && e.level === 'ok')).toBe(true)
  })

  it('span.fail("unavailable: ...") khi có service không cài đặt', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handlePreflightCheck(1, { services: ['not-a-real-binary-xyz'] }, config)
    unregister()
    const fail = events.find(e => e.flow === 'agent:preflight' && e.level === 'fail')
    expect(fail?.fields.failedCount).toBe(1)
  })
})
```

## 5. Acceptance Criteria

- [ ] `handleGitHubAuthStatus`/`handleGitLabAuthStatus` phân biệt `cli: 'gh'` vs `cli: 'glab'` và `exitCode`/`authenticated` trong mọi event, đúng CR-TRACE-014 §Acceptance Criteria dòng "phân biệt được cli và exitCode"
- [ ] Không field nào trong `agent:ext-api` (bất kỳ handler nào trong `external-api-connector.ts`) chứa giá trị token/PAT hoặc nội dung `stdout`/`stderr` thô của `gh`/`glab`
- [ ] 5 handler chưa được CR-TRACE-014 liệt kê tường minh (`issue.list`, `issue.create`, `mr.create`, `mr.list`, `pipeline.status`) đều có tracing nhất quán với `handleGitHubPrCreate`/`handleGitHubPrMerge` đã có sẵn
- [ ] `handlePreflightCheck` (`fs-agent-extensions.ts`) có `agent:preflight` với `fail()` khi có service không khả dụng — không chỉ dựa vào exception
- [ ] Không tracer nào trong solution này trùng tên với `agent:credential` (AI provider credential — khác domain) hoặc `agent:fs` (đã có trong cùng file `fs-agent-extensions.ts`, dùng cho `fs.*`, không dùng cho preflight)
- [ ] `remoteIntegration:credentialDecrypt` KHÔNG xuất hiện trong bất kỳ file `src/relay/*.ts` nào — tracer đó chỉ tồn tại ở Main process (`GitProviderCredentialService.ts`), xác nhận qua review code trước khi merge
- [ ] `external-api-connector.test.ts` có thêm ≥ 3 test case theo mục 4.1; `fs-agent-extensions` có test file với ≥ 2 test case theo mục 4.2
- [ ] `pnpm vitest run src/relay/__tests__/external-api-connector.test.ts src/relay/__tests__/fs-agent-extensions.test.ts` pass

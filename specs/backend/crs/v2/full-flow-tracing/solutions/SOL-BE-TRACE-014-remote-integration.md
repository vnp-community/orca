# SOL-BE-TRACE-014: Remote Integration — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**TDD Ref:** TDD-20 (Remote Git UI — không có method liên quan trực tiếp credential; tham chiếu cho bối cảnh `relay.call()` chung), TDD-05 (SSH & Relay — không định nghĩa `CliAuthProxy`; xác nhận lại rằng cơ chế này không tồn tại), TDD-04 §7 (RPC Method Categories — `ACCOUNT_METHODS`, không có `credentials.*`/`preflight.*` liệt kê tường minh, xem ghi chú mục 1.3)
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ instrument phía Main process (Backend/Gateway); KHÔNG chạm vào code chạy trên Dev Server

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Ranh giới quan trọng nhất của solution này

CR-TRACE-014 định nghĩa 4 tracer: `remoteIntegration:credentialDecrypt`, `remoteIntegration:credentialStore`, `remoteIntegration:ghExec`, `remoteIntegration:preflight`. Trong đó **`remoteIntegration:ghExec` chạy trên Dev Server** (`src/relay/external-api-connector.ts`, verified: file này thực thi `gh`/`glab` CLI trực tiếp trên máy Dev Server, là relay binary — không phải Backend/Gateway). Theo phạm vi công việc được giao ("instrument các RPC method Backend kích hoạt `gh`/`glab` TRƯỚC KHI chúng chạm `relay.call()`"), **`remoteIntegration:ghExec` nằm ngoài phạm vi solution này** — đây là việc của companion solution phía Agent.

Solution này chỉ implement 3 tracer còn lại: `remoteIntegration:credentialDecrypt`, `remoteIntegration:credentialStore`, `remoteIntegration:preflight` — cả 3 đều nằm trong `src/main/` (Main process / Backend Gateway).

### 1.2 Gap Analysis (verified qua Read trực tiếp code)

| Sub-flow | File:function (verified) | Hiện trạng | Việc cần làm | Layer |
|----------|---------------------------|-----------|---------------|-------|
| BL-INT-01 (phần Main) — đọc PAT | `src/main/project/GitProviderCredentialService.ts` — `getGitHubPAT()`, `getGitLabPAT()` (verified: gọi `store.getToken('bitbucket')`/`store.getToken('gitea')` — slot tái dùng, xem comment dòng 36-37 gốc) | Không tracer | Thêm `remoteIntegrationCredentialDecryptFlow` | Main (Backend) |
| BL-INT-01 (phần Main) — decrypt AES-256-GCM | `src/main/credentials/web-credential-store.ts:127` (`getToken()`) | Không tracer | `step('decrypt')` trong `getGitHubPAT()`/`getGitLabPAT()` (KHÔNG sửa `WebCredentialStore.getToken()` chính nó — instrument ở call site để tránh đổi shared low-level API) | Main (Backend) |
| BL-INT-01 (phần Dev Server) — `gh auth status`/`glab auth status` | `src/relay/external-api-connector.ts` — `handleGitHubAuthStatus()` (dòng 301), `handleGitLabAuthStatus()` (dòng 428) | Không tracer | **NGOÀI PHẠM VI** — chạy trên Dev Server, companion solution Agent xử lý | Dev Server (Agent) |
| BL-INT-02 — store/revoke token | `src/main/runtime/rpc/methods/credentials.ts` — verified: `credentials.set`, `credentials.revoke`, `credentials.status`, `credentials.list` (4 method, khớp TDD-00-index §F.6 "`credentials.*` 4 → WebCredentialStore (Backend) → TDD-11") | Không tracer | Thêm `remoteIntegrationCredentialStoreFlow` bọc `credentials.set`/`credentials.revoke` | Main (Backend) |
| BL-INT-03 — Preflight merge local+remote | `src/main/runtime/rpc/methods/preflight.ts` — `preflight.check` handler (verified full nội dung, xem mục 1.4) | Không tracer | Thêm `remoteIntegrationPreflightFlow` | Main (Backend) |

### 1.3 Phát hiện sai lệch giữa CR-TRACE-014 và code thực tế (đã verify, cần sửa trước khi implement)

CR-TRACE-014 mục 4 (`BL-INT-02`) đưa ra mẫu code dùng `ctx.credentialStore.setToken(...)` — verify thực tế cho thấy **`RpcContext` không có field `credentialStore`** (`src/main/runtime/rpc/core.ts:42-84`, đã đọc toàn bộ type, không có field này). Code thật của `credentials.set` dùng singleton import:

```typescript
// src/main/runtime/rpc/methods/credentials.ts (THỰC TẾ — đã verify)
import { isWebCredentialMode, getWebCredentialStore } from '../../../credentials'
// ...
handler: async (params, _ctx) => {
  if (!isWebCredentialMode()) { throw new Error(/* ... */) }
  const store = getWebCredentialStore()
  await store.setToken(params.service as CredentialService, params.token, params.config as Record<string, string> | undefined)
  return { success: true }
}
```

Solution này viết instrumentation dựa trên `getWebCredentialStore()` thực tế, KHÔNG theo mẫu `ctx.credentialStore` của CR gốc (mẫu đó là bản nháp "dự kiến", không phải code đã verify).

Ngoài ra, `ServiceEnum` thực tế trong `credentials.ts` chỉ gồm `['bitbucket', 'azure-devops', 'gitea', 'linear', 'jira']` — **không có `'github'`/`'gitlab'` trực tiếp** (khớp với phát hiện của CR-TRACE-014 mục 1 rằng `GitProviderCredentialService` "mượn" slot `bitbucket`/`gitea`). Field `service` trong `remoteIntegration:credentialStore` vì vậy sẽ luôn là 1 trong 5 giá trị enum này, không bao giờ literal `'github'`/`'gitlab'` — annotate rõ trong code để tránh nhầm lẫn khi đọc TracePanel.

### 1.4 `preflight.check` — xác nhận luồng thực tế (verified toàn bộ handler)

```typescript
// src/main/runtime/rpc/methods/preflight.ts (THỰC TẾ, đã đọc toàn bộ file — 75 dòng)
handler: async (params, ctx) => {
  if (params.devServerId && ctx.devServerManager) {
    const relay = ctx.devServerManager.getRelay(params.devServerId)
    if (!relay) { throw new Error(`Dev server '${params.devServerId}' relay is not connected. ...`) }
    const result = await relay.call<Record<string, unknown>>('preflight.check', {}, 30_000)
    return result
  }
  return runPreflightCheck(params.force)
}
```

Khớp với mô tả CR-TRACE-014 mục 4 (BL-INT-03) — không có sai lệch. `ctx.devServerManager` là optional field trên `RpcContext` (verified `core.ts`), inject bởi `RpcDispatcher` từ `ServerBootstrapResult.devServerManager` — chỉ có giá trị ở Web Server mode.

### 1.5 Ràng buộc bảo mật (bắt buộc, nhắc lại rõ ràng theo yêu cầu nhiệm vụ)

**Giá trị token/PAT đã giải mã KHÔNG BAO GIỜ được đưa vào bất kỳ `TraceFields` nào trong cả 2 tracer `remoteIntegration:credentialDecrypt` và `remoteIntegration:credentialStore`.** `TraceFields` hiển thị plaintext trong console/TracePanel không có redaction tự động (`serializeFields()`, `src/shared/trace/index.ts`) — đưa token vào field là lộ secret trực tiếp. Chỉ các field sau được phép:

| Tracer | Field cho phép |
|--------|-----------------|
| `remoteIntegration:credentialDecrypt` | `provider` (`'github'\|'gitlab'`), `userId`, `found` (boolean) |
| `remoteIntegration:credentialStore` | `service` (1 trong 5 giá trị `ServiceEnum`), `userId`, `errCode` |

Không field nào khác (đặc biệt: không `token`, không `params.token`, không object `config` thô — `config` có thể chứa `apiBaseUrl`/`email` là an toàn nhưng để tránh drift theo thời gian khi field mới được thêm vào `config`, solution này **không đưa `config` vào TraceFields**, chỉ đưa `service`).

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 2 tracer (không thêm `ghExec` — ngoài phạm vi)

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  browseDirFlow: createTracer('devServer:browseDir'),
  mkdirFlow:     createTracer('devServer:mkdir'),
  rmdirFlow:     createTracer('devServer:rmdir'),
  agentWsFlow:   createTracer('agentWs:lifecycle'),
  ipcProxyFlow:  createTracer('ipc:devServerProxy'),

  // ─── CR-TRACE-014: Remote Integration (Backend-side only) ─────────────────
  /** BL-INT-01 (phần Main): đọc + giải mã PAT cho gh/glab trước khi Dev Server
   *  dùng để build env cho CLI. KHÔNG bao gồm bước gh/glab auth status thật —
   *  đó là remoteIntegration:ghExec, chạy trên Dev Server (companion solution). */
  remoteIntegrationCredentialDecryptFlow: createTracer('remoteIntegration:credentialDecrypt'),
  /** BL-INT-02: store/revoke token qua credentials.set/credentials.revoke RPC */
  remoteIntegrationCredentialStoreFlow:   createTracer('remoteIntegration:credentialStore'),
  /** BL-INT-03: preflight check (local host hoặc relay-delegated) */
  remoteIntegrationPreflightFlow:         createTracer('remoteIntegration:preflight'),
} as const
```

`remoteIntegration:ghExec` **không được khai báo** trong solution này — companion solution phía Agent (`src/relay/agent-*.ts` scope) sẽ khai báo tracer này khi implement `handleGitHubAuthStatus()`/`handleGitLabAuthStatus()`.

### 2.2 `src/main/project/GitProviderCredentialService.ts` — BL-INT-01 (phần Main)

```typescript
// src/main/project/GitProviderCredentialService.ts
import type { WebCredentialStore } from '../credentials/web-credential-store'
import { Tracers } from '../../shared/trace/tracers'

export class GitProviderCredentialService {
  constructor(
    private readonly getUserStore: (userId: string) => WebCredentialStore
  ) {}

  // ── GitHub ──────────────────────────────────────────────────────────────────

  async setGitHubPAT(userId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.setToken('bitbucket', token, { provider: 'github', userId })
    // Note: reusing 'bitbucket' slot for github since WebCredentialStore is per-userId
  }

  async getGitHubPAT(userId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'github', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'github' })
    // FIX-note: slot 'bitbucket' tái dùng cho github (xem setGitHubPAT ở trên) —
    // KHÔNG đưa giá trị `token` trả về vào bất kỳ field nào của span.
    const token = await store.getToken('bitbucket')
    span.ok({ provider: 'github', found: token !== null })
    return token
  }

  async deleteGitHubPAT(userId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('bitbucket')
  }

  // ── GitLab ──────────────────────────────────────────────────────────────────

  async setGitLabPAT(userId: string, projectId: string, token: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.setToken('gitea', token, { provider: 'gitlab', userId, projectId })
  }

  async getGitLabPAT(userId: string, projectId: string): Promise<string | null> {
    const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'gitlab', userId })
    const store = this.getUserStore(userId)
    span.step('decrypt', { provider: 'gitlab' })
    const token = await store.getToken('gitea')
    span.ok({ provider: 'gitlab', found: token !== null })
    return token
  }

  async deleteGitLabPAT(userId: string, projectId: string): Promise<void> {
    const store = this.getUserStore(userId)
    await store.deleteToken('gitea')
  }
}
```

`getGitHubPAT`/`getGitLabPAT` không có `try/catch` hiện tại (verify: `WebCredentialStore.getToken()` trả `null` khi không tìm thấy thay vì throw — xem mục 2.3). Nếu `getToken()` throw (file `.enc` corrupt, decrypt fail), lỗi tự propagate lên caller — instrumentation ở đây không nuốt lỗi, chỉ thêm `try/catch` khi cần `span.fail()`:

```typescript
async getGitHubPAT(userId: string): Promise<string | null> {
  const span = Tracers.remoteIntegrationCredentialDecryptFlow.start({ provider: 'github', userId })
  const store = this.getUserStore(userId)
  span.step('decrypt', { provider: 'github' })
  try {
    const token = await store.getToken('bitbucket')
    span.ok({ provider: 'github', found: token !== null })
    return token
  } catch (err) {
    span.fail(err, { provider: 'github' })
    throw err
  }
}
```

Bản có `try/catch` là bản khuyến nghị dùng khi implement thật — bọc cả decrypt fail lẫn found/not-found trong 1 span hoàn chỉnh.

### 2.3 `src/main/runtime/rpc/methods/credentials.ts` — BL-INT-02

```typescript
// src/main/runtime/rpc/methods/credentials.ts
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { isWebCredentialMode, getWebCredentialStore } from '../../../credentials'
import type { CredentialService } from '../../../credentials/web-credential-store'
import { Tracers } from '../../../../shared/trace/tracers'

// ...ServiceEnum, SetTokenParams, ServiceParams, SAFE_CONFIG_FIELDS, sanitizeConfig unchanged...

export const CREDENTIAL_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'credentials.set',
    params: SetTokenParams,
    handler: async (params, ctx) => {
      const span = Tracers.remoteIntegrationCredentialStoreFlow.start({
        service: params.service, userId: ctx.userId
      })
      if (!isWebCredentialMode()) {
        const err = new Error(
          'credentials.set is only available in Web Server mode (ORCA_MULTI_USER=1). ' +
          'In Electron mode, use the native integration connect UI.'
        )
        span.fail(err, { service: params.service })
        throw err
      }
      try {
        const store = getWebCredentialStore()
        span.step('encryptWrite', { service: params.service })
        // KHÔNG đưa params.token vào bất kỳ field nào — chỉ dùng làm argument thực thi.
        await store.setToken(
          params.service as CredentialService,
          params.token,
          params.config as Record<string, string> | undefined
        )
        span.ok({ service: params.service })
        return { success: true }
      } catch (err) {
        span.fail(err, { service: params.service })
        throw err
      }
    }
  }),

  defineMethod({
    name: 'credentials.revoke',
    params: ServiceParams,
    handler: async (params, ctx) => {
      const span = Tracers.remoteIntegrationCredentialStoreFlow.start({
        service: params.service, userId: ctx.userId
      })
      if (!isWebCredentialMode()) {
        const err = new Error('credentials.revoke is only available in Web Server mode (ORCA_MULTI_USER=1).')
        span.fail(err, { service: params.service })
        throw err
      }
      try {
        const store = getWebCredentialStore()
        await store.deleteToken(params.service as CredentialService)
        span.ok({ service: params.service })
        return { success: true }
      } catch (err) {
        span.fail(err, { service: params.service })
        throw err
      }
    }
  }),

  // ...credentials.status, credentials.list KHÔNG instrument — chỉ đọc metadata
  // (sanitizeConfig), không encrypt/decrypt token, tần suất gọi cao (UI polling),
  // theo CR-TRACE-000 mục 5 (không step() cho các bước không băng qua boundary
  // và không có khả năng fail độc lập) — không đáng 1 span riêng.
]
```

`credentials.status`/`credentials.list` **không được instrument** — cả 2 chỉ đọc metadata đã sanitize (`SAFE_CONFIG_FIELDS`), không gọi `getToken()`/decrypt, và UI có thể poll các method này thường xuyên (over-instrumentation risk theo CR-TRACE-000 mục 5).

### 2.4 `src/main/runtime/rpc/methods/preflight.ts` — BL-INT-03

```typescript
// src/main/runtime/rpc/methods/preflight.ts
import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import {
  detectRemoteAgents, detectRemoteWindowsTerminalCapabilities,
  detectInstalledAgentsWithShellPathHydration, refreshShellPathAndDetectAgents,
  runPreflightCheck
} from '../../../ipc/preflight'
import { Tracers } from '../../../../shared/trace/tracers'

const PreflightCheck = z.object({
  force: z.boolean().optional(),
  devServerId: z.string().optional(),
  traceId: z.string().optional(),   // [NEW] CR-TRACE-000 §3.2 — resume span từ Browser
})
// ...PreflightDetectRemoteAgents, PreflightDetectRemoteWindowsTerminalCapabilities unchanged...

export const PREFLIGHT_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'preflight.check',
    params: PreflightCheck,
    handler: async (params, ctx) => {
      const span = Tracers.remoteIntegrationPreflightFlow.start(
        { devServerId: params.devServerId, force: params.force ?? false },
        params.traceId ? { id: params.traceId } : undefined
      )
      try {
        if (params.devServerId && ctx.devServerManager) {
          const relay = ctx.devServerManager.getRelay(params.devServerId)
          if (!relay) {
            span.fail('relay-not-connected', { devServerId: params.devServerId })
            throw new Error(
              `Dev server '${params.devServerId}' relay is not connected. ` +
              `Connect to the dev server before running preflight check.`
            )
          }
          span.step('relayDelegate', { devServerId: params.devServerId })
          // relay.call() bên dưới tự có span `relay:agentCall` riêng. Forward traceId
          // khi core API resume (CR-TRACE-000 mục 3) ship — tạm thời params rỗng như code gốc.
          const result = await relay.call<Record<string, unknown>>(
            'preflight.check',
            { traceId: span.id },
            30_000
          )
          span.ok({ devServerId: params.devServerId })
          return result
        }

        span.step('localCheck')
        const result = await runPreflightCheck(params.force)
        span.ok({ mode: 'local' })
        return result
      } catch (err) {
        // Tránh double-fail: nếu đã fail() ở nhánh 'relay-not-connected' phía trên,
        // exception đó (throw new Error(...)) rơi vào catch này lần nữa — dùng cờ
        // để không gọi fail() 2 lần cho cùng 1 outcome.
        if (!(err instanceof Error && err.message.startsWith(`Dev server '${params.devServerId}'`))) {
          span.fail(err, { devServerId: params.devServerId })
        }
        throw err
      }
    }
  }),
  defineMethod({
    name: 'preflight.detectAgents',
    params: null,
    handler: async () => detectInstalledAgentsWithShellPathHydration()
  }),
  defineMethod({
    name: 'preflight.detectRemoteAgents',
    params: PreflightDetectRemoteAgents,
    handler: async (params) => detectRemoteAgents(params)
  }),
  defineMethod({
    name: 'preflight.detectRemoteWindowsTerminalCapabilities',
    params: PreflightDetectRemoteWindowsTerminalCapabilities,
    handler: async (params) => detectRemoteWindowsTerminalCapabilities(params)
  }),
  defineMethod({
    name: 'preflight.refreshAgents',
    params: null,
    handler: async () => refreshShellPathAndDetectAgents()
  })
]
```

`preflight.detectAgents`/`detectRemoteAgents`/`detectRemoteWindowsTerminalCapabilities`/`refreshAgents` **không instrument** — đây là các sub-feature khác của preflight (agent detection, không phải BL-INT-03 "merge local+remote status"), ngoài phạm vi CR-TRACE-014 (CR chỉ định nghĩa 1 tracer `remoteIntegration:preflight` cho đúng `preflight.check`).

### 2.5 `relay.call('preflight.check', {traceId: span.id}, 30_000)` — thay đổi so với code gốc

Code gốc gọi `relay.call<Record<string, unknown>>('preflight.check', {}, 30_000)` với params rỗng. Solution này đổi thành `{ traceId: span.id }` để forward theo CR-TRACE-000 §3.3 hàng `relay.call()`. Đây là thay đổi behavior nhỏ nhất có thể — relay-side (`src/relay/preflight-handler.ts`, `PreflightHandler.checkFullPreflight()`) hiện không đọc field `traceId` trong params (companion Agent solution xử lý việc đọc nó nếu muốn resume span phía Dev Server); ở phía Backend, field thừa này không ảnh hưởng gì vì `checkFullPreflight()` hiện tại destructure params theo tên field cụ thể, bỏ qua field lạ.

---

## 3. Test Plan (Vitest)

### 3.1 File test mới

```
src/main/project/__tests__/GitProviderCredentialService-tracing.test.ts
src/main/runtime/rpc/methods/__tests__/credentials-tracing.test.ts
src/main/runtime/rpc/methods/__tests__/preflight-tracing.test.ts
```

### 3.2 Test cases

**`GitProviderCredentialService-tracing.test.ts`**
- `getGitHubPAT() — found`: mock `store.getToken('bitbucket')` trả token string → assert `start({provider:'github', userId})`, `step('decrypt')`, `ok({provider:'github', found:true})`; assert KHÔNG có field nào trong bất kỳ event nào chứa giá trị token thật (so sánh toàn bộ `fields` object với token value, phải không match)
- `getGitHubPAT() — not found`: mock trả `null` → assert `ok({found:false})`
- `getGitHubPAT() — store throws`: mock `getToken` reject → assert `fail(err, {provider:'github'})` trước khi re-throw
- `getGitLabPAT() — found/not found/throws`: 3 test tương tự, `provider:'gitlab'`

**`credentials-tracing.test.ts`**
- `credentials.set — success`: mock `getWebCredentialStore().setToken` resolve → assert `start({service, userId})`, `step('encryptWrite')`, `ok({service})`; assert `params.token` KHÔNG xuất hiện trong bất kỳ field nào của bất kỳ event nào
- `credentials.set — not web credential mode`: mock `isWebCredentialMode()` false → assert `fail(err, {service})` được gọi trước khi throw
- `credentials.set — setToken throws`: assert `fail(err, {service})`
- `credentials.revoke — success/not-web-mode/throws`: 3 test tương tự
- `credentials.status/list — không tạo span`: gọi 2 method này, assert KHÔNG có event nào với flow `remoteIntegration:credentialStore` phát sinh

**`preflight-tracing.test.ts`**
- `preflight.check — local mode`: `params.devServerId` undefined → assert `step('localCheck')`, `ok({mode:'local'})`
- `preflight.check — remote mode success`: mock `ctx.devServerManager.getRelay()` trả relay mock, `relay.call` resolve → assert `step('relayDelegate', {devServerId})`, `ok({devServerId})`, và `relay.call` được gọi với params chứa `traceId: span.id`
- `preflight.check — relay not connected`: mock `getRelay()` trả `undefined` → assert `fail('relay-not-connected', {devServerId})` được gọi TRƯỚC khi throw (không chỉ dựa vào exception ở caller)
- `preflight.check — relay.call rejects`: mock `relay.call` reject với lỗi khác (không phải relay-not-connected) → assert `fail(err, {devServerId})` được gọi đúng 1 lần (không double-fail với nhánh relay-not-connected)
- `preflight.check — traceId resume`: gọi với `params.traceId='xyz'` → assert `span.id === 'xyz'`

### 3.3 Test Targets

| Test file | Target số test |
|-----------|---------------|
| `GitProviderCredentialService-tracing.test.ts` | ≥ 6 |
| `credentials-tracing.test.ts` | ≥ 7 |
| `preflight-tracing.test.ts` | ≥ 5 |
| **Total** | **≥ 18** |

---

## 4. Acceptance Criteria

- [ ] `remoteIntegration:credentialDecrypt` và `remoteIntegration:credentialStore` không bao giờ chứa giá trị token/PAT plaintext trong bất kỳ field nào — chỉ `provider`/`service`/`userId`/`found`/`errCode` (verify bằng test assertion so khớp field values với input token, phải không trùng)
- [ ] `remoteIntegration:preflight` phân biệt rõ `mode: 'local'` (từ `step('localCheck')`/`ok({mode:'local'})`) vs relay-delegated (`devServerId` có giá trị trong `ok()`)
- [ ] Khi relay không connected, `remoteIntegration:preflight` gọi `fail()` với `reason: 'relay-not-connected'` (thực tế field key là literal string thứ nhất của `span.fail('relay-not-connected', ...)`) TRƯỚC khi throw, không chỉ dựa vào exception phía caller
- [ ] `relay.call('preflight.check', { traceId: span.id }, 30_000)` được cập nhật để gửi `traceId` trong params (thay vì params rỗng như code gốc)
- [ ] `credentials.status`/`credentials.list` KHÔNG có tracer mới — xác nhận qua code review, chỉ 2 RPC method ghi (`set`/`revoke`) được instrument
- [ ] `remoteIntegration:ghExec` KHÔNG được khai báo hoặc wire trong solution này — thuộc phạm vi companion solution phía Agent (`src/relay/external-api-connector.ts`)
- [ ] Không tracer nào trong solution này trùng tên với `agent:ext-api` (đã có, trace `github.pr.create`/`github.pr.merge` trong `external-api-connector.ts`) hoặc `agent:credential` (đã có, trace AI provider credential — CR-TRACE-016)
- [ ] Xác nhận lại rằng `CliAuthProxy`/`credential.request`-`credential.response` request-response bridge (mô tả trong flow doc) thực sự không tồn tại trong `src/main/`/`src/relay/` — nếu phát hiện tồn tại ở nơi khác, cập nhật solution trước khi coi BL-INT-01 là "chỉ có 2 layer độc lập không nối được"

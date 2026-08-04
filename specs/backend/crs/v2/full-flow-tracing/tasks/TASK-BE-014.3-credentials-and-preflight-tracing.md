# TASK-BE-014.3: Instrument `credentials.set`/`credentials.revoke` (BL-INT-02) và `preflight.check` (BL-INT-03)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-014](../solutions/SOL-BE-TRACE-014-remote-integration.md) §2.3, §2.4, §2.5
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-014.1
**Status:** ✅ Done (2026-08-04) — `credentials.set`/`credentials.revoke` instrumented with `getWebCredentialStore()` singleton (confirmed no `ctx.credentialStore` in real code); `preflight.check` instrumented with local/relay-delegated branching, `traceId` added to `PreflightCheck` zod schema and forwarded to `relay.call`. No drift beyond what CR-TRACE-014 already flagged. `pnpm run typecheck:node` clean for these files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "credentials.set"
codegraph explore "credentials.revoke"
codegraph explore "preflight.check"
```

Cả 3 là RPC handler đã tồn tại (MODIFY case). Chạy:

```
gitnexus_impact({ target: "credentials.set", direction: "upstream" })
gitnexus_impact({ target: "preflight.check", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận dùng đúng `getWebCredentialStore()` singleton (không phải `ctx.credentialStore` như CR gốc mô tả sai). Tuyệt đối không đưa `params.token` vào trace field. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc 2 RPC method ghi credential (`credentials.set`, `credentials.revoke` trong `src/main/runtime/rpc/methods/credentials.ts`) bằng `Tracers.remoteIntegrationCredentialStoreFlow`, và bọc `preflight.check` (`src/main/runtime/rpc/methods/preflight.ts`) bằng `Tracers.remoteIntegrationPreflightFlow` với forward `traceId`.

**Sửa lệch quan trọng so với CR gốc:** CR-TRACE-014 mục 4 (BL-INT-02) đưa ra mẫu code dùng `ctx.credentialStore.setToken(...)` — đã verify thực tế `RpcContext` (`src/main/runtime/rpc/core.ts:42-84`) **không có field `credentialStore`**. Code thật của `credentials.set`/`credentials.revoke` dùng singleton import `getWebCredentialStore()` từ `../../../credentials`. Task này **PHẢI dùng đúng pattern singleton thật** dưới đây — không được viết theo mẫu `ctx.credentialStore` sai của CR gốc.

`credentials.status`/`credentials.list` **KHÔNG được instrument** — cả 2 chỉ đọc metadata đã sanitize, không gọi `getToken()`/decrypt, và UI có thể poll các method này thường xuyên (over-instrumentation risk theo CR-TRACE-000 §5).

`preflight.detectAgents`/`detectRemoteAgents`/`detectRemoteWindowsTerminalCapabilities`/`refreshAgents` **KHÔNG được instrument** — ngoài phạm vi CR-TRACE-014 (CR chỉ định nghĩa 1 tracer `remoteIntegration:preflight` cho đúng `preflight.check`).

## File: `src/main/runtime/rpc/methods/credentials.ts` [MODIFY]

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

**Ràng buộc bảo mật (bắt buộc):** `params.token` **KHÔNG BAO GIỜ** được đưa vào bất kỳ field nào của `remoteIntegrationCredentialStoreFlow`. `ServiceEnum` thực tế chỉ gồm `['bitbucket', 'azure-devops', 'gitea', 'linear', 'jira']` — field `service` luôn là 1 trong 5 giá trị này, không bao giờ literal `'github'`/`'gitlab'` trực tiếp (do `GitProviderCredentialService` mượn slot, xem TASK-BE-014.2) — annotate rõ trong code để tránh nhầm lẫn khi đọc TracePanel. Không đưa object `config` thô vào field (dù `config` hiện chỉ chứa `apiBaseUrl`/`email` an toàn — tránh drift khi field mới được thêm vào `config` sau này); chỉ đưa `service`.

## File: `src/main/runtime/rpc/methods/preflight.ts` [MODIFY]

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

Đây là 1 thay đổi behavior nhỏ so với code gốc: `relay.call('preflight.check', {}, 30_000)` (params rỗng) → `relay.call('preflight.check', { traceId: span.id }, 30_000)`. Relay-side (`src/relay/preflight-handler.ts`, `PreflightHandler.checkFullPreflight()`) hiện không đọc field `traceId` trong params (companion Agent solution xử lý việc đọc nó) — field thừa này không ảnh hưởng gì vì `checkFullPreflight()` destructure params theo tên field cụ thể, bỏ qua field lạ.

**Ràng buộc bắt buộc:**
- Dùng đúng pattern `getWebCredentialStore()` singleton — KHÔNG dùng `ctx.credentialStore` (mẫu sai của CR gốc).
- `credentials.status`/`credentials.list` không được thêm tracer.
- `preflight.detectAgents`/`detectRemoteAgents`/`detectRemoteWindowsTerminalCapabilities`/`refreshAgents` không được thêm tracer.
- Không đưa `params.token` hoặc bất kỳ giá trị credential/config thô nào vào `TraceFields`.

## Verification

```bash
pnpm run typecheck:node
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `credentials.set`/`credentials.revoke` bọc bằng `Tracers.remoteIntegrationCredentialStoreFlow`, dùng `getWebCredentialStore()` singleton (không phải `ctx.credentialStore`)
- [ ] `remoteIntegration:credentialStore` không bao giờ chứa giá trị token/PAT plaintext hay object `config` thô — chỉ `service`/`userId`/`errCode`
- [ ] `credentials.status`/`credentials.list` KHÔNG có tracer mới — xác nhận qua code review, chỉ 2 RPC method ghi (`set`/`revoke`) được instrument
- [ ] `remoteIntegration:preflight` phân biệt rõ `mode: 'local'` (`step('localCheck')`/`ok({mode:'local'})`) vs relay-delegated (`devServerId` có giá trị trong `ok()`)
- [ ] Khi relay không connected, `remoteIntegration:preflight` gọi `fail('relay-not-connected', ...)` TRƯỚC khi throw, không double-fail ở catch ngoài
- [ ] `relay.call('preflight.check', { traceId: span.id }, 30_000)` được cập nhật để gửi `traceId` trong params (thay vì params rỗng như code gốc)
- [ ] `preflight.detectAgents`/`detectRemoteAgents`/`detectRemoteWindowsTerminalCapabilities`/`refreshAgents` không có tracer mới
- [ ] `pnpm run typecheck:node` pass, không lỗi mới

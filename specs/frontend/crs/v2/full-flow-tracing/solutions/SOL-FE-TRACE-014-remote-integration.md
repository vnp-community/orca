# SOL-FE-TRACE-014: Remote Integration — Frontend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**TDD Ref:** TDD-FE-16 (Remote Git UI — `16-remote-git-ui.md`), TDD-FE-09 (Onboarding — Integrations step)
**Status:** Proposed
**Dependency:** F40 core tracing infra (đã implement) — `src/shared/trace/browser.ts`, `src/shared/trace/tracers.ts`, TracePanel. CR-TRACE-000 (naming convention, quy ước field `traceId`).

---

## 1. Điểm khởi tạo trace trong Renderer

### 1.1 BL-INT-01 + BL-INT-03 hội tụ vào CÙNG một nút bấm: "Re-check" trên Integration Card

CR-TRACE-014 (backend) đã tự kết luận rằng `CliAuthProxy`/`credential.request` round-trip mô tả trong flow doc **không tồn tại** — cơ chế thật là `gh`/`glab` CLI tự quản lý auth state trên Dev Server, và Orca chỉ *kiểm tra* trạng thái đó qua `preflight.check`. Grep trên renderer xác nhận đúng điều này, đồng thời chỉ ra rằng BL-INT-01 (auth status check) và BL-INT-03 (preflight) **dùng chung một entry point UI duy nhất** — không phải hai luồng tách biệt như flow doc gốc mô tả:

**Entry point UI thật:** nút "Re-check" trong `GitHubIntegrationCard`/`GitLabIntegrationCard` (`src/renderer/src/components/settings/cli-source-control-integration-cards.tsx:38-165,167-298`) → `usePreflightCardStatuses('gh' | 'glab')` (`source-control-preflight-card-status.ts:53-95`) → `refresh()` (dòng 81-88) → `useAppStore().refreshPreflightStatus({ force: true })` — action Zustand thật tại `src/renderer/src/store/slices/preflight.ts:78-115`.

`refreshPreflightStatus()` là **action dùng chung** cho toàn bộ preflight (không riêng gh/glab) — rẽ nhánh:
- `runtimeTarget.kind === 'environment'` (web/remote): `callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', params)` (dòng 112) — **method có thật**, đã xác nhận đăng ký tại `src/main/runtime/rpc/methods/preflight.ts`
- `runtimeTarget.kind === 'local'` (desktop): `window.api.preflight.check(preflightArgs)` (dòng 114) — Electron IPC, cùng máy

Hai component card này được mount thật ở 2 nơi: `IntegrationsPane.tsx` (Settings) và `ConnectIntegrationsList.tsx` (feature-wall onboarding) — xác nhận qua grep `GitHubIntegrationCard|GitLabIntegrationCard`.

**Các trigger tự động khác (cùng action, không phải nút bấm):** `Landing.tsx:245,285` (`window.api.preflight.check(...)` khi mở app / user bấm "Try again"), `TaskPage.tsx:3394`, `AutomationsPage.tsx:834` — tất cả đều đi qua cùng RPC `preflight.check`, nên instrument tại `refreshPreflightStatus()` (action dùng chung) tự động bao phủ mọi entry point này, theo đúng pattern đã dùng ở SOL-FE-TRACE-001 (đặt span ở action Zustand dùng chung, không đặt lặp lại ở từng call site UI).

### 1.2 BL-INT-02 (WebCredentialStore) — code thật, KHÔNG mount

`src/renderer/src/components/settings/CredentialInputForm.tsx` — form thật gọi `window.api.credentials.set(service, token, config)` (dòng 68) và `window.api.credentials.revoke(service)` (dòng 86), đúng với `credentials.set`/`credentials.revoke` RPC method mà CR-TRACE-014 backend mô tả. Nhưng: đã grep `CredentialInputForm` trên toàn bộ `src/renderer/src` (loại trừ chính nó và test) — **0 kết quả** mount component này ở đâu cả.

`CredentialService` type trong file này (dòng 14) là `'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'` — **không có `'github'`/`'gitlab'` trực tiếp**, khớp đúng ghi chú của backend CR rằng `GitProviderCredentialService` "mượn" slot `bitbucket` cho GitHub PAT và `gitea` cho GitLab PAT. Điều này nghĩa là: kể cả khi `CredentialInputForm` được mount trong tương lai, UI sẽ hiển thị nhãn "Bitbucket"/"Gitea" — không có form nào literally nói "Nhập GitHub Personal Access Token" trong renderer hiện tại.

Instrumentation cho `handleSave()`/`handleRevoke()` vẫn được viết đầy đủ ở mục 2.3 (code thật, cụ thể, không bịa), nhưng đánh dấu rõ trong Acceptance Criteria rằng span này chưa reachable.

### 1.3 BL-INT-01 phần "gh/glab CLI thô" — không có entry point browser riêng

Backend CR-TRACE-014 định nghĩa thêm tracer `remoteIntegrationGhExecFlow` cho phần exec `gh auth status`/`glab auth status` **trên Dev Server** (`src/relay/external-api-connector.ts`). Đây là code chạy trong relay process, được kích hoạt gián tiếp bởi cùng request `preflight.check` ở mục 1.1 (qua `PreflightHandler.checkFullPreflight()`) — không có lời gọi trực tiếp nào từ renderer tới `gh`/`glab` CLI. Do đó phía renderer **không cần** một tracer riêng cho phần này — `Tracers.remoteIntegrationPreflightFlow` (mục 2.1) là điểm khởi tạo duy nhất phía browser; phần `ghExec`/`credentialDecrypt` là span riêng của backend/relay, tự nối tiếp qua `traceId` forward trong `relay.call('preflight.check', { traceId: span.id })` (companion backend CR chịu trách nhiệm forward tiếp).

---

## 2. Full Implementation

### 2.1 Thêm tracer mới vào `tracers.ts`

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged...
  remoteIntegrationPreflightFlow:       createTracer('ui:remoteIntegration.preflight'),       // BL-INT-01 + BL-INT-03 (renderer entry point chung)
  remoteIntegrationCredentialStoreFlow: createTracer('ui:remoteIntegration.credentialStore'), // BL-INT-02
} as const
```

> `remoteIntegration:ghExec`/`remoteIntegration:credentialDecrypt` (BL-INT-01 phần backend/relay) do companion backend solution định nghĩa — renderer không tạo, chỉ forward `traceId` để chúng resume đúng (mục 1.3).

### 2.2 `refreshPreflightStatus()` — `src/renderer/src/store/slices/preflight.ts`

```typescript
// src/renderer/src/store/slices/preflight.ts
import { Tracers } from '../../../../shared/trace/tracers'

// ...existing types/helpers unchanged...

refreshPreflightStatus: async (options) => {
  const force = options?.force === true
  const context = getLocalPreflightContext(get())
  const contextKey = localPreflightContextKey(context)
  if (!force && forcedPreflightRequest?.key === contextKey) {
    return forcedPreflightRequest.promise
  }
  if (!force && nonForcedPreflightRequest?.key === contextKey) {
    return nonForcedPreflightRequest.promise
  }
  if (force && forcedPreflightRequest?.key === contextKey) {
    return forcedPreflightRequest.promise
  }

  const requestId = ++latestPreflightRequestId
  const contextChanged = get().preflightStatusContextKey !== contextKey
  const runtimeTarget = getActiveRuntimeTarget(get().settings)
  const preflightArgs = buildPreflightArgs(force, context)
  set({
    preflightStatus: contextChanged ? null : get().preflightStatus,
    preflightStatusChecked: contextChanged ? false : get().preflightStatusChecked,
    preflightStatusLoading: true,
    preflightStatusError: null
  })

  // Why: span bọc toàn bộ request kể cả khi bị coalesce bởi
  // forcedPreflightRequest/nonForcedPreflightRequest guard phía trên — mỗi
  // request THẬT (không bị return sớm ở 3 nhánh guard) là 1 user-perceived
  // "check" action, dù trigger từ nút Re-check hay Landing.tsx tự động.
  const span = Tracers.remoteIntegrationPreflightFlow.start({
    force, mode: runtimeTarget.kind,
  })

  const request = (
    runtimeTarget.kind === 'environment'
      ? (() => {
          const activeDevServerId = get().activeDevServerId
          const params: Record<string, unknown> = force ? { force } : {}
          if (activeDevServerId) params.devServerId = activeDevServerId
          params.traceId = span.id
          span.step('relayDelegate', { devServerId: activeDevServerId ?? '' })
          return callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', params)
        })()
      : window.api.preflight.check(preflightArgs)
  )
    .then((status) => {
      if (requestId !== latestPreflightRequestId) {
        span.ok({ stale: true })
        return
      }
      set({
        preflightStatus: status,
        preflightStatusChecked: true,
        preflightStatusContextKey: contextKey,
        preflightStatusLoading: false,
        preflightStatusError: null
      })
      span.ok({
        ghAuthenticated: Boolean((status as { ghStatus?: { authenticated?: boolean } })?.ghStatus?.authenticated),
        glabAuthenticated: Boolean((status as { glabStatus?: { authenticated?: boolean } })?.glabStatus?.authenticated),
      })
    })
    .catch((error) => {
      if (requestId !== latestPreflightRequestId) {
        span.fail(error, { stale: true })
        return
      }
      set({
        preflightStatusChecked: true,
        preflightStatusContextKey: contextKey,
        preflightStatusLoading: false,
        preflightStatusError: getErrorMessage(error)
      })
      span.fail(error, { force, mode: runtimeTarget.kind })
      throw error
    })

  // ...existing forcedPreflightRequest/nonForcedPreflightRequest bookkeeping unchanged...
  return request
}
```

> `traceId: span.id` chỉ thêm vào nhánh `runtimeTarget.kind === 'environment'` (đi qua WebSocket RPC tới Orca Server, đúng CR-TRACE-000 §3.3 hàng "WebSocket RPC"). Nhánh `window.api.preflight.check(...)` là Electron IPC cùng máy — span vẫn bọc để đo latency phía renderer, nhưng không có `traceId` để forward (không băng qua network boundary nào cần liên kết).

### 2.3 `CredentialInputForm.tsx` — BL-INT-02 (code thật, chưa reachable — xem 1.2)

```typescript
// src/renderer/src/components/settings/CredentialInputForm.tsx
import { Tracers } from '../../../../shared/trace/tracers'

// ...existing state unchanged...

const handleSave = async () => {
  const missing = fields.filter(f => f.required && !values[f.key]?.trim())
  if (missing.length > 0) {
    setError(`Required: ${missing.map(f => f.label).join(', ')}`)
    return
  }

  setSaving(true)
  setError(null)
  // Why: span bọc trước validate token/config để cover cả case validate fail
  // — nhưng KHÔNG đưa token/config vào TraceFields (bảo mật, xem CR-TRACE-014 §4).
  const span = Tracers.remoteIntegrationCredentialStoreFlow.start({ service, op: 'set' })
  try {
    const tokenKey = fields.find(f => f.type === 'password')?.key ?? 'token'
    const token = values[tokenKey] ?? ''

    const config: Record<string, string> = {}
    for (const field of fields) {
      if (field.key !== tokenKey && values[field.key]?.trim()) {
        config[field.key] = values[field.key].trim()
      }
    }

    await window.api.credentials.set(service, token, Object.keys(config).length ? config : undefined)

    setValues({})
    setSaved(true)
    setTimeout(() => setSaved(false), 3000)
    onSaved()
    span.ok({ service })
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to save credentials')
    span.fail(err, { service })
  } finally {
    setSaving(false)
  }
}

const handleRevoke = async () => {
  if (!confirm(`Remove ${service} credentials? This cannot be undone.`)) return
  setRevoking(true)
  const span = Tracers.remoteIntegrationCredentialStoreFlow.start({ service, op: 'revoke' })
  try {
    await window.api.credentials.revoke(service)
    onRevoked()
    span.ok({ service })
  } catch (err) {
    setError(err instanceof Error ? err.message : 'Failed to revoke credentials')
    span.fail(err, { service })
  } finally {
    setRevoking(false)
  }
}
```

> **Ràng buộc bảo mật (bắt buộc, kế thừa từ CR-TRACE-014 §4 phía backend):** `token`/`config` (raw PAT/API key) **không bao giờ** được đưa vào `TraceFields` của `Tracers.remoteIntegrationCredentialStoreFlow` — chỉ `service`/`op`. Đây là quy tắc áp dụng cả 2 phía browser và backend vì `serializeFields()` (`src/shared/trace/index.ts:98-106`) không có redaction tự động, và browser console log (`ORCA_TRACE=1` trong localStorage) hiển thị field y nguyên cho bất kỳ ai mở DevTools.

`window.api.credentials.set/revoke` là Electron `contextBridge` IPC (không phải `callRuntimeRpc`) — không có `traceId` để forward theo quy ước WS RPC của CR-TRACE-000 §3.3; span chỉ đo latency phía renderer, không liên kết được với span backend nào trừ khi Main process tự log riêng.

---

## 3. Test Plan (Vitest)

```
src/renderer/src/store/slices/__tests__/preflight.test.ts   (file đã tồn tại — thêm test case)
├── refreshPreflightStatus() gọi Tracers.remoteIntegrationPreflightFlow.start({ force, mode }) trước khi gọi RPC/IPC
├── runtimeTarget.kind === 'environment' → span.step('relayDelegate', { devServerId }) trước callRuntimeRpc
├── traceId: span.id có trong params của callRuntimeRpc('preflight.check', ...) khi mode === 'environment'
├── KHÔNG có traceId khi runtimeTarget.kind === 'local' (nhánh window.api.preflight.check)
├── request coalesce bởi forcedPreflightRequest/nonForcedPreflightRequest KHÔNG tạo span mới (chỉ request thật mới start())
├── thành công → span.ok({ ghAuthenticated, glabAuthenticated })
├── stale response (requestId !== latestPreflightRequestId) → span.ok({ stale: true }), không ghi đè state
└── lỗi → span.fail(error, { force, mode })

src/renderer/src/components/settings/__tests__/cli-source-control-integration-cards.test.tsx   (file đã tồn tại — thêm test case)
├── click "Re-check" trên GitHubIntegrationCard → gọi refreshPreflightStatus({ force: true }) (gián tiếp trigger span)
└── click "Re-check" trên GitLabIntegrationCard → cùng hành vi, khác provider label

src/renderer/src/components/settings/__tests__/CredentialInputForm.test.tsx   (mới)
├── handleSave() thành công → Tracers.remoteIntegrationCredentialStoreFlow.start({ service, op: 'set' }) rồi ok()
├── handleSave() reject → span.fail(err, { service }) — KHÔNG chứa token/config trong fields (assert bằng cách kiểm tra object keys của fail() call)
├── handleRevoke() thành công → span với op: 'revoke' → ok()
└── validate fail (missing required field) → KHÔNG tạo span (return sớm trước Tracers.start())

src/shared/trace/__tests__/tracers.test.ts   (file đã tồn tại — thêm assertion)
└── Tracers.remoteIntegrationPreflightFlow/CredentialStoreFlow tồn tại đúng flow name 'ui:remoteIntegration.preflight|credentialStore'
```

**Mock pattern:** dùng `vi.spyOn(Tracers.remoteIntegrationCredentialStoreFlow, 'fail')` để assert trực tiếp field passed vào `fail()` không chứa `token`/`config`/giá trị input nhạy cảm nào — test bảo mật này nên chạy trong mọi PR review liên quan tới credential UI, không chỉ CR này.

**Target:** ≥ 14 test case mới.

---

## 4. Acceptance Criteria

- [ ] `Tracers.remoteIntegrationPreflightFlow`/`Tracers.remoteIntegrationCredentialStoreFlow` được thêm vào `src/shared/trace/tracers.ts` đúng tên `ui:remoteIntegration.preflight|credentialStore`
- [ ] `refreshPreflightStatus()` (`preflight.ts:78`) là điểm instrument DUY NHẤT cho BL-INT-01+BL-INT-03 phía renderer — không thêm span trùng lặp ở `cli-source-control-integration-cards.tsx`, `Landing.tsx`, `TaskPage.tsx`, hay `AutomationsPage.tsx` (đều gọi chung action này)
- [ ] `traceId: span.id` chỉ được thêm vào params khi `runtimeTarget.kind === 'environment'` (WebSocket RPC), không thêm vào nhánh `window.api.preflight.check` (Electron IPC cùng máy)
- [ ] Request bị coalesce bởi `forcedPreflightRequest`/`nonForcedPreflightRequest` guard (3 nhánh return sớm đầu hàm) KHÔNG tạo span mới — chỉ request thực sự đi tới RPC/IPC mới `start()`
- [ ] `Tracers.remoteIntegrationCredentialStoreFlow` không BAO GIỜ chứa `token`/`config`/giá trị credential trong bất kỳ field nào của `start()`/`ok()`/`fail()` — chỉ `service`/`op`
- [ ] `CredentialInputForm.tsx` được instrument đầy đủ dù hiện KHÔNG mount ở đâu trong app (xác nhận qua grep) — Acceptance Criteria không yêu cầu span này thực sự emit cho tới khi có companion CR mount component
- [ ] Không tạo tracer `remoteIntegration:ghExec`/`remoteIntegration:credentialDecrypt` ở phía renderer — hai tracer này thuộc companion backend/relay solution, renderer chỉ forward `traceId` qua `preflight.check`
- [ ] Test suite xác nhận field bảo mật (không leak token) bằng assertion trực tiếp trên object truyền vào `span.fail()`/`span.ok()`, không chỉ kiểm tra UI không hiển thị token

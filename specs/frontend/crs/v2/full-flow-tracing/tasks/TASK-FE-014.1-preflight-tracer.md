# TASK-FE-014.1: Đăng ký tracer remote integration + instrument `refreshPreflightStatus()`

**Phase:** 2
**SOL Ref:** [SOL-FE-TRACE-014 §1.1, §2.1, §2.2](../solutions/SOL-FE-TRACE-014-remote-integration.md)
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001)
**Status:** ✅ Done (2026-08-04) — implemented; key names drifted from spec sample due to a real collision with concurrent backend tracer entries (see note below)

---

## Drift ghi nhận khi thực thi (2026-08-04)

- **Key name collision:** `tracers.ts` hiện tại (chỉnh sửa bởi companion backend task chạy song song) đã claim sẵn key `remoteIntegrationPreflightFlow`/`remoteIntegrationCredentialStoreFlow` cho tracer BACKEND (`remoteIntegration:preflight`/`remoteIntegration:credentialStore`, xem block "CR-TRACE-014: Remote Integration (Backend-side only)"). Theo quy tắc additive-only/no-rename, 2 tracer renderer mới trong task này dùng key **`uiRemoteIntegrationPreflightFlow`**/**`uiRemoteIntegrationCredentialStoreFlow`** thay vì tên gốc trong spec — cùng pattern `ui` prefix đã dùng cho `uiWorktreeCreateFlow`/`uiAgentOrchSpawnFlow`/`uiTerminalCreateFlow`. Flow name string (`ui:remoteIntegration.preflight`/`ui:remoteIntegration.credentialStore`) không đổi, chỉ JS property key đổi. Toàn bộ call site trong task này (`preflight.ts`, `CredentialInputForm.tsx` ở TASK-FE-014.2) dùng tên mới.
- **`status.ghStatus`/`status.glabStatus` không tồn tại:** code mẫu trong spec dùng `(status as {ghStatus...}).ghStatus?.authenticated`, nhưng `PreflightStatus` thật (`src/preload/api-types.ts:592-598`) có field `gh`/`glab`, không phải `ghStatus`/`glabStatus`. Implementation dùng `status?.gh?.authenticated`/`status?.glab?.authenticated`.
- **KHÔNG thêm `throw error`** sau `span.fail()` trong nhánh `.catch()` — code hiện tại (trước khi sửa) không rethrow, chỉ set state; spec sample thêm `throw error` nhưng đó sẽ là thay đổi hành vi ngoài phạm vi "chỉ thêm tracing" (ảnh hưởng 5 caller thật của `refreshPreflightStatus`). Giữ nguyên hành vi không rethrow.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "refreshPreflightStatus"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "refreshPreflightStatus", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

BL-INT-01 (auth status check) và BL-INT-03 (preflight) **dùng chung một entry point UI duy nhất** — nút "Re-check" trong `GitHubIntegrationCard`/`GitLabIntegrationCard` → `usePreflightCardStatuses()` → `refreshPreflightStatus({ force: true })` (`preflight.ts:78-115`), action Zustand dùng chung. Các trigger tự động khác (`Landing.tsx`, `TaskPage.tsx`, `AutomationsPage.tsx`) đều đi qua cùng RPC `preflight.check`, nên instrument tại action dùng chung tự động bao phủ mọi entry point — không đặt span lặp lại ở từng call site UI.

`CliAuthProxy`/`credential.request` round-trip mô tả trong flow doc gốc **không tồn tại** — `gh`/`glab` CLI tự quản lý auth state trên Dev Server, Orca chỉ kiểm tra qua `preflight.check`.

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  remoteIntegrationPreflightFlow:       createTracer('ui:remoteIntegration.preflight'),       // BL-INT-01 + BL-INT-03
  remoteIntegrationCredentialStoreFlow: createTracer('ui:remoteIntegration.credentialStore'), // BL-INT-02 — dùng ở TASK-FE-014.2
} as const
```

> N.B. prefix `ui:`: bắt buộc theo convention chung (xem các task Phase 1/2 trước, `00-index.md` mục 1) — 2 tracer trên dùng prefix `ui:` nhất quán với toàn bộ 10 CR. `remoteIntegration:ghExec`/`remoteIntegration:credentialDecrypt` (phần backend/relay exec `gh`/`glab` CLI) do companion backend solution định nghĩa — renderer KHÔNG tạo tracer này, chỉ forward `traceId` để chúng resume đúng.

## File: `src/renderer/src/store/slices/preflight.ts` [MODIFY]

```typescript
import { Tracers } from '../../../../shared/trace/tracers'

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

  // Why: span bọc toàn bộ request kể cả khi bị coalesce bởi 3 guard phía trên —
  // mỗi request THẬT (không return sớm) là 1 user-perceived "check" action.
  const span = Tracers.remoteIntegrationPreflightFlow.start({ force, mode: runtimeTarget.kind })

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
        preflightStatus: status, preflightStatusChecked: true, preflightStatusContextKey: contextKey,
        preflightStatusLoading: false, preflightStatusError: null
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
        preflightStatusChecked: true, preflightStatusContextKey: contextKey,
        preflightStatusLoading: false, preflightStatusError: getErrorMessage(error)
      })
      span.fail(error, { force, mode: runtimeTarget.kind })
      throw error
    })

  // ...existing forcedPreflightRequest/nonForcedPreflightRequest bookkeeping unchanged...
  return request
}
```

> `traceId: span.id` chỉ thêm vào nhánh `runtimeTarget.kind === 'environment'` (WebSocket RPC). Nhánh `window.api.preflight.check(...)` là Electron IPC cùng máy — span vẫn bọc để đo latency phía renderer, nhưng không có `traceId` để forward.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/store/slices/__tests__/preflight.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.remoteIntegrationPreflightFlow`/`Tracers.remoteIntegrationCredentialStoreFlow` thêm vào `tracers.ts` đúng tên `ui:remoteIntegration.preflight|credentialStore`
- [ ] `refreshPreflightStatus()` là điểm instrument DUY NHẤT cho BL-INT-01+03 phía renderer — không thêm span trùng lặp ở `cli-source-control-integration-cards.tsx`, `Landing.tsx`, `TaskPage.tsx`, hay `AutomationsPage.tsx`
- [ ] `traceId: span.id` chỉ thêm vào params khi `runtimeTarget.kind === 'environment'` (WebSocket RPC), không thêm vào nhánh `window.api.preflight.check` (Electron IPC)
- [ ] Request bị coalesce bởi 3 nhánh guard đầu hàm KHÔNG tạo span mới — chỉ request thực sự đi tới RPC/IPC mới `start()`
- [ ] Không tạo tracer `remoteIntegration:ghExec`/`remoteIntegration:credentialDecrypt` ở phía renderer
- [ ] Test suite đạt ≥ 8 test case mới theo Test Plan của SOL-FE-TRACE-014 §3 (start trước RPC/IPC, `relayDelegate` step, `traceId` chỉ khi environment, không traceId khi local, coalesce không tạo span mới, `ok`/`fail`/`stale`)

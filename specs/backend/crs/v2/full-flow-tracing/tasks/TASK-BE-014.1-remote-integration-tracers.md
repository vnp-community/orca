# TASK-BE-014.1: Thêm 3 tracer cho Remote Integration vào `tracers.ts`

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-014](../solutions/SOL-BE-TRACE-014-remote-integration.md) §2.1
**CR Ref:** [CR-TRACE-014](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-014-remote-integration.md)
**Prerequisite:** Phase 0 (TASK-BE-000)
**Status:** ✅ Done (2026-08-04) — 3 tracers appended to `Tracers` (`remoteIntegrationCredentialDecryptFlow`, `remoteIntegrationCredentialStoreFlow`, `remoteIntegrationPreflightFlow`); no drift from doc, `pnpm run typecheck:node` clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` là object đã tồn tại (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

Task này chỉ thêm 3 entry mới, không đổi entry cũ (đặc biệt `agent:ext-api`, `agent:credential`). Fan-in lớn của `Tracers` là bình thường; chỉ dừng lại nếu risk HIGH/CRITICAL đến từ nguyên nhân khác, xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 3 tracer mới trong `src/shared/trace/tracers.ts` cho phần Backend/Main-process của Remote Integration (credential decrypt, credential store, preflight). CR-TRACE-014 định nghĩa 4 tracer tổng cộng, nhưng `remoteIntegration:ghExec` chạy trên Dev Server (`src/relay/external-api-connector.ts`) — **KHÔNG thuộc phạm vi Backend, KHÔNG được khai báo trong task này** (thuộc companion solution phía Agent).

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các tracer hiện có, chỉ append):

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...các tracer hiện có (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow, ...) giữ nguyên...

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

**Ràng buộc bắt buộc:**
- `remoteIntegration:ghExec` **không được khai báo** trong task này.
- Không tracer nào trong task này trùng tên với `agent:ext-api` (đã có, trace `github.pr.create`/`github.pr.merge` trong `external-api-connector.ts`) hoặc `agent:credential` (đã có, trace AI provider credential — CR-TRACE-016).

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

- [ ] `Tracers.remoteIntegrationCredentialDecryptFlow` (`remoteIntegration:credentialDecrypt`), `remoteIntegrationCredentialStoreFlow` (`remoteIntegration:credentialStore`), `remoteIntegrationPreflightFlow` (`remoteIntegration:preflight`) được export từ `tracers.ts`
- [ ] `remoteIntegration:ghExec` KHÔNG xuất hiện trong `tracers.ts` sau task này
- [ ] Không tracer nào trong task này trùng tên với `agent:ext-api` hoặc `agent:credential`
- [ ] `pnpm run typecheck:node` pass, không lỗi mới

# FE-CV-TASK-050-12: `useCodeIntelSupport` qua `settings.get`

**From Solution:** [FE-CV-SOL-050-store-and-query-hooks](../solutions/FE-CV-SOL-050-store-and-query-hooks.md) mục 4.5
**Priority:** P0
**Area:** frontend / hooks
**File:** `frontend/src/renderer/src/hooks/useCodeIntelSupport.ts` (mới), test `useCodeIntelSupport.test.tsx`
**Depends on:** FE-CV-TASK-050-07, FE-CV-TASK-050-10
**Status:** [x] DONE

## Context

- UI-API §6: `settings.get` khi khởi động và mỗi 60 s khi tab Review mở; `effective.codeIntelEnabled=false` ⇒ ẩn mọi lối vào, không gọi kênh khác, không `subscribe`; `qualityGateEnabled=false` ⇒ ẩn phần chất lượng. U8: phiên thiết bị bị `NOT_AUTHORIZED` trừ `settings.get`.
- Không dùng `typeof window.api.codeIntel` (Proxy `withFallback`).

## Việc cần làm

1. Gọi `settings.get` (`{}`) qua `codeIntelClient` theo `environmentId` của worktree; ghi `codeIntelSupportByEnvironment`.
2. Ánh xạ: ok & `effective.codeIntelEnabled` ⇒ `enabled`; `false` hoặc `CODEINTEL_DISABLED` ⇒ `disabled`; `UNAVAILABLE`/`unsupported` ⇒ `unsupported`; `offline`/lỗi tạm ⇒ giữ trạng thái cũ (lần đầu `unknown`).
3. `flags` = `{codeIntel, qualityGate, securityScan, aiReview}` từ `effective`.
4. Polling 60 s ref-count (`retainCodeIntelSettingsPolling`), chỉ khi có tab Review mở và cửa sổ hiển thị; TTL cho trạng thái không-enabled 60 s.
5. Không gọi kênh nào khác khi ≠ `enabled`.

## Kiểm thử

- Mỗi nhánh ánh xạ; polling bật/tắt theo ref; đổi môi trường reset; không gọi kênh khác khi disabled.

## Tiêu chí hoàn thành

- [ ] Test xanh; không toast; chuỗi (nếu có) qua `translate()`.

## Rủi ro

- Tắt cờ phía server có hiệu lực tối đa ~65 s ở client (cache 5 s + poll 60 s).

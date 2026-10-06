# FE-CV-TASK-087-03: Hook `useQualitySupport`, `useQualityGate`, `useQualityRun`, `useQualityProfiles`

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.4
**Priority:** P0
**Area:** frontend / hooks
**File:** `frontend/src/renderer/src/hooks/useQualitySupport.ts`, `useQualityGate.ts`, `useQualityRun.ts`, `useQualityProfiles.ts` (mới) và `*.test.tsx`
**Depends on:** 087-01, 087-02; FE-CV-SOL-050-store-and-query-hooks (`useCodeIntelSupport`, settings)
**Status:** [ ] TODO

## Context

- Hợp đồng §6: nguồn chuẩn là `settings.get.effective.qualityGateEnabled`; không dò `typeof window.api.codeIntel` (web bọc `window.api` bằng Proxy). `CODEINTEL_QUALITY_GATE_DISABLED` ẩn phần chất lượng.
- Mẫu hook huỷ khi unmount: `useTaskSource.ts` (cờ `cancelled`).

## Việc cần làm

1. `useQualitySupport()` theo SOL-087 2.4; lỗi `quality-disabled` → `disabled`.
2. `useQualityGate(worktreeId)`: đọc selector, gọi `loadQualityGate` khi `enabled` và có người dùng; trả `{gate, waivers, comparison, evaluatedAt, status, stale, error, refetch}`.
3. `useQualityRun(worktreeId)`: trả `{run, phase, start(profile, scope), cancel()}` + gắn lại khi `runs` có `running`.
4. `useQualityProfiles(worktreeId)`: `runnable`, `profile`, mặc định chọn (`quality-profile-selection.ts`).
5. Không gọi mạng khi `support !== 'enabled'`; trả tham chiếu ổn định.

## Kiểm thử

`renderHook` với store giả: không gọi RPC khi `disabled|unsupported|unknown`; huỷ khi unmount; selector ổn định (không render lại thừa); `attach` lần chạy đang diễn ra sau mở lại. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useQuality`.

## Tiêu chí hoàn thành

- [ ] Không RPC khi không `enabled`; không toast trong hook.
- [ ] Hook không dùng `any`.

## Rủi ro

- Selector settings của CR-050 chưa tồn tại; nếu thiếu, hook gọi `codeIntel.settings.get` một lần (ghi lại trong PR).

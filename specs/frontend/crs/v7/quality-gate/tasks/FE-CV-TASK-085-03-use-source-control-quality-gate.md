# FE-CV-TASK-085-03: Hook `useSourceControlQualityGate`

**From Solution:** [FE-CV-SOL-085-source-control-quality-notice](../solutions/FE-CV-SOL-085-source-control-quality-notice.md) mục 2.3
**Priority:** P0
**Area:** frontend
**File:** `frontend/src/renderer/src/components/right-sidebar/use-source-control-quality-gate.ts` (mới) + `.test.tsx`
**Depends on:** FE-CV-TASK-085-01, 085-02; FE-CV-SOL-050-store-and-query-hooks (bus sự kiện, `codeIntelClient`); FE-CV-SOL-061-review-entry-points (`openReviewFromEntryPoint`)
**Status:** [ ] TODO

## Context

- `quality.gate` T/o 8 s (hợp đồng 3.2); `CODEINTEL_TIMEOUT` có hậu tố `inProgress` → thử lại tối đa 90 s (2.4).
- Push `quality.gateChanged`/`quality.finished` (mục 5); session-client chỉ có `obj.event`.
- `Worktree.projectId` tuỳ chọn (shared/types.ts:487).

## Việc cần làm

1. `visible = flags.quality && worktreeId && projectId`; không visible → không subscribe, không gọi RPC.
2. Gọi `quality.gate {projectId, worktreeId, base}`; làm mới khi `headOid` đổi (debounce 400 ms), push đúng worktree, tab hiện lại; bỏ qua khi tab ẩn.
3. Timer hiển thị 3 s → `timedOut` (không huỷ RPC; kết quả muộn vẫn cập nhật).
4. Phân loại lỗi theo hợp đồng 2.3: quality-disabled/disabled/unsupported/forbidden/no-binding → ẩn lặng lẽ.
5. `runChecks()`: `quality.profile.get` → profile `ready && !heavy` → `quality.start {scope:"changed"}`; `running` đến khi `quality.finished`; xử lý ENV_NOT_READY, RUN_IN_PROGRESS, PROFILE_UNKNOWN không toast đỏ.
6. `openReason()` gọi `openReviewFromEntryPoint(worktreeId,"source-control",{lens:"quality"})`, thiếu lens thì mở Review.

## Kiểm thử

- Cờ tắt → 0 lời gọi; headOid đổi; push worktree khác bị bỏ; timeout 3 s + kết quả muộn; huỷ khi unmount; runChecks các nhánh lỗi.
- Chạy: `pnpm --filter orca-frontend test -- <đường dẫn>` (chưa chạy).

## Tiêu chí hoàn thành

- [ ] Cờ tắt không tạo tải.
- [ ] Không ném lỗi ra render.
- [ ] Không dùng `error.code`.

## Rủi ro

- Tên API 050/061 là giả định; `projectId` thiếu ở workspace cũ.

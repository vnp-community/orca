# FE-CV-TASK-087-02: State `codeIntelQualityByWorktree`, hành động chạy/huỷ, xử lý push và polling

**From Solution:** [FE-CV-SOL-087-quality-scorecard-and-state](../solutions/FE-CV-SOL-087-quality-scorecard-and-state.md) mục 2.3
**Priority:** P0
**Area:** frontend / store
**File:** `frontend/src/renderer/src/store/slices/code-intel-quality-state.ts` (mới), `store/slices/code-intel.ts` (sửa: trải đoạn state, thêm khoá vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`, mở rộng `applyCodeIntelEvent`), `store/slices/code-intel-quality-worktree-removal-leak.test.ts` (mới), `code-intel-quality-state.test.ts`
**Depends on:** 087-01; FE-CV-SOL-050-store-and-query-hooks
**Status:** [x] DONE (verified 2026-10-07: code-intel-quality-state.test.ts 25 + code-intel-quality-worktree-removal-leak.test.ts 2, đều pass; oxlint + tsc sạch)

## Context

- Mẫu slice: `StateCreator<AppState, [], [], Slice>` (TDD 02 mục 3); mẫu leak test: `generation-records-worktree-removal-leak.test.ts`, `editor-state-worktree-purge-leak.test.ts` (đã có).
- Push quality mang `worktreeId` (hợp đồng §5), nên không cần ánh xạ `runId → worktree`. `quality.finished` → tải lại gate/runs/findings; `gateChanged` → tải lại gate; `codeIntel.changed` → gate.stale.
- Polling dự phòng `quality.run` mỗi 2 s.

## Việc cần làm

1. Cài `QualityWorktreeState` và `ActiveQualityRun` như SOL-087 2.3; một bản mỗi loại mỗi worktree.
2. Actions: `startQualityRun` (phase `starting` ngay, gọi `quality.start`, đặt `runId`; `RUN_IN_PROGRESS` → `attachQualityRun(runId từ data)`), `cancelQualityRun` (phase `cancelling`; không khẳng định huỷ tới khi `finished`), `loadQualityGate/Runs/Profiles({force?})`, `attachQualityRun`, `invalidateQuality`, `setQualityUi`.
3. `applyCodeIntelEvent`: xử lý ba sự kiện như mục 2.3, bao gồm lần chạy do client khác khởi chạy (`run` rỗng hoặc `finished`).
4. Polling: bộ hẹn giờ theo worktree khi trạng thái sự kiện là `polling` hoặc sau `codeIntelResyncCounter`; dừng khi run kết thúc; huỷ khi prune.
5. Thêm `codeIntelQualityByWorktree` vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS`; không lưu `localStorage`.

## Kiểm thử

Hành động với bridge giả; sự kiện (progress → finished; `percent:null`; `interrupted`; worktree lạ bị bỏ qua); chống bấm đúp `startQualityRun`; leak test **hai đường** (`removeWorktree`, `buildWorktreePurgeState`) + timer huỷ; polling bằng fake timers. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/store/slices/code-intel-quality`.

## Tiêu chí hoàn thành

- [ ] Hai đường xoá worktree dọn sạch; không timer treo.
- [ ] Push không thuộc worktree nào bị bỏ qua, không ném.
- [ ] `startQualityRun` khoá ngay (phase `starting`).

## Rủi ro

- Chạm `code-intel.ts` của CR-050; trước khi sửa chạy GitNexus `impact` (chưa chạy khi soạn).
- Nhiều worktree mở Review cùng polling 2 s: chưa đo.

## Ghi chú triển khai (2026-10-07)

- Slice viết lại (bản cũ là stub không gọi RPC). Tách: `code-intel-quality-state.ts`, `-state-types.ts`, `-slice-context.ts`, `-load-actions.ts`, `-run-actions.ts` (giới hạn 300 dòng). Nối vào `store/index.ts`, `store/types.ts`, `store-test-helpers.ts`, và HAI đường xoá trong `worktrees.ts` (`buildWorktreePurgeState`, `removeWorktree`).
- Sai lệch: không mở rộng `applyCodeIntelEvent` (không tồn tại); sự kiện đến từ event bus qua `lib/code-intel-quality-event-sync.ts` (ref-count, mount bởi hook). `hotspots` không nằm trong slice (dùng `useCodeIntelQuery`, cache đã được dọn theo worktree). Thêm `findingsByFile` (≤16 tệp), `epoch`, `errors`, `loading`.
- Đường `removeWorktree` chỉ bỏ state; timer polling tự dừng ở tick kế tiếp vì run không còn. `pruneCodeIntelQualityWorktrees` huỷ timer ngay.
- Polling `quality.run` mỗi 2 s chỉ khi `codeIntelEventsState !== 'streaming'`; resync stream gọi `refreshQualityRun` một lần (hook `useQualityRun`).

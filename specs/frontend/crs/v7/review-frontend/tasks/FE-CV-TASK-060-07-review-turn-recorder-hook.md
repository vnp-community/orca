# FE-CV-TASK-060-07: Hook ghi mốc lượt `useReviewTurnRecorder`

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.5
**Priority:** P1
**Area:** frontend / renderer hooks
**File:** `frontend/src/renderer/src/components/review-map/turns/useReviewTurnRecorder.ts` (mới) + test
**Depends on:** FE-CV-TASK-061-01 (`AgentTurnCompletion`); FE-CV-TASK-060-05, 060-06; FE-CV-SOL-089-agent-turn-recorder (dùng chung khử trùng lặp)
**Status:** [~] PARTIAL — hook đã mount trong `ReviewWorkspace` qua `shell/use-review-companions.ts` (đọc lại danh sách đã lưu bằng `use-review-turn-markers.ts`, `onSaved` mới của hook; useReviewTurnRecorder.test 6/6 + ReviewWorkspace.companions.test 8/8 PASS); thiếu: chỉ ghi khi tab Review đang mount (spec muốn mức App/workspace để lượt kết thúc lúc tab đóng vẫn được ghi); chưa có test khẳng định bản ghi mới từ workspace

## Context

- PQ-22: marker `turnId = ${paneKey}:${doneAt}`, ≤ 5, dòng worktree-level, **không prompt**. PQ-35: không dùng `hintAgentTurnFinished`.

## Việc cần làm

1. Đăng ký ở mức `ReviewWorkspace` và cả khi tab Review chưa mở nếu `effective.codeIntelEnabled`; nhận completion từ `useAgentTurnCompletions`.
2. Khử trùng lặp theo `turnId`; debounce ~3 s chờ git ổn định (tái dùng cơ chế làm mới git status); dựng `ReviewTurnMarker` (`overlayAvailable`, `symbolKeys` từ overlay nếu có).
3. Gọi `saveTurnMarkers`; lỗi ⇒ trạng thái inline "Không lưu được mốc lượt" + thử lại; không chặn gửi ghi chú.
4. Không ghi khi cờ tắt; không lưu `prompt`.

## Kiểm thử

- `renderHook` với fixture `AgentStatusEntry` chuyển `working→done` (mẫu `DashboardAgentRow.test.tsx`): một marker/lượt, trùng lặp bỏ qua, cờ tắt không ghi, giới hạn 5, không có trường prompt.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/turns`.

## Tiêu chí hoàn thành

- [ ] Tiêu chí 7 của SOL-060 mục 5.
- [ ] Dọn timer/ref khi unmount.

## Rủi ro

- Mốc mất nếu renderer đóng lúc agent xong; không tự phục hồi.

## Ghi chú triển khai (2026-10-07)

- Nguồn "agent xong": dùng `detectAgentTurnCompletions` có sẵn trong `turns/` (SOL-061-01 chưa có). Khử trùng lặp theo `turnId`, trễ 3 s chờ git ổn định, không có trường prompt (test kiểm). Đọc `gitStatusByWorktree`/`gitBranchCompareSummaryByWorktree`; `symbolKeys` lấy qua `getSymbolKeys(worktreeId)` do nơi mount cấp.
- Lưu qua `saveTurnMarkerRow` (dòng worktree-level, ≤ 5 marker); lỗi trả `failedWorktreeId` + `retry()` để UI hiện inline.

## Ghi chú tích hợp (W6, 2026-10-07)

Mount tại `shell/use-review-companions.ts`. Sai lệch so với "mount ở mức App": App.tsx thuộc agent khác nên chưa đụng; cần một component headless ở App nếu muốn ghi khi tab đóng. Thêm tuỳ chọn `onSaved(worktreeId, markers)` vào hook để switcher cập nhật không cần đọc lại.

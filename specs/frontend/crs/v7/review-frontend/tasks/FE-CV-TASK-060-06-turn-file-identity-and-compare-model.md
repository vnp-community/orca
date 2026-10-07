# FE-CV-TASK-060-06: Dấu vân tay tệp theo lượt và mô hình so sánh lượt (hàm thuần)

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.5
**Priority:** P1
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/turns/turn-file-identity.ts`, `turn-compare-model.ts` (mới) + test
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (`ReviewTurnMarker`)
**Status:** [x] DONE

## Context

- `GitStatusEntry` (`shared/git-status-types.ts`) và `GitBranchCompareResult`; mẫu identity: `mobile/src/session/mobile-diff-review-queue.ts` (`statusEntryIdentity`). Renderer desktop chưa có hàm tương đương (theo CR).

## Việc cần làm

1. `fileIdentity(entry, {headOid, mergeBase})` băm ổn định `status, oldPath, path, added, removed, area/scope`; trả `{p, o?, h}` (cap 500 tệp, báo `truncated`).
2. `compareTurns(prev, curr)` → map tệp ⇒ `new_in_turn|changed_in_turn|unchanged_since|reverted_in_turn`; symbol chỉ khi cả hai marker `overlayAvailable` và có `symbolKeys`.
3. `noteProgressHint(sentNote, currentIdentity)` → `'file_changed'|'file_unchanged'|'unknown'`.

## Kiểm thử

- Ổn định; khác khi đổi path/số dòng/headOid; bốn nhãn; thiếu `symbolKeys` một phía ⇒ chỉ mức tệp; cap; trùng vân tay (cùng số dòng) được ghi nhận là giới hạn đã biết.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/turns`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh.
- [ ] Kết quả luôn mang cờ `estimated:true`.

## Rủi ro

- Vân tay thô bỏ sót thay đổi cùng số dòng.

# FE-CV-TASK-059-03: Hook `useCodeIntelFindings`, `useCodeIntelContractDiff`, `useFindingDismissal` và selector

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.2, 2.3
**Priority:** P1
**Area:** frontend / renderer hooks + store
**File:** `frontend/src/renderer/src/hooks/useCodeIntelFindings.ts`, `useCodeIntelContractDiff.ts`, `useFindingDismissal.ts` (mới) + test; `frontend/src/renderer/src/store/slices/code-intel.ts` (sửa nhỏ: `findingFilters`, `contractFilters`, `selectOpenFindingsBySymbolKey`)
**Depends on:** FE-CV-SOL-050-store-and-query-hooks; FE-CV-TASK-059-02; G4 fake backend (FE-CV-TASK-073-02)
**Status:** [x] DONE

## Context

- UI-API §3.1: `findings` nhận `(sel), rules?, severities?, pathPrefix?, includeDismissed?, scope?, base?, limit ≤ 200, pageToken?`; `dismissFinding` nhận `(sel), findingKey, action?, disposition?, reason?, note?` (≤ 500), timeout ghi 8 s; `contractDiff` nhận `(sel), base?, kinds?, detail?`.
- Phản hồi cũ phải bị bỏ; lỗi phân loại theo §2.3; không toast trong hook.

## Việc cần làm

1. `useCodeIntelFindings({worktreeId, filters})`: phân trang (`nextPageToken`, "Tải thêm", khử trùng theo `findingKey`), `dismissedCount`, `indexFreshness`; đổi bộ lọc server (`severities`, `pathPrefix`, `includeDismissed`, `scope`) ⇒ huỷ/bỏ cũ và tải lại.
2. `useCodeIntelContractDiff({worktreeId, kinds, detail})`: `detail:'summary'` cho chip, `'full'` cho bảng; `truncated`/`totalCount`.
3. `useFindingDismissal(worktreeId)` → `{dismiss(finding, {reason, note?}), resolve(finding, {note?}), restore(finding), pendingKeys}`: khoá theo `findingKey`, cập nhật lạc quan vào cache của `useCodeIntelFindings`, hoàn nguyên khi lỗi và trả `CodeIntelUiError` (`forbidden`, `offline`, `conflict`, `not-found`); không gửi `reason` rỗng khi `disposition:'ignored'`; cắt `reason`/`note` ≤ 500; không gửi `args[1+]` (U1).
4. Selector `selectOpenFindingsBySymbolKey(worktreeId)` → `Record<symbolKey, {count, topSeverity}>` (loại `dismissed`), dùng cho lens khác (SOL-053/054/055).
5. Khoá `findingFilters`, `contractFilters` vào `CODE_INTEL_WORKTREE_KEYED_STATE_KEYS` + test rò rỉ.

## Kiểm thử

- `renderHook` + mock `codeIntelClient.call`: phân trang không trùng; lạc quan + hoàn nguyên; nút khoá trong lúc chờ; `restore` gửi `action:'restore'`; `reason` rỗng bị chặn client; `CODEINTEL_VERSION_CONFLICT`/`NOT_FOUND` ⇒ tải lại; bỏ phản hồi cũ; selector.
- Rò rỉ khi xoá worktree (mẫu `*-worktree-purge-leak.test.ts`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/hooks/useCodeIntelFindings src/renderer/src/hooks/useCodeIntelContractDiff src/renderer/src/hooks/useFindingDismissal`.

## Tiêu chí hoàn thành

- [ ] Hook dùng `useCodeIntelQuery`/`codeIntelClient`, không `window.api` trực tiếp.
- [ ] Không gọi kênh nào khi cờ tắt.
- [ ] Test rò rỉ xanh.

## Rủi ro

- Nhiều người cùng bỏ qua một phát hiện: kết quả cuối thắng, UI chỉ tải lại (không có `expectedVersion` ở kênh này).

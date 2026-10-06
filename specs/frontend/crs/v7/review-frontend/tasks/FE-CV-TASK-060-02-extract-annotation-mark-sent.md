# FE-CV-TASK-060-02: Trích `markAnnotationsSentBestEffort` ra module dùng chung

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.3
**Priority:** P1
**Area:** frontend / renderer lib
**File:** `frontend/src/renderer/src/lib/annotation-mark-sent-best-effort.ts` (mới) + test; `frontend/src/renderer/src/components/editor/DiffNotesSendMenu.tsx` (sửa: chỉ import lại)
**Depends on:** không (độc lập; làm trước 060-04)
**Status:** [ ] TODO

## Context

- Đã xác minh: hàm riêng tư ở `DiffNotesSendMenu.tsx:16`, gọi ở dòng 136; gọi `callRuntimeRpc(target,'annotation.markSent',{ids},{timeoutMs:8000})`, bỏ qua target `local`, nuốt lỗi.
- AGENTS.md/CLAUDE.md: **chạy `gitnexus_impact` trên `markAnnotationsSentBestEffort` và `DiffNotesSendMenu` trước khi sửa, báo blast radius; `detect_changes` trước khi commit.**

## Việc cần làm

1. Chạy impact (upstream) cho hai symbol, ghi kết quả vào PR.
2. Chuyển hàm sang `lib/annotation-mark-sent-best-effort.ts` (export, giữ nguyên hành vi: id rỗng ⇒ return; target local ⇒ return; lỗi bị nuốt).
3. `DiffNotesSendMenu.tsx` import lại; không đổi props/hành vi.

## Kiểm thử

- Test mới: id rỗng không gọi; local không gọi; environment gọi đúng method/params/timeout; lỗi không ném.
- Hồi quy: `NotesSendMenu.test.tsx`, `ReviewNotesSendMenuContent.test.tsx` và test hiện có quanh `DiffNotesSendMenu`.
- `pnpm --filter orca-frontend test -- src/renderer/src/lib/annotation-mark-sent src/renderer/src/components/editor`.

## Tiêu chí hoàn thành

- [ ] Hành vi `DiffNotesSendMenu` không đổi (test hồi quy xanh).
- [ ] Impact đã chạy và ghi lại.

## Rủi ro

- `DiffNotesSendMenu` có 3 nơi gọi (theo CR; chưa kiểm lại): sai import làm hỏng Source Control.

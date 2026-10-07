# FE-CV-TASK-060-01: Neo ghi chú đồ thị và dựng nội dung (hàm thuần)

**From Solution:** [FE-CV-SOL-060](../solutions/FE-CV-SOL-060-review-notes-and-turn-compare.md) mục 2.2
**Priority:** P1
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/notes/review-note-anchor.ts` (mới) + `review-note-anchor.test.ts`
**Depends on:** FE-CV-SOL-050-types-and-runtime-bridge (kiểu `ReviewNoteAnchor`); FE-CV-TASK-057-01 (`maskSensitiveText`)
**Status:** [x] DONE

## Context

- Hợp đồng §4.6: `ReviewNoteAnchor` = `diff-line` | `graph-node` {lens, nodeKey, filePath, startLine?, endLine?, label} | `finding` {findingKey,…}.
- `DiffComment` bắt buộc `filePath`; `lineNumber===0` nghĩa là ghi chú cấp tệp.

## Việc cần làm

1. `resolveGraphNodeCommentTarget(anchor)` → `{filePath, startLine?, lineNumber} | null` theo bảng 2.2 (symbol, bảng ERD, hợp đồng, phát hiện; nút không gắn tệp ⇒ `null`).
2. `buildGraphNoteBody(anchor, userText)` → `[Review map · <lens> · <label>] <text>`; `label` qua `maskSensitiveText`; cắt label ≤ 120 ký tự.
3. `isGraphAnchor(anchor)`, `anchorLensId(anchor)` cho panel nhóm.

## Kiểm thử

- Bảng ca cho từng loại nút; `lineNumber` 0 cho cấp tệp; `null` cho cụm/service; tiền tố đúng; label có DSN bị che; neo lens lạ.
- `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/notes/review-note-anchor`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh.
- [ ] Không thêm trường vào `DiffComment`.

## Rủi ro

- Định dạng tiền tố là hợp đồng ngầm với agent nhận; đổi sau sẽ ảnh hưởng prompt.

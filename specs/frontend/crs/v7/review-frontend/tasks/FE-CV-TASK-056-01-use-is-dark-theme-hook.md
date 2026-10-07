# FE-CV-TASK-056-01: Hook `useIsDarkTheme`

**From Solution:** [FE-CV-SOL-056-dataflow-lens](../solutions/FE-CV-SOL-056-dataflow-lens.md) mục 4.1
**Priority:** P1
**Area:** frontend / hooks
**File:** `frontend/src/renderer/src/hooks/useIsDarkTheme.ts` (mới), test
**Depends on:** không
**Status:** [x] DONE

## Context

- `isDark` hiện tính lặp ở `MarkdownPreview.tsx:588`, `MermaidViewer.tsx`, `sidebar/CommentMermaidBlock.tsx`: `settings?.theme === 'dark' || (settings?.theme === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)`. Chưa có hook dùng chung.

## Việc cần làm

1. Hook đọc `settings.theme` từ store, phản ứng với `matchMedia` khi `system`; `matchMedia` thiếu ⇒ `false`.
2. Không sửa ba nơi cũ (ngoài phạm vi).

## Kiểm thử

- `dark`/`light`/`system` với `matchMedia` giả; đổi `change` sự kiện.

## Tiêu chí hoàn thành

- [ ] Test xanh.

## Rủi ro

- Công thức phải trùng ba nơi cũ.

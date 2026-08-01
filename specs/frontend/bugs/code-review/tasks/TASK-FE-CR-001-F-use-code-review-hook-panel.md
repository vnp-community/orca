# TASK-FE-CR-001-F: `useCodeReview` hook + `CodeReviewPanel` assembly

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 §Hook + Assembly  
**Priority:** 🟡 P2  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

1. Tạo `useCodeReview` hook — quản lý state cho toàn bộ code review flow
2. Tạo `CodeReviewPanel` — assembly tất cả components thành panel hoàn chỉnh

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/hooks/useCodeReview.ts`
- **TẠO MỚI:** `src/renderer/src/components/code-review/code-review-panel.tsx`

---

## Bước 1: `useCodeReview.ts`

Hook tổng hợp:
- **`loadChangedFiles()`**: Gọi `rpc.call('git.exec', { args: ['diff', 'HEAD', '--numstat'] })` → parse output thành `ChangedFile[]`
  - Format numstat: `<additions>\t<deletions>\t<path>`
  - Handle renames: `{old => new}` pattern
  - Auto-select first file nếu `selectedFile == null`
- **`handleLineClick(lineNumber)`**: set `annotationLine`
- **`closeAnnotation()`**: clear `annotationLine`
- **Re-load trigger**: `useEffect` on `gitStatus` changes

```typescript
export function useCodeReview() {
  // Return: changedFiles, selectedFile, setSelectedFile,
  //         annotationLine, handleLineClick, closeAnnotation,
  //         isLoadingFiles, refreshChangedFiles
}
```

## Bước 2: `CodeReviewPanel.tsx`

Layout 2 cột:
```
[Left 256px: ChangedFilesTree + "Create PR" button]
[Right flex: DiffViewer | AnnotationPanel | CommitMessageGenerator]
```

Dùng `useCodeReview()` để lấy state, truyền xuống các child components.

---

## Verify

```bash
grep -n "useCodeReview\|loadChangedFiles" \
  src/renderer/src/hooks/useCodeReview.ts

grep -n "CodeReviewPanel\|ChangedFilesTree" \
  src/renderer/src/components/code-review/code-review-panel.tsx
```

## Test

```typescript
// hooks/__tests__/useCodeReview.test.ts
// - parses git numstat format correctly (added/deleted/modified)
// - renamed files parsed with oldPath
// - handleLineClick sets annotationLine
// - closeAnnotation resets to null
```

## Depends on
TASK-FE-CR-001-A, TASK-FE-CR-001-B, TASK-FE-CR-001-C, TASK-FE-CR-001-D, TASK-FE-CR-001-E

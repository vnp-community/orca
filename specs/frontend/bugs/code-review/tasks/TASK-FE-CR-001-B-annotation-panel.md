# TASK-FE-CR-001-B: Tạo `AnnotationPanel` component — line-level comments (BL-CR-02)

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 Component 2  
**Priority:** 🟠 P1  
**Estimated:** 45 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `AnnotationPanel` component để thêm inline comments vào line cụ thể trong diff.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/code-review/annotation-panel.tsx`

---

## Các bước thực thi

Tạo file với nội dung từ SOL-FE-CR-001 §Component 2:

1. **Props:** `filePath`, `reviewId`, `lineNumber: number | null`, `onClose()`
2. **State:** `annotations[]`, `newComment`, `isSaving`
3. **Submit:** `rpc.call('annotation.create', { projectId, reviewId, filePath, lineNumber, content })`
4. **Display:** list existing annotations + textarea + send button
5. **Header:** `Line {lineNumber} — {filePath}` + close button

```typescript
interface Annotation {
  id: string
  lineNumber: number
  filePath: string
  content: string
  author: string
  createdAt: number
}
```

**Lưu ý:** `annotation.create` RPC method có thể chưa tồn tại ở backend — task này chỉ implement frontend, backend là task riêng.

---

## Verify

```bash
grep -n "AnnotationPanel\|annotation.create" \
  src/renderer/src/components/code-review/annotation-panel.tsx
```

## Depends on
Không có

## Blocking
TASK-FE-CR-001-F (CodeReviewPanel)

# TASK-FE-CR-001-A: Tạo `DiffViewer` component — Monaco diff mode (BL-CR-01)

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 Component 1  
**Priority:** 🔴 P0  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `DiffViewer` component dùng Monaco Editor diff mode để hiển thị so sánh HEAD vs working tree cho một file.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/code-review/diff-viewer.tsx`

---

## Dependency check

```bash
grep "@monaco-editor/react" package.json
# Nếu không có: npm install @monaco-editor/react
```

---

## Các bước thực thi

Tạo file `diff-viewer.tsx` với nội dung từ SOL-FE-CR-001 §Component 1. Logic chính:

1. **Props:** `filePath`, `worktreePath?`, `staged?`, `onLineClick?`
2. **Effect:** Load original (HEAD) + modified (working tree) via RPC:
   - Original: `rpc.call('git.exec', { args: ['show', `HEAD:${filePath}`] })`
   - Modified (unstaged): `rpc.call('fs.readFile', { path })`
   - Modified (staged): `rpc.call('git.diff', { filePath, staged: true })`
3. **Render:** `<MonacoDiffEditor original={...} modified={...} language={detectLanguage(filePath)} options={{ readOnly: true, renderSideBySide: true }} />`
4. **onLineClick:** `editor.onMouseDown(e => onLineClick(e.target.position?.lineNumber))`
5. **Loading:** `<Skeleton />` khi `isLoading`
6. **Error:** message khi fetch fail

**Key helper cần tìm/tạo:** `detectLanguage(filePath)` — có thể đã có trong codebase.

```bash
grep -rn "detectLanguage\|getLanguageFromPath" src/renderer/src/lib/
```

---

## Verify

```bash
grep -n "MonacoDiffEditor\|DiffViewer" \
  src/renderer/src/components/code-review/diff-viewer.tsx
```

## Test (Vitest + happy-dom)

```typescript
// src/renderer/src/components/code-review/__tests__/diff-viewer.test.tsx
// @vitest-environment happy-dom
// - isLoading shows Skeleton
// - calls git.exec 'show HEAD:path' for original
// - staged=true calls git.diff instead of fs.readFile
// - onLineClick fires with lineNumber
```

## Depends on
Không có (độc lập)

## Blocking
TASK-FE-CR-001-D (useCodeReview hook), TASK-FE-CR-001-F (CodeReviewPanel)

# BUG-FE-CR-001: DiffViewer và Annotation UI không tồn tại trong Renderer — Code Review UI thiếu core components

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-CR-01 → BL-CR-05) mô tả frontend components:
```
[Renderer] DiffViewer component
    ├─ Render syntax-highlighted diff (Monaco / CodeMirror)
    ├─ Files tree với change counts
    └─ Line-level navigation

[Renderer] click trên dòng code trong DiffViewer
    → inline annotation marker

[Renderer] "AI: Generate Commit Message" → pre-fill commit input

[Renderer] "Create PR" → PR URL → open in browser
```

Grep toàn bộ `src/renderer/` không tìm thấy:
```
DiffViewer          → No results
AnnotationMarker    → No results
diff.load           → No results (RPC call)
annotation.create   → No results
generateCommitMessage → No results
pr.create           → No results (code-review context)
```

## Ảnh hưởng

1. **BL-CR-01**: Diff viewer với Monaco/CodeMirror — không có.
2. **BL-CR-02**: Line annotation click handler — không có.
3. **BL-CR-04**: AI commit message UI (pre-fill input) — không có.
4. **BL-CR-05**: PR creation dialog với AI description — không có.

## Files không tồn tại

- `src/renderer/src/components/code-review/diff-viewer.tsx`
- `src/renderer/src/components/code-review/annotation-panel.tsx`
- `src/renderer/src/components/code-review/pr-create-dialog.tsx`
- `src/renderer/src/components/code-review/commit-message-generator.tsx`

## Liên quan đến luồng

- **BL-CR-01 → BL-CR-05**: Toàn bộ Code Review UI không có.

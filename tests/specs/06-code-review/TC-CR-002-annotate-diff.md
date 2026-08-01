# TC-CR-002 — Annotate Dòng Code trong Diff

**BL Reference:** BL-CR-02  
**Priority:** P1  
**Type:** Integration  
**Actor:** Maya, Alex

---

## TC-CR-002-01: Thêm annotation vào dòng cụ thể

**Priority:** P1

### Steps
1. Open diff view for `auth.ts`
2. Click line 42 (function signature)
3. `review.addAnnotation { worktreeId, file: 'auth.ts', line: 42, comment: 'Consider using async/await here' }`

### Expected Results
- Annotation stored
- DB: `orca_diff_annotations { worktreeId, file, line, comment, userId }`

---

## TC-CR-002-02: Xem tất cả annotations của worktree

**Priority:** P1

### Steps
1. `review.getAnnotations { worktreeId }`

### Expected Results
- List all annotations grouped by file

---

## TC-CR-002-03: Delete annotation

**Priority:** P1

### Steps
1. `review.deleteAnnotation { annotationId }`

### Expected Results
- Annotation deleted
- Owner only, or admin


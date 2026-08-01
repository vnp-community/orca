# TASK-FE-CR-001-E: Tạo `PrCreateDialog` component — PR creation + AI description (BL-CR-05)

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 Component 4  
**Priority:** 🟡 P2  
**Estimated:** 50 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `PrCreateDialog` — dialog tạo Pull Request với AI auto-generate title/description, draft mode, open PR URL.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/code-review/pr-create-dialog.tsx`

---

## Các bước thực thi

Tạo file với nội dung từ SOL-FE-CR-001 §Component 4:

1. **Props:** `open`, `onOpenChange`, `currentBranch`, `baseBranch?`

2. **State:** `title`, `body`, `isDraft`, `isCreating`, `isGeneratingDesc`, `prUrl`

3. **AI Generate (Sparkles button):**
   - `rpc.call('git.generatePrDescription', { projectId, worktreePath, branch, baseBranch })`
   - Returns `{ title: string; body: string }`

4. **Create PR:**
   - `rpc.call('git.pr.create', { title, body, base, draft })`
   - Returns `{ url: string; number: number }`
   - Sau khi tạo: hiện `prUrl` với "Open on GitHub" button

5. **Form fields:** title (Input), body (Textarea 5 rows), draft checkbox

6. **Success state:** ✅ message + "Open PR on GitHub" button (ExternalLink icon)

---

## Verify

```bash
grep -n "PrCreateDialog\|git.pr.create" \
  src/renderer/src/components/code-review/pr-create-dialog.tsx
```

## Test

```typescript
// pr-create-dialog.test.tsx
// - createPr calls git.pr.create with correct title/body/base/draft
// - AI generate calls git.generatePrDescription
// - prUrl displayed with ExternalLink button after success
// - cancel button closes dialog
```

## Depends on
Không có

## Blocking
TASK-FE-CR-001-F (CodeReviewPanel)

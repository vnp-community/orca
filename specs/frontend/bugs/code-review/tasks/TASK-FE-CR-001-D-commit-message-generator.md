# TASK-FE-CR-001-D: Tạo `CommitMessageGenerator` component — AI commit message (BL-CR-04)

**Domain:** code-review  
**Solution Ref:** SOL-FE-CR-001 Component 3  
**Priority:** 🟡 P2  
**Estimated:** 40 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `CommitMessageGenerator` component — textarea commit message có nút AI ✨ để auto-generate từ staged diff.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/code-review/commit-message-generator.tsx`

---

## Các bước thực thi

Tạo file với nội dung từ SOL-FE-CR-001 §Component 3:

1. **Props:**
   ```typescript
   interface CommitMessageGeneratorProps {
     value: string
     onChange: (value: string) => void
     onCommit: (push: boolean) => Promise<void>
     isCommitting: boolean
   }
   ```

2. **AI Generate button (✨ Sparkles icon):**
   - Gọi `rpc.call('git.generateCommitMessage', { projectId, worktreePath })`
   - Handle error code `GIT_NO_STAGED_CHANGES` → toast "No staged changes"
   - Loading: `Loader2` spinner animation

3. **Textarea:** `maxLength={500}`, placeholder `"feat(scope): message"`

4. **Action buttons:**
   - "Commit" → `onCommit(false)`
   - "Commit & Push" → `onCommit(true)` (outline variant)
   - Disabled khi `isCommitting || !value.trim()`

---

## Verify

```bash
grep -n "CommitMessageGenerator\|generateCommitMessage" \
  src/renderer/src/components/code-review/commit-message-generator.tsx
```

## Depends on
Không có

## Blocking
TASK-FE-CR-001-F (CodeReviewPanel)

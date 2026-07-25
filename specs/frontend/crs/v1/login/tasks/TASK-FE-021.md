# TASK-FE-021 — Tạo `auth-utils.ts` (`toLinuxUsername`) + Tests

**Phase:** 4 — SSH UI
**Solution:** [SOL-FE-LG-004](../solutions/SOL-FE-LG-004-ssh-ui.md) §5.1, §8
**Depends on:** TASK-FE-001
**Blocks:** TASK-FE-023, TASK-FE-024
**Effort:** XS (~15 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo utility function `toLinuxUsername()` — mirror chính xác của hàm cùng tên ở backend (SOL-LG-003).
Dùng để hiển thị predicted linux username cho user trước khi provisioning hoàn tất.

---

## Files cần tạo

### `src/renderer/src/auth/auth-utils.ts` [NEW]

```typescript
/**
 * Compute predicted linux username from Orca user email.
 * Mirrors backend toLinuxUsername() in src/main/ssh/ssh-user-resolver.ts
 *
 * Examples:
 *   "alice@company.com"      → "orca-alice"
 *   "alice.smith@co.com"     → "orca-alice-smith"
 *   "alice+filter@co.com"    → "orca-alice-filter"
 *   "verylongemailname@x.co" → "orca-verylongemailnam" (truncated at 20)
 */
export function toLinuxUsername(email: string): string {
  const local = email.split('@')[0]
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')
    .slice(0, 20)
  const sanitized = local.replace(/^-+|-+$/g, '') || 'user'
  return `orca-${sanitized}`
}
```

### `src/renderer/src/auth/__tests__/auth-utils.test.ts` [NEW]

Sao chép test spec từ [SOL-FE-LG-004 §8](../solutions/SOL-FE-LG-004-ssh-ui.md).

Test cases (4 tests):
- `toLinuxUsername("alice@company.com")` === `"orca-alice"`
- Replaces dots với dashes: `"alice.smith@co.com"` → `"orca-alice-smith"`
- Truncates local part tại 20 chars
- Special chars (+ filter) → only alphanumeric + dash

---

## Verify

```bash
npx vitest run src/renderer/src/auth/__tests__/auth-utils.test.ts
# Expected: 4 pass

# QUAN TRỌNG: Output phải khớp với backend toLinuxUsername() test cases
```

# TASK-FE-010 — Tạo `UserRoleBadge.tsx` + Tests

**Phase:** 2 — User Identity
**Solution:** [SOL-FE-LG-002](../solutions/SOL-FE-LG-002-user-identity.md) §4.4, §3.2
**Depends on:** — (không cần dependency)
**Blocks:** TASK-FE-011
**Effort:** XS (~15 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo component badge nhỏ hiển thị role của user trong UI.

---

## Files cần tạo

### `src/renderer/src/components/auth/UserRoleBadge.tsx` [NEW]

```typescript
type Role = 'developer' | 'lead' | 'admin'
type Props = { role: Role }

const ROLE_LABELS: Record<Role, string> = {
  developer: 'developer',
  lead: 'lead',
  admin: 'admin',
}

export function UserRoleBadge({ role }: Props) {
  return (
    <span className={`role-badge role-badge--${role}`}>
      {ROLE_LABELS[role]}
    </span>
  )
}
```

### `src/renderer/src/components/auth/__tests__/UserRoleBadge.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-002 §3.2](../solutions/SOL-FE-LG-002-user-identity.md).

Test cases (3 tests):
- Renders "developer" cho developer role
- Renders với class `role-badge--admin` cho admin
- Renders "lead" cho lead role

---

## Verify

```bash
npx vitest run src/renderer/src/components/auth/__tests__/UserRoleBadge.test.tsx
# Expected: 3 pass
```

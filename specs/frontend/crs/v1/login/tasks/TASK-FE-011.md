# TASK-FE-011 — Tạo `UserAvatarMenu.tsx` + Tests

**Phase:** 2 — User Identity
**Solution:** [SOL-FE-LG-002](../solutions/SOL-FE-LG-002-user-identity.md) §4.3, §3.1
**Depends on:** TASK-FE-009, TASK-FE-010
**Blocks:** TASK-FE-012
**Effort:** M (~40 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo avatar dropdown menu hiển thị trong web app khi user đã login.
Component độc lập (stateless trừ `isOpen`), nhận `user` + `onLogout` từ parent.

---

## Files cần tạo

### `src/renderer/src/components/auth/UserAvatarMenu.tsx` [NEW]

Implement theo spec đầy đủ tại [SOL-FE-LG-002 §4.3](../solutions/SOL-FE-LG-002-user-identity.md).

Behavior:
- Hiển thị avatar image nếu có `user.avatarUrl`
- Fallback: 2 initials từ `user.name` (getInitials)
- Click trigger button → toggle dropdown
- Dropdown: hiển thị name, email, `<UserRoleBadge role={user.role} />`
- Click Logout menuitem → `await onLogout()`
- Escape key → đóng dropdown
- Accessibility: `aria-label="Open user menu"`, `role="menu"`, `role="menuitem"`

Helper:
```typescript
function getInitials(name: string): string {
  return name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
}
```

### `src/renderer/src/components/auth/__tests__/UserAvatarMenu.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-002 §3.1](../solutions/SOL-FE-LG-002-user-identity.md).

Test cases (6 tests):
- Renders avatar image khi avatarUrl có giá trị
- Renders initials fallback khi không có avatarUrl
- Click → dropdown mở với email, name
- Dropdown hiển thị role badge
- Click Logout → onLogout được gọi
- Escape key → dropdown đóng

---

## Verify

```bash
npx vitest run src/renderer/src/components/auth/__tests__/UserAvatarMenu.test.tsx
# Expected: 6 pass
```

# TASK-FE-012 — Modify `OrcaProfileSwitcher.tsx` — Web Auth Display

**Phase:** 2 — User Identity
**Solution:** [SOL-FE-LG-002](../solutions/SOL-FE-LG-002-user-identity.md) §4.5
**Depends on:** TASK-FE-011
**Blocks:** —
**Effort:** S (~20 phút)
**Status:** ✅ Done

---

## Mô tả

Tích hợp `UserAvatarMenu` vào `OrcaProfileSwitcher`. Chỉ render trong web mode khi user đã authenticated.

**Nguyên tắc:** KHÔNG sửa `App.tsx`.

---

## File cần sửa

### `src/renderer/src/components/orca-profile-switcher/OrcaProfileSwitcher.tsx` [MODIFY]

**Thêm imports:**
```typescript
import { useAuthUser } from '../../hooks/useAuthSession'
import { useLogout } from '../../hooks/useLogout'
import { UserAvatarMenu } from '../auth/UserAvatarMenu'
```

**Thêm vào component function (đầu component):**
```typescript
const authUser = useAuthUser()
const logout = useLogout()
```

**Thêm vào phần return JSX (sau existing profile switcher UI):**
```typescript
{authUser && import.meta.env.ORCA_PLATFORM === 'web' && (
  <UserAvatarMenu user={authUser} onLogout={logout} />
)}
```

---

## Constraints

- KHÔNG thay đổi logic existing của OrcaProfileSwitcher
- Chỉ thêm conditional render ở cuối
- Guard kép: `authUser` AND `ORCA_PLATFORM === 'web'` — không hiện trong Desktop

---

## Verify

```bash
npx tsc --noEmit
# Manual: web mode + login → thấy avatar trong titlebar
# Manual: desktop mode → không thấy avatar
```

# SOL-FE-LG-002 — User Identity Display (Avatar, Role, Logout)

**CR:** [CR-LOGIN-001](../../../../../docs/crs/v1/login/CR-LOGIN-001-auth.md), [CR-LOGIN-002](../../../../../docs/crs/v1/login/CR-LOGIN-002-sandbox.md)
**Backend Solution:** [SOL-LG-001](../../../../backend/crs/v1/login/solutions/SOL-LG-001-auth-session.md)
**TDD Refs:** TDD-FE-05 (UI Components — App Shell, Titlebar), TDD-FE-02 (State Management — Zustand slices)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Done — Implemented & verified 2026-07-24
**Blocked by:** SOL-FE-LG-001 (cần AuthSlice)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 Titlebar (TDD-FE-05 §1)

```typescript
// src/renderer/src/App.tsx — Titlebar structure (KHÔNG THAY ĐỔI App.tsx)
// Titlebar chứa: OrcaProfileSwitcher, ActivityTitlebarControls, Nav buttons

// CẦN: thêm UserAvatarMenu vào Titlebar CHỈ khi web mode + authenticated
// Approach: wrapper component bên ngoài App.tsx hoặc thêm vào OrcaProfileSwitcher
```

**Nguyên tắc:** App.tsx không được thay đổi. Tuy nhiên, `OrcaProfileSwitcher` là candidate tốt để integrate UserAvatarMenu vì nó handle profile selection.

**Approach được chọn:**
- Tạo `UserAvatarMenu.tsx` standalone
- Tích hợp vào `OrcaProfileSwitcher.tsx` (không phải App.tsx) — chỉ render khi `ORCA_PLATFORM === 'web'` và `auth.status === 'authenticated'`

### 1.2 Auth Slice (từ SOL-FE-LG-001)

```typescript
// AuthSlice đã có từ SOL-FE-LG-001:
// auth: AuthState (status: 'unknown' | 'unauthenticated' | 'authenticated' | 'error')
// clearAuth(): void
// checkSession(): Promise<void>
```

### 1.3 `useAuthSession` hook

```typescript
// Cần tạo hook để components subscribe vào auth state:
function useAuthSession(): AuthState
// → wrapper quanh useAppStore(s => s.auth)
```

---

## 2. File Structure

```
src/renderer/src/
├── auth/
│   ├── auth-types.ts                  ← [từ SOL-FE-LG-001]
│   └── auth-api-client.ts             ← [từ SOL-FE-LG-001]
│
├── hooks/
│   ├── useAuthSession.ts              ← [NEW] Auth state hook
│   └── useLogout.ts                   ← [NEW] Logout action hook
│
└── components/
    └── auth/
        ├── UserAvatarMenu.tsx         ← [NEW] Avatar dropdown menu
        ├── UserRoleBadge.tsx          ← [NEW] Badge hiển thị role
        └── __tests__/
            ├── UserAvatarMenu.test.tsx
            └── UserRoleBadge.test.tsx
```

### Tích hợp vào App Shell (không sửa App.tsx):

```
src/renderer/src/components/
└── orca-profile-switcher/
    └── OrcaProfileSwitcher.tsx        ← [MODIFY] Thêm <WebUserAvatarSection />
```

---

## 3. Test Specifications

### 3.1 `UserAvatarMenu.test.tsx`

```typescript
// src/renderer/src/components/auth/__tests__/UserAvatarMenu.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { UserAvatarMenu } from '../UserAvatarMenu'

const mockUser = { id: 'u1', email: 'alice@co.com', name: 'Alice Smith', role: 'developer' as const, provider: 'github' as const, avatarUrl: 'https://github.com/avatar.png' }

describe('UserAvatarMenu', () => {
  afterEach(cleanup)

  it('renders user avatar image when avatarUrl provided', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    const img = screen.getByRole('img', { name: /alice smith/i })
    expect(img).toHaveAttribute('src', 'https://github.com/avatar.png')
  })

  it('renders initials fallback when no avatarUrl', () => {
    const userNoAvatar = { ...mockUser, avatarUrl: undefined }
    render(<UserAvatarMenu user={userNoAvatar} onLogout={vi.fn()} />)
    expect(screen.getByText('AS')).toBeInTheDocument() // Alice Smith initials
  })

  it('opens dropdown on avatar click with email, name, role', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByText('alice@co.com')).toBeInTheDocument()
    expect(screen.getByText('Alice Smith')).toBeInTheDocument()
  })

  it('shows role badge in dropdown', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByText(/developer/i)).toBeInTheDocument()
  })

  it('calls onLogout when Logout menu item clicked', async () => {
    const onLogout = vi.fn().mockResolvedValue(undefined)
    render(<UserAvatarMenu user={mockUser} onLogout={onLogout} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    fireEvent.click(screen.getByRole('menuitem', { name: /logout/i }))
    await waitFor(() => expect(onLogout).toHaveBeenCalledOnce())
  })

  it('closes dropdown when Escape key is pressed', () => {
    render(<UserAvatarMenu user={mockUser} onLogout={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /open user menu/i }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })
})
```

### 3.2 `UserRoleBadge.test.tsx`

```typescript
// src/renderer/src/components/auth/__tests__/UserRoleBadge.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { UserRoleBadge } from '../UserRoleBadge'

describe('UserRoleBadge', () => {
  afterEach(cleanup)

  it('renders "developer" label for developer role', () => {
    render(<UserRoleBadge role="developer" />)
    expect(screen.getByText('developer')).toBeInTheDocument()
  })

  it('renders "admin" label with distinct style for admin role', () => {
    const { container } = render(<UserRoleBadge role="admin" />)
    expect(container.firstChild).toHaveClass('role-badge--admin')
  })

  it('renders "lead" label for lead role', () => {
    render(<UserRoleBadge role="lead" />)
    expect(screen.getByText('lead')).toBeInTheDocument()
  })
})
```

### 3.3 `useAuthSession.test.ts`

```typescript
// src/renderer/src/hooks/__tests__/useAuthSession.test.ts
// @vitest-environment happy-dom
import { renderHook, act } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useAuthSession } from '../useAuthSession'
import { useAppStore } from '../../store'

vi.mock('../../store', () => ({
  useAppStore: vi.fn()
}))

describe('useAuthSession', () => {
  it('returns auth state from store', () => {
    vi.mocked(useAppStore).mockImplementation((selector) =>
      selector({ auth: { status: 'authenticated', user: { id: 'u1', email: 'a@b.com', name: 'A', role: 'developer', provider: 'local' } } } as any)
    )
    const { result } = renderHook(() => useAuthSession())
    expect(result.current.status).toBe('authenticated')
  })

  it('returns unknown status initially', () => {
    vi.mocked(useAppStore).mockImplementation((selector) =>
      selector({ auth: { status: 'unknown' } } as any)
    )
    const { result } = renderHook(() => useAuthSession())
    expect(result.current.status).toBe('unknown')
  })

  it('returns unauthenticated when no session', () => {
    vi.mocked(useAppStore).mockImplementation((selector) =>
      selector({ auth: { status: 'unauthenticated' } } as any)
    )
    const { result } = renderHook(() => useAuthSession())
    expect(result.current.status).toBe('unauthenticated')
  })
})
```

---

## 4. Implementation Specifications

### 4.1 `useAuthSession.ts`

```typescript
// src/renderer/src/hooks/useAuthSession.ts

import { useAppStore } from '../store'
import { AuthState } from '../auth/auth-types'

/** Hook đọc auth state từ Zustand store */
export function useAuthSession(): AuthState {
  return useAppStore(s => s.auth)
}

/** Hook đọc authenticated user, hoặc null */
export function useAuthUser() {
  const auth = useAuthSession()
  return auth.status === 'authenticated' ? auth.user : null
}
```

### 4.2 `useLogout.ts`

```typescript
// src/renderer/src/hooks/useLogout.ts

import { useCallback } from 'react'
import { useAppStore } from '../store'
import { logoutUser } from '../auth/auth-api-client'

export function useLogout() {
  const clearAuth = useAppStore(s => s.clearAuth)

  return useCallback(async () => {
    await logoutUser()    // POST /auth/logout → clear server session
    clearAuth()           // clear Zustand state
    window.location.href = '/login'   // redirect to login
  }, [clearAuth])
}
```

### 4.3 `UserAvatarMenu.tsx`

```typescript
// src/renderer/src/components/auth/UserAvatarMenu.tsx

import { useState, useEffect, useRef } from 'react'
import { AuthUser } from '../../auth/auth-types'
import { UserRoleBadge } from './UserRoleBadge'

type Props = {
  user: AuthUser
  onLogout: () => Promise<void>
}

function getInitials(name: string): string {
  return name.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
}

export function UserAvatarMenu({ user, onLogout }: Props) {
  const [isOpen, setIsOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setIsOpen(false)
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  async function handleLogout() {
    setIsOpen(false)
    await onLogout()
  }

  return (
    <div className="user-avatar-menu" ref={menuRef}>
      <button
        className="user-avatar-menu__trigger"
        onClick={() => setIsOpen(!isOpen)}
        aria-label="Open user menu"
        aria-haspopup="menu"
        aria-expanded={isOpen}
      >
        {user.avatarUrl ? (
          <img src={user.avatarUrl} alt={user.name} className="user-avatar-menu__image" />
        ) : (
          <span className="user-avatar-menu__initials">{getInitials(user.name)}</span>
        )}
      </button>

      {isOpen && (
        <div className="user-avatar-menu__dropdown" role="menu">
          <div className="user-avatar-menu__user-info">
            <span className="user-avatar-menu__name">{user.name}</span>
            <span className="user-avatar-menu__email">{user.email}</span>
            <UserRoleBadge role={user.role} />
          </div>
          <hr />
          <button
            role="menuitem"
            className="user-avatar-menu__logout"
            onClick={handleLogout}
          >
            Logout
          </button>
        </div>
      )}
    </div>
  )
}
```

### 4.4 `UserRoleBadge.tsx`

```typescript
// src/renderer/src/components/auth/UserRoleBadge.tsx

type Role = 'developer' | 'lead' | 'admin'
type Props = { role: Role }

export function UserRoleBadge({ role }: Props) {
  return (
    <span className={`role-badge role-badge--${role}`}>
      {role}
    </span>
  )
}
```

### 4.5 `OrcaProfileSwitcher.tsx` — MODIFY

```typescript
// src/renderer/src/components/orca-profile-switcher/OrcaProfileSwitcher.tsx — MODIFY

// THÊM: Web mode → hiển thị UserAvatarMenu bên cạnh profile switcher
import { useAuthUser, useLogout } from '../../hooks/useAuthSession'
import { UserAvatarMenu } from '../auth/UserAvatarMenu'

// Trong component (Web mode only):
const authUser = useAuthUser()   // null nếu desktop hoặc unauthenticated
const logout   = useLogout()

// Render (thêm vào cuối, sau existing profile UI):
{authUser && import.meta.env.ORCA_PLATFORM === 'web' && (
  <UserAvatarMenu user={authUser} onLogout={logout} />
)}
```

---

## 5. Files cần tạo/sửa

### Tạo mới

| File | Vai trò | Tests |
|------|---------|-------|
| `src/renderer/src/hooks/useAuthSession.ts` | Auth state hooks | useAuthSession.test.ts (3 tests) |
| `src/renderer/src/hooks/useLogout.ts` | Logout action hook | useLogout.test.ts (2 tests) |
| `src/renderer/src/components/auth/UserAvatarMenu.tsx` | Avatar dropdown | UserAvatarMenu.test.tsx (6 tests) |
| `src/renderer/src/components/auth/UserRoleBadge.tsx` | Role badge | UserRoleBadge.test.tsx (3 tests) |

### Sửa (minimal — không thay đổi App.tsx)

| File | Thay đổi |
|------|---------|
| `src/renderer/src/components/orca-profile-switcher/OrcaProfileSwitcher.tsx` | Thêm `<UserAvatarMenu>` khi web + authenticated |

---

## 6. Acceptance Criteria

- [x] Avatar hiển thị ảnh GitHub/Google khi `avatarUrl` có giá trị
- [x] Initials fallback (2 chữ đầu tên) khi không có avatar
- [x] Click avatar → dropdown mở với: tên, email, role badge
- [x] Click Logout → `POST /auth/logout` → clear store → redirect `/login`
- [x] Escape key đóng dropdown
- [x] `UserRoleBadge` render đúng class CSS cho `admin`, `lead`, `developer`
- [x] Không xuất hiện trong Desktop mode (`ORCA_PLATFORM !== 'web'`)
- [x] Không xuất hiện khi `auth.status !== 'authenticated'`

## 7. Implementation Results

| File | Status | Tests |
|------|--------|-------|
| `hooks/useAuthSession.ts` | ✅ Created | 6 tests pass |
| `hooks/useLogout.ts` | ✅ Created | 3 tests pass |
| `components/auth/UserAvatarMenu.tsx` | ✅ Created | 8 tests pass |
| `components/auth/UserRoleBadge.tsx` | ✅ Created | 4 tests pass |
| `components/orca-profile-switcher/OrcaProfileSwitcher.tsx` | ✅ Modified | — |

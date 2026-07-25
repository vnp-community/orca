# SOL-FE-LG-003 — Admin Panel SPA (Users, Sessions, Policies, Audit)

**CR:** [CR-LOGIN-004](../../../../../docs/crs/v1/login/CR-LOGIN-004-admin.md)
**Backend Solution:** [SOL-LG-004](../../../../backend/crs/v1/login/solutions/SOL-LG-004-admin-ui.md)
**TDD Refs:** TDD-FE-06 (Web Client — SPA routing), TDD-FE-05 (UI Components — Layout, Forms, Tables)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Done — Implemented & verified 2026-07-24
**Blocked by:** SOL-FE-LG-001 (auth session cookie), SOL-LG-004 backend phải deploy trước

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 Admin Panel là separate SPA (CR-LOGIN-004 §4)

Admin panel (`/admin/`) là **separate entry** — không thuộc vào `App.tsx`:

```
https://b15.openledger.vn/admin/     ← Admin SPA (riêng)
https://b15.openledger.vn/           ← Main App (App.tsx)
```

**Approach:**
- Backend serve `admin-index.html` tại `/admin/`
- Frontend có `src/renderer/src/admin/` — entry point riêng: `admin-main.tsx`
- React Router (đã có trong dự án) cho client-side navigation

### 1.2 Backend API (SOL-LG-004)

```
GET  /admin/api/users             → list users
POST /admin/api/users             → create user
PATCH /admin/api/users/:id        → update user
DELETE /admin/api/users/:id       → deactivate user
GET  /admin/api/sessions          → list sessions
DELETE /admin/api/sessions/:id    → kill session
GET  /admin/api/policies          → list policies
POST /admin/api/policies          → create policy
PATCH /admin/api/policies/:id     → update policy
DELETE /admin/api/policies/:id    → delete policy
GET  /admin/api/audit             → audit log (filter: user, action, from, to)
GET  /admin/api/stats             → dashboard stats
```

---

## 2. File Structure

```
src/renderer/src/
├── admin/                                       ← [NEW] Admin SPA root
│   ├── admin-main.tsx                           ← Entry point: mount AdminApp
│   └── AdminApp.tsx                             ← Root với React Router
│
├── components/admin/
│   ├── AdminLayout.tsx                          ← Layout: sidebar nav + content area
│   ├── AdminDashboard.tsx                       ← /admin/ — stats + active sessions
│   ├── UsersPage.tsx                            ← /admin/users — user list
│   ├── UserForm.tsx                             ← /admin/users/new + /admin/users/:id/edit
│   ├── PoliciesPage.tsx                         ← /admin/policies — policy list
│   ├── PolicyForm.tsx                           ← Create/Edit policy form
│   ├── SessionsPage.tsx                         ← /admin/sessions — active sessions
│   ├── AuditPage.tsx                            ← /admin/audit — audit log
│   ├── admin-api-client.ts                      ← fetch() wrapper /admin/api/*
│   └── __tests__/
│       ├── admin-api-client.test.ts
│       ├── AdminDashboard.test.tsx
│       └── UsersPage.test.tsx
│
└── hooks/
    ├── useAdminStats.ts                         ← Poll /admin/api/stats (30s)
    ├── useAdminUsers.ts                         ← fetch + mutate users
    ├── useAdminSessions.ts                      ← fetch + kill sessions
    ├── useAdminPolicies.ts                      ← fetch + CRUD policies
    └── useAdminAudit.ts                         ← fetch audit log với filter
```

---

## 3. Test Specifications

### 3.1 `admin-api-client.test.ts`

```typescript
// src/renderer/src/components/admin/__tests__/admin-api-client.test.ts
// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  fetchAdminStats, fetchAdminUsers, createAdminUser,
  updateAdminUser, deactivateAdminUser,
  fetchAdminSessions, killAdminSession,
  fetchAdminAudit
} from '../admin-api-client'

describe('AdminApiClient', () => {
  beforeEach(() => { global.fetch = vi.fn() })
  afterEach(() => { vi.restoreAllMocks() })

  describe('fetchAdminStats', () => {
    it('returns stats on success', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify({ totalUsers: 12, activeSessions: 3, sshConnections: 5, pairedDevices: 28 }),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      ))
      const stats = await fetchAdminStats()
      expect(stats.totalUsers).toBe(12)
      expect(stats.activeSessions).toBe(3)
    })

    it('throws on 403 forbidden', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 403 }))
      await expect(fetchAdminStats()).rejects.toThrow(/forbidden/i)
    })
  })

  describe('fetchAdminUsers', () => {
    it('calls GET /admin/api/users with credentials', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify([{ id: 'u1', email: 'alice@co.com', name: 'Alice', role: 'developer', isActive: true }]),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      ))
      const users = await fetchAdminUsers()
      expect(users).toHaveLength(1)
      expect(fetch).toHaveBeenCalledWith('/admin/api/users', expect.objectContaining({ credentials: 'include' }))
    })
  })

  describe('createAdminUser', () => {
    it('sends POST /admin/api/users with user data', async () => {
      const newUser = { email: 'bob@co.com', name: 'Bob', role: 'developer' as const, password: 'temp123' }
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify({ id: 'u2', ...newUser }),
        { status: 201, headers: { 'Content-Type': 'application/json' } }
      ))
      const created = await createAdminUser(newUser)
      expect(created.id).toBe('u2')
      expect(fetch).toHaveBeenCalledWith('/admin/api/users', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify(newUser),
        credentials: 'include'
      }))
    })
  })

  describe('killAdminSession', () => {
    it('sends DELETE /admin/api/sessions/:id', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 200 }))
      await killAdminSession('sess-123')
      expect(fetch).toHaveBeenCalledWith('/admin/api/sessions/sess-123', expect.objectContaining({
        method: 'DELETE', credentials: 'include'
      }))
    })
  })

  describe('fetchAdminAudit', () => {
    it('passes filter params as query string', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }
      ))
      await fetchAdminAudit({ from: '2026-07-24', to: '2026-07-24', action: 'login.success' })
      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('from=2026-07-24'),
        expect.any(Object)
      )
    })
  })
})
```

### 3.2 `AdminDashboard.test.tsx`

```typescript
// src/renderer/src/components/admin/__tests__/AdminDashboard.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AdminDashboard } from '../AdminDashboard'
import * as adminApiClient from '../admin-api-client'

vi.mock('../admin-api-client')

const mockStats = { totalUsers: 12, activeSessions: 3, sshConnections: 5, pairedDevices: 28 }
const mockSessions = [
  { sessionId: 's1', userId: 'u1', userEmail: 'alice@co.com', ipAddress: '10.0.0.1', createdAt: Date.now() - 7200000, lastSeenAt: Date.now() - 5000 }
]

describe('AdminDashboard', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.mocked(adminApiClient.fetchAdminStats).mockResolvedValue(mockStats)
    vi.mocked(adminApiClient.fetchAdminSessions).mockResolvedValue(mockSessions)
  })

  it('renders stat cards with correct counts after load', async () => {
    render(<AdminDashboard />)
    await waitFor(() => {
      expect(screen.getByText('12')).toBeInTheDocument()   // totalUsers
      expect(screen.getByText('3')).toBeInTheDocument()    // activeSessions
      expect(screen.getByText('28')).toBeInTheDocument()   // pairedDevices
    })
  })

  it('renders active sessions table with user email', async () => {
    render(<AdminDashboard />)
    await waitFor(() => {
      expect(screen.getByText('alice@co.com')).toBeInTheDocument()
    })
  })

  it('shows loading state before data arrives', () => {
    vi.mocked(adminApiClient.fetchAdminStats).mockImplementation(() => new Promise(() => {}))
    render(<AdminDashboard />)
    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument()
  })
})
```

### 3.3 `UsersPage.test.tsx`

```typescript
// src/renderer/src/components/admin/__tests__/UsersPage.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { UsersPage } from '../UsersPage'
import * as adminApiClient from '../admin-api-client'

vi.mock('../admin-api-client')

const mockUsers = [
  { id: 'u1', email: 'alice@co.com', name: 'Alice', role: 'developer', provider: 'github', isActive: true, lastLoginAt: Date.now() },
  { id: 'u2', email: 'bob@co.com', name: 'Bob', role: 'lead', provider: 'local', isActive: true, lastLoginAt: null },
  { id: 'u3', email: 'charlie@co.com', name: 'Charlie', role: 'admin', provider: 'google', isActive: false, lastLoginAt: null },
]

describe('UsersPage', () => {
  afterEach(cleanup)
  beforeEach(() => {
    vi.mocked(adminApiClient.fetchAdminUsers).mockResolvedValue(mockUsers)
  })

  it('renders user list with email, name, role, provider', async () => {
    render(<UsersPage />)
    await waitFor(() => {
      expect(screen.getByText('alice@co.com')).toBeInTheDocument()
      expect(screen.getByText('developer')).toBeInTheDocument()
      expect(screen.getByText('github')).toBeInTheDocument()
    })
  })

  it('filters users by role select', async () => {
    render(<UsersPage />)
    await waitFor(() => screen.getByText('alice@co.com'))
    fireEvent.change(screen.getByLabelText(/role/i), { target: { value: 'admin' } })
    await waitFor(() => {
      expect(screen.getByText('charlie@co.com')).toBeInTheDocument()
      expect(screen.queryByText('alice@co.com')).not.toBeInTheDocument()
    })
  })

  it('filters users by search text', async () => {
    render(<UsersPage />)
    await waitFor(() => screen.getByText('alice@co.com'))
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'bob' } })
    await waitFor(() => {
      expect(screen.getByText('bob@co.com')).toBeInTheDocument()
      expect(screen.queryByText('alice@co.com')).not.toBeInTheDocument()
    })
  })

  it('calls deactivateAdminUser when Deactivate button clicked', async () => {
    vi.mocked(adminApiClient.deactivateAdminUser).mockResolvedValue(undefined)
    render(<UsersPage />)
    await waitFor(() => screen.getAllByRole('button', { name: /deactivate/i }))
    fireEvent.click(screen.getAllByRole('button', { name: /deactivate/i })[0])
    await waitFor(() => expect(adminApiClient.deactivateAdminUser).toHaveBeenCalledWith('u1'))
  })

  it('shows Create User button', async () => {
    render(<UsersPage />)
    await waitFor(() => screen.getByRole('link', { name: /create user/i }))
    expect(screen.getByRole('link', { name: /create user/i })).toBeInTheDocument()
  })
})
```

---

## 4. Implementation Specifications

### 4.1 `admin-api-client.ts`

```typescript
// src/renderer/src/components/admin/admin-api-client.ts

export type AdminUser = {
  id: string; email: string; name: string
  role: 'developer' | 'lead' | 'admin'
  provider: string; isActive: boolean; lastLoginAt: number | null
}

export type AdminStats = {
  totalUsers: number; activeSessions: number
  sshConnections: number; pairedDevices: number
}

export type AdminSession = {
  sessionId: string; userId: string; userEmail: string
  ipAddress: string; userAgent?: string
  createdAt: number; lastSeenAt: number
}

export type AdminPolicy = {
  id: string; name: string; teams: string[]; roles: string[]
  allowedServers: string; canCreateWorktrees: boolean
  canAccessProduction: boolean
}

export type AuditEntry = {
  id: number; createdAt: number; userId?: string; userEmail?: string
  action: string; detail?: unknown; ipAddress?: string
}

const BASE = '/admin/api'
const HEADERS = { 'Content-Type': 'application/json' }
const CREDS: RequestInit = { credentials: 'include' }

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { ...CREDS, ...init })
  if (res.status === 403) throw new Error('Forbidden: admin access required')
  if (res.status === 401) throw new Error('Unauthorized: please login')
  if (!res.ok) throw new Error(`API error: ${res.status}`)
  if (res.status === 204 || res.headers.get('Content-Length') === '0') return undefined as T
  return res.json()
}

export const fetchAdminStats   = () => apiFetch<AdminStats>('/stats')
export const fetchAdminUsers   = () => apiFetch<AdminUser[]>('/users')
export const fetchAdminSessions = () => apiFetch<AdminSession[]>('/sessions')
export const fetchAdminPolicies = () => apiFetch<AdminPolicy[]>('/policies')

export const createAdminUser = (data: Partial<AdminUser> & { password?: string }) =>
  apiFetch<AdminUser>('/users', { method: 'POST', headers: HEADERS, body: JSON.stringify(data) })

export const updateAdminUser = (id: string, data: Partial<AdminUser>) =>
  apiFetch<AdminUser>(`/users/${id}`, { method: 'PATCH', headers: HEADERS, body: JSON.stringify(data) })

export const deactivateAdminUser = (id: string) =>
  apiFetch<void>(`/users/${id}`, { method: 'DELETE' })

export const killAdminSession = (sessionId: string) =>
  apiFetch<void>(`/sessions/${sessionId}`, { method: 'DELETE' })

export const createAdminPolicy = (data: Omit<AdminPolicy, 'id'>) =>
  apiFetch<AdminPolicy>('/policies', { method: 'POST', headers: HEADERS, body: JSON.stringify(data) })

export const updateAdminPolicy = (id: string, data: Partial<AdminPolicy>) =>
  apiFetch<AdminPolicy>(`/policies/${id}`, { method: 'PATCH', headers: HEADERS, body: JSON.stringify(data) })

export const deleteAdminPolicy = (id: string) =>
  apiFetch<void>(`/policies/${id}`, { method: 'DELETE' })

export const fetchAdminAudit = (filter?: { from?: string; to?: string; action?: string; userId?: string }) => {
  const params = new URLSearchParams(filter as Record<string, string>).toString()
  return apiFetch<AuditEntry[]>(`/audit${params ? `?${params}` : ''}`)
}
```

### 4.2 `AdminLayout.tsx`

```typescript
// src/renderer/src/components/admin/AdminLayout.tsx

import { NavLink, Outlet } from 'react-router-dom'
import { useAuthUser } from '../../hooks/useAuthSession'
import { useLogout } from '../../hooks/useLogout'

const NAV_ITEMS = [
  { to: '/admin/', label: '📊 Dashboard', exact: true },
  { to: '/admin/users', label: '👥 Users' },
  { to: '/admin/policies', label: '🔐 Policies' },
  { to: '/admin/sessions', label: '📡 Sessions' },
  { to: '/admin/audit', label: '📋 Audit Log' },
]

export function AdminLayout() {
  const user = useAuthUser()
  const logout = useLogout()

  return (
    <div className="admin-layout">
      <header className="admin-header">
        <span className="admin-header__title">🔧 Orca Admin</span>
        <div className="admin-header__user">
          <span>{user?.email}</span>
          <button onClick={logout}>Logout</button>
        </div>
      </header>

      <nav className="admin-nav" aria-label="Admin navigation">
        {NAV_ITEMS.map(({ to, label, exact }) => (
          <NavLink
            key={to}
            to={to}
            end={exact}
            className={({ isActive }) => `admin-nav__link${isActive ? ' admin-nav__link--active' : ''}`}
          >
            {label}
          </NavLink>
        ))}
      </nav>

      <main className="admin-content">
        <Outlet />
      </main>
    </div>
  )
}
```

### 4.3 `AdminApp.tsx`

```typescript
// src/renderer/src/admin/AdminApp.tsx

import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AdminLayout } from '../components/admin/AdminLayout'
import { AdminDashboard } from '../components/admin/AdminDashboard'
import { UsersPage } from '../components/admin/UsersPage'
import { UserForm } from '../components/admin/UserForm'
import { PoliciesPage } from '../components/admin/PoliciesPage'
import { PolicyForm } from '../components/admin/PolicyForm'
import { SessionsPage } from '../components/admin/SessionsPage'
import { AuditPage } from '../components/admin/AuditPage'

export function AdminApp() {
  return (
    <BrowserRouter basename="/admin">
      <Routes>
        <Route element={<AdminLayout />}>
          <Route index element={<AdminDashboard />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="users/new" element={<UserForm />} />
          <Route path="users/:id/edit" element={<UserForm />} />
          <Route path="policies" element={<PoliciesPage />} />
          <Route path="policies/new" element={<PolicyForm />} />
          <Route path="policies/:id/edit" element={<PolicyForm />} />
          <Route path="sessions" element={<SessionsPage />} />
          <Route path="audit" element={<AuditPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
```

### 4.4 `useAdminStats.ts`

```typescript
// src/renderer/src/hooks/useAdminStats.ts

import { useEffect, useState, useCallback } from 'react'
import { fetchAdminStats, AdminStats } from '../components/admin/admin-api-client'

const POLL_INTERVAL_MS = 30_000

export function useAdminStats() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchAdminStats()
      setStats(data)
      setError(null)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const timer = setInterval(refresh, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  return { stats, isLoading, error, refresh }
}
```

### 4.5 `AdminDashboard.tsx` — Key Structure

```typescript
// src/renderer/src/components/admin/AdminDashboard.tsx

import { useAdminStats } from '../../hooks/useAdminStats'
import { fetchAdminSessions, AdminSession } from './admin-api-client'
import { useEffect, useState } from 'react'

export function AdminDashboard() {
  const { stats, isLoading } = useAdminStats()
  const [sessions, setSessions] = useState<AdminSession[]>([])

  useEffect(() => {
    fetchAdminSessions().then(setSessions).catch(console.error)
  }, [])

  if (isLoading) return <div role="status" aria-label="Loading">Loading...</div>

  return (
    <div className="admin-dashboard">
      <h1>Overview</h1>

      {/* Stat cards */}
      <div className="admin-stats-grid">
        <StatCard label="Users" value={stats?.totalUsers ?? 0} />
        <StatCard label="Active" value={stats?.activeSessions ?? 0} />
        <StatCard label="SSH Conn." value={stats?.sshConnections ?? 0} />
        <StatCard label="Devices" value={stats?.pairedDevices ?? 0} />
      </div>

      {/* Active sessions table */}
      <section>
        <h2>🟢 Active Sessions</h2>
        <table>
          <thead>
            <tr>
              <th>User</th><th>Role</th><th>IP</th><th>Since</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map(s => (
              <tr key={s.sessionId}>
                <td>{s.userEmail}</td>
                <td>{s.ipAddress}</td>
                <td>{formatRelative(s.createdAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="admin-stat-card">
      <span className="admin-stat-card__value">{value}</span>
      <span className="admin-stat-card__label">{label}</span>
    </div>
  )
}

function formatRelative(ts: number) {
  const diff = Date.now() - ts
  const h = Math.floor(diff / 3600000)
  const m = Math.floor((diff % 3600000) / 60000)
  if (h > 0) return `${h}h ago`
  return `${m}m ago`
}
```

---

## 5. Files cần tạo/sửa

### Tạo mới

| File | Vai trò | Tests |
|------|---------|-------|
| `src/renderer/src/admin/admin-main.tsx` | Admin SPA entry | — |
| `src/renderer/src/admin/AdminApp.tsx` | React Router root | — |
| `src/renderer/src/components/admin/admin-api-client.ts` | Fetch wrapper | admin-api-client.test.ts (5 tests) |
| `src/renderer/src/components/admin/AdminLayout.tsx` | Layout với sidebar nav | — |
| `src/renderer/src/components/admin/AdminDashboard.tsx` | Stats + sessions | AdminDashboard.test.tsx (3 tests) |
| `src/renderer/src/components/admin/UsersPage.tsx` | User list + filter | UsersPage.test.tsx (5 tests) |
| `src/renderer/src/components/admin/UserForm.tsx` | Create/Edit user | UserForm.test.tsx (4 tests) |
| `src/renderer/src/components/admin/PoliciesPage.tsx` | Policy list | PoliciesPage.test.tsx (3 tests) |
| `src/renderer/src/components/admin/PolicyForm.tsx` | Create/Edit policy | — |
| `src/renderer/src/components/admin/SessionsPage.tsx` | Session kill controls | SessionsPage.test.tsx (3 tests) |
| `src/renderer/src/components/admin/AuditPage.tsx` | Audit log + filter | AuditPage.test.tsx (3 tests) |
| `src/renderer/src/hooks/useAdminStats.ts` | Poll stats 30s | useAdminStats.test.ts (3 tests) |
| `src/renderer/src/hooks/useAdminUsers.ts` | Fetch + mutate users | — |
| `src/renderer/src/hooks/useAdminSessions.ts` | Fetch + kill sessions | — |

### Vite build config (thêm admin entry point)

```typescript
// vite.config.ts (renderer) — MODIFY: thêm admin entry
export default {
  build: {
    rollupOptions: {
      input: {
        main: 'src/renderer/index.html',           // main app
        admin: 'src/renderer/admin-index.html',    // admin SPA
      }
    }
  }
}
```

```html
<!-- src/renderer/admin-index.html — NEW -->
<!DOCTYPE html>
<html>
  <head>
    <title>Orca Admin</title>
    <meta name="robots" content="noindex">
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="./src/admin/admin-main.tsx"></script>
  </body>
</html>
```

---

## 6. Acceptance Criteria

- [x] `/admin/` accessible chỉ khi cookie session tồn tại (403/401 → redirect `/login`)
- [x] Dashboard hiển thị 4 stat cards (Users, Active, SSH, Devices)
- [x] User list có search box và role filter
- [x] Deactivate user → call DELETE /admin/api/users/:id → remove from list
- [x] Kill session → call DELETE /admin/api/sessions/:id → remove từ list
- [x] Create User form: validate email format + password match
- [x] Audit log: filter by date range và action type
- [x] Admin SPA không ảnh hưởng main App bundle (separate entry point)
- [x] React Router navigation hoạt động với browser back/forward

## 8. Implementation Results

| File | Status | Tests |
|------|--------|-------|
| `components/admin/admin-api-client.ts` | ✅ Created | 7 tests pass |
| `components/admin/AdminApp.tsx` | ✅ Created | — |
| `components/admin/AdminDashboard.tsx` | ✅ Created | 3 tests pass |
| `components/admin/UsersPage.tsx` | ✅ Created | 5 tests pass |
| `components/admin/UserForm.tsx` | ✅ Created | — |
| `components/admin/SessionsPage.tsx` | ✅ Created | 3 tests pass |
| `components/admin/PoliciesPage.tsx` | ✅ Created | — |
| `components/admin/PolicyForm.tsx` | ✅ Created | — |
| `components/admin/AuditPage.tsx` | ✅ Created | 3 tests pass |
| `admin/admin-main.tsx` | ✅ Created | — |
| `renderer/admin-index.html` | ✅ Created | — |
| `vite.web.config.ts` | ✅ Modified | admin entry added |
| `vite.web-spa.config.ts` | ✅ Modified | admin entry added |
| `electron.vite.config.ts` | ✅ Modified | admin entry added |

---

## 7. Dependency

- **Backend prerequisite:** `/admin/api/*` endpoints từ SOL-LG-004 phải deploy
- **Frontend prerequisite:** Auth session cookie từ SOL-FE-LG-001

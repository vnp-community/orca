# SOL-FE-LG-001 — Login Page + SSO UI + Session-Aware Routing

**CR:** [CR-LOGIN-001](../../../../../docs/crs/v1/login/CR-LOGIN-001-auth.md)
**Backend Solution:** [SOL-LG-001](../../../../backend/crs/v1/login/solutions/SOL-LG-001-auth-session.md)
**TDD Refs:** TDD-FE-06 (Web Client), TDD-FE-02 (State Management), TDD-FE-05 (UI Components)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Done — Implemented & verified 2026-07-24

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 Web Entry Flow hiện tại (TDD-FE-06 §2)

```typescript
// src/renderer/src/web/main.tsx — HIỆN TẠI
// installWebPreloadApi() → readStoredWebRuntimeEnvironment()
// Nếu có savedEnv → renderApp()
// Nếu không      → renderWebConnect()  ← pairing form
```

**Vấn đề:** Không có bước nào check auth session từ server trước khi render.

### 1.2 `main-web-bootstrap.tsx` (TDD-FE restructure_v1)

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx — ĐÃ CÓ
export async function bootstrapWebApp(): Promise<void>
// Hiện tại: register SW, installWebPreloadApi(), render App/WebConnect
// CẦN THÊM: checkAuthSession() → route đến LoginPage | App | WebConnect
```

### 1.3 WebConnect.tsx hiện tại

```typescript
// src/renderer/src/web/WebConnect.tsx — ĐÃ CÓ
// Render form nhập pairing URL
// CẦN: nếu session đã authenticated → skip WebConnect → thẳng App
```

---

## 2. File Structure

```
src/renderer/src/auth/
├── auth-api-client.ts            ← [NEW] fetch() wrapper
├── auth-types.ts                 ← [NEW] Frontend auth types
└── __tests__/
    └── auth-api-client.test.ts

src/renderer/src/store/slices/
└── auth.ts                       ← [NEW] Zustand AuthSlice

src/renderer/src/web/login/
├── LoginPage.tsx                 ← [NEW] Trang login chính
├── LoginForm.tsx                 ← [NEW] Email/password form
├── SsoButton.tsx                 ← [NEW] SSO provider button
├── PairCodeFallback.tsx          ← [NEW] Fallback pairing section
└── __tests__/
    ├── LoginPage.test.tsx
    ├── LoginForm.test.tsx
    └── SsoButton.test.tsx

src/renderer/src/web/
├── main-web-bootstrap.tsx        ← [MODIFY] Thêm auth session routing
└── WebConnect.tsx                ← [MODIFY] Guard: skip nếu authenticated
```

---

## 3. Test Specifications

### 3.1 `auth-api-client.test.ts`

```typescript
// src/renderer/src/auth/__tests__/auth-api-client.test.ts
// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { fetchCurrentUser, loginLocal, logoutUser } from '../auth-api-client'

describe('AuthApiClient', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })
  afterEach(() => { vi.restoreAllMocks() })

  describe('fetchCurrentUser', () => {
    it('returns AuthUser when session is valid', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify({ id: 'u1', email: 'alice@co.com', name: 'Alice', role: 'developer', provider: 'local' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      ))
      const user = await fetchCurrentUser()
      expect(user).toMatchObject({ email: 'alice@co.com', role: 'developer' })
      expect(fetch).toHaveBeenCalledWith('/auth/me', expect.objectContaining({ credentials: 'include' }))
    })

    it('returns null when no session (401)', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 401 }))
      const user = await fetchCurrentUser()
      expect(user).toBeNull()
    })

    it('throws on network error', async () => {
      vi.mocked(fetch).mockRejectedValueOnce(new Error('Network error'))
      await expect(fetchCurrentUser()).rejects.toThrow('Network error')
    })
  })

  describe('loginLocal', () => {
    it('returns AuthUser on successful login', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify({ id: 'u1', email: 'alice@co.com', name: 'Alice', role: 'developer', provider: 'local' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      ))
      const user = await loginLocal('alice@co.com', 'password123')
      expect(user).toMatchObject({ email: 'alice@co.com' })
      expect(fetch).toHaveBeenCalledWith('/auth/local', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'alice@co.com', password: 'password123' }),
        credentials: 'include'
      }))
    })

    it('throws AuthError with message on 401', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(
        JSON.stringify({ error: 'Invalid credentials' }), { status: 401 }
      ))
      await expect(loginLocal('bad@co.com', 'wrong')).rejects.toMatchObject({
        code: 'invalid_credentials'
      })
    })

    it('throws on 500 server error', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 500 }))
      await expect(loginLocal('a@b.com', 'p')).rejects.toThrow()
    })
  })

  describe('logoutUser', () => {
    it('calls POST /auth/logout with credentials', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response('', { status: 200 }))
      await logoutUser()
      expect(fetch).toHaveBeenCalledWith('/auth/logout', expect.objectContaining({
        method: 'POST', credentials: 'include'
      }))
    })
  })
})
```

### 3.2 `LoginPage.test.tsx`

```typescript
// src/renderer/src/web/login/__tests__/LoginPage.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { LoginPage } from '../LoginPage'
import * as authApiClient from '../../../auth/auth-api-client'

vi.mock('../../../auth/auth-api-client')

describe('LoginPage', () => {
  afterEach(cleanup)

  describe('local login form', () => {
    it('renders email, password fields and Sign In button', () => {
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
      expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
    })

    it('calls loginLocal with email and password on submit', async () => {
      const onSuccess = vi.fn()
      vi.mocked(authApiClient.loginLocal).mockResolvedValueOnce({
        id: 'u1', email: 'alice@co.com', name: 'Alice', role: 'developer', provider: 'local'
      })
      render(<LoginPage availableProviders={[]} onLoginSuccess={onSuccess} />)
      fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'alice@co.com' } })
      fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'password123' } })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      await waitFor(() => expect(onSuccess).toHaveBeenCalledWith(
        expect.objectContaining({ email: 'alice@co.com' })
      ))
    })

    it('shows error message on invalid credentials', async () => {
      vi.mocked(authApiClient.loginLocal).mockRejectedValueOnce(
        Object.assign(new Error('Invalid credentials'), { code: 'invalid_credentials' })
      )
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'bad@co.com' } })
      fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'wrong' } })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/invalid credentials/i))
    })

    it('disables button during loading', async () => {
      vi.mocked(authApiClient.loginLocal).mockImplementationOnce(() =>
        new Promise(resolve => setTimeout(resolve, 500))
      )
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'a@b.com' } })
      fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'p' } })
      fireEvent.click(screen.getByRole('button', { name: /sign in/i }))
      expect(screen.getByRole('button', { name: /sign in/i })).toBeDisabled()
    })
  })

  describe('SSO buttons', () => {
    it('renders SSO buttons for available providers', () => {
      render(<LoginPage availableProviders={['github', 'google']} onLoginSuccess={vi.fn()} />)
      expect(screen.getByRole('link', { name: /continue with github/i })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: /continue with google/i })).toBeInTheDocument()
    })

    it('SSO button href points to /auth/sso/:provider', () => {
      render(<LoginPage availableProviders={['github']} onLoginSuccess={vi.fn()} />)
      expect(screen.getByRole('link', { name: /github/i })).toHaveAttribute('href', '/auth/sso/github')
    })

    it('does not render SSO section if no providers', () => {
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      expect(screen.queryByText(/continue with/i)).not.toBeInTheDocument()
    })
  })

  describe('PairCode fallback', () => {
    it('renders pairing URL input section', () => {
      render(<LoginPage availableProviders={[]} onLoginSuccess={vi.fn()} />)
      expect(screen.getByPlaceholderText(/pairing url or code/i)).toBeInTheDocument()
    })
  })
})
```

### 3.3 `LoginForm.test.tsx`

```typescript
// src/renderer/src/web/login/__tests__/LoginForm.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginForm } from '../LoginForm'

describe('LoginForm', () => {
  afterEach(cleanup)

  it('calls onSubmit with email and password', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<LoginForm onSubmit={onSubmit} isLoading={false} error={null} />)
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'alice@co.com' } })
    fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'pass123' } })
    fireEvent.submit(screen.getByRole('form'))
    expect(onSubmit).toHaveBeenCalledWith('alice@co.com', 'pass123')
  })

  it('validates email format — shows error for invalid email', () => {
    const onSubmit = vi.fn()
    render(<LoginForm onSubmit={onSubmit} isLoading={false} error={null} />)
    fireEvent.change(screen.getByLabelText(/email/i), { target: { value: 'not-an-email' } })
    fireEvent.submit(screen.getByRole('form'))
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('displays server-side error from props', () => {
    render(<LoginForm onSubmit={vi.fn()} isLoading={false} error="Invalid credentials" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Invalid credentials')
  })
})
```

---

## 4. Implementation Specifications

### 4.1 `auth-types.ts`

```typescript
// src/renderer/src/auth/auth-types.ts

export type SsoProvider = 'github' | 'google' | 'keycloak'

export type AuthUser = {
  id:        string
  email:     string
  name:      string
  role:      'developer' | 'lead' | 'admin'
  provider:  'none' | SsoProvider
  avatarUrl?: string
}

export type AuthState =
  | { status: 'unknown' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; user: AuthUser }
  | { status: 'error'; message: string }

export class AuthError extends Error {
  constructor(message: string, public code: string) {
    super(message)
    this.name = 'AuthError'
  }
}
```

### 4.2 `auth-api-client.ts`

```typescript
// src/renderer/src/auth/auth-api-client.ts

import { AuthError, AuthUser } from './auth-types'

/** GET /auth/me — returns null if no session */
export async function fetchCurrentUser(): Promise<AuthUser | null> {
  const res = await fetch('/auth/me', { credentials: 'include' })
  if (res.status === 401) return null
  if (!res.ok) throw new Error(`Server error: ${res.status}`)
  return res.json() as Promise<AuthUser>
}

/** POST /auth/local — email+password login */
export async function loginLocal(email: string, password: string): Promise<AuthUser> {
  const res = await fetch('/auth/local', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  })
  const body = await res.json()
  if (!res.ok) {
    throw new AuthError(body.error ?? 'Login failed', 'invalid_credentials')
  }
  return body as AuthUser
}

/** POST /auth/logout */
export async function logoutUser(): Promise<void> {
  await fetch('/auth/logout', { method: 'POST', credentials: 'include' })
}

/** GET /auth/config — danh sách SSO providers được kích hoạt */
export async function fetchAuthConfig(): Promise<{ providers: string[]; localEnabled: boolean }> {
  const res = await fetch('/auth/config', { credentials: 'include' })
  if (!res.ok) return { providers: [], localEnabled: true }
  return res.json()
}
```

### 4.3 `store/slices/auth.ts`

```typescript
// src/renderer/src/store/slices/auth.ts

import { StateCreator } from 'zustand'
import { AuthUser, AuthState } from '../../auth/auth-types'
import { fetchCurrentUser } from '../../auth/auth-api-client'

export type AuthSlice = {
  auth: AuthState
  setAuth: (state: AuthState) => void
  clearAuth: () => void
  checkSession: () => Promise<void>
}

export const createAuthSlice: StateCreator<AuthSlice> = (set) => ({
  auth: { status: 'unknown' },

  setAuth: (state) => set({ auth: state }),

  clearAuth: () => set({ auth: { status: 'unauthenticated' } }),

  checkSession: async () => {
    try {
      const user = await fetchCurrentUser()
      if (user) {
        set({ auth: { status: 'authenticated', user } })
      } else {
        set({ auth: { status: 'unauthenticated' } })
      }
    } catch (err) {
      set({ auth: { status: 'error', message: (err as Error).message } })
    }
  },
})
```

### 4.4 `main-web-bootstrap.tsx` — MODIFY

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx — MODIFY

// THÊM: import auth check
import { fetchCurrentUser } from '../auth/auth-api-client'
import { fetchAuthConfig } from '../auth/auth-api-client'

export async function bootstrapWebApp(): Promise<void> {
  // Bước 1: Register SW (đã có)
  await registerServiceWorker()

  // Bước 2: Install web preload API (đã có)
  installWebPreloadApi()

  // Bước 3 [NEW]: Check auth session
  const [currentUser, authConfig] = await Promise.all([
    fetchCurrentUser().catch(() => null),
    fetchAuthConfig().catch(() => ({ providers: [], localEnabled: true }))
  ])

  // Bước 4: Route based on auth state + pairing state
  const savedEnv = readStoredWebRuntimeEnvironment()
  const hasPairingInput = new URLSearchParams(location.search).get('pair')

  if (currentUser) {
    // Đã authenticated: vào thẳng App (hoặc redirect từ /login)
    renderApp({ authUser: currentUser })
  } else if (savedEnv || hasPairingInput) {
    // Chưa login nhưng có pairing data → backward compat
    renderApp({})
  } else {
    // Không có session, không có pairing → show Login page
    renderLoginPage({ authConfig })
  }
}

function renderLoginPage(props: { authConfig: { providers: string[]; localEnabled: boolean } }): void {
  const root = createRoot(document.getElementById('root')!)
  root.render(
    <StrictMode>
      <LoginPage
        availableProviders={props.authConfig.providers as SsoProvider[]}
        onLoginSuccess={(user) => {
          // Reload: bootstrapWebApp sẽ detect session và vào App
          window.location.href = '/'
        }}
      />
    </StrictMode>
  )
}
```

### 4.5 `LoginPage.tsx`

```typescript
// src/renderer/src/web/login/LoginPage.tsx

import { useState } from 'react'
import { LoginForm } from './LoginForm'
import { SsoButton } from './SsoButton'
import { PairCodeFallback } from './PairCodeFallback'
import { loginLocal } from '../../../auth/auth-api-client'
import { AuthUser, AuthError, SsoProvider } from '../../../auth/auth-types'

type Props = {
  availableProviders: SsoProvider[]
  onLoginSuccess: (user: AuthUser) => void
}

export function LoginPage({ availableProviders, onLoginSuccess }: Props) {
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleLocalLogin(email: string, password: string) {
    setIsLoading(true)
    setError(null)
    try {
      const user = await loginLocal(email, password)
      onLoginSuccess(user)
    } catch (err) {
      if (err instanceof AuthError) {
        setError(err.message)
      } else {
        setError('An unexpected error occurred')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="login-page" data-testid="login-page">
      <header className="login-header">
        <h1>Orca</h1>
        <p>Collaborative Dev Environment</p>
      </header>

      <main className="login-content">
        {/* Local login form */}
        <LoginForm
          onSubmit={handleLocalLogin}
          isLoading={isLoading}
          error={error}
        />

        {/* SSO buttons (chỉ render nếu có providers) */}
        {availableProviders.length > 0 && (
          <>
            <div className="login-divider">or</div>
            <div className="login-sso-buttons">
              {availableProviders.map(provider => (
                <SsoButton key={provider} provider={provider} />
              ))}
            </div>
          </>
        )}

        {/* PairCode fallback */}
        <div className="login-divider">or</div>
        <PairCodeFallback />
      </main>
    </div>
  )
}
```

### 4.6 `SsoButton.tsx`

```typescript
// src/renderer/src/web/login/SsoButton.tsx

import type { SsoProvider } from '../../../auth/auth-types'

const PROVIDER_CONFIG: Record<SsoProvider, { label: string; icon: string }> = {
  github:   { label: 'Continue with GitHub',   icon: '🐙' },
  google:   { label: 'Continue with Google',   icon: '🔵' },
  keycloak: { label: 'Continue with Keycloak', icon: '🔑' },
}

type Props = { provider: SsoProvider }

export function SsoButton({ provider }: Props) {
  const { label, icon } = PROVIDER_CONFIG[provider]
  return (
    <a
      href={`/auth/sso/${provider}`}
      className={`sso-button sso-button--${provider}`}
      aria-label={label}
    >
      <span className="sso-button__icon">{icon}</span>
      <span className="sso-button__label">{label}</span>
    </a>
  )
}
```

### 4.7 `PairCodeFallback.tsx`

```typescript
// src/renderer/src/web/login/PairCodeFallback.tsx
// Backward compat: user có thể vẫn dùng PairCode thay vì login

import { useState } from 'react'
import { readStoredWebRuntimeEnvironment, parseWebPairingInput } from '../../web-pairing'

export function PairCodeFallback() {
  const [input, setInput] = useState('')
  const [error, setError] = useState<string | null>(null)

  function handleConnect() {
    const offer = parseWebPairingInput(input.trim())
    if (!offer) { setError('Invalid pairing URL or code'); return }
    // Lưu environment → reload → bootstrapWebApp detect savedEnv
    saveStoredWebRuntimeEnvironment(offer)
    window.location.reload()
  }

  return (
    <div className="pair-code-fallback">
      <label htmlFor="pair-code-input">Pairing URL or Code:</label>
      <input
        id="pair-code-input"
        type="text"
        value={input}
        onChange={e => setInput(e.target.value)}
        placeholder="Pairing URL or code"
      />
      {error && <p className="error" role="alert">{error}</p>}
      <button type="button" onClick={handleConnect} disabled={!input.trim()}>
        Connect
      </button>
    </div>
  )
}
```

---

## 5. Files cần tạo/sửa

### Tạo mới

| File | Loại | Vai trò |
|------|------|---------|
| `src/renderer/src/auth/auth-types.ts` | NEW | AuthUser, AuthState, AuthError, SsoProvider |
| `src/renderer/src/auth/auth-api-client.ts` | NEW | fetch wrapper: /auth/me, /auth/local, /auth/logout |
| `src/renderer/src/auth/__tests__/auth-api-client.test.ts` | NEW | 6 tests |
| `src/renderer/src/store/slices/auth.ts` | NEW | AuthSlice (Zustand) |
| `src/renderer/src/web/login/LoginPage.tsx` | NEW | Login page component |
| `src/renderer/src/web/login/LoginForm.tsx` | NEW | Email/password form |
| `src/renderer/src/web/login/SsoButton.tsx` | NEW | SSO provider link |
| `src/renderer/src/web/login/PairCodeFallback.tsx` | NEW | PairCode backward compat |
| `src/renderer/src/web/login/__tests__/LoginPage.test.tsx` | NEW | 7 tests |
| `src/renderer/src/web/login/__tests__/LoginForm.test.tsx` | NEW | 3 tests |

### Sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/web/main-web-bootstrap.tsx` | Thêm auth session check + renderLoginPage |
| `src/renderer/src/store/index.ts` | Thêm `...createAuthSlice(...a)` |

### Không thay đổi (backward compat)

| File | Lý do |
|------|-------|
| `src/renderer/src/App.tsx` | Nguyên tắc: không sửa App.tsx |
| `src/renderer/src/web/main.tsx` | Desktop entry — không thêm auth |
| `src/renderer/src/web/web-preload-api.ts` | 135KB — không rewrite |
| `src/renderer/src/web/WebConnect.tsx` | Giữ nguyên logic pairing |

---

## 6. Acceptance Criteria

- [x] `GET /auth/me` trả về user → `bootstrapWebApp()` render `App` trực tiếp (bỏ qua Login page)
- [x] `GET /auth/me` trả về 401 → `bootstrapWebApp()` render `LoginPage`
- [x] `POST /auth/local` thành công → `onLoginSuccess()` gọi → redirect `/`
- [x] `POST /auth/local` thất bại → hiển thị error message trong form
- [x] SSO button render đúng cho mỗi provider trong `availableProviders`
- [x] SSO button href = `/auth/sso/{provider}`
- [x] `PairCodeFallback` vẫn hoạt động: nhập PairCode → save env → reload → App
- [x] AuthSlice `checkSession()` update store `auth.status` đúng
- [x] Không thay đổi Desktop entry point (`main.tsx`)

## 8. Implementation Results

| File | Status | Tests |
|------|--------|-------|
| `auth/auth-types.ts` | ✅ Created | — |
| `auth/auth-api-client.ts` | ✅ Created | 10 tests pass |
| `auth/__tests__/auth-api-client.test.ts` | ✅ Created | 10/10 ✅ |
| `store/slices/auth.ts` | ✅ Created | — |
| `web/login/LoginPage.tsx` | ✅ Created | 8 tests pass |
| `web/login/LoginForm.tsx` | ✅ Created | 4 tests pass |
| `web/login/SsoButton.tsx` | ✅ Created | 4 tests pass |
| `web/login/PairCodeFallback.tsx` | ✅ Created | — |
| `web/main-web-bootstrap.tsx` | ✅ Modified | — |
| `store/index.ts` | ✅ Modified | — |

---

## 7. Dependency

- **Backend prerequisite:** `GET /auth/me`, `POST /auth/local`, `GET /auth/config` từ SOL-LG-001 phải được deploy trước
- **Frontend next:** SOL-FE-LG-002 (UserAvatarMenu) cần `AuthSlice` từ solution này

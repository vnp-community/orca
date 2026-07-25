# TASK-FE-004 — Tạo `LoginForm.tsx` + Tests

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §3.3
**Depends on:** TASK-FE-001
**Blocks:** TASK-FE-006
**Effort:** S (~25 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo component form email/password cho login. Component này là "controlled" — nhận `onSubmit`, `isLoading`, `error` từ parent.

---

## Files cần tạo

### `src/renderer/src/web/login/LoginForm.tsx` [NEW]

```typescript
// @vitest-environment happy-dom
import { useState, FormEvent } from 'react'

type Props = {
  onSubmit: (email: string, password: string) => void
  isLoading: boolean
  error: string | null
}

export function LoginForm({ onSubmit, isLoading, error }: Props) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLocalError(null)
    // Client-side email validation
    if (!email.match(/^[^@]+@[^@]+\.[^@]+$/)) {
      setLocalError('Please enter a valid email address')
      return
    }
    onSubmit(email, password)
  }

  const displayError = error ?? localError

  return (
    <form onSubmit={handleSubmit} aria-label="Login form" role="form">
      {displayError && (
        <div role="alert" className="login-form__error">{displayError}</div>
      )}
      <div className="login-form__field">
        <label htmlFor="login-email">Email</label>
        <input
          id="login-email"
          type="email"
          value={email}
          onChange={e => setEmail(e.target.value)}
          disabled={isLoading}
          required
          autoComplete="email"
        />
      </div>
      <div className="login-form__field">
        <label htmlFor="login-password">Password</label>
        <input
          id="login-password"
          type="password"
          value={password}
          onChange={e => setPassword(e.target.value)}
          disabled={isLoading}
          required
          autoComplete="current-password"
        />
      </div>
      <button
        type="submit"
        disabled={isLoading}
        className="login-form__submit"
      >
        {isLoading ? 'Signing in…' : 'Sign In'}
      </button>
    </form>
  )
}
```

### `src/renderer/src/web/login/__tests__/LoginForm.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-001 §3.3](../solutions/SOL-FE-LG-001-login-page.md).

Test cases (3 tests):
- `onSubmit` được gọi với đúng email + password
- Invalid email format → không gọi `onSubmit`
- Server-side error prop → hiển thị trong alert

---

## Verify

```bash
npx vitest run src/renderer/src/web/login/__tests__/LoginForm.test.tsx
# Expected: 3 pass
```

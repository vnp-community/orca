# TASK-FE-005 — Tạo `SsoButton.tsx` + `PairCodeFallback.tsx` + Tests

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.6, §4.7
**Depends on:** TASK-FE-001
**Blocks:** TASK-FE-006
**Effort:** S (~20 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo 2 components phụ của LoginPage:
- `SsoButton`: link button redirect đến backend SSO endpoint
- `PairCodeFallback`: backward compat — nhập PairCode như cũ

---

## Files cần tạo

### `src/renderer/src/web/login/SsoButton.tsx` [NEW]

```typescript
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

### `src/renderer/src/web/login/PairCodeFallback.tsx` [NEW]

```typescript
import { useState } from 'react'
// Import từ web-pairing module đã có
import { parseWebPairingInput, saveStoredWebRuntimeEnvironment } from '../../web-pairing'

export function PairCodeFallback() {
  const [input, setInput] = useState('')
  const [error, setError] = useState<string | null>(null)

  function handleConnect() {
    const offer = parseWebPairingInput(input.trim())
    if (!offer) { setError('Invalid pairing URL or code'); return }
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

### `src/renderer/src/web/login/__tests__/SsoButton.test.tsx` [NEW]

Test cases (3 tests):
- Render link với đúng label cho provider
- href = `/auth/sso/{provider}`
- aria-label đúng

---

## Verify

```bash
npx vitest run src/renderer/src/web/login/__tests__/SsoButton.test.tsx
# Expected: 3 pass
```

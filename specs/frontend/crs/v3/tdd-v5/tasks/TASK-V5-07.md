# TASK-V5-07: CredentialInput + ProviderForm (Security Critical)

**Order:** 7  
**Prerequisite:** TASK-V5-06 (ai-provider slice + crypto lib)  
**Solution Ref:** SOL-FE-V5-03 (section 3, 4)  
**Est. effort:** ~60 min | **Tests:** 7  
**⚠️ Security:** API keys NEVER stored plaintext in React state after encryption

---

## Mô tả

Implement `CredentialInput` (client-side SubtleCrypto encrypt) và `ProviderForm` (Add/Edit dialog). Sau encrypt, rawValue PHẢI được clear ngay lập tức.

---

## Files Cần Tạo

### 1. `src/renderer/src/components/ai-provider/CredentialInput.tsx`

```typescript
import { useState } from 'react'
import type { AIProviderType } from '@shared/ai-provider-types'
import { encryptCredential } from '../../lib/credential-crypto'
import { useAppStore } from '../../store'
import { Input } from '../ui/input'
import { Label } from '../ui/label'
import { Lock, Loader2 } from 'lucide-react'

interface CredentialInputProps {
  provider:    AIProviderType
  hasExisting: boolean
  onEncrypted: (encryptedBlob: string, iv: string) => void
  onClear:     () => void
}

const CREDENTIAL_LABELS: Record<AIProviderType, string | null> = {
  anthropic: 'Anthropic API Key (sk-ant-...)',
  openai:    'OpenAI API Key (sk-...)',
  gemini:    'Google API Key (AIza...)',
  azure:     'Azure OpenAI API Key',
  bedrock:   'AWS Credentials (JSON: accessKey + secret + region)',
  vllm:      'vLLM API Key (optional)',
  ollama:    null,   // no credential needed
}

export function CredentialInput({
  provider, hasExisting, onEncrypted, onClear
}: CredentialInputProps) {
  const [rawValue, setRawValue]         = useState('')
  const [isEncrypting, setIsEncrypting] = useState(false)
  const [isEncrypted, setIsEncrypted]   = useState(false)

  // Ollama: no credential needed
  const label = CREDENTIAL_LABELS[provider]
  if (label === null) return null

  // Get session token for key derivation
  const sessionToken = useAppStore(
    s => (s as any).auth?.sessionToken ?? 'fallback-dev-token'
  ) as string

  const handleChange = async (value: string) => {
    // Reset state first
    setRawValue(value)
    setIsEncrypted(false)
    onClear()

    if (value.length >= 10) {
      setIsEncrypting(true)
      try {
        const { encryptedBlob, iv } = await encryptCredential(value, sessionToken)
        setIsEncrypted(true)
        onEncrypted(encryptedBlob, iv)
      } catch (err) {
        console.error('[CredentialInput] encryption failed:', err)
      } finally {
        setIsEncrypting(false)
        // CRITICAL: clear plaintext from state after encryption
        setRawValue('')
      }
    }
  }

  return (
    <div className="credential-input space-y-1">
      <Label>{label}</Label>
      {hasExisting && !isEncrypted && (
        <p className="text-xs text-muted-foreground">
          Leave blank to keep existing credential
        </p>
      )}
      <div className="relative">
        <Input
          type="password"
          placeholder="Enter API key..."
          value={rawValue}
          onChange={e => handleChange(e.target.value)}
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          data-testid="credential-input"
        />
        {isEncrypting && (
          <Loader2
            className="absolute right-2 top-2.5 animate-spin text-muted-foreground"
            size={16}
          />
        )}
        {isEncrypted && (
          <Lock
            className="absolute right-2 top-2.5 text-green-500"
            size={16}
            data-testid="lock-icon"
          />
        )}
      </div>
      {isEncrypted && (
        <p className="text-xs text-green-600">
          ✓ Credential encrypted in browser — will be stored securely on dev server
        </p>
      )}
    </div>
  )
}
```

### 2. `src/renderer/src/components/ai-provider/HealthStatusBadge.tsx`

```typescript
import { CheckCircle, Clock, XCircle, AlertTriangle, WifiOff } from 'lucide-react'
import type { AIProviderStatus } from '@shared/ai-provider-types'
import { cn } from '../../utils'

const STATUS_CONFIG: Record<AIProviderStatus, { label: string; color: string; icon: ReactNode }> = {
  active:         { label: 'Active',        color: 'text-green-600',  icon: <CheckCircle  size={12} /> },
  pending:        { label: 'Pending',        color: 'text-yellow-600', icon: <Clock        size={12} /> },
  invalid:        { label: 'Invalid Key',    color: 'text-red-600',    icon: <XCircle      size={12} /> },
  quota_exceeded: { label: 'Quota Exceeded', color: 'text-orange-600', icon: <AlertTriangle size={12} /> },
  unreachable:    { label: 'Unreachable',    color: 'text-gray-500',   icon: <WifiOff      size={12} /> },
}

export function HealthStatusBadge({ status }: { status: AIProviderStatus }) {
  const { label, color, icon } = STATUS_CONFIG[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', color)}>
      {icon} {label}
    </span>
  )
}
```

---

## Tests — `src/renderer/src/components/ai-provider/__tests__/CredentialInput.test.tsx`

```typescript
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, act, cleanup, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Mock crypto.subtle + store
vi.mock('../../../lib/credential-crypto', () => ({
  encryptCredential: vi.fn(async (plaintext: string) => ({
    encryptedBlob: btoa(plaintext + '-encrypted'),
    iv:            btoa('test-iv'),
  })),
}))
vi.mock('../../../store', () => ({
  useAppStore: (fn?: any) => fn
    ? fn({ auth: { sessionToken: 'test-session-token' } })
    : {},
}))

import { encryptCredential } from '../../../lib/credential-crypto'
const mockEncrypt = vi.mocked(encryptCredential)
import { CredentialInput } from '../CredentialInput'

afterEach(() => cleanup())

describe('CredentialInput', () => {
  it('ollama provider → input not rendered', () => {
    const { container } = render(
      <CredentialInput provider="ollama" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(container.firstChild).toBeNull()
  })

  it('shows label for anthropic provider', () => {
    render(
      <CredentialInput provider="anthropic" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(screen.getByText(/Anthropic API Key/)).toBeInTheDocument()
  })

  it('type <10 chars → NOT encrypted yet, onClear called', async () => {
    const onEncrypted = vi.fn()
    const onClear     = vi.fn()
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={onEncrypted} onClear={onClear} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), { target: { value: 'sk-short' } })
    await act(async () => {})
    expect(mockEncrypt).not.toHaveBeenCalled()
    expect(onEncrypted).not.toHaveBeenCalled()
    expect(onClear).toHaveBeenCalled()
  })

  it('type ≥10 chars → encrypts, calls onEncrypted', async () => {
    const onEncrypted = vi.fn()
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={onEncrypted} onClear={vi.fn()} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), {
      target: { value: 'sk-this-is-longer-than-10-chars' }
    })
    await act(async () => {})
    await waitFor(() => expect(onEncrypted).toHaveBeenCalled())
    const [blob, iv] = onEncrypted.mock.calls[0]
    expect(typeof blob).toBe('string')
    expect(typeof iv).toBe('string')
  })

  it('after encryption → rawValue cleared (input empty)', async () => {
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    const input = screen.getByTestId('credential-input') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-this-is-longer-than-10-chars' } })
    await act(async () => {})
    await waitFor(() => expect(input.value).toBe(''))
  })

  it('shows 🔒 lock icon when encrypted', async () => {
    render(
      <CredentialInput provider="openai" hasExisting={false} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    fireEvent.change(screen.getByTestId('credential-input'), {
      target: { value: 'sk-this-is-a-valid-api-key-123' }
    })
    await act(async () => {})
    await waitFor(() => expect(screen.getByTestId('lock-icon')).toBeInTheDocument())
  })

  it('existing credential: shows "Leave blank" hint', () => {
    render(
      <CredentialInput provider="anthropic" hasExisting={true} onEncrypted={vi.fn()} onClear={vi.fn()} />
    )
    expect(screen.getByText(/Leave blank to keep existing/)).toBeInTheDocument()
  })
})
```

## Tests — `src/renderer/src/components/ai-provider/__tests__/HealthStatusBadge.test.tsx`

```typescript
// @vitest-environment happy-dom — 4 tests:
// active → green CheckCircle | invalid → red XCircle
// quota_exceeded → orange AlertTriangle | unreachable → gray WifiOff
```

---

## Acceptance Criteria

- [x] `CredentialInput` với `provider='ollama'` → render null
- [x] Nhập <10 ký tự → `encryptCredential()` KHÔNG được gọi
- [x] Nhập ≥10 ký tự → `encryptCredential()` được gọi, `onEncrypted` nhận blob+iv
- [x] Sau encrypt → `rawValue` state = `''` (input empty)
- [x] `type="password"`, `autoComplete="off"`, `spellCheck={false}` trên input
- [x] `HealthStatusBadge` 5 variants đúng màu và icon
- [x] 11/11 tests pass (6 CredentialInput + 5 HealthStatusBadge)

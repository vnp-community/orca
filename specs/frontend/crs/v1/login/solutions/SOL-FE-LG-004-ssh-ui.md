# SOL-FE-LG-004 — SSH User Indicator + Provisioning Progress UI

**CR:** [CR-LOGIN-003](../../../../../docs/crs/v1/login/CR-LOGIN-003-ssh-isolation.md)
**Backend Solution:** [SOL-LG-003](../../../../backend/crs/v1/login/solutions/SOL-LG-003-ssh-isolation.md)
**TDD Refs:** TDD-FE-05 (UI Components — Sidebar, SSH status), TDD-FE-09 (Onboarding — Agent Detection, progress)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Done — Implemented & verified 2026-07-24
**Blocked by:** SOL-FE-LG-001 (cần auth.user.email để tính linux username)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 SSH Connection State hiện tại (TDD-FE-02 §2)

```typescript
// src/renderer/src/store/slices/ssh.ts — HIỆN TẠI
// Đã có: SshSlice với server list, connection state
// Đã có (từ remote-server CRs): fleetImportStatus, serverHealthMetrics, fleetAlerts

// CẦN THÊM (CR-LOGIN-003):
// sshUsernames: Map<serverId, linuxUsername>  — orca-alice cho mỗi server
// provisioningStatus: Map<serverId, ProvisioningStatus>  — progress khi tạo unix account
```

### 1.2 SSH Status trong Sidebar (TDD-FE-05 §5)

```typescript
// src/renderer/src/components/sidebar/ — đã có SSH status indicators
// CẦN: thêm "logged in as orca-{name}" indicator bên cạnh server status
```

### 1.3 Backend provisioning API (SOL-LG-003)

Backend cung cấp:
- `GET /api/ssh/user-account?serverId=xxx` — trả về `{ linuxUsername, provisioned, provisioningStatus }`
- Hoặc qua RPC: `ssh.getUserAccount({ serverId })`
- Provisioning progress qua WebSocket events: `ssh.provision.progress`

### 1.4 `toLinuxUsername()` — cần replicate ở frontend

```typescript
// Backend (SOL-LG-003):
// toLinuxUsername("alice@company.com") → "orca-alice"

// Frontend cần hàm tương đương để hiển thị preview trước khi provisioning
```

---

## 2. File Structure

```
src/renderer/src/
├── store/slices/
│   └── ssh.ts                                   ← [MODIFY] Thêm sshUsernames, provisioningStatus
│
├── components/
│   └── ssh/                                     ← [NEW subdir hoặc extend existing]
│       ├── SshUserIndicator.tsx                 ← [NEW] "Connected as orca-alice" badge
│       ├── SshProvisioningProgress.tsx          ← [NEW] Progress bar khi tạo unix account
│       └── __tests__/
│           ├── SshUserIndicator.test.tsx
│           └── SshProvisioningProgress.test.tsx
│
└── hooks/
    ├── useSshUserAccount.ts                     ← [NEW] Fetch linux username per server
    └── useSshProvisioning.ts                    ← [NEW] Track provisioning events
```

---

## 3. Types mới trong SSH Slice

```typescript
// src/renderer/src/store/slices/ssh.ts — EXTEND

type ProvisioningStatus =
  | { phase: 'idle' }
  | { phase: 'checking' }
  | { phase: 'provisioning'; step: string; progress: number }   // 0–100
  | { phase: 'done'; linuxUsername: string }
  | { phase: 'error'; message: string }

type SshUserAccount = {
  linuxUsername: string    // e.g. "orca-alice"
  provisioned: boolean
  provisioningStatus: ProvisioningStatus
}

// Thêm vào SshSlice:
type SshSliceExtension = {
  sshUserAccounts: Map<string, SshUserAccount>       // key: serverId
  setSshUserAccount: (serverId: string, account: SshUserAccount) => void
  updateProvisioningStatus: (serverId: string, status: ProvisioningStatus) => void
}
```

---

## 4. Test Specifications

### 4.1 `SshUserIndicator.test.tsx`

```typescript
// src/renderer/src/components/ssh/__tests__/SshUserIndicator.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SshUserIndicator } from '../SshUserIndicator'

describe('SshUserIndicator', () => {
  afterEach(cleanup)

  it('renders linux username when provisioned', () => {
    render(<SshUserIndicator
      serverId="srv1"
      linuxUsername="orca-alice"
      provisioned={true}
      provisioningStatus={{ phase: 'done', linuxUsername: 'orca-alice' }}
    />)
    expect(screen.getByText(/orca-alice/)).toBeInTheDocument()
  })

  it('shows provisioning spinner when in progress', () => {
    render(<SshUserIndicator
      serverId="srv1"
      linuxUsername="orca-alice"
      provisioned={false}
      provisioningStatus={{ phase: 'provisioning', step: 'Creating user', progress: 40 }}
    />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
    expect(screen.getByText(/creating user/i)).toBeInTheDocument()
  })

  it('shows error state with message', () => {
    render(<SshUserIndicator
      serverId="srv1"
      linuxUsername="orca-alice"
      provisioned={false}
      provisioningStatus={{ phase: 'error', message: 'Permission denied on dev server' }}
    />)
    expect(screen.getByRole('alert')).toHaveTextContent(/permission denied/i)
  })

  it('renders idle state without progress bar', () => {
    render(<SshUserIndicator
      serverId="srv1"
      linuxUsername="orca-alice"
      provisioned={false}
      provisioningStatus={{ phase: 'idle' }}
    />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    // Still shows predicted username:
    expect(screen.getByText(/orca-alice/)).toBeInTheDocument()
  })
})
```

### 4.2 `SshProvisioningProgress.test.tsx`

```typescript
// src/renderer/src/components/ssh/__tests__/SshProvisioningProgress.test.tsx
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { SshProvisioningProgress } from '../SshProvisioningProgress'

describe('SshProvisioningProgress', () => {
  afterEach(cleanup)

  it('renders progress bar with correct width for 40%', () => {
    render(<SshProvisioningProgress step="Creating user" progress={40} />)
    const bar = screen.getByRole('progressbar')
    expect(bar).toHaveAttribute('aria-valuenow', '40')
  })

  it('renders step description text', () => {
    render(<SshProvisioningProgress step="Setting up SSH keys" progress={70} />)
    expect(screen.getByText(/setting up ssh keys/i)).toBeInTheDocument()
  })

  it('shows 100% complete state', () => {
    render(<SshProvisioningProgress step="Done" progress={100} />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  })
})
```

### 4.3 `useSshUserAccount.test.ts`

```typescript
// src/renderer/src/hooks/__tests__/useSshUserAccount.test.ts
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useSshUserAccount } from '../useSshUserAccount'
import * as rpcClient from '../../runtime/runtime-rpc-client'

vi.mock('../../runtime/runtime-rpc-client')

describe('useSshUserAccount', () => {
  beforeEach(() => { vi.resetAllMocks() })

  it('fetches linux username for serverId', async () => {
    vi.mocked(rpcClient.callRuntimeRpc).mockResolvedValueOnce({
      linuxUsername: 'orca-alice', provisioned: true
    })
    const { result } = renderHook(() => useSshUserAccount('srv-1'))
    await waitFor(() => expect(result.current.linuxUsername).toBe('orca-alice'))
    expect(result.current.provisioned).toBe(true)
  })

  it('returns null linuxUsername while loading', () => {
    vi.mocked(rpcClient.callRuntimeRpc).mockImplementation(() => new Promise(() => {}))
    const { result } = renderHook(() => useSshUserAccount('srv-1'))
    expect(result.current.linuxUsername).toBeNull()
    expect(result.current.isLoading).toBe(true)
  })

  it('computes predicted username from current auth user email', () => {
    // Even before provisioning, compute preview from email
    vi.mocked(rpcClient.callRuntimeRpc).mockImplementation(() => new Promise(() => {}))
    // Mock useAuthUser → email 'alice@company.com'
    const { result } = renderHook(() =>
      useSshUserAccount('srv-1', { previewFromEmail: 'alice@company.com' })
    )
    expect(result.current.previewUsername).toBe('orca-alice')
  })
})
```

---

## 5. Implementation Specifications

### 5.1 `toLinuxUsername()` — frontend util

```typescript
// src/renderer/src/auth/auth-utils.ts

/**
 * Compute predicted linux username from email (mirror of backend toLinuxUsername).
 * "alice@company.com" → "orca-alice"
 * "alice.smith@co.com" → "orca-alice-smith"
 */
export function toLinuxUsername(email: string): string {
  const local = email.split('@')[0]
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '-')
    .slice(0, 20)
  // Remove leading/trailing dashes
  const sanitized = local.replace(/^-+|-+$/g, '') || 'user'
  return `orca-${sanitized}`
}
```

### 5.2 `useSshUserAccount.ts`

```typescript
// src/renderer/src/hooks/useSshUserAccount.ts

import { useEffect, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { toLinuxUsername } from '../auth/auth-utils'

type SshUserAccountResult = {
  linuxUsername: string | null
  previewUsername: string | null   // preview from email before provisioned
  provisioned: boolean
  isLoading: boolean
  error: string | null
}

type Options = {
  previewFromEmail?: string
}

export function useSshUserAccount(serverId: string, options?: Options): SshUserAccountResult {
  const [linuxUsername, setLinuxUsername] = useState<string | null>(null)
  const [provisioned, setProvisioned] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const previewUsername = options?.previewFromEmail
    ? toLinuxUsername(options.previewFromEmail)
    : null

  useEffect(() => {
    let cancelled = false
    callRuntimeRpc(
      { kind: 'local' },    // hoặc 'environment' tuỳ theo context
      'ssh.getUserAccount',
      { serverId }
    )
      .then((result: { linuxUsername: string; provisioned: boolean }) => {
        if (!cancelled) {
          setLinuxUsername(result.linuxUsername)
          setProvisioned(result.provisioned)
        }
      })
      .catch((err: Error) => { if (!cancelled) setError(err.message) })
      .finally(() => { if (!cancelled) setIsLoading(false) })

    return () => { cancelled = true }
  }, [serverId])

  return { linuxUsername, previewUsername, provisioned, isLoading, error }
}
```

### 5.3 `SshUserIndicator.tsx`

```typescript
// src/renderer/src/components/ssh/SshUserIndicator.tsx

import { SshProvisioningProgress } from './SshProvisioningProgress'

type ProvisioningStatus =
  | { phase: 'idle' }
  | { phase: 'checking' }
  | { phase: 'provisioning'; step: string; progress: number }
  | { phase: 'done'; linuxUsername: string }
  | { phase: 'error'; message: string }

type Props = {
  serverId: string
  linuxUsername: string        // predicted or actual
  provisioned: boolean
  provisioningStatus: ProvisioningStatus
}

export function SshUserIndicator({ linuxUsername, provisioningStatus }: Props) {
  return (
    <div className="ssh-user-indicator">
      {/* Always show the linux username (actual or predicted) */}
      <span className="ssh-user-indicator__username">
        👤 {linuxUsername}
        {!provisioningStatus.phase.includes('done') &&
         !provisioningStatus.phase.includes('idle') && (
          <span className="ssh-user-indicator__status-dot ssh-user-indicator__status-dot--pending" />
        )}
      </span>

      {/* Provisioning in-progress */}
      {provisioningStatus.phase === 'provisioning' && (
        <SshProvisioningProgress
          step={provisioningStatus.step}
          progress={provisioningStatus.progress}
        />
      )}

      {/* Error state */}
      {provisioningStatus.phase === 'error' && (
        <div className="ssh-user-indicator__error" role="alert">
          ⚠️ {provisioningStatus.message}
        </div>
      )}

      {/* Done: show checkmark */}
      {provisioningStatus.phase === 'done' && (
        <span className="ssh-user-indicator__done" aria-label="Provisioned">✅</span>
      )}
    </div>
  )
}
```

### 5.4 `SshProvisioningProgress.tsx`

```typescript
// src/renderer/src/components/ssh/SshProvisioningProgress.tsx

type Props = { step: string; progress: number }

export function SshProvisioningProgress({ step, progress }: Props) {
  return (
    <div className="ssh-provisioning-progress">
      <p className="ssh-provisioning-progress__step">{step}</p>
      <div
        role="progressbar"
        aria-valuenow={progress}
        aria-valuemin={0}
        aria-valuemax={100}
        className="ssh-provisioning-progress__bar"
      >
        <div
          className="ssh-provisioning-progress__fill"
          style={{ width: `${progress}%` }}
        />
      </div>
    </div>
  )
}
```

### 5.5 SSH Slice extension (store/slices/ssh.ts — MODIFY)

```typescript
// src/renderer/src/store/slices/ssh.ts — MODIFY: thêm user accounts state

// Thêm vào state:
sshUserAccounts: new Map<string, SshUserAccount>(),

// Thêm actions:
setSshUserAccount: (serverId, account) => set(state => ({
  sshUserAccounts: new Map(state.sshUserAccounts).set(serverId, account)
})),

updateProvisioningStatus: (serverId, status) => set(state => {
  const existing = state.sshUserAccounts.get(serverId)
  if (!existing) return state
  return {
    sshUserAccounts: new Map(state.sshUserAccounts).set(serverId, {
      ...existing,
      provisioningStatus: status
    })
  }
}),
```

### 5.6 Tích hợp vào Sidebar SSH server card

```typescript
// src/renderer/src/components/sidebar/SshServerCard.tsx — MODIFY (hoặc wrapper)
// Thêm <SshUserIndicator> bên dưới server connection status

import { SshUserIndicator } from '../ssh/SshUserIndicator'
import { useSshUserAccount } from '../../hooks/useSshUserAccount'
import { useAuthUser } from '../../hooks/useAuthSession'

// Trong SshServerCard render:
const authUser = useAuthUser()
const { linuxUsername, provisioned, previewUsername } = useSshUserAccount(server.id, {
  previewFromEmail: authUser?.email
})
const provisioningStatus = useAppStore(s => s.sshUserAccounts.get(server.id)?.provisioningStatus) ?? { phase: 'idle' }

// Render:
<SshUserIndicator
  serverId={server.id}
  linuxUsername={linuxUsername ?? previewUsername ?? 'orca-?'}
  provisioned={provisioned}
  provisioningStatus={provisioningStatus}
/>
```

---

## 6. WebSocket provisioning events

```typescript
// src/renderer/src/hooks/useSshProvisioning.ts
// Subscribe to backend provisioning events qua WS RPC

export function useSshProvisioning(serverId: string) {
  const updateProvisioningStatus = useAppStore(s => s.updateProvisioningStatus)

  useEffect(() => {
    // Subscribe: ssh.provision.progress events từ backend
    const handler = (event: SshProvisioningEvent) => {
      if (event.serverId !== serverId) return
      updateProvisioningStatus(serverId, {
        phase: 'provisioning',
        step: event.step,
        progress: event.progress
      })
      if (event.progress === 100) {
        updateProvisioningStatus(serverId, { phase: 'done', linuxUsername: event.linuxUsername })
      }
    }

    // window.api.ssh.onProvisionProgress(handler)  ← Desktop
    // WS event từ runtime  ← Web mode
    return () => {
      // cleanup
    }
  }, [serverId, updateProvisioningStatus])
}
```

---

## 7. Files cần tạo/sửa

### Tạo mới

| File | Vai trò | Tests |
|------|---------|-------|
| `src/renderer/src/auth/auth-utils.ts` | `toLinuxUsername()` util | auth-utils.test.ts (4 tests) |
| `src/renderer/src/components/ssh/SshUserIndicator.tsx` | Username badge | SshUserIndicator.test.tsx (4 tests) |
| `src/renderer/src/components/ssh/SshProvisioningProgress.tsx` | Progress bar | SshProvisioningProgress.test.tsx (3 tests) |
| `src/renderer/src/hooks/useSshUserAccount.ts` | Fetch linux username per server | useSshUserAccount.test.ts (3 tests) |
| `src/renderer/src/hooks/useSshProvisioning.ts` | Track provisioning WS events | — |

### Sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/slices/ssh.ts` | Thêm `sshUserAccounts`, `provisioningStatus` state + actions |
| `src/renderer/src/components/sidebar/SshServerCard.tsx` | Thêm `<SshUserIndicator>` render |

---

## 8. `toLinuxUsername()` — Unit Tests

```typescript
// src/renderer/src/auth/__tests__/auth-utils.test.ts
import { describe, it, expect } from 'vitest'
import { toLinuxUsername } from '../auth-utils'

describe('toLinuxUsername', () => {
  it('converts simple email to orca-{local}', () => {
    expect(toLinuxUsername('alice@company.com')).toBe('orca-alice')
  })

  it('replaces dots and underscores with dashes', () => {
    expect(toLinuxUsername('alice.smith@co.com')).toBe('orca-alice-smith')
  })

  it('truncates at 20 chars for local part', () => {
    expect(toLinuxUsername('averylongemailaddress@co.com')).toHaveLength('orca-'.length + 20)
  })

  it('handles special chars in email', () => {
    const result = toLinuxUsername('alice+filter@co.com')
    expect(result).toMatch(/^orca-[a-z0-9-]+$/)
  })
})
```

---

## 9. Acceptance Criteria

- [x] `toLinuxUsername('alice@company.com')` === `'orca-alice'` (match với backend)
- [x] `SshUserIndicator` hiển thị predicted username ngay cả khi chưa provisioned
- [x] Progress bar xuất hiện trong quá trình provisioning (phase='provisioning')
- [x] Error message hiển thị khi provisioning thất bại (phase='error')
- [x] ✅ icon xuất hiện khi provisioning hoàn tất (phase='done')
- [x] Sidebar SSH server card hiển thị `orca-{name}` bên dưới connection status
- [x] WS provisioning events cập nhật UI real-time (không cần refresh)

## 11. Implementation Results

| File | Status | Tests |
|------|--------|-------|
| `auth/auth-utils.ts` | ✅ Created | 4 tests pass |
| `store/slices/ssh.ts` | ✅ Extended | ProvisioningStatus + SshUserAccount |
| `components/ssh/SshProvisioningProgress.tsx` | ✅ Created | 3 tests pass |
| `components/ssh/SshUserIndicator.tsx` | ✅ Created | 4 tests pass |
| `hooks/useSshUserAccount.ts` | ✅ Created | 3 tests pass |
| `hooks/useSshProvisioning.ts` | ✅ Created | — |
| `components/sidebar/SshTargetRow.tsx` | ✅ Modified | SshUserIndicator integrated |

---

## 10. Dependency

- **Backend prerequisite:** `ssh.getUserAccount` RPC method từ SOL-LG-003
- **Frontend prerequisite:** `useAuthUser()` từ SOL-FE-LG-001 (lấy email để tính username)

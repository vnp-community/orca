# TDD-FE-08: SSH User Provisioning UI

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/components/ssh/`, `src/renderer/src/auth/auth-utils.ts`

---

## 1. Mục tiêu

Khi user kết nối với dev server (lần đầu tiên với auth), cần:
1. Tính toán linux username (`toLinuxUsername()`)
2. Provision linux user account trên remote server
3. Hiển thị progress UI cho user
4. Sau khi done → SshUserIndicator hiển thị username + status

---

## 2. toLinuxUsername (Frontend mirror)

```typescript
// src/renderer/src/auth/auth-utils.ts

export function toLinuxUsername(email: string): string {
  // PHẢI mirror chính xác backend impl:
  // 1. Split email tại '@'
  // 2. Lowercase
  // 3. Regex: keep only [a-z0-9]
  // 4. Truncate tới 20 chars
  // 5. Prefix với 'orca-'
  // e.g., 'BinhNT@example.com' → 'orca-binhnt'
  // e.g., 'user.name+tag@example.com' → 'orca-username'
  const local = email.split('@')[0]
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
    .slice(0, 20)
  return `orca-${local}`
}
```

---

## 3. SshProvisioningProgress Component

```tsx
// src/renderer/src/components/ssh/SshProvisioningProgress.tsx

function SshProvisioningProgress({ serverId }: { serverId: string }) {
  const { status } = useSshProvisioning(serverId)

  if (!status) return null

  return (
    <div className="provisioning-progress">
      <ProgressBar value={status.step} max={status.total} />
      <span className="step-label">{status.message}</span>

      {/* Steps: */}
      {/* 1. Detecting platform */}
      {/* 2. Creating linux user */}
      {/* 3. Deploying relay binary */}
      {/* 4. Authorizing SSH key */}
      {/* 5. Verifying */}

      {status.error && <ErrorMessage message={status.error} />}
      {status.done && <SuccessMessage />}
    </div>
  )
}
```

---

## 4. SshUserIndicator Component

```tsx
// src/renderer/src/components/ssh/SshUserIndicator.tsx
// Inject vào SshTargetRow (additive — không sửa SshTargetRow core)

function SshUserIndicator({ serverId }: { serverId: string }) {
  const { account, loading } = useSshUserAccount(serverId)

  if (loading) return <Spinner size="xs" />
  if (!account) return null

  return (
    <div className="ssh-user-indicator">
      <UserIcon />
      <code>{account.linuxUsername}</code>
      {account.provisioned
        ? <CheckIcon className="text-green" />
        : <ClockIcon className="text-yellow" />
      }
    </div>
  )
}
```

---

## 5. useSshUserAccount Hook

```typescript
// src/renderer/src/hooks/useSshUserAccount.ts

function useSshUserAccount(serverId: string): {
  account: SshUserAccount | null
  loading: boolean
  error:   string | null
  refresh: () => void
}

// Fetches: window.api.getSshUserAccount(serverId)
// Caches in SSH slice: store.sshUserAccounts.get(serverId)
// Auto-refresh khi provisioning complete
```

---

## 6. useSshProvisioning Hook

```typescript
// src/renderer/src/hooks/useSshProvisioning.ts

function useSshProvisioning(serverId: string): {
  status:   ProvisioningStatus | null
  trigger:  () => Promise<void>   // start provisioning
  cancel:   () => void
}

// Subscribes to IPC events:
// 'ssh:provisioning:step'  → update step in store
// 'ssh:provisioning:done'  → mark complete
// 'ssh:provisioning:error' → set error

// trigger() calls: window.api.provisionSshUser(serverId)
```

---

## 7. ProvisioningStatus (Zustand)

```typescript
// store/slices/ssh.ts extension

type ProvisioningStatus = {
  serverId: string
  step:     number        // current step (0-indexed)
  total:    number        // total steps = 4
  message:  string        // human readable step description
  done:     boolean
  error:    string | null
}

// Store:
provisioningStatus: Map<string, ProvisioningStatus>
setProvisioningStatus(serverId: string, s: ProvisioningStatus): void
```

---

## 8. Provisioning Step Messages

| Step | Message |
|------|---------|
| 0 | "Detecting platform..." |
| 1 | "Creating Linux user account..." |
| 2 | "Deploying relay binary..." |
| 3 | "Authorizing SSH key..." |
| 4 | "Verifying setup..." |
| Done | "SSH user provisioned successfully" |

---

## 9. Tests (13 tests)

| File | Tests |
|------|-------|
| `auth/__tests__/auth-utils.test.ts` | 4 |
| `components/ssh/__tests__/SshProvisioningProgress.test.tsx` | 3 |
| `components/ssh/__tests__/SshUserIndicator.test.tsx` | 4 |
| `hooks/__tests__/useSshUserAccount.test.ts` | 2 |

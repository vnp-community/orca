# TASK-FE-024 — Tạo `useSshUserAccount.ts` + `useSshProvisioning.ts`

**Phase:** 4 — SSH UI
**Solution:** [SOL-FE-LG-004](../solutions/SOL-FE-LG-004-ssh-ui.md) §5.2, §6, §4.3
**Depends on:** TASK-FE-022, TASK-FE-023
**Blocks:** TASK-FE-025
**Effort:** M (~35 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo 2 hooks:
- `useSshUserAccount()` — fetch linux username từ backend RPC per server
- `useSshProvisioning()` — subscribe WS provisioning events → cập nhật store

---

## Files cần tạo

### `src/renderer/src/hooks/useSshUserAccount.ts` [NEW]

```typescript
import { useEffect, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { toLinuxUsername } from '../auth/auth-utils'

type Result = {
  linuxUsername: string | null
  previewUsername: string | null   // computed from email before provisioned
  provisioned: boolean
  isLoading: boolean
  error: string | null
}

export function useSshUserAccount(
  serverId: string,
  options?: { previewFromEmail?: string }
): Result {
  const [linuxUsername, setLinuxUsername] = useState<string | null>(null)
  const [provisioned, setProvisioned] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const previewUsername = options?.previewFromEmail
    ? toLinuxUsername(options.previewFromEmail)
    : null

  useEffect(() => {
    let cancelled = false
    callRuntimeRpc({ kind: 'local' }, 'ssh.getUserAccount', { serverId })
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

### `src/renderer/src/hooks/useSshProvisioning.ts` [NEW]

```typescript
import { useEffect } from 'react'
import { useAppStore } from '../store'

type SshProvisioningEvent = {
  serverId: string
  step: string
  progress: number         // 0–100
  linuxUsername?: string   // populated khi progress=100
}

export function useSshProvisioning(serverId: string): void {
  const updateProvisioningStatus = useAppStore(s => s.updateProvisioningStatus)

  useEffect(() => {
    // Handler cho WS events từ runtime
    function handleEvent(event: SshProvisioningEvent) {
      if (event.serverId !== serverId) return

      if (event.progress < 100) {
        updateProvisioningStatus(serverId, {
          phase: 'provisioning',
          step: event.step,
          progress: event.progress
        })
      } else {
        updateProvisioningStatus(serverId, {
          phase: 'done',
          linuxUsername: event.linuxUsername!
        })
      }
    }

    // Subscribe: Desktop mode qua window.api.ssh.onProvisionProgress
    // Web mode: sẽ được thiết lập qua sync-runtime-graph
    // NOTE: actual event subscription cần adapt theo platform
    const off = window.api?.ssh?.onProvisionProgress?.(handleEvent)
    return () => { off?.() }
  }, [serverId, updateProvisioningStatus])
}
```

### `src/renderer/src/hooks/__tests__/useSshUserAccount.test.ts` [NEW]

Sao chép test spec từ [SOL-FE-LG-004 §4.3](../solutions/SOL-FE-LG-004-ssh-ui.md).

Test cases (3 tests):
- Fetches linux username cho serverId
- Returns null linuxUsername while loading
- Computes predicted username từ email

---

## Verify

```bash
npx vitest run src/renderer/src/hooks/__tests__/useSshUserAccount.test.ts
# Expected: 3 pass
```

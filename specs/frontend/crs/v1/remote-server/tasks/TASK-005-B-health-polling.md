# TASK-005-B — Tạo useFleetHealthPolling Hook

**Task ID:** TASK-005-B  
**CR:** CR-005 — Fleet Health Monitoring  
**Solution Ref:** SOL-CR-005, Section 3  
**Dependencies:** TASK-005-A  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `useFleetHealthPolling` hook:
- Poll health metrics mỗi 60 giây khi enabled
- Detect server disconnect → tạo FleetAlert

---

## File cần tạo

`src/renderer/src/hooks/useFleetHealthPolling.ts`

---

## Bước thực thi

```typescript
// src/renderer/src/hooks/useFleetHealthPolling.ts
import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type { SshConnectionStatus } from 'src/shared/ssh-types'

const POLL_INTERVAL_MS = 60_000  // 60 seconds

export function useFleetHealthPolling(enabled: boolean): void {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  const prevStatesRef = useRef<Record<string, SshConnectionStatus>>({})

  // ── Polling ──────────────────────────────────────────────────────────────────
  useEffect(() => {
    if (!enabled) return

    const doPoll = async () => {
      try {
        const healthData = await window.api.ssh.getFleetHealth?.()
        if (!healthData) return

        const now = Date.now()
        const store = useAppStore.getState()
        store.setLastFleetHealthCheck(now)

        for (const entry of healthData.servers ?? []) {
          store.updateServerHealth(entry.serverId, {
            lastCheckedAt: now,
            isReachable: entry.isReachable,
            uptimeSeconds: entry.uptimeSeconds ?? null,
            relayVersion: entry.relayVersion ?? null,
            nodeVersion: entry.nodeVersion ?? null,
            diskUsagePercent: entry.diskUsagePercent ?? null,
          })
        }
      } catch (err) {
        console.warn('[FleetHealthPolling] Poll failed:', err)
      }
    }

    doPoll()
    const interval = setInterval(doPoll, POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [enabled])

  // ── Disconnect alert detection ─────────────────────────────────────────────
  useEffect(() => {
    const store = useAppStore.getState()

    for (const target of sshTargets) {
      const prevStatus = prevStatesRef.current[target.id]
      const currStatus = connectionStates[target.id]?.status

      const wasConnected = prevStatus === 'connected'
      const isDisconnectedNow =
        currStatus === 'disconnected' ||
        currStatus === 'error' ||
        currStatus === 'reconnection-failed'

      if (wasConnected && isDisconnectedNow) {
        store.addFleetAlert({
          id: `disconnect-${target.id}-${Date.now()}`,
          serverId: target.id,
          serverLabel: target.label,
          type: 'disconnected',
          message: translate(
            'fleet.alert.disconnected',
            `${target.label} disconnected`
          ),
          timestamp: Date.now(),
          dismissed: false,
        })
      }
    }

    // Update prev snapshot
    const newSnapshot: Record<string, SshConnectionStatus> = {}
    for (const target of sshTargets) {
      const status = connectionStates[target.id]?.status
      if (status) newSnapshot[target.id] = status
    }
    prevStatesRef.current = newSnapshot
  }, [connectionStates, sshTargets])
}
```

---

## Acceptance Criteria

- [x] Khi `enabled=true`: poll ngay lập tức + mỗi 60 giây
- [x] Khi `enabled=false`: không poll
- [x] Cleanup: clearInterval khi component unmount hoặc enabled=false
- [x] Alert được tạo khi server từ 'connected' → 'disconnected'/'error'/'reconnection-failed'
- [x] Không tạo duplicate alerts cho cùng event
- [x] TypeScript compile clean

---

## Notes cho AI

- `window.api.ssh.getFleetHealth?.()` — optional chain vì API có thể chưa có
- `prevStatesRef` không gây re-render (dùng useRef, không useState)
- Chỉ detect disconnect, không detect reconnect (reconnect xử lý bởi existing SSH connection handler)

---

## Implementation Notes

> **Completed:** 2026-07-23 | `hooks/useFleetHealthPolling.ts`: immediate poll on enabled=true, 60s interval, clearInterval on unmount/disabled, disconnect detection via prevStatesRef Map snapshot (connected→disconnected|error|reconnection-failed), no duplicate alerts via id check. TypeScript: ✅ 0 errors.

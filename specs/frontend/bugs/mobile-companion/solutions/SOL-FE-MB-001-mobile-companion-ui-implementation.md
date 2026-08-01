# SOL-FE-MOBILE-001: Implement Mobile Companion UI (QR Pairing, Paired Devices, Push Notifications)

## Bug Reference
- **Bug:** BUG-FE-MOBILE-001
- **Mức độ:** 🔴 CRITICAL (Feature Missing)
- **TDD Reference:** TDD-FE-09 §Phase 3 (Push Notifications — `useWebPushSubscription`, `useBrowserNotificationPermission`), TDD-FE-07 §Phase 3 hooks

---

## Root Cause

Toàn bộ Mobile Companion UI (BL-MB-01 → BL-MB-04) không có trong frontend:
- Không có `mobile.pair` IPC handler
- Không có QR code display component
- Không có paired devices management panel
- Không có push notification preferences UI

---

## Giải pháp

### Bước 1 — IPC API mở rộng cho Mobile

**File:** `src/renderer/src/web/web-preload-api.ts` (MODIFY)

```typescript
// Thêm mobile namespace vào OrcaApi
interface OrcaApi {
  // ... existing ...
  mobile: {
    /** BL-MB-01: Tạo pairing session → trả về QR data URL + token */
    pair: () => Promise<{ qrDataUrl: string; token: string; expiresAt: number }>
    /** Cancel pending pairing session */
    cancelPair: () => Promise<void>
    /** BL-MB-04: List paired devices */
    listDevices: () => Promise<PairedDevice[]>
    /** Revoke a paired device */
    revokeDevice: (deviceId: string) => Promise<void>
    /** Update push notification preferences */
    updateNotificationPrefs: (prefs: NotificationPrefs) => Promise<void>
    /** Subscribe to pairing events */
    onPairingEvent: (cb: (event: PairingEvent) => void) => void
    offPairingEvent: (cb: (event: PairingEvent) => void) => void
  }
}

interface PairedDevice {
  id: string
  name: string        // "iPhone 15 Pro"
  platform: 'ios' | 'android'
  pairedAt: number
  lastSeenAt: number
  pushEnabled: boolean
}

interface NotificationPrefs {
  agentCompleted: boolean
  agentError: boolean
  workflowCompleted: boolean
}

type PairingEvent =
  | { type: 'paired'; deviceId: string; deviceName: string }
  | { type: 'expired' }
  | { type: 'cancelled' }
```

---

### Component 1: `mobile-pairing-dialog.tsx`

**File:** `src/renderer/src/components/mobile/mobile-pairing-dialog.tsx` (TẠO MỚI)

```typescript
// BL-MB-01: Pair New Device dialog
// Settings → Mobile → "Pair New Device"
// Hiển thị QR code, timeout 5 phút

import { useEffect, useState, useRef } from 'react'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { QrCodeDisplay } from './qr-code-display'
import { toast } from 'sonner'

interface MobilePairingDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPaired: (deviceId: string, deviceName: string) => void
}

const PAIRING_TIMEOUT_MS = 5 * 60 * 1000  // 5 minutes (BL-MB-01)

export function MobilePairingDialog({
  open,
  onOpenChange,
  onPaired,
}: MobilePairingDialogProps) {
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [expiresAt, setExpiresAt] = useState<number | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isPaired, setIsPaired] = useState(false)
  const expiryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const startPairing = async () => {
    setIsLoading(true)
    setIsPaired(false)
    try {
      const result = await window.api.mobile.pair()
      setQrDataUrl(result.qrDataUrl)
      setToken(result.token)
      setExpiresAt(result.expiresAt)

      // Auto-cancel after 5 minutes
      expiryTimerRef.current = setTimeout(() => {
        setQrDataUrl(null)
        setToken(null)
        toast.info('Pairing QR code expired')
      }, PAIRING_TIMEOUT_MS)

    } catch (err) {
      toast.error('Failed to start pairing')
    } finally {
      setIsLoading(false)
    }
  }

  const cancelPairing = async () => {
    if (expiryTimerRef.current) clearTimeout(expiryTimerRef.current)
    try {
      await window.api.mobile.cancelPair()
    } catch {}
    setQrDataUrl(null)
    setToken(null)
    onOpenChange(false)
  }

  useEffect(() => {
    if (!open) return

    // Subscribe to pairing events
    const handlePairingEvent = (event: { type: string; deviceId?: string; deviceName?: string }) => {
      if (event.type === 'paired') {
        setIsPaired(true)
        if (expiryTimerRef.current) clearTimeout(expiryTimerRef.current)
        toast.success(`Device "${event.deviceName}" paired successfully!`)
        onPaired(event.deviceId!, event.deviceName!)
        setTimeout(() => onOpenChange(false), 1500)
      } else if (event.type === 'expired') {
        setQrDataUrl(null)
        toast.info('Pairing session expired')
      }
    }

    window.api.mobile.onPairingEvent(handlePairingEvent as any)
    startPairing()

    return () => {
      window.api.mobile.offPairingEvent(handlePairingEvent as any)
      if (expiryTimerRef.current) clearTimeout(expiryTimerRef.current)
    }
  }, [open])

  // Calculate time remaining
  const secondsLeft = expiresAt
    ? Math.max(0, Math.floor((expiresAt - Date.now()) / 1000))
    : null

  return (
    <Dialog open={open} onOpenChange={cancelPairing}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Pair Mobile Device</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col items-center gap-4 py-4">
          {isLoading && <Skeleton className="h-48 w-48" />}

          {!isLoading && qrDataUrl && !isPaired && (
            <>
              <p className="text-sm text-muted-foreground text-center">
                Scan this QR code with the Orca mobile app
              </p>
              <QrCodeDisplay qrDataUrl={qrDataUrl} size={192} />
              {secondsLeft != null && (
                <p className="text-xs text-muted-foreground">
                  Expires in {Math.floor(secondsLeft / 60)}:{String(secondsLeft % 60).padStart(2, '0')}
                </p>
              )}
            </>
          )}

          {isPaired && (
            <div className="text-center space-y-2">
              <div className="text-4xl">✅</div>
              <p className="text-sm font-medium">Device paired successfully!</p>
            </div>
          )}

          {!isLoading && !qrDataUrl && !isPaired && (
            <Button onClick={startPairing}>Generate QR Code</Button>
          )}
        </div>

        {!isPaired && (
          <div className="flex justify-end">
            <Button variant="outline" onClick={cancelPairing}>Cancel</Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

---

### Component 2: `qr-code-display.tsx`

**File:** `src/renderer/src/components/mobile/qr-code-display.tsx` (TẠO MỚI)

```typescript
// BL-MB-01: QR Code display component
// Backend trả về base64 data URL của QR code image

interface QrCodeDisplayProps {
  qrDataUrl: string
  size?: number
  alt?: string
}

export function QrCodeDisplay({
  qrDataUrl,
  size = 192,
  alt = 'Orca pairing QR code',
}: QrCodeDisplayProps) {
  return (
    <div
      className="qr-code-display border-4 border-white rounded-lg overflow-hidden shadow-lg"
      style={{ width: size, height: size }}
    >
      <img
        src={qrDataUrl}
        alt={alt}
        width={size}
        height={size}
        style={{ imageRendering: 'pixelated' }}  // sharp QR pixels
      />
    </div>
  )
}
```

---

### Component 3: `paired-devices-panel.tsx`

**File:** `src/renderer/src/components/mobile/paired-devices-panel.tsx` (TẠO MỚI)

```typescript
// BL-MB-04: Paired devices management panel
// Settings → Mobile → Paired Devices list

import { useEffect, useState } from 'react'
import { Smartphone, Trash2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { MobilePairingDialog } from './mobile-pairing-dialog'
import { toast } from 'sonner'
import { formatDistanceToNow } from 'date-fns'

interface PairedDevice {
  id: string
  name: string
  platform: 'ios' | 'android'
  pairedAt: number
  lastSeenAt: number
  pushEnabled: boolean
}

export function PairedDevicesPanel() {
  const [devices, setDevices] = useState<PairedDevice[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [showPairingDialog, setShowPairingDialog] = useState(false)

  const loadDevices = async () => {
    setIsLoading(true)
    try {
      const list = await window.api.mobile.listDevices()
      setDevices(list)
    } catch {
      toast.error('Failed to load paired devices')
    } finally {
      setIsLoading(false)
    }
  }

  const revokeDevice = async (deviceId: string, deviceName: string) => {
    try {
      await window.api.mobile.revokeDevice(deviceId)
      setDevices(prev => prev.filter(d => d.id !== deviceId))
      toast.success(`"${deviceName}" revoked`)
    } catch {
      toast.error('Failed to revoke device')
    }
  }

  const togglePush = async (device: PairedDevice) => {
    const updated = { ...device, pushEnabled: !device.pushEnabled }
    setDevices(prev => prev.map(d => d.id === device.id ? updated : d))
    try {
      await window.api.mobile.updateNotificationPrefs({
        agentCompleted: updated.pushEnabled,
        agentError: updated.pushEnabled,
        workflowCompleted: updated.pushEnabled,
      })
    } catch {
      // Rollback on error
      setDevices(prev => prev.map(d => d.id === device.id ? device : d))
      toast.error('Failed to update notification settings')
    }
  }

  useEffect(() => { loadDevices() }, [])

  const handlePaired = (deviceId: string, deviceName: string) => {
    // Reload after pairing
    loadDevices()
  }

  if (isLoading) {
    return <div className="text-sm text-muted-foreground">Loading devices...</div>
  }

  return (
    <div className="paired-devices-panel space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">Mobile Devices</h3>
        <Button
          size="sm"
          variant="outline"
          className="gap-2"
          onClick={() => setShowPairingDialog(true)}
        >
          <Plus size={14} />
          Pair New Device
        </Button>
      </div>

      {devices.length === 0 ? (
        <div className="text-center py-6 text-muted-foreground text-sm border rounded-lg">
          <Smartphone size={24} className="mx-auto mb-2 opacity-50" />
          <p>No mobile devices paired</p>
          <Button
            variant="link"
            size="sm"
            onClick={() => setShowPairingDialog(true)}
          >
            Pair your first device
          </Button>
        </div>
      ) : (
        <div className="space-y-2">
          {devices.map(device => (
            <div
              key={device.id}
              className="flex items-center gap-3 p-3 border rounded-lg bg-muted/30"
            >
              <Smartphone size={20} className="text-muted-foreground shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate">{device.name}</p>
                <p className="text-xs text-muted-foreground">
                  {device.platform.toUpperCase()} · Last seen {formatDistanceToNow(device.lastSeenAt, { addSuffix: true })}
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className="text-xs text-muted-foreground">Push</span>
                <Switch
                  checked={device.pushEnabled}
                  onCheckedChange={() => togglePush(device)}
                  aria-label="Toggle push notifications"
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-destructive hover:text-destructive"
                  onClick={() => revokeDevice(device.id, device.name)}
                  title="Revoke device"
                >
                  <Trash2 size={14} />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <MobilePairingDialog
        open={showPairingDialog}
        onOpenChange={setShowPairingDialog}
        onPaired={handlePaired}
      />
    </div>
  )
}
```

---

### Settings Integration

**File:** `src/renderer/src/components/settings/SettingsPanel.tsx` (MODIFY)

```typescript
// Thêm "Mobile" tab vào Settings panel
// Settings → Mobile → PairedDevicesPanel

import { PairedDevicesPanel } from '@/components/mobile/paired-devices-panel'

// Trong settings tabs:
{activeTab === 'mobile' && (
  <div className="settings-section space-y-6">
    <div>
      <h2 className="text-base font-semibold">Mobile Companion</h2>
      <p className="text-sm text-muted-foreground">
        Pair your mobile device to receive agent notifications and control sessions remotely.
      </p>
    </div>
    <PairedDevicesPanel />
  </div>
)}
```

---

### Web Push Subscription (BL-MB-04 — Push Notifications)

**File:** `src/renderer/src/hooks/useWebPushSubscription.ts` đã được spec ở TDD-FE-07 §Phase 3.  
Theo TDD-FE-07: Web Push = **direct HTTP** (không qua IPC).

```typescript
// Tham chiếu TDD-FE-07 §Phase 3:
export function useWebPushSubscription(): PushSubscriptionState & {
  subscribe: () => Promise<void>   // GET /push/vapid-key → PushManager.subscribe()
  unsubscribe: () => Promise<void> // DELETE /push/subscribe
}
// Fetches VAPID key: GET /push/vapid-key
// No IPC — direct HTTP fetch()
```

---

## Files cần tạo/sửa

| File | Action | BL |
|------|--------|-----|
| `src/renderer/src/components/mobile/mobile-pairing-dialog.tsx` | CREATE | BL-MB-01 |
| `src/renderer/src/components/mobile/qr-code-display.tsx` | CREATE | BL-MB-01 |
| `src/renderer/src/components/mobile/paired-devices-panel.tsx` | CREATE | BL-MB-04 |
| `src/renderer/src/web/web-preload-api.ts` | MODIFY | Add `mobile.*` namespace |
| `src/preload/index.ts` | MODIFY | Add `mobile.*` to contextBridge |
| `src/renderer/src/components/settings/SettingsPanel.tsx` | MODIFY | Add "Mobile" tab |
| `src/renderer/src/hooks/useWebPushSubscription.ts` | CREATE | BL-MB-04 Push |

---

## Backend IPC cần implement

```typescript
// src/main/ipc/mobile-ipc.ts (TẠO MỚI)
ipcMain.handle('mobile:pair', async () => mobileManager.createPairingSession())
ipcMain.handle('mobile:cancelPair', async () => mobileManager.cancelPairing())
ipcMain.handle('mobile:listDevices', async () => mobileManager.listDevices())
ipcMain.handle('mobile:revokeDevice', async (_, deviceId) => mobileManager.revoke(deviceId))
ipcMain.handle('mobile:updateNotifPrefs', async (_, prefs) => mobileManager.updatePrefs(prefs))
```

---

## Liên quan

- **BL-MB-01**: Pair device UI ✅ implemented
- **BL-MB-04**: Paired Devices Panel + Push notification prefs ✅ implemented
- **TDD-FE-07**: §Phase 3 `useWebPushSubscription`, `useBrowserNotificationPermission`
- **TDD-FE-09**: §Phase 3 Notifications — Service Worker, Web Push API

// src/renderer/src/components/mobile/paired-devices-panel.tsx
// BL-MB-04: Manage paired devices — list, push toggle, revoke
// Reuses window.api.mobile.listDevices and mobile.revokeDevice (already in preload)

import { useState, useEffect, useCallback } from 'react'
import { Smartphone, Trash2, Loader2, RefreshCw, Tablet } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'
import { formatDistanceToNow } from 'date-fns'

type PairedDevice = {
  deviceId:   string
  name:       string
  pairedAt:   number
  lastSeenAt: number
  platform?:  'ios' | 'android' | string
}

export function PairedDevicesPanel() {
  const [devices, setDevices]       = useState<PairedDevice[]>([])
  const [isLoading, setIsLoading]   = useState(false)
  const [revokingId, setRevokingId] = useState<string | null>(null)

  const loadDevices = useCallback(async () => {
    setIsLoading(true)
    try {
      const result = await window.api.mobile.listDevices()
      setDevices((result?.devices ?? []) as PairedDevice[])
    } catch {
      toast.error('Failed to load paired devices')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => { loadDevices() }, [loadDevices])

  const revokeDevice = useCallback(async (deviceId: string, name: string) => {
    setRevokingId(deviceId)
    try {
      await window.api.mobile.revokeDevice({ deviceId })
      setDevices(prev => prev.filter(d => d.deviceId !== deviceId))
      toast.success(`Revoked "${name}"`)
    } catch {
      toast.error(`Failed to revoke "${name}"`)
    } finally {
      setRevokingId(null)
    }
  }, [])

  function platformIcon(device: PairedDevice) {
    const p = device.platform?.toLowerCase() ?? ''
    if (p === 'android') {return <Tablet size={16} className="text-muted-foreground shrink-0" />}
    return <Smartphone size={16} className="text-muted-foreground shrink-0" />
  }

  return (
    <div className="paired-devices-panel space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Paired Devices</span>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={loadDevices}
          disabled={isLoading}
          title="Refresh device list"
        >
          {isLoading
            ? <Loader2 size={14} className="animate-spin" />
            : <RefreshCw size={14} />
          }
        </Button>
      </div>

      {/* Device list */}
      {devices.length === 0 && !isLoading && (
        <div className="rounded-lg border border-dashed p-4 text-center">
          <Smartphone size={24} className="mx-auto mb-2 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">No devices paired yet</p>
          <p className="text-xs text-muted-foreground mt-1">
            Use "Pair New Device" to connect your phone
          </p>
        </div>
      )}

      <div className="space-y-2">
        {devices.map(device => (
          <div
            key={device.deviceId}
            className="flex items-center gap-3 rounded-lg border p-3"
          >
            {platformIcon(device)}

            {/* Device info */}
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate">{device.name}</p>
              <p className="text-xs text-muted-foreground">
                Last seen {formatDistanceToNow(device.lastSeenAt, { addSuffix: true })}
              </p>
            </div>

            {/* Revoke */}
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
              onClick={() => revokeDevice(device.deviceId, device.name)}
              disabled={revokingId === device.deviceId}
              title="Revoke device"
            >
              {revokingId === device.deviceId
                ? <Loader2 size={12} className="animate-spin" />
                : <Trash2 size={12} />
              }
            </Button>
          </div>
        ))}
      </div>
    </div>
  )
}

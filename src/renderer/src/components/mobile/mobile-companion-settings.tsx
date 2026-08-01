// src/renderer/src/components/mobile/mobile-companion-settings.tsx
// MB-001-C: Assembly of all mobile companion settings into one tabbed page
// Added to Settings panel under "Mobile" tab

import { useState } from 'react'
import { Smartphone, Bell, Monitor, Plus, QrCode } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PairedDevicesPanel } from './paired-devices-panel'
import { MobileNotificationSettings } from './mobile-notification-settings'

// ─── QR Pair Dialog ────────────────────────────────────────────────────────────
// Shows the existing QR pairing flow using window.api.mobile.getPairingQR
// (already implemented in preload/mobile IPC)

function PairNewDeviceDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const [qrData, setQrData] = useState<{ qrDataUrl: string; pairingUrl: string } | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function loadQR() {
    setIsLoading(true)
    setError(null)
    try {
      const result = await window.api.mobile.getPairingQR()
      if (result.available) {
        setQrData({ qrDataUrl: result.qrDataUrl, pairingUrl: result.pairingUrl })
      } else {
        setError('Mobile pairing server is not running. Ensure Orca is running with mobile support.')
      }
    } catch (e: any) {
      setError(e?.message ?? 'Failed to generate QR code')
    } finally {
      setIsLoading(false)
    }
  }

  // Load QR on open
  function handleOpenChange(v: boolean) {
    onOpenChange(v)
    if (v) loadQR()
    else setQrData(null)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <QrCode size={16} />
            Pair New Device
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {isLoading && (
            <div className="flex items-center justify-center h-40">
              <div className="text-sm text-muted-foreground">Generating QR code…</div>
            </div>
          )}

          {error && (
            <div className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {qrData && (
            <div className="flex flex-col items-center gap-3">
              <img
                src={qrData.qrDataUrl}
                alt="Pairing QR code"
                className="w-48 h-48 rounded-lg border"
              />
              <p className="text-xs text-muted-foreground text-center">
                Scan this QR code with the Orca mobile app
              </p>
              <p className="text-[10px] font-mono text-muted-foreground break-all text-center">
                {qrData.pairingUrl}
              </p>
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={loadQR}
              >
                Refresh QR
              </Button>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Assembly ─────────────────────────────────────────────────────────────

export function MobileCompanionSettings() {
  const [pairDialogOpen, setPairDialogOpen] = useState(false)

  return (
    <div className="mobile-companion-settings space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Smartphone size={16} className="text-muted-foreground" />
          <h3 className="text-base font-semibold">Mobile Companion</h3>
        </div>
        <Button
          size="sm"
          className="h-7 gap-1 text-xs"
          onClick={() => setPairDialogOpen(true)}
        >
          <Plus size={12} />
          Pair New Device
        </Button>
      </div>

      <p className="text-xs text-muted-foreground">
        Connect the Orca mobile app to monitor agents and receive push notifications.
      </p>

      {/* Tabs */}
      <Tabs defaultValue="devices">
        <TabsList className="h-8">
          <TabsTrigger value="devices" className="text-xs gap-1">
            <Monitor size={12} />
            Devices
          </TabsTrigger>
          <TabsTrigger value="notifications" className="text-xs gap-1">
            <Bell size={12} />
            Notifications
          </TabsTrigger>
        </TabsList>

        <TabsContent value="devices" className="mt-3">
          <PairedDevicesPanel />
        </TabsContent>

        <TabsContent value="notifications" className="mt-3">
          <MobileNotificationSettings />
        </TabsContent>
      </Tabs>

      <PairNewDeviceDialog
        open={pairDialogOpen}
        onOpenChange={setPairDialogOpen}
      />
    </div>
  )
}

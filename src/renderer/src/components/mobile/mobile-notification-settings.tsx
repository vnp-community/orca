// src/renderer/src/components/mobile/mobile-notification-settings.tsx
// BL-MB-02: Push notification preferences for mobile companion
// Reuses existing notifications IPC (notifications:dispatch etc.) + adds
// mobile-specific per-category toggles.

import { useState, useEffect, useCallback } from 'react'
import { Bell, BellOff, Loader2 } from 'lucide-react'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'

export interface MobileNotificationPrefs {
  agentCompleted:    boolean
  agentError:        boolean
  workflowCompleted: boolean
  workflowFailed:    boolean
  quotaWarning:      boolean
}

const DEFAULT_PREFS: MobileNotificationPrefs = {
  agentCompleted:    true,
  agentError:        true,
  workflowCompleted: true,
  workflowFailed:    true,
  quotaWarning:      true,
}

const PREF_ROWS: { key: keyof MobileNotificationPrefs; label: string; description: string }[] = [
  { key: 'agentCompleted',    label: 'Agent completed',    description: 'When an AI agent finishes a task' },
  { key: 'agentError',        label: 'Agent error',        description: 'When an AI agent encounters an error' },
  { key: 'workflowCompleted', label: 'Workflow completed', description: 'When a workflow finishes successfully' },
  { key: 'workflowFailed',    label: 'Workflow failed',    description: 'When a workflow fails or is cancelled' },
  { key: 'quotaWarning',      label: 'Usage quota warning', description: 'When approaching your usage limit' },
]

const STORAGE_KEY = 'orca.mobile.notificationPrefs'

function loadPrefs(): MobileNotificationPrefs {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return { ...DEFAULT_PREFS, ...JSON.parse(raw) }
  } catch {}
  return DEFAULT_PREFS
}

function savePrefs(prefs: MobileNotificationPrefs): void {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs)) } catch {}
}

export function MobileNotificationSettings() {
  const [prefs, setPrefs] = useState<MobileNotificationPrefs>(DEFAULT_PREFS)
  const [isSaving, setIsSaving] = useState(false)
  const [permStatus, setPermStatus] = useState<'granted' | 'denied' | 'default' | 'unknown'>('unknown')

  useEffect(() => {
    setPrefs(loadPrefs())
    // Check system notification permission
    if ('Notification' in window) {
      setPermStatus(Notification.permission as 'granted' | 'denied' | 'default')
    }
  }, [])

  const requestPermission = useCallback(async () => {
    if (!('Notification' in window)) return
    const result = await Notification.requestPermission()
    setPermStatus(result)
  }, [])

  const updatePref = useCallback((key: keyof MobileNotificationPrefs, value: boolean) => {
    setPrefs(prev => {
      const next = { ...prev, [key]: value }
      savePrefs(next)
      return next
    })
  }, [])

  const allEnabled  = Object.values(prefs).every(Boolean)
  const toggleAll   = useCallback(() => {
    const next = Object.fromEntries(
      Object.keys(prefs).map(k => [k, !allEnabled])
    ) as MobileNotificationPrefs
    setPrefs(next)
    savePrefs(next)
  }, [prefs, allEnabled])

  return (
    <div className="mobile-notification-settings space-y-4">
      {/* Permission banner */}
      {permStatus !== 'granted' && (
        <div className="flex items-center justify-between rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-3 py-2">
          <div className="flex items-center gap-2 text-sm text-yellow-700 dark:text-yellow-400">
            <BellOff size={14} />
            <span>Notifications {permStatus === 'denied' ? 'blocked by browser' : 'not yet enabled'}</span>
          </div>
          {permStatus !== 'denied' && (
            <button
              onClick={requestPermission}
              className="text-xs font-medium text-yellow-700 dark:text-yellow-400 hover:underline"
            >
              Enable
            </button>
          )}
        </div>
      )}

      {/* Header with toggle all */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bell size={14} className="text-muted-foreground" />
          <span className="text-sm font-medium">Notification types</span>
        </div>
        <button
          onClick={toggleAll}
          className="text-xs text-muted-foreground hover:text-foreground"
        >
          {allEnabled ? 'Disable all' : 'Enable all'}
        </button>
      </div>

      {/* Per-category switches */}
      <div className="space-y-1">
        {PREF_ROWS.map(({ key, label, description }) => (
          <div
            key={key}
            className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted/50"
          >
            <div>
              <p className="text-sm font-medium">{label}</p>
              <p className="text-xs text-muted-foreground">{description}</p>
            </div>
            <Switch
              checked={prefs[key]}
              onCheckedChange={v => updatePref(key, v)}
              disabled={permStatus === 'denied'}
            />
          </div>
        ))}
      </div>

      {isSaving && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 size={12} className="animate-spin" />
          Saving…
        </div>
      )}
    </div>
  )
}

import { useState, useCallback } from 'react'

// ─── Types ────────────────────────────────────────────────────────────────────

type NotificationPermissionState = 'default' | 'granted' | 'denied' | 'unsupported'

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useBrowserNotificationPermission(): {
  state: NotificationPermissionState
  requestPermission: () => Promise<void>
} {
  const getInitial = (): NotificationPermissionState => {
    if (typeof Notification === 'undefined') {return 'unsupported'}
    return Notification.permission as NotificationPermissionState
  }

  const [state, setState] = useState<NotificationPermissionState>(getInitial)

  const requestPermission = useCallback(async () => {
    if (typeof Notification === 'undefined') {
      setState('unsupported')
      return
    }
    const result = await Notification.requestPermission()
    setState(result as NotificationPermissionState)
  }, [])

  return { state, requestPermission }
}

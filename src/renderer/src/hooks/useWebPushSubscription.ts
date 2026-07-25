import { useState, useCallback } from 'react'

// ─── Types ────────────────────────────────────────────────────────────────────

type PushSubscriptionState = 'idle' | 'subscribing' | 'subscribed' | 'failed'

// ─── Utility ──────────────────────────────────────────────────────────────────

/** Convert base64 VAPID public key to Uint8Array for pushManager.subscribe() */
export function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const rawData = atob(base64)
  return Uint8Array.from([...rawData].map((char) => char.charCodeAt(0)))
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useWebPushSubscription(): {
  state: PushSubscriptionState
  subscribe: () => Promise<void>
  unsubscribe: () => Promise<void>
  isSupported: boolean
} {
  const isSupported =
    typeof window !== 'undefined' &&
    'serviceWorker' in navigator &&
    'PushManager' in window

  const [state, setState] = useState<PushSubscriptionState>('idle')

  const subscribe = useCallback(async () => {
    if (!isSupported) return
    setState('subscribing')
    try {
      const keyRes = await fetch('/api/vapid-public-key')
      const { publicKey } = (await keyRes.json()) as { publicKey: string }

      const registration = await navigator.serviceWorker.ready
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(publicKey),
      })

      await fetch('/api/push-subscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ subscription }),
      })

      setState('subscribed')
    } catch {
      setState('failed')
    }
  }, [isSupported])

  const unsubscribe = useCallback(async () => {
    if (!isSupported) return
    try {
      const registration = await navigator.serviceWorker.ready
      const sub = await registration.pushManager.getSubscription()
      if (sub) {
        await sub.unsubscribe()
        await fetch('/api/push-unsubscribe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ endpoint: sub.endpoint }),
        })
      }
      setState('idle')
    } catch {
      setState('failed')
    }
  }, [isSupported])

  return { state, subscribe, unsubscribe, isSupported }
}

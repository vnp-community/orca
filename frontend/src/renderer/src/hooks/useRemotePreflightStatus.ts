import { useState, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'

// ─── Types ────────────────────────────────────────────────────────────────────

type RemotePreflightStatusResult = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useRemotePreflightStatus(devServerId: string | null): {
  status: RemotePreflightStatusResult | null
  loading: boolean
  refresh: (force?: boolean) => Promise<void>
} {
  const [loading, setLoading] = useState(false)
  const setRemotePreflightStatus = useAppStore((s) => s.setRemotePreflightStatus)
  const statusFromStore = useAppStore(
    (s) => (devServerId ? (s.remotePreflightByServer as Record<string, RemotePreflightStatusResult>)[devServerId] ?? null : null)
  )

  const refresh = useCallback(async (force = false) => {
    if (!devServerId) {return}
    setLoading(true)
    try {
      const result = await window.api.onboarding.getPreflightStatus({ devServerId, force })
      setRemotePreflightStatus(devServerId, result)
    } catch {
      // Non-fatal: stale data shown
    } finally {
      setLoading(false)
    }
  }, [devServerId, setRemotePreflightStatus])

  useEffect(() => {
    if (!devServerId) {return}
    void refresh()
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { status: statusFromStore, loading, refresh }
}

// ─── Derived helpers ──────────────────────────────────────────────────────────

export function useGhInstalled(devServerId: string | null): boolean {
  const status = useAppStore(
    (s) => (devServerId ? (s.remotePreflightByServer as Record<string, RemotePreflightStatusResult>)[devServerId] ?? null : null)
  )
  return status?.gh.installed === true
}

export function useGhAuthenticated(devServerId: string | null): boolean {
  const status = useAppStore(
    (s) => (devServerId ? (s.remotePreflightByServer as Record<string, RemotePreflightStatusResult>)[devServerId] ?? null : null)
  )
  return status?.gh.authenticated === true
}

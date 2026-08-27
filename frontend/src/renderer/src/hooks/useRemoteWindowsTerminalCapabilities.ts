import { useState, useCallback, useEffect } from 'react'
import { useAppStore } from '../store'
import { detectRuntimeOnboardingWindowsCapabilities } from '../runtime/runtime-onboarding-client'

// ─── Types ────────────────────────────────────────────────────────────────────

type RemoteWindowsCapabilities = {
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string
  gitBashAvailable: boolean
  gitBashPath?: string
  loading: boolean
  error: string | null
}

const DEFAULT_CAPS: RemoteWindowsCapabilities = {
  wslAvailable: false,
  wslDistros: [],
  pwshAvailable: false,
  gitBashAvailable: false,
  loading: false,
  error: null,
}

// ─── Module-level cache ───────────────────────────────────────────────────────

const capsCache = new Map<string, { caps: RemoteWindowsCapabilities; cachedAt: number }>()
const CACHE_TTL = 60_000

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useRemoteWindowsTerminalCapabilities(
  devServerId: string | null,
  enabled: boolean
): RemoteWindowsCapabilities & { retry: () => void } {
  const [caps, setCaps] = useState<RemoteWindowsCapabilities>(DEFAULT_CAPS)
  const [fetchKey, setFetchKey] = useState(0)

  const fetch = useCallback(async (id: string) => {
    const cached = capsCache.get(id)
    if (cached && Date.now() - cached.cachedAt < CACHE_TTL) {
      setCaps(cached.caps)
      return
    }
    setCaps((prev) => ({ ...prev, loading: true, error: null }))
    try {
      const result = await detectRuntimeOnboardingWindowsCapabilities(useAppStore.getState().settings, {
        devServerId: id
      })
      const next: RemoteWindowsCapabilities = {
        wslAvailable: result.wslAvailable,
        wslDistros: result.wslDistros,
        pwshAvailable: result.pwshAvailable,
        pwshVersion: result.pwshVersion,
        gitBashAvailable: result.gitBashAvailable,
        gitBashPath: result.gitBashPath,
        loading: false,
        error: null,
      }
      capsCache.set(id, { caps: next, cachedAt: Date.now() })
      setCaps(next)
    } catch (err) {
      setCaps((prev) => ({ ...prev, loading: false, error: (err as Error).message }))
    }
  }, [])

  useEffect(() => {
    if (!devServerId || !enabled) {
      setCaps(DEFAULT_CAPS)
      return
    }
    void fetch(devServerId)
  }, [devServerId, enabled, fetch, fetchKey])

  const retry = useCallback(() => {
    if (devServerId) {
      capsCache.delete(devServerId)
      setFetchKey((k) => k + 1)
    }
  }, [devServerId])

  return { ...caps, retry }
}

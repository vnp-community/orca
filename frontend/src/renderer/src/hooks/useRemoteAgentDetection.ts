import { useState, useEffect, useCallback } from 'react'
import type { DevServer } from '../../../shared/dev-server-types'

export type AgentDetectionState = {
  agents: string[]
  platform: NodeJS.Platform | null
  loading: boolean
  error: string | null
  lastDetectedAt: number | null
}

const DEFAULT_STATE: AgentDetectionState = {
  agents: [],
  platform: null,
  loading: false,
  error: null,
  lastDetectedAt: null,
}

// Module-level cache: survives React re-renders
const detectionCache = new Map<string, AgentDetectionState>()
const CACHE_TTL_MS = 60_000

export function useRemoteAgentDetection(devServerId: string | null): AgentDetectionState & {
  redetect: () => Promise<void>
} {
  const cached = devServerId ? detectionCache.get(devServerId) : undefined
  const isCacheValid = !!cached?.lastDetectedAt && Date.now() - cached.lastDetectedAt < CACHE_TTL_MS

  const [state, setState] = useState<AgentDetectionState>(
    isCacheValid ? cached! : DEFAULT_STATE
  )

  const detect = useCallback(async () => {
    if (!devServerId) {
      setState(DEFAULT_STATE)
      return
    }
    setState((prev) => ({ ...prev, loading: true, error: null }))
    try {
      const result = await window.api.onboarding.detectAgents({ devServerId })
      const next: AgentDetectionState = {
        agents: result.agents,
        platform: result.platform,
        loading: false,
        error: null,
        lastDetectedAt: Date.now(),
      }
      detectionCache.set(devServerId, next)
      setState(next)
    } catch (err) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: (err as Error).message,
      }))
    }
  }, [devServerId])

  useEffect(() => {
    if (!devServerId) {
      setState(DEFAULT_STATE)
      return
    }
    if (isCacheValid) {
      setState(cached!)
      return
    }
    void detect()
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { ...state, redetect: detect }
}

export function useAllServersAgentDetection(
  devServers: DevServer[]
): Record<string, AgentDetectionState> {
  const [results, setResults] = useState<Record<string, AgentDetectionState>>({})

  const serverIds = devServers
    .filter((ds) => ds.status === 'connected')
    .map((ds) => ds.id)
    .join(',')

  useEffect(() => {
    if (!serverIds) {return}
    void window.api.onboarding.detectAgentsAllServers().then((raw) => {
      const mapped: Record<string, AgentDetectionState> = {}
      for (const [id, data] of Object.entries(raw)) {
        mapped[id] = {
          agents: data.agents,
          platform: data.platform,
          loading: false,
          error: data.error ?? null,
          lastDetectedAt: Date.now(),
        }
      }
      setResults(mapped)
    })
  }, [serverIds])

  return results
}

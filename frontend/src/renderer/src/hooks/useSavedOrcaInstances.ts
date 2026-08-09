// useSavedOrcaInstances — localStorage-backed hook for managing Orca server instances (CR-006)
import { useState, useCallback } from 'react'

export type OrcaInstance = {
  id: string
  /** User-friendly display name: e.g. "Team Backend — vnp-blc" */
  label: string
  /** Full URL: e.g. "https://orca.team.internal" */
  url: string
  team?: string
  lastConnectedAt?: number
}

const STORAGE_KEY = 'orca.saved-instances'

function loadInstances(): OrcaInstance[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as OrcaInstance[]) : []
  } catch {
    return []
  }
}

function saveInstances(instances: OrcaInstance[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(instances))
  } catch {
    // Quota exceeded or incognito mode — fail silently
  }
}

export function useSavedOrcaInstances(): {
  instances: OrcaInstance[]
  addInstance: (instance: OrcaInstance) => void
  removeInstance: (id: string) => void
  updateLastConnected: (id: string) => void
} {
  const [instances, setInstancesState] = useState<OrcaInstance[]>(loadInstances)

  const addInstance = useCallback((instance: OrcaInstance): void => {
    setInstancesState((prev) => {
      const next = [...prev, instance]
      saveInstances(next)
      return next
    })
  }, [])

  const removeInstance = useCallback((id: string): void => {
    setInstancesState((prev) => {
      const next = prev.filter((i) => i.id !== id)
      saveInstances(next)
      return next
    })
  }, [])

  const updateLastConnected = useCallback((id: string): void => {
    setInstancesState((prev) => {
      const next = prev.map((i) =>
        i.id === id ? { ...i, lastConnectedAt: Date.now() } : i
      )
      saveInstances(next)
      return next
    })
  }, [])

  return { instances, addInstance, removeInstance, updateLastConnected }
}

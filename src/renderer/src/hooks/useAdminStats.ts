import { useState, useCallback, useEffect } from 'react'
import { fetchAdminStats, AdminStats } from '../components/admin/admin-api-client'

const POLL_INTERVAL_MS = 30_000

export function useAdminStats() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const data = await fetchAdminStats()
      setStats(data)
      setError(null)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    refresh()
    const timer = setInterval(refresh, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  return { stats, isLoading, error, refresh }
}

// TASK-FE-017: SessionsPage — manage active sessions.
import { useState, useEffect, useCallback } from 'react'
import { fetchAdminSessions, killAdminSession, AdminSession } from './admin-api-client'

function formatRelative(ts: number): string {
  const diff = Date.now() - ts
  const h = Math.floor(diff / 3_600_000)
  const m = Math.floor((diff % 3_600_000) / 60_000)
  if (h > 0) return `${h}h ago`
  return `${m}m ago`
}

export function SessionsPage() {
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadSessions = useCallback(async () => {
    try {
      setLoading(true)
      const data = await fetchAdminSessions()
      setSessions(data)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSessions()
  }, [loadSessions])

  const handleKill = async (sessionId: string) => {
    if (!confirm('Are you sure you want to kill this session?')) return
    
    // Optimistic update
    setSessions(prev => prev.filter(s => s.sessionId !== sessionId))
    
    try {
      await killAdminSession(sessionId)
    } catch (err) {
      // Revert if error
      alert(`Failed to kill session: ${(err as Error).message}`)
      loadSessions()
    }
  }

  const handleKillAll = async () => {
    if (!confirm('Are you sure you want to kill ALL sessions? This will log everyone out.')) return
    
    const ids = sessions.map(s => s.sessionId)
    setSessions([])
    
    try {
      await Promise.all(ids.map(id => killAdminSession(id)))
    } catch (err) {
      alert(`Failed to kill some sessions: ${(err as Error).message}`)
      loadSessions()
    }
  }

  return (
    <div className="admin-sessions-page">
      <div className="admin-page-header">
        <h1>Active Sessions</h1>
        {sessions.length > 0 && (
          <button type="button" onClick={handleKillAll}>Kill All</button>
        )}
      </div>

      {error && <div className="admin-error">{error}</div>}

      {loading ? (
        <div role="status" aria-label="Loading sessions">Loading sessions...</div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>User Email</th>
              <th>IP Address</th>
              <th>Started</th>
              <th>Last Seen</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map(session => (
              <tr key={session.sessionId}>
                <td>{session.userEmail}</td>
                <td>{session.ipAddress}</td>
                <td>{formatRelative(session.createdAt)}</td>
                <td>{formatRelative(session.lastSeenAt)}</td>
                <td>
                  <button type="button" onClick={() => handleKill(session.sessionId)}>Kill</button>
                </td>
              </tr>
            ))}
            {sessions.length === 0 && (
              <tr>
                <td colSpan={5}>No active sessions.</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}

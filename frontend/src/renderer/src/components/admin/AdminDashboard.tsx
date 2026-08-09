// TASK-FE-015: AdminDashboard — dashboard page for the Admin SPA.
import { useEffect, useState, useCallback } from 'react'
import { useAdminStats } from '../../hooks/useAdminStats'
import type { AdminSession } from './admin-api-client';
import { fetchAdminSessions } from './admin-api-client'

export function AdminDashboard() {
  const { stats, isLoading: statsLoading, error: statsError } = useAdminStats()
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [sessionsLoading, setSessionsLoading] = useState(true)
  const [sessionsError, setSessionsError] = useState<string | null>(null)

  const loadSessions = useCallback(async () => {
    try {
      setSessionsLoading(true)
      const data = await fetchAdminSessions()
      setSessions(data)
    } catch (err) {
      setSessionsError((err as Error).message)
    } finally {
      setSessionsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSessions()
  }, [loadSessions])

  if (statsLoading) {
    return (
      <div role="status" aria-label="Loading">
        Loading...
      </div>
    )
  }

  if (statsError) {
    return <div className="admin-error">Error loading stats: {statsError}</div>
  }

  return (
    <div className="admin-dashboard">
      <h1>Admin Dashboard</h1>
      
      <div className="admin-dashboard__stats-grid">
        <div className="stat-card">
          <span className="stat-card__label">Users</span>
          <span className="stat-card__value">{stats?.totalUsers ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-card__label">Active Sessions</span>
          <span className="stat-card__value">{stats?.activeSessions ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-card__label">SSH Conn.</span>
          <span className="stat-card__value">{stats?.sshConnections ?? 0}</span>
        </div>
        <div className="stat-card">
          <span className="stat-card__label">Devices</span>
          <span className="stat-card__value">{stats?.pairedDevices ?? 0}</span>
        </div>
      </div>

      <div className="admin-dashboard__sessions">
        <h2>Active Sessions</h2>
        {sessionsLoading ? (
          <div role="status" aria-label="Loading sessions">Loading sessions...</div>
        ) : sessionsError ? (
          <div className="admin-error">{sessionsError}</div>
        ) : (
          <table className="admin-table">
            <thead>
              <tr>
                <th>User</th>
                <th>IP Address</th>
                <th>Started</th>
                <th>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map(session => (
                <tr key={session.sessionId}>
                  <td>{session.userEmail}</td>
                  <td>{session.ipAddress}</td>
                  <td>{new Date(session.createdAt).toLocaleString()}</td>
                  <td>{new Date(session.lastSeenAt).toLocaleString()}</td>
                </tr>
              ))}
              {sessions.length === 0 && (
                <tr>
                  <td colSpan={4}>No active sessions</td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

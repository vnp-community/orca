// TASK-FE-019: AuditPage — view audit logs with filtering.
import { useState, useEffect, useCallback, useMemo } from 'react'
import type { AuditEntry } from './admin-api-client';
import { fetchAdminAudit } from './admin-api-client'

export function AuditPage() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [fromDate, setFromDate] = useState('')
  const [toDate, setToDate] = useState('')
  const [actionFilter, setActionFilter] = useState('')
  
  const [page, setPage] = useState(1)
  const limit = 50

  const loadAudit = useCallback(async () => {
    try {
      setLoading(true)
      const from = fromDate ? new Date(fromDate).getTime() : undefined
      const to = toDate ? new Date(toDate).getTime() : undefined
      const data = await fetchAdminAudit({ from, to, action: actionFilter || undefined })
      setEntries(data)
      setPage(1)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [fromDate, toDate, actionFilter])

  useEffect(() => {
    loadAudit()
  }, [loadAudit])

  const handleExportCSV = () => {
    const csvContent = [
      ['Time', 'User', 'Action', 'Detail', 'IP'].join(','),
      ...entries.map(e => 
        [
          new Date(e.createdAt).toISOString(),
          e.userEmail || '',
          e.action,
          `"${(e.detail || '').replace(/"/g, '""')}"`,
          e.ipAddress || ''
        ].join(',')
      )
    ].join('\n')

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'audit-log.csv'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
  }

  const paginatedEntries = useMemo(() => {
    const start = (page - 1) * limit
    return entries.slice(start, start + limit)
  }, [entries, page])

  const totalPages = Math.ceil(entries.length / limit)

  return (
    <div className="admin-audit-page">
      <div className="admin-page-header">
        <h1>Audit Log</h1>
        {entries.length > 0 && <button type="button" onClick={handleExportCSV}>Export CSV</button>}
      </div>

      <div className="admin-filters">
        <label>
          From:
          <input type="date" value={fromDate} onChange={e => setFromDate(e.target.value)} />
        </label>
        <label>
          To:
          <input type="date" value={toDate} onChange={e => setToDate(e.target.value)} />
        </label>
        <label>
          Action:
          <select value={actionFilter} onChange={e => setActionFilter(e.target.value)}>
            <option value="">All</option>
            <option value="login.success">login.success</option>
            <option value="login.fail">login.fail</option>
            <option value="logout">logout</option>
            <option value="ssh.connect">ssh.connect</option>
            <option value="ssh.disconnect">ssh.disconnect</option>
            <option value="user.create">user.create</option>
            <option value="user.update">user.update</option>
            <option value="user.deactivate">user.deactivate</option>
            <option value="agent.run">agent.run</option>
            <option value="policy.create">policy.create</option>
            <option value="policy.update">policy.update</option>
          </select>
        </label>
        {/* We rely on useEffect[loadAudit] to trigger, or a manual Refresh button */}
        <button type="button" onClick={loadAudit}>Refresh</button>
      </div>

      {error && <div className="admin-error">{error}</div>}

      {loading ? (
        <div role="status" aria-label="Loading audit logs">Loading audit logs...</div>
      ) : (
        <>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>User</th>
                <th>Action</th>
                <th>Detail</th>
                <th>IP</th>
              </tr>
            </thead>
            <tbody>
              {paginatedEntries.map(entry => (
                <tr key={entry.id}>
                  <td>{new Date(entry.createdAt).toLocaleString()}</td>
                  <td>{entry.userEmail}</td>
                  <td>{entry.action}</td>
                  <td>{entry.detail}</td>
                  <td>{entry.ipAddress}</td>
                </tr>
              ))}
              {entries.length === 0 && (
                <tr>
                  <td colSpan={5}>No audit logs found.</td>
                </tr>
              )}
            </tbody>
          </table>
          
          {totalPages > 1 && (
            <div className="admin-pagination">
              <button 
                type="button" 
                disabled={page === 1} 
                onClick={() => setPage(p => p - 1)}
              >
                Previous
              </button>
              <span>Page {page} of {totalPages}</span>
              <button 
                type="button" 
                disabled={page === totalPages} 
                onClick={() => setPage(p => p + 1)}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// TASK-FE-016: UsersPage — lists users with search, filter, and deactivate actions.
import { useState, useEffect, useCallback, useMemo } from 'react'
import { fetchAdminUsers, deactivateAdminUser, AdminUser } from './admin-api-client'
import { type AdminRoute } from './AdminLayout'

export function UsersPage({ onNavigate }: { onNavigate: (r: AdminRoute) => void }) {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState<string>('all')

  const loadUsers = useCallback(async () => {
    try {
      setLoading(true)
      const data = await fetchAdminUsers()
      setUsers(data)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadUsers()
  }, [loadUsers])

  const handleDeactivate = async (id: string) => {
    if (!confirm('Are you sure you want to deactivate this user?')) return
    try {
      await deactivateAdminUser(id)
      setUsers(users.map(u => u.id === id ? { ...u, isActive: false } : u))
    } catch (err) {
      alert(`Failed to deactivate: ${(err as Error).message}`)
    }
  }

  const filteredUsers = useMemo(() => {
    return users.filter(u => {
      const matchRole = roleFilter === 'all' || u.role === roleFilter
      const matchSearch = search.trim() === '' || 
        u.name.toLowerCase().includes(search.toLowerCase()) || 
        u.email.toLowerCase().includes(search.toLowerCase())
      return matchRole && matchSearch
    })
  }, [users, search, roleFilter])

  return (
    <div className="admin-users-page">
      <div className="admin-page-header">
        <h1>Users</h1>
        <button type="button" onClick={() => onNavigate('/users/new')}>Create User</button>
      </div>

      {error && <div className="admin-error">{error}</div>}

      <div className="admin-filters">
        <input 
          type="search" 
          role="searchbox"
          placeholder="Search by name or email..." 
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <label>
          Role:
          <select value={roleFilter} onChange={(e) => setRoleFilter(e.target.value)}>
            <option value="all">All</option>
            <option value="admin">Admin</option>
            <option value="lead">Lead</option>
            <option value="developer">Developer</option>
          </select>
        </label>
      </div>

      {loading ? (
        <div role="status" aria-label="Loading users">Loading users...</div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Status</th>
              <th>Name</th>
              <th>Email</th>
              <th>Role</th>
              <th>Provider</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {filteredUsers.map(user => (
              <tr key={user.id}>
                <td>{user.isActive ? '🟢' : '🔴'}</td>
                <td>{user.name}</td>
                <td>{user.email}</td>
                <td>{user.role}</td>
                <td>{user.provider}</td>
                <td>
                  <button type="button" onClick={() => onNavigate(`/users/${user.id}/edit` as AdminRoute)}>Edit</button>
                  {user.isActive && (
                    <button type="button" onClick={() => handleDeactivate(user.id)}>Deactivate</button>
                  )}
                </td>
              </tr>
            ))}
            {filteredUsers.length === 0 && (
              <tr>
                <td colSpan={6}>No users found.</td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </div>
  )
}

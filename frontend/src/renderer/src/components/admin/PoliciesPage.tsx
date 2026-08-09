// TASK-FE-018: PoliciesPage — lists access policies.
import { useState, useEffect, useCallback } from 'react'
import type { AdminPolicy } from './admin-api-client';
import { fetchAdminPolicies, deleteAdminPolicy } from './admin-api-client'
import type { AdminRoute } from './AdminLayout'

export function PoliciesPage({ onNavigate }: { onNavigate: (r: AdminRoute) => void }) {
  const [policies, setPolicies] = useState<AdminPolicy[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadPolicies = useCallback(async () => {
    try {
      setLoading(true)
      const data = await fetchAdminPolicies()
      setPolicies(data)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadPolicies()
  }, [loadPolicies])

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this policy?')) {return}
    try {
      await deleteAdminPolicy(id)
      setPolicies(policies.filter(p => p.id !== id))
    } catch (err) {
      alert(`Failed to delete: ${(err as Error).message}`)
    }
  }

  return (
    <div className="admin-policies-page">
      <div className="admin-page-header">
        <h1>Access Policies</h1>
        <button type="button" onClick={() => onNavigate('/policies/new')}>+ New Policy</button>
      </div>

      {error && <div className="admin-error">{error}</div>}

      {loading ? (
        <div role="status" aria-label="Loading policies">Loading policies...</div>
      ) : (
        <div className="admin-policy-cards">
          {policies.map(policy => (
            <div key={policy.id} className="policy-card">
              <h3>{policy.name}</h3>
              <div className="policy-card__details">
                <p><strong>Applies to:</strong></p>
                <ul>
                  <li>Teams: {policy.teams.length > 0 ? policy.teams.join(', ') : 'None'}</li>
                  <li>Roles: {policy.roles.length > 0 ? policy.roles.join(', ') : 'None'}</li>
                </ul>
                <p><strong>Allowed Servers:</strong> {policy.allowedServers.join(', ')}</p>
                <p><strong>Permissions:</strong></p>
                <ul>
                  <li>Create Worktrees: {policy.canCreateWorktrees ? 'Yes' : 'No'}</li>
                  <li>Delete Worktrees: {policy.canDeleteWorktrees ? 'Yes' : 'No'}</li>
                  <li>Access Production: {policy.canAccessProduction ? 'Yes' : 'No'}</li>
                </ul>
              </div>
              <div className="policy-card__actions">
                <button type="button" onClick={() => onNavigate(`/policies/${policy.id}/edit` as AdminRoute)}>Edit</button>
                <button type="button" onClick={() => handleDelete(policy.id)}>Delete</button>
              </div>
            </div>
          ))}
          {policies.length === 0 && (
            <p>No policies found.</p>
          )}
        </div>
      )}
    </div>
  )
}

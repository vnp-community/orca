// TASK-FE-018: PolicyForm — create/edit access policy form.
import { useState, useEffect, useCallback } from 'react'
import { createAdminPolicy, updateAdminPolicy, fetchAdminPolicies, AdminPolicy } from './admin-api-client'

type Props = 
  | { mode: 'create'; onDone: () => void }
  | { mode: 'edit'; policyId: string; onDone: () => void }

export function PolicyForm(props: Props) {
  const [name, setName] = useState('')
  const [teams, setTeams] = useState('')
  const [roles, setRoles] = useState<Set<AdminPolicy['roles'][0]>>(new Set())
  const [allowedServers, setAllowedServers] = useState('*')
  const [canCreateWorktrees, setCanCreateWorktrees] = useState(false)
  const [canDeleteWorktrees, setCanDeleteWorktrees] = useState(false)
  const [canAccessProduction, setCanAccessProduction] = useState(false)

  const [loading, setLoading] = useState(props.mode === 'edit')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isEdit = props.mode === 'edit'

  const loadPolicy = useCallback(async (id: string) => {
    try {
      const policies = await fetchAdminPolicies()
      const policy = policies.find(p => p.id === id)
      if (policy) {
        setName(policy.name)
        setTeams(policy.teams.join(', '))
        setRoles(new Set(policy.roles))
        setAllowedServers(policy.allowedServers.join(', '))
        setCanCreateWorktrees(policy.canCreateWorktrees)
        setCanDeleteWorktrees(policy.canDeleteWorktrees)
        setCanAccessProduction(policy.canAccessProduction)
      } else {
        setError('Policy not found')
      }
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (props.mode === 'edit') {
      loadPolicy(props.policyId)
    }
  }, [props, loadPolicy])

  const handleRoleToggle = (role: AdminPolicy['roles'][0]) => {
    const newRoles = new Set(roles)
    if (newRoles.has(role)) {
      newRoles.delete(role)
    } else {
      newRoles.add(role)
    }
    setRoles(newRoles)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    const parsedTeams = teams.split(',').map(t => t.trim()).filter(Boolean)
    const parsedServers = allowedServers.split(',').map(s => s.trim()).filter(Boolean)

    const payload = {
      name,
      teams: parsedTeams,
      roles: Array.from(roles),
      allowedServers: parsedServers,
      canCreateWorktrees,
      canDeleteWorktrees,
      canAccessProduction
    }

    try {
      if (props.mode === 'create') {
        await createAdminPolicy(payload)
      } else {
        await updateAdminPolicy(props.policyId, payload)
      }
      props.onDone()
    } catch (err) {
      setError((err as Error).message)
      setSubmitting(false)
    }
  }

  if (loading) {
    return <div role="status">Loading...</div>
  }

  return (
    <div className="admin-policy-form">
      <h1>{isEdit ? 'Edit Policy' : 'Create Policy'}</h1>
      
      {error && <div className="admin-error">{error}</div>}

      <form onSubmit={handleSubmit}>
        <label>
          Name:
          <input type="text" required value={name} onChange={e => setName(e.target.value)} />
        </label>
        
        <label>
          Teams (comma-separated):
          <input type="text" value={teams} onChange={e => setTeams(e.target.value)} placeholder="e.g. backend, frontend" />
        </label>
        
        <fieldset>
          <legend>Roles</legend>
          <label>
            <input type="checkbox" checked={roles.has('developer')} onChange={() => handleRoleToggle('developer')} />
            Developer
          </label>
          <label>
            <input type="checkbox" checked={roles.has('lead')} onChange={() => handleRoleToggle('lead')} />
            Lead
          </label>
          <label>
            <input type="checkbox" checked={roles.has('admin')} onChange={() => handleRoleToggle('admin')} />
            Admin
          </label>
        </fieldset>
        
        <label>
          Allowed Servers (comma-separated, * for all):
          <input type="text" required value={allowedServers} onChange={e => setAllowedServers(e.target.value)} />
        </label>
        
        <fieldset>
          <legend>Permissions</legend>
          <label>
            <input type="checkbox" checked={canCreateWorktrees} onChange={e => setCanCreateWorktrees(e.target.checked)} />
            Can create worktrees
          </label>
          <label>
            <input type="checkbox" checked={canDeleteWorktrees} onChange={e => setCanDeleteWorktrees(e.target.checked)} />
            Can delete worktrees
          </label>
          <label>
            <input type="checkbox" checked={canAccessProduction} onChange={e => setCanAccessProduction(e.target.checked)} />
            Can access production
          </label>
        </fieldset>

        <div className="form-actions">
          <button type="button" onClick={props.onDone} disabled={submitting}>Cancel</button>
          <button type="submit" disabled={submitting}>{isEdit ? 'Save Changes' : 'Create Policy'}</button>
        </div>
      </form>
    </div>
  )
}

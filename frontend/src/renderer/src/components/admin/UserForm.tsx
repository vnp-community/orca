// TASK-FE-016: UserForm — create/edit user form.
import { useState, useEffect, useCallback } from 'react'
import type { AdminUser } from './admin-api-client';
import { createAdminUser, updateAdminUser, fetchAdminUsers } from './admin-api-client'

type Props = 
  | { mode: 'create'; onDone: () => void }
  | { mode: 'edit'; userId: string; onDone: () => void }

export function UserForm(props: Props) {
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState<AdminUser['role']>('developer')
  const [provider, setProvider] = useState<AdminUser['provider']>('none')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [isActive, setIsActive] = useState(true)

  const [loading, setLoading] = useState(props.mode === 'edit')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isEdit = props.mode === 'edit'

  const loadUser = useCallback(async (id: string) => {
    try {
      const users = await fetchAdminUsers()
      const user = users.find(u => u.id === id)
      if (user) {
        setEmail(user.email)
        setName(user.name)
        setRole(user.role)
        setProvider(user.provider)
        setIsActive(user.isActive)
      } else {
        setError('User not found')
      }
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (props.mode === 'edit') {
      loadUser(props.userId)
    }
  }, [props, loadUser])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (provider === 'none' && !isEdit && password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    try {
      setSubmitting(true)
      if (props.mode === 'create') {
        await createAdminUser({
          email,
          name,
          role,
          provider,
          isActive,
          password: provider === 'none' ? password : undefined
        })
      } else {
        await updateAdminUser(props.userId, {
          email,
          name,
          role,
          provider,
          isActive
        })
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
    <div className="admin-user-form">
      <h1>{isEdit ? 'Edit User' : 'Create User'}</h1>
      
      {error && <div className="admin-error">{error}</div>}

      <form onSubmit={handleSubmit}>
        <label>
          Email:
          <input type="email" required value={email} onChange={e => setEmail(e.target.value)} disabled={isEdit} />
        </label>
        <label>
          Name:
          <input type="text" required value={name} onChange={e => setName(e.target.value)} />
        </label>
        <label>
          Role:
          <select value={role} onChange={e => setRole(e.target.value as any)}>
            <option value="developer">Developer</option>
            <option value="lead">Lead</option>
            <option value="admin">Admin</option>
          </select>
        </label>
        <label>
          Provider:
          <select value={provider} onChange={e => setProvider(e.target.value as any)} disabled={isEdit}>
            <option value="none">Local (Password)</option>
            <option value="github">GitHub SSO</option>
            <option value="google">Google SSO</option>
            <option value="keycloak">Keycloak</option>
          </select>
        </label>
        
        {provider === 'none' && !isEdit && (
          <>
            <label>
              Password:
              <input type="password" required value={password} onChange={e => setPassword(e.target.value)} />
            </label>
            <label>
              Confirm Password:
              <input type="password" required value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} />
            </label>
          </>
        )}

        {isEdit && (
          <label>
            <input type="checkbox" checked={isActive} onChange={e => setIsActive(e.target.checked)} />
            Active Account
          </label>
        )}

        <div className="form-actions">
          <button type="button" onClick={props.onDone} disabled={submitting}>Cancel</button>
          <button type="submit" disabled={submitting}>{isEdit ? 'Save Changes' : 'Create User'}</button>
        </div>
      </form>
    </div>
  )
}

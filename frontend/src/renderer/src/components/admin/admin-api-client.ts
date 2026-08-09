// Admin API Client — HTTP wrapper for all /admin/api/* endpoints (TASK-FE-013)
// All requests carry credentials: 'include' (session cookie).
// Throws descriptive Error on 401/403/non-OK responses.

// ─── Types ────────────────────────────────────────────────────────────────────

export type AdminUser = {
  id: string
  email: string
  name: string
  role: 'developer' | 'lead' | 'admin'
  provider: 'none' | 'github' | 'google' | 'keycloak'
  isActive: boolean
  lastLoginAt: number | null
}

export type AdminStats = {
  totalUsers: number
  activeSessions: number
  sshConnections: number
  pairedDevices: number
}

export type AdminSession = {
  sessionId: string
  userId: string
  userEmail: string
  ipAddress: string
  userAgent?: string
  createdAt: number
  lastSeenAt: number
}

export type AdminPolicy = {
  id: string
  name: string
  teams: string[]
  roles: ('developer' | 'lead' | 'admin')[]
  allowedServers: string[]  // '*' means all
  canCreateWorktrees: boolean
  canDeleteWorktrees: boolean
  canAccessProduction: boolean
}

export type AuditEntry = {
  id: string
  createdAt: number
  userId?: string
  userEmail?: string
  action: string
  detail?: string
  ipAddress?: string
}

export type AuditFilter = {
  from?: number
  to?: number
  action?: string
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

async function adminFetch<T>(
  path: string,
  init?: RequestInit
): Promise<T> {
  const res = await fetch(`/admin/api${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init
  })
  if (res.status === 401) {throw new Error('Unauthorized — please log in again')}
  if (res.status === 403) {throw new Error('Forbidden — admin access required')}
  if (!res.ok) {throw new Error(`Admin API error: ${res.status}`)}
  // 204 No Content → return empty object cast to T
  if (res.status === 204) {return {} as T}
  return res.json() as Promise<T>
}

// ─── Stats ────────────────────────────────────────────────────────────────────

export function fetchAdminStats(): Promise<AdminStats> {
  return adminFetch<AdminStats>('/stats')
}

// ─── Users ────────────────────────────────────────────────────────────────────

export function fetchAdminUsers(): Promise<AdminUser[]> {
  return adminFetch<AdminUser[]>('/users')
}

export function createAdminUser(
  data: Omit<AdminUser, 'id' | 'lastLoginAt'> & { password?: string }
): Promise<AdminUser> {
  return adminFetch<AdminUser>('/users', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export function updateAdminUser(
  id: string,
  data: Partial<Omit<AdminUser, 'id'>>
): Promise<AdminUser> {
  return adminFetch<AdminUser>(`/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data)
  })
}

export function deactivateAdminUser(id: string): Promise<void> {
  return adminFetch<void>(`/users/${id}`, { method: 'DELETE' })
}

// ─── Sessions ─────────────────────────────────────────────────────────────────

export function fetchAdminSessions(): Promise<AdminSession[]> {
  return adminFetch<AdminSession[]>('/sessions')
}

export function killAdminSession(sessionId: string): Promise<void> {
  return adminFetch<void>(`/sessions/${sessionId}`, { method: 'DELETE' })
}

// ─── Policies ─────────────────────────────────────────────────────────────────

export function fetchAdminPolicies(): Promise<AdminPolicy[]> {
  return adminFetch<AdminPolicy[]>('/policies')
}

export function createAdminPolicy(
  data: Omit<AdminPolicy, 'id'>
): Promise<AdminPolicy> {
  return adminFetch<AdminPolicy>('/policies', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export function updateAdminPolicy(
  id: string,
  data: Partial<Omit<AdminPolicy, 'id'>>
): Promise<AdminPolicy> {
  return adminFetch<AdminPolicy>(`/policies/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data)
  })
}

export function deleteAdminPolicy(id: string): Promise<void> {
  return adminFetch<void>(`/policies/${id}`, { method: 'DELETE' })
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

export function fetchAdminAudit(filter?: AuditFilter): Promise<AuditEntry[]> {
  const params = new URLSearchParams()
  if (filter?.from !== undefined) {params.set('from', String(filter.from))}
  if (filter?.to !== undefined) {params.set('to', String(filter.to))}
  if (filter?.action) {params.set('action', filter.action)}
  const qs = params.toString()
  return adminFetch<AuditEntry[]>(`/audit${qs ? `?${qs}` : ''}`)
}

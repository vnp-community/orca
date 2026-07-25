/**
 * Admin Types — Shared types for Admin Panel subsystem
 *
 * @module main/admin/admin-types
 */

export type AdminStats = {
  totalUsers:     number   // All users (including inactive)
  activeUsers:    number   // Users with is_active = 1
  activeSessions: number   // Sessions not yet expired
  pairedDevices:  number   // DeviceRegistry count (stub: 0 for now)
}

export type AuditEvent = {
  id:        number
  createdAt: number
  userId:    string | null
  userEmail: string | null
  action:    string
  detail:    Record<string, unknown> | null
  ipAddress: string | null
}

export type AuditLogInput = {
  userId?:    string
  userEmail?: string
  action:     string
  ipAddress?: string
  detail?:    Record<string, unknown>
}

export type AuditQueryFilter = {
  userId?:  string
  action?:  string
  from?:    number   // timestamp ms
  to?:      number   // timestamp ms
  limit?:   number   // default 100
  offset?:  number
}

export type PolicyInput = {
  name:                  string
  teams?:                string[]
  roles?:                string[]
  users?:                string[]
  allowedServers?:       string | string[]   // '*' or array of server IDs
  allowedProjects?:      string | string[]
  agentTrust?:           'minimal' | 'standard' | 'full'
  canCreateWorktrees?:   boolean
  canDeleteWorktrees?:   boolean
  canAccessProduction?:  boolean
}

/**
 * Known audit action constants.
 * Not an enum — string values so they are serializable and extensible
 * without breaking stored audit records.
 */
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:    'login.success',
  LOGIN_FAILURE:    'login.failure',
  LOGOUT:           'logout',
  SSO_LOGIN:        'sso.login',
  USER_CREATE:      'user.create',
  USER_DEACTIVATE:  'user.deactivate',
  SESSION_KILL:     'session.kill',
  SESSION_KILL_ALL: 'session.kill_all',
  SSH_CONNECT:      'ssh.connect',
  SSH_DISCONNECT:   'ssh.disconnect',
  SERVER_START:     'server.start',
  SERVER_STOP:      'server.stop',
} as const

export type AuditAction = typeof AUDIT_ACTIONS[keyof typeof AUDIT_ACTIONS]

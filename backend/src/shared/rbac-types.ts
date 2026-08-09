// src/shared/rbac-types.ts
// RBAC (Role-Based Access Control) types for Orca fleet access policies.
// Used for scoped pairing tokens when multi-user access is needed.

export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'

export type OrcaSsoConfig = {
  provider: OrcaIdentityProvider
  clientId: string
  /** OIDC discovery URL (for Keycloak, custom OIDC providers) */
  discoveryUrl?: string
  /** Only allow users from this GitHub organization */
  allowedOrg?: string
  /** Only allow emails from this Google Workspace domain */
  allowedDomain?: string
  /** OAuth2 redirect/callback URL */
  redirectUri?: string
}

export type OrcaUser = {
  id: string
  email: string
  name: string
  avatarUrl?: string
  teams: string[]
  projects: string[]
  role: 'developer' | 'lead' | 'admin'
  provider: OrcaIdentityProvider
  providerUserId: string
}

export type OrcaAccessPolicy = {
  id: string
  name: string
  /** Apply to users in these teams */
  teams?: string[]
  /** Apply to users with these roles */
  roles?: OrcaUser['role'][]
  /** Apply to specific user emails */
  users?: string[]
  /** Which fleet servers are allowed: '*' = all, or list of fleetIds */
  allowedServers: '*' | string[]
  /** Which projects are allowed: '*' = all, or list of project names */
  allowedProjects?: '*' | string[]
  /** Agent trust level for matched users */
  agentTrust?: 'minimal' | 'standard' | 'full'
  canCreateWorktrees?: boolean
  canDeleteWorktrees?: boolean
  canAccessProduction?: boolean
}

export type ScopedPairingToken = {
  token: string
  userId: string
  userEmail: string
  userName: string
  teams: string[]
  /** Resolved fleet server IDs this token can access. '*' means all. */
  allowedServerIds: string[]
  allowedProjects: string[]
  agentTrust: 'minimal' | 'standard' | 'full'
  issuedAt: number
  expiresAt: number
}

// ── Permission resolution ──────────────────────────────────────

/**
 * Resolve effective permissions for a user by merging all matching policies.
 * Union semantics: if any policy grants '*', the result is '*'.
 * AgentTrust takes the highest granted level across all matching policies.
 */
export function resolveUserPermissions(
  user: OrcaUser,
  policies: OrcaAccessPolicy[]
): {
  allowedServerIds: string[]
  allowedProjects: string[]
  agentTrust: 'minimal' | 'standard' | 'full'
} {
  const matchingPolicies = policies.filter((policy) => {
    if (policy.roles?.includes(user.role)) return true
    if (policy.teams?.some((t) => user.teams.includes(t))) return true
    if (policy.users?.includes(user.email)) return true
    return false
  })

  let allServers = false
  const serverIds = new Set<string>()
  let allProjects = false
  const projectIds = new Set<string>()
  let maxTrust: 'minimal' | 'standard' | 'full' = 'minimal'

  const trustRank = { minimal: 0, standard: 1, full: 2 } as const

  for (const policy of matchingPolicies) {
    if (policy.allowedServers === '*') {
      allServers = true
    } else {
      policy.allowedServers.forEach((id) => serverIds.add(id))
    }

    if (policy.allowedProjects === '*' || !policy.allowedProjects) {
      allProjects = true
    } else {
      policy.allowedProjects.forEach((id) => projectIds.add(id))
    }

    if (policy.agentTrust && trustRank[policy.agentTrust] > trustRank[maxTrust]) {
      maxTrust = policy.agentTrust
    }
  }

  return {
    allowedServerIds: allServers ? ['*'] : [...serverIds],
    allowedProjects: allProjects ? ['*'] : [...projectIds],
    agentTrust: maxTrust,
  }
}

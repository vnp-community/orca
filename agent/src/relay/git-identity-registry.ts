// agent/src/relay/git-identity-registry.ts
// Per-RPC-client git author/committer identity — BUG-AG-HLD-003.
//
// Why: `git config --global` is a single mutable file shared by every
// client connected to this relay daemon. Scoping identity to `clientId`
// instead means one user's `preflight.setGitIdentity` call can never leak
// into another concurrently-connected user's `git commit`.

export type GitIdentity = {
  readonly name: string
  readonly email: string
}

const identityByClientId = new Map<number, GitIdentity>()

export function setClientGitIdentity(clientId: number, identity: GitIdentity): void {
  identityByClientId.set(clientId, identity)
}

export function getClientGitIdentity(clientId: number): GitIdentity | undefined {
  return identityByClientId.get(clientId)
}

export function clearClientGitIdentity(clientId: number): void {
  identityByClientId.delete(clientId)
}

/** Per-invocation env override — never touches global git config. */
export function buildGitIdentityEnv(identity: GitIdentity | undefined): NodeJS.ProcessEnv {
  if (!identity) {return {}}
  return {
    GIT_AUTHOR_NAME: identity.name,
    GIT_AUTHOR_EMAIL: identity.email,
    GIT_COMMITTER_NAME: identity.name,
    GIT_COMMITTER_EMAIL: identity.email
  }
}

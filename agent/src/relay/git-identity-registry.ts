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

// ─── Part A (direct-websocket/relay-websocket) variant ─────────────────────
//
// Why a separate store keyed by the WebSocket connection instead of a
// numeric clientId: Part A (agent-rpc-dispatch.ts) has no multi-tenant
// "many simultaneously-attached clients" concept the way the SSH relay
// daemon does — it's architecturally one agent process talking to one
// backend gateway connection at a time. Reusing the same GitIdentity shape
// and buildGitIdentityEnv() keeps the two RPC surfaces' identity semantics
// identical (BUG-AG-HLD-003 parity — see
// specs/agent/api/gaps-and-findings.md #5), without threading a synthetic
// clientId through agent-rpc-dispatch.ts's dispatch chain just to reuse the
// Map<number,...> above. A WeakMap means a closed/GC'd connection's
// identity is cleaned up automatically, with no explicit detach hook needed
// (unlike Part B, which has a dispatcher onClientDetached callback to clear
// the numeric-keyed entry).
const identityByConnection = new WeakMap<object, GitIdentity>()

export function setConnectionGitIdentity(connection: object, identity: GitIdentity): void {
  identityByConnection.set(connection, identity)
}

export function getConnectionGitIdentity(connection: object): GitIdentity | undefined {
  return identityByConnection.get(connection)
}

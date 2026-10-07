import type { QualityCheckProfile } from './quality-profile-schema'
import { registerBuiltinProfiles } from './quality-profile-catalog'

export const SECURITY_SECRETS_DIFF_PROFILE: QualityCheckProfile = {
  id: 'security-secrets-diff',
  kind: 'repo-rules',
  title: 'Secret Detection (Diff)',
  parser: 'rules@diff',
  argv: [],
  scopes: ['changed', 'commitRange'],
  scopeStrategy: 'none',
  heavy: false,
  timeoutMs: 60000,
  maxOutputBytes: 10 * 1024 * 1024,
  enabled: true
}

export const DEPENDENCY_DIFF_PROFILE: QualityCheckProfile = {
  id: 'dependency-diff',
  kind: 'repo-rules',
  title: 'Dependency Diff',
  parser: 'deps@diff',
  argv: [],
  scopes: ['changed', 'commitRange'],
  scopeStrategy: 'none',
  heavy: false,
  timeoutMs: 60000,
  maxOutputBytes: 10 * 1024 * 1024,
  enabled: true
}

export const SECURITY_GO_VULN_PROFILE: QualityCheckProfile = {
  id: 'security-go-vuln',
  title: 'Go Vulnerability Scan (govulncheck)',
  parser: 'govulncheck@json',
  cwd: 'backend-go',
  argv: ['{bin:govulncheck}', '-format', 'json', './...'],
  scopes: ['worktree', 'changed', 'commitRange'],
  scopeStrategy: 'go-modules',
  heavy: true,
  network: true,
  enabled: false, // Gated pending approval O12
  requires: [{ file: 'backend-go/go.work' }],
  timeoutMs: 300000,
  maxOutputBytes: 20 * 1024 * 1024
}

export const SECURITY_DEPS_OSV_PROFILE: QualityCheckProfile = {
  id: 'security-deps-osv',
  title: 'OSV Dependency Scan',
  parser: 'osv-scanner@json',
  cwd: '.',
  argv: ['{bin:osv-scanner}', '--format', 'json', '--lockfile', 'pnpm-lock.yaml'],
  scopes: ['worktree', 'changed', 'commitRange'],
  scopeStrategy: 'none',
  heavy: false,
  network: true,
  enabled: false, // Gated pending approval O12
  requires: [{ file: 'pnpm-lock.yaml' }],
  timeoutMs: 180000,
  maxOutputBytes: 20 * 1024 * 1024
}

export function checkSecurityNetworkPolicy(
  profile: QualityCheckProfile,
  env: NodeJS.ProcessEnv = process.env
): { allowed: boolean; reason?: string } {
  if (profile.network && env.ORCA_QUALITY_NETWORK === 'deny') {
    return {
      allowed: false,
      reason: 'network_policy'
    }
  }
  return { allowed: true }
}

let registered = false

export function registerSecurityProfiles(): void {
  if (registered) return
  try {
    registerBuiltinProfiles([
      SECURITY_SECRETS_DIFF_PROFILE,
      DEPENDENCY_DIFF_PROFILE,
      SECURITY_GO_VULN_PROFILE,
      SECURITY_DEPS_OSV_PROFILE
    ])
    registered = true
  } catch {
    registered = true
  }
}

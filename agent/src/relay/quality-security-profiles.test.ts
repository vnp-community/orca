import { describe, it, expect } from 'vitest'
import {
  SECURITY_SECRETS_DIFF_PROFILE,
  DEPENDENCY_DIFF_PROFILE,
  SECURITY_GO_VULN_PROFILE,
  SECURITY_DEPS_OSV_PROFILE,
  checkSecurityNetworkPolicy,
  registerSecurityProfiles
} from './quality-security-profiles'
import { validateProfile } from './quality-profile-schema'
import { getCatalog } from './quality-profile-catalog'

describe('quality-security-profiles', () => {
  it('validates all 4 security and dependency profiles', () => {
    expect(validateProfile(SECURITY_SECRETS_DIFF_PROFILE).ok).toBe(true)
    expect(validateProfile(DEPENDENCY_DIFF_PROFILE).ok).toBe(true)
    expect(validateProfile(SECURITY_GO_VULN_PROFILE).ok).toBe(true)
    expect(validateProfile(SECURITY_DEPS_OSV_PROFILE).ok).toBe(true)
  })

  it('verifies gated profiles have enabled:false and builtin profiles have enabled:true', () => {
    expect(SECURITY_SECRETS_DIFF_PROFILE.enabled).toBe(true)
    expect(DEPENDENCY_DIFF_PROFILE.enabled).toBe(true)
    expect(SECURITY_GO_VULN_PROFILE.enabled).toBe(false)
    expect(SECURITY_DEPS_OSV_PROFILE.enabled).toBe(false)
  })

  it('gates network access based on ORCA_QUALITY_NETWORK env policy', () => {
    const denied = checkSecurityNetworkPolicy(SECURITY_GO_VULN_PROFILE, {
      ORCA_QUALITY_NETWORK: 'deny'
    })
    expect(denied.allowed).toBe(false)
    expect(denied.reason).toBe('network_policy')

    const allowed = checkSecurityNetworkPolicy(SECURITY_GO_VULN_PROFILE, {
      ORCA_QUALITY_NETWORK: 'allow'
    })
    expect(allowed.allowed).toBe(true)

    // Diff profile without network requirement is always allowed
    const diffAllowed = checkSecurityNetworkPolicy(SECURITY_SECRETS_DIFF_PROFILE, {
      ORCA_QUALITY_NETWORK: 'deny'
    })
    expect(diffAllowed.allowed).toBe(true)
  })

  it('registers security profiles into catalog', () => {
    registerSecurityProfiles()
    const catalog = getCatalog()
    const secDiff = catalog.profiles.find(p => p.id === 'security-secrets-diff')
    const depDiff = catalog.profiles.find(p => p.id === 'dependency-diff')
    expect(secDiff).toBeDefined()
    expect(depDiff).toBeDefined()
  })
})

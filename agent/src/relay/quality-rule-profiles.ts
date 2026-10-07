import type { QualityCheckProfile } from './quality-profile-schema'
import { registerBuiltinProfiles } from './quality-profile-catalog'

export const REPO_RULES_PROFILE: QualityCheckProfile = {
  id: 'repo-rules',
  kind: 'repo-rules',
  title: 'Orca Convention Diff Rules',
  parser: 'rules@diff',
  argv: [],
  scopes: ['changed', 'commitRange'],
  scopeStrategy: 'none',
  heavy: false,
  timeoutMs: 60000,
  maxOutputBytes: 10 * 1024 * 1024
}

export const REPO_RULES_SCRIPTS_PROFILE: QualityCheckProfile = {
  id: 'repo-rules-scripts',
  kind: 'repo-rules',
  title: 'Orca Convention Script Rules',
  parser: 'rules@script',
  argv: [],
  scopes: ['changed', 'commitRange'],
  scopeStrategy: 'none',
  heavy: false,
  timeoutMs: 120000,
  maxOutputBytes: 10 * 1024 * 1024,
  requires: [{ nodeModules: true }]
}

export function selectScriptRules(changedFiles: readonly string[]): string[] {
  const selected: string[] = []
  
  const hasI18n = changedFiles.some(f => f.includes('/i18n/') || f.startsWith('i18n/'))
  if (hasI18n) {
    selected.push('ORCA-004')
  }

  const hasRendererTsx = changedFiles.some(f => f.endsWith('.tsx') && f.includes('renderer/'))
  if (hasRendererTsx) {
    selected.push('ORCA-005')
  }

  const hasFeatureWall = changedFiles.some(f => f.includes('resources/onboarding/feature-wall'))
  if (hasFeatureWall) {
    selected.push('ORCA-006')
  }

  return selected
}

let registered = false

export function registerRuleProfiles(): void {
  if (registered) return
  try {
    registerBuiltinProfiles([REPO_RULES_PROFILE, REPO_RULES_SCRIPTS_PROFILE])
    registered = true
  } catch {
    // already registered
    registered = true
  }
}

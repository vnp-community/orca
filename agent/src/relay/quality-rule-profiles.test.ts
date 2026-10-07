import { describe, it, expect } from 'vitest'
import {
  REPO_RULES_PROFILE,
  REPO_RULES_SCRIPTS_PROFILE,
  selectScriptRules,
  registerRuleProfiles
} from './quality-rule-profiles'
import { validateProfile } from './quality-profile-schema'
import { getCatalog } from './quality-profile-catalog'
import { planRun } from './quality-run-planning'

describe('quality-rule-profiles', () => {
  it('validates REPO_RULES_PROFILE and REPO_RULES_SCRIPTS_PROFILE schemas', () => {
    const valDiff = validateProfile(REPO_RULES_PROFILE)
    expect(valDiff.ok).toBe(true)

    const valScript = validateProfile(REPO_RULES_SCRIPTS_PROFILE)
    expect(valScript.ok).toBe(true)
  })

  it('selectScriptRules selects rules based on changed files correctly', () => {
    expect(selectScriptRules(['frontend/src/i18n/en.json'])).toContain('ORCA-004')
    expect(selectScriptRules(['desktop/src/renderer/components/App.tsx'])).toContain('ORCA-005')
    expect(selectScriptRules(['resources/onboarding/feature-wall/banner.png'])).toContain('ORCA-006')
    expect(selectScriptRules(['docs/README.md'])).toEqual([])
  })

  it('registers rule profiles and makes them visible in catalog', () => {
    registerRuleProfiles()
    const catalog = getCatalog()
    const diff = catalog.profiles.find(p => p.id === 'repo-rules')
    const script = catalog.profiles.find(p => p.id === 'repo-rules-scripts')
    expect(diff).toBeDefined()
    expect(script).toBeDefined()

    const standard = catalog.suites.find(s => s.id === 'standard')
    expect(standard?.profiles).toContain('repo-rules')
    expect(standard?.profiles).toContain('repo-rules-scripts')
  })

  it('plans repo-rules with inProcess:true and skips on worktree scope', () => {
    registerRuleProfiles()
    const catalog = getCatalog()

    // Worktree scope (changedFiles = null) -> skipped with base_required
    const planWorktree = planRun({
      profileIds: ['repo-rules'],
      catalog: catalog.profiles,
      suites: catalog.suites,
      workspaceRoot: '/test',
      changedFiles: null,
      tmpRunDir: '/tmp',
      sourceEnv: {},
      deps: {
        resolveBin: () => '/bin/mock',
        gitCommonDir: '/test/.git',
        readGoWork: () => []
      }
    })

    expect(planWorktree.steps).toHaveLength(1)
    expect(planWorktree.steps[0].inProcess).toBe(true)
    expect(planWorktree.steps[0].skipReason).toBe('base_required')
    expect(planWorktree.steps[0].toolPath).toBe('')

    // Changed files with base -> inProcess: true without skipReason
    const planChanged = planRun({
      profileIds: ['repo-rules'],
      catalog: catalog.profiles,
      suites: catalog.suites,
      workspaceRoot: '/test',
      changedFiles: ['src/index.ts'],
      tmpRunDir: '/tmp',
      base: 'main',
      sourceEnv: {},
      deps: {
        resolveBin: () => '/bin/mock',
        gitCommonDir: '/test/.git',
        readGoWork: () => []
      }
    })

    expect(planChanged.steps).toHaveLength(1)
    expect(planChanged.steps[0].inProcess).toBe(true)
    expect(planChanged.steps[0].skipReason).toBeUndefined()
  })

  it('plans repo-rules-scripts selecting scripts based on changed files', () => {
    registerRuleProfiles()
    const catalog = getCatalog()

    const planWithI18n = planRun({
      profileIds: ['repo-rules-scripts'],
      catalog: catalog.profiles,
      suites: catalog.suites,
      workspaceRoot: '/test',
      changedFiles: ['i18n/locales.json'],
      tmpRunDir: '/tmp',
      sourceEnv: {},
      deps: {
        resolveBin: () => '/bin/mock',
        gitCommonDir: '/test/.git',
        readGoWork: () => []
      }
    })

    expect(planWithI18n.steps).toHaveLength(1)
    expect(planWithI18n.steps[0].args).toContain('ORCA-004')

    const planUnrelated = planRun({
      profileIds: ['repo-rules-scripts'],
      catalog: catalog.profiles,
      suites: catalog.suites,
      workspaceRoot: '/test',
      changedFiles: ['unrelated.txt'],
      tmpRunDir: '/tmp',
      sourceEnv: {},
      deps: {
        resolveBin: () => '/bin/mock',
        gitCommonDir: '/test/.git',
        readGoWork: () => []
      }
    })

    expect(planUnrelated.steps).toHaveLength(0)
  })
})

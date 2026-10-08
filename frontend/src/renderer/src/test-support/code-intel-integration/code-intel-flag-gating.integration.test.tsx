// @vitest-environment happy-dom
/**
 * code-intel-flag-gating.integration.test.tsx — FE-CV-TASK-073-01
 *
 * Integration tests for flag-gating logic: verifies that when feature flags
 * are off/unknown, the system:
 * 1. Does NOT call any code-intel channels other than settings.get
 * 2. Hides the appropriate UI entry points
 * 3. Shows UI when flags are enabled
 *
 * Uses mock store state; entry points are exercised through the hooks/pure rules their components
 * render from (DOM coverage of the same entry points lives in tests/e2e/code-intel-web).
 *
 * @see docs/frontend-debug-standards
 */

import { describe, it, expect } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useQualityFeatureFlags } from '../../hooks/useQualityFeatureFlags'
import { createFakeCodeIntelBackend } from '../code-intel-fake-backend'
import { useReviewEntryAvailability } from '../../components/review-map/entry/useReviewEntryAvailability'
import { useSourceControlReviewEntry } from '../../components/right-sidebar/source-control-review-entry'
import { openReviewFromEntryPoint } from '../../components/review-map/entry/open-review-entry'
import { getReviewActionAvailability } from '../../components/cmd-j/quick-action-context'
import { getVisibleRightSidebarActivityItems } from '../../components/right-sidebar/right-sidebar-activity-visibility'
import type { ActivityBarItem } from '../../components/right-sidebar/activity-bar-buttons'
import { computeReviewViewState } from '../../components/review-map/review-view-state'
import { useSourceControlQualityGate } from '../../components/right-sidebar/use-source-control-quality-gate'

// Mock useAppStore to return synthetic state
import { vi } from 'vitest'

vi.mock('@/store', () => ({
  useAppStore: Object.assign(
    vi.fn((selector: (s: unknown) => unknown) => selector(storeState)),
    { getState: () => storeState }
  )
}))

// Why: gated entry points must not reach any channel; every client call lands on this spy.
const clientCall = vi.fn()
vi.mock('../../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: clientCall, callEnvelope: clientCall })
}))

let storeState: Record<string, unknown> = {}

function setSupport(
  state: 'unknown' | 'enabled' | 'disabled' | 'unsupported',
  effective?: {
    codeIntelEnabled?: boolean
    qualityGateEnabled?: boolean
    aiReviewEnabled?: boolean
  }
) {
  storeState = { codeIntelSupportState: { state, effective } }
}

// ---------------------------------------------------------------------------
// Flag cascade matrix
// ---------------------------------------------------------------------------

describe('Flag cascade matrix: codeIntel → quality → ai', () => {
  it('unknown state → all flags false (fail-closed)', () => {
    setSupport('unknown')
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('disabled state → all flags false', () => {
    setSupport('disabled')
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('unsupported state → all flags false', () => {
    setSupport('unsupported')
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
  })

  it('enabled + codeIntelEnabled=false → all flags false', () => {
    setSupport('enabled', {
      codeIntelEnabled: false,
      qualityGateEnabled: true,
      aiReviewEnabled: true
    })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('enabled + codeIntel=true, quality=false → only codeIntel flag true', () => {
    setSupport('enabled', {
      codeIntelEnabled: true,
      qualityGateEnabled: false,
      aiReviewEnabled: true
    })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('enabled + quality=true, ai=false → codeIntel+quality true, ai false', () => {
    setSupport('enabled', {
      codeIntelEnabled: true,
      qualityGateEnabled: true,
      aiReviewEnabled: false
    })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(true)
    expect(flags.ai).toBe(false)
  })

  it('all enabled → all flags true', () => {
    setSupport('enabled', {
      codeIntelEnabled: true,
      qualityGateEnabled: true,
      aiReviewEnabled: true
    })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(true)
    expect(flags.ai).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Error handling edge cases
// ---------------------------------------------------------------------------

describe('Error state handling', () => {
  it('CODEINTEL_DISABLED → flags false (maps to disabled state)', () => {
    // Backend sends DISABLED error → support state becomes 'disabled'
    setSupport('disabled')
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
  })

  it('PROFILE_UNKNOWN does not hide quality feature', () => {
    // PQ-01: PROFILE_UNKNOWN is a run-time error, not a flag event
    // Quality flag is still enabled; only the profile UI shows an error state
    setSupport('enabled', {
      codeIntelEnabled: true,
      qualityGateEnabled: true,
      aiReviewEnabled: false
    })
    const flags = useQualityFeatureFlags()
    // Quality remains enabled even when profile is unknown
    expect(flags.quality).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Entry point visibility (component-level) — to be wired as components exist
// ---------------------------------------------------------------------------

describe('UI entry point gating (component integration)', () => {
  const WT = 'repo-1::/srv/demo'
  const worktreeState = {
    repos: [{ id: 'repo-1', projectId: 'project-1' }],
    worktreesByRepo: { 'repo-1': [{ id: WT, repoId: 'repo-1', projectId: 'project-1' }] },
    settings: { activeRuntimeEnvironmentId: 'env-1' },
    activeWorktreeId: WT,
    tabsByWorktree: {},
    agentStatusByPaneKey: {},
    retainedAgentsByPaneKey: {},
    reviewUiByWorktree: {}
  }
  const withFlag = (
    state: 'unknown' | 'enabled' | 'disabled' | 'unsupported',
    effective?: {
      codeIntelEnabled?: boolean
      qualityGateEnabled?: boolean
      aiReviewEnabled?: boolean
    }
  ): void => {
    storeState = { ...worktreeState, codeIntelSupportState: { state, effective } }
  }
  const ON = { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false }
  const reviewItem = {
    id: 'review',
    title: 'Review',
    gitOnly: true,
    codeIntelOnly: true
  } as unknown as ActivityBarItem
  const cmdCtx = (codeIntelEnabled: boolean) => ({
    activeView: 'terminal' as const,
    activeWorktreeId: WT,
    isLoading: false,
    sshStatus: null,
    codeIntelEnabled,
    reviewLensAvailable: () => true
  })

  it('Agent row Review button: hidden when codeIntel is off/unknown (FE-CV-SOL-061)', () => {
    for (const state of ['unknown', 'disabled', 'unsupported'] as const) {
      withFlag(state)
      expect(renderHook(() => useReviewEntryAvailability(WT)).result.current.visible).toBe(false)
    }
    withFlag('enabled', ON)
    expect(renderHook(() => useReviewEntryAvailability(WT)).result.current).toEqual({
      visible: true
    })
    expect(clientCall).not.toHaveBeenCalled()
  })

  it('Source Control "Review changes": hidden when codeIntel=false, open() is a no-op', () => {
    withFlag('disabled')
    const off = renderHook(() => useSourceControlReviewEntry({ worktreeId: WT })).result.current
    expect(off.visible).toBe(false)
    off.open()
    expect(openReviewFromEntryPoint(WT, 'source-control')).toBe(false)
    withFlag('enabled', ON)
    expect(
      renderHook(() => useSourceControlReviewEntry({ worktreeId: WT })).result.current.visible
    ).toBe(true)
    expect(clientCall).not.toHaveBeenCalled()
  })

  it('Cmd+K review actions: unavailable when codeIntel=false (FE-CV-TASK-050-18)', () => {
    expect(getReviewActionAvailability(cmdCtx(false), 'impact')).toEqual({
      available: false,
      reason: 'code-intel-disabled'
    })
    expect(getReviewActionAvailability(cmdCtx(true), 'impact')).toEqual({ available: true })
  })

  it('Right sidebar Review tab: filtered out when codeIntel=false (FE-CV-TASK-050-17)', () => {
    const base = { isFolder: false, isFolderWorkspace: false, isSshRepo: false }
    expect(
      getVisibleRightSidebarActivityItems([reviewItem], { ...base, codeIntelEnabled: false })
    ).toEqual([])
    expect(getVisibleRightSidebarActivityItems([reviewItem], base)).toEqual([])
    expect(
      getVisibleRightSidebarActivityItems([reviewItem], { ...base, codeIntelEnabled: true })
    ).toEqual([reviewItem])
  })

  it('Review tab content: disabled/unsupported screens instead of data (FE-CV-TASK-050-17)', () => {
    const input = {
      selectorUnsupported: false,
      scope: { status: 'loading' as const },
      status: { data: null, error: null },
      overlay: { data: null, error: null, pending: false },
      hasNewDataSignal: false
    }
    expect(computeReviewViewState({ ...input, support: 'disabled' })).toEqual({
      kind: 'screen',
      screen: { id: 'disabled' }
    })
    expect(computeReviewViewState({ ...input, support: 'unsupported' })).toEqual({
      kind: 'screen',
      screen: { id: 'unsupported' }
    })
  })

  it('Quality gate notice: hidden and silent when quality=false (FE-CV-TASK-085-04)', () => {
    withFlag('enabled', { ...ON, qualityGateEnabled: false })
    const opts = { worktreeId: WT, projectId: 'project-1', headOid: 'abc', base: 'main' }
    const gate = renderHook(() => useSourceControlQualityGate(opts)).result.current
    expect(gate.viewModel.visible).toBe(false)
    expect(clientCall).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// Regression: typeof window.api.codeIntel must not be used for feature detection
// ---------------------------------------------------------------------------

describe('Regression: window.api proxy guard', () => {
  it('typeof window.api.codeIntel is not used for feature detection', () => {
    // When window.api is a withFallback Proxy (all methods are functions),
    // typeof window.api.codeIntel === 'object' should NOT be used to determine
    // if code-intel is supported. Only settings.get result matters.
    // This is enforced by useQualityFeatureFlags reading from store state,
    // not from window.api directly.
    setSupport('disabled') // settings.get says disabled
    const flags = useQualityFeatureFlags()
    // Even if window.api.codeIntel exists (it's always a Proxy), flags must be false
    expect(flags.codeIntel).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Flag matrix driven by the fake backend (settings.get is the only flag source)
// ---------------------------------------------------------------------------

describe('Fake backend settings feed the flag hook; disabled tiers refuse view channels', () => {
  const sel = { projectId: 'project-1', worktreeId: 'worktree-1' }
  const cases = [
    { name: 'all off', patch: { codeIntelEnabled: false }, expected: [false, false, false] },
    {
      name: 'code-intel only',
      patch: { qualityGateEnabled: false },
      expected: [true, false, false]
    },
    { name: 'code-intel + quality', patch: {}, expected: [true, true, false] },
    { name: 'all on', patch: { aiReviewEnabled: true }, expected: [true, true, true] },
    // Why: quality/AI cannot outlive code-intel even when the tenant flags stay true.
    {
      name: 'master off beats tenant quality/ai',
      patch: { codeIntelEnabled: false, aiReviewEnabled: true },
      expected: [false, false, false]
    }
  ]
  for (const c of cases) {
    it(`${c.name}`, async () => {
      const backend = createFakeCodeIntelBackend()
      backend.setSettings(c.patch)
      const settings = (await backend.call('codeIntel.settings.get', {})) as {
        effective: Record<string, boolean>
      }
      setSupport('enabled', settings.effective)
      const flags = useQualityFeatureFlags()
      expect([flags.codeIntel, flags.quality, flags.ai]).toEqual(c.expected)
      // Gated consumers must not call anything but settings.get when their tier is off.
      if (!flags.codeIntel) {
        await expect(backend.call('codeIntel.status', sel)).rejects.toThrow(/^CODEINTEL_DISABLED/)
      }
      if (flags.codeIntel && !flags.quality) {
        await expect(backend.call('codeIntel.quality.gate', sel)).rejects.toThrow(
          /^CODEINTEL_QUALITY_GATE_DISABLED/
        )
      }
    })
  }

  it('mid-session disable: next call returns CODEINTEL_DISABLED, no push-based flag channel exists', async () => {
    const backend = createFakeCodeIntelBackend({ role: 'admin' })
    await expect(backend.call('codeIntel.status', sel)).resolves.toBeTruthy()
    await backend.call('codeIntel.settings.set', { codeIntelEnabled: false })
    await expect(backend.call('codeIntel.status', sel)).rejects.toThrow(/^CODEINTEL_DISABLED/)
    expect(backend.streamCount()).toBe(0)
  })

  it('PROFILE_UNKNOWN is a run-time error and leaves the flags untouched', async () => {
    const backend = createFakeCodeIntelBackend()
    backend.failNext('codeIntel.quality.gate', 'CODEINTEL_PROFILE_UNKNOWN: no profile')
    await expect(backend.call('codeIntel.quality.gate', sel)).rejects.toThrow(/PROFILE_UNKNOWN/)
    const settings = (await backend.call('codeIntel.settings.get', {})) as {
      effective: Record<string, boolean>
    }
    setSupport('enabled', settings.effective)
    expect(useQualityFeatureFlags().quality).toBe(true)
  })
})

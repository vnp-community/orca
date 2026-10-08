/**
 * code-intel-flag-gating.integration.test.tsx — FE-CV-TASK-073-01
 *
 * Integration tests for flag-gating logic: verifies that when feature flags
 * are off/unknown, the system:
 * 1. Does NOT call any code-intel channels other than settings.get
 * 2. Hides the appropriate UI entry points
 * 3. Shows UI when flags are enabled
 *
 * Uses mock store state rather than rendering components (per No-Browser-Debug rule).
 * Entry points that don't have components yet are marked as it.todo.
 *
 * @see docs/frontend-debug-standards
 */

import { describe, it, expect } from 'vitest'
import { useQualityFeatureFlags } from '../../hooks/useQualityFeatureFlags'
import { createFakeCodeIntelBackend } from '../code-intel-fake-backend'

// Mock useAppStore to return synthetic state
import { vi } from 'vitest'

vi.mock('@/store', () => ({
  useAppStore: vi.fn((selector: (s: unknown) => unknown) => selector(storeState))
}))

let storeState: Record<string, unknown> = {}

function setSupport(
  state: 'unknown' | 'enabled' | 'disabled' | 'unsupported',
  effective?: { codeIntelEnabled?: boolean; qualityGateEnabled?: boolean; aiReviewEnabled?: boolean }
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
    setSupport('enabled', { codeIntelEnabled: false, qualityGateEnabled: true, aiReviewEnabled: true })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('enabled + codeIntel=true, quality=false → only codeIntel flag true', () => {
    setSupport('enabled', { codeIntelEnabled: true, qualityGateEnabled: false, aiReviewEnabled: true })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('enabled + quality=true, ai=false → codeIntel+quality true, ai false', () => {
    setSupport('enabled', { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false })
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(true)
    expect(flags.ai).toBe(false)
  })

  it('all enabled → all flags true', () => {
    setSupport('enabled', { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: true })
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
    setSupport('enabled', { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false })
    const flags = useQualityFeatureFlags()
    // Quality remains enabled even when profile is unknown
    expect(flags.quality).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Entry point visibility (component-level) — to be wired as components exist
// ---------------------------------------------------------------------------

describe('UI entry point gating (component integration)', () => {
  it.todo('Agent toolbar button: hidden when codeIntel=false (FE-CV-SOL-061)')
  it.todo('Source Control panel: hidden when codeIntel=false (FE-CV-SOL-050-review-tab-wiring)')
  it.todo('Cmd+K palette entry: hidden when codeIntel=false (FE-CV-TASK-050-18)')
  it.todo('Sidebar tab: hidden when codeIntel=false (FE-CV-TASK-050-17)')
  it.todo('Review tab content: hidden when codeIntel=false (FE-CV-TASK-050-17)')
  it.todo('Quality gate notice: hidden when quality=false (FE-CV-TASK-085-04)')
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
    { name: 'code-intel only', patch: { qualityGateEnabled: false }, expected: [true, false, false] },
    { name: 'code-intel + quality', patch: {}, expected: [true, true, false] },
    { name: 'all on', patch: { aiReviewEnabled: true }, expected: [true, true, true] },
    // Why: quality/AI cannot outlive code-intel even when the tenant flags stay true.
    { name: 'master off beats tenant quality/ai', patch: { codeIntelEnabled: false, aiReviewEnabled: true }, expected: [false, false, false] }
  ]
  for (const c of cases) {
    it(`${c.name}`, async () => {
      const backend = createFakeCodeIntelBackend()
      backend.setSettings(c.patch)
      const settings = (await backend.call('codeIntel.settings.get', {})) as { effective: Record<string, boolean> }
      setSupport('enabled', settings.effective)
      const flags = useQualityFeatureFlags()
      expect([flags.codeIntel, flags.quality, flags.ai]).toEqual(c.expected)
      // Gated consumers must not call anything but settings.get when their tier is off.
      if (!flags.codeIntel) {
        await expect(backend.call('codeIntel.status', sel)).rejects.toThrow(/^CODEINTEL_DISABLED/)
      }
      if (flags.codeIntel && !flags.quality) {
        await expect(backend.call('codeIntel.quality.gate', sel)).rejects.toThrow(/^CODEINTEL_QUALITY_GATE_DISABLED/)
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
    const settings = (await backend.call('codeIntel.settings.get', {})) as { effective: Record<string, boolean> }
    setSupport('enabled', settings.effective)
    expect(useQualityFeatureFlags().quality).toBe(true)
  })
})

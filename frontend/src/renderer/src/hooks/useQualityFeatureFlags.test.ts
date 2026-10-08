/**
 * Tests for useQualityFeatureFlags.ts (FE-CV-TASK-085-01)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useQualityFeatureFlags } from './useQualityFeatureFlags'

// Mock useAppStore to return synthetic state
vi.mock('@/store', () => ({
  useAppStore: vi.fn((selector: (s: unknown) => unknown) => selector(mockState))
}))

let mockState: Record<string, unknown> = {}


describe('useQualityFeatureFlags — all flags false when unknown', () => {
  beforeEach(() => {
    mockState = {}
  })

  it('returns fail-closed when slice is absent', () => {
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
    expect(flags.state).toBe('unknown')
  })

  it('returns fail-closed when state is unknown', () => {
    mockState = { codeIntelSupportState: { state: 'unknown', effective: null } }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
  })

  it('returns fail-closed when state is disabled', () => {
    mockState = { codeIntelSupportState: { state: 'disabled', effective: undefined } }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.state).toBe('disabled')
  })

  it('returns fail-closed when state is unsupported', () => {
    mockState = { codeIntelSupportState: { state: 'unsupported' } }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
  })
})

describe('useQualityFeatureFlags — cascading flag logic', () => {
  it('all true when enabled and all effective flags true', () => {
    mockState = {
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: true }
      }
    }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(true)
    expect(flags.ai).toBe(true)
    expect(flags.state).toBe('enabled')
  })

  it('quality false when codeIntelEnabled false', () => {
    mockState = {
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: false, qualityGateEnabled: true, aiReviewEnabled: true }
      }
    }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(false)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('ai false when qualityGateEnabled false', () => {
    mockState = {
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: false, aiReviewEnabled: true }
      }
    }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(false)
    expect(flags.ai).toBe(false)
  })

  it('codeIntel true, quality true, ai false when aiReviewEnabled false', () => {
    mockState = {
      codeIntelSupportState: {
        state: 'enabled',
        effective: { codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false }
      }
    }
    const flags = useQualityFeatureFlags()
    expect(flags.codeIntel).toBe(true)
    expect(flags.quality).toBe(true)
    expect(flags.ai).toBe(false)
  })
})

import { afterEach, describe, expect, it } from 'vitest'
import {
  getReviewLenses,
  registerReviewLens,
  REVIEW_LENS_DEFINITIONS,
  resolveActiveLensId
} from './review-lens-registry'

const snapshot = REVIEW_LENS_DEFINITIONS.map((d) => ({ ...d }))
afterEach(() => {
  REVIEW_LENS_DEFINITIONS.splice(0, REVIEW_LENS_DEFINITIONS.length, ...snapshot)
})

describe('review lens registry', () => {
  it('canonical order is Impact, Architecture, Flows, ERD, Storage, Structure, Contracts', () => {
    expect(getReviewLenses({ quality: false }).map((l) => l.id)).toEqual([
      'impact',
      'architecture',
      'dataflow',
      'erd',
      'storage',
      'structure',
      'contract'
    ])
  })
  it('registering an existing id replaces it, a new id slots in by order', () => {
    const load = async () => ({ default: () => null })
    registerReviewLens({ ...REVIEW_LENS_DEFINITIONS[0], load })
    registerReviewLens({
      id: 'requirements',
      order: 65,
      labelKey: 'k',
      labelFallback: 'Requirements'
    })
    const lenses = getReviewLenses({ quality: false })
    expect(lenses.find((l) => l.id === 'impact')?.load).toBe(load)
    expect(lenses.map((l) => l.id).indexOf('requirements')).toBe(lenses.length - 2)
    expect(lenses.filter((l) => l.id === 'impact')).toHaveLength(1)
  })
  it('quality-only lenses are hidden without the flag', () => {
    registerReviewLens({
      id: 'quality',
      order: 80,
      labelKey: 'k',
      labelFallback: 'Quality',
      requiresQuality: true
    })
    expect(getReviewLenses({ quality: false }).some((l) => l.id === 'quality')).toBe(false)
    expect(getReviewLenses({ quality: true }).some((l) => l.id === 'quality')).toBe(true)
  })
  it('resolveActiveLensId falls back to the first visible lens', () => {
    const lenses = getReviewLenses({ quality: false })
    expect(resolveActiveLensId('erd', lenses)).toBe('erd')
    expect(resolveActiveLensId('nope', lenses)).toBe('impact')
    expect(resolveActiveLensId(null, [])).toBeNull()
  })
})

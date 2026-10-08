import { describe, expect, it } from 'vitest'
import { getReviewLenses } from '../review-lens-registry'
import { REQUIREMENTS_LENS_ID, requirementsLensDefinition } from './requirements-lens-registration'

describe('requirements lens registration', () => {
  it('is hidden without the quality flag and appears last with it', () => {
    expect(getReviewLenses({ quality: false }).some((l) => l.id === REQUIREMENTS_LENS_ID)).toBe(false)
    const visible = getReviewLenses({ quality: true })
    expect(visible.at(-1)?.id).toBe(REQUIREMENTS_LENS_ID)
  })

  it('is lazy: nothing is imported until load() runs', () => {
    expect(requirementsLensDefinition.requiresQuality).toBe(true)
    expect(typeof requirementsLensDefinition.load).toBe('function')
  })
})

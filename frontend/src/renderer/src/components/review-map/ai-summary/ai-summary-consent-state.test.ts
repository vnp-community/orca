import { beforeEach, describe, expect, it } from 'vitest'
import { hasConsentForLevel, recordConsentForLevel, resetAiSummaryConsent } from './ai-summary-consent-state'

beforeEach(resetAiSummaryConsent)

describe('ai summary consent state', () => {
  it('is per level and asked again for a different level', () => {
    expect(hasConsentForLevel('metadata')).toBe(false)
    recordConsentForLevel('metadata')
    expect(hasConsentForLevel('metadata')).toBe(true)
    expect(hasConsentForLevel('diff')).toBe(false)
  })

  it('does not persist beyond memory', () => {
    recordConsentForLevel('diff')
    resetAiSummaryConsent()
    expect(hasConsentForLevel('diff')).toBe(false)
  })
})

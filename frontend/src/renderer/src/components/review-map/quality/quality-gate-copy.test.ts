import { describe, expect, it } from 'vitest'
import type { QualityGate } from '../../../../../shared/code-intel-quality-types'
import { QUALITY_SCORECARD_COPY } from './quality-scorecard-copy'
import { modeNotice, unknownReason, verdictHeadline } from './quality-gate-copy'

const gate = (reasons: QualityGate['reasons'] = []): QualityGate => ({
  verdict: 'unknown',
  reasons,
  mode: 'inform',
  profile: 'full@repo/v1',
  basedOn: { runIds: [], indexCommit: '', stale: false }
})

describe('verdictHeadline', () => {
  it('names the profile for a pass and never over-claims', () => {
    expect(verdictHeadline('pass', 'full@repo/v3')).toBe('The checks of profile full all passed')
    expect(verdictHeadline('warn', 'full')).toBe('Has warnings')
    expect(verdictHeadline('fail', 'full')).toBe('Did not pass')
  })

  it('unknown says there is not enough data and borrows no pass wording', () => {
    const text = verdictHeadline('unknown', 'full')
    expect(text).toBe('Not enough data to conclude')
    expect(text.toLowerCase()).not.toMatch(/pass|ok|success/)
  })

  it('an unrecognised verdict value is treated as unknown', () => {
    expect(verdictHeadline('???' as never, 'full')).toBe('Not enough data to conclude')
  })
})

describe('modeNotice', () => {
  it('inform and block both stay advisory', () => {
    expect(modeNotice('inform')).toContain('does not block')
    expect(modeNotice('block')).toContain('only warns')
    expect(modeNotice('unknown')).toContain('not known')
  })
})

describe('unknownReason', () => {
  it('uses the first reason or states there is none', () => {
    expect(
      unknownReason(gate([{ check: 'coverage', observed: '', threshold: '', result: 'unknown' }]))
    ).toBe('coverage')
    expect(unknownReason(gate())).toBe('The backend gave no reason')
  })
})

describe('copy table wording', () => {
  it('never claims safety or cleanliness', () => {
    const text = Object.values(QUALITY_SCORECARD_COPY).join('\n').toLowerCase()
    for (const banned of [
      'safe',
      'clean',
      'all clear',
      'requirements met',
      'ai reviewed',
      'score'
    ]) {
      expect(text, banned).not.toContain(banned)
    }
  })
})

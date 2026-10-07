import { describe, it, expect } from 'vitest'
import { MAX_FINDINGS_PER_STEP, MAX_FINDINGS_PER_RUN, MESSAGE_MAX_BYTES, FIX_HINT_MAX_BYTES } from './quality-finding-limits'

describe('quality-finding-limits', () => {
  it('has correct defaults', () => {
    expect(MAX_FINDINGS_PER_STEP).toBe(5000)
    expect(MAX_FINDINGS_PER_RUN).toBe(20000)
    expect(MESSAGE_MAX_BYTES).toBe(2048)
    expect(FIX_HINT_MAX_BYTES).toBe(500)
  })
})

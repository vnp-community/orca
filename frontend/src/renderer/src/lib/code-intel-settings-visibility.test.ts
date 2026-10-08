import { describe, expect, it } from 'vitest'
import { selectCodeIntelSettingsVisible } from './code-intel-settings-visibility'

describe('selectCodeIntelSettingsVisible', () => {
  it('offers the section unless the backend is known to lack code intelligence', () => {
    expect(selectCodeIntelSettingsVisible({ codeIntelSupportState: { state: 'unknown' } })).toBe(true)
    expect(selectCodeIntelSettingsVisible({ codeIntelSupportState: { state: 'disabled' } })).toBe(true)
    expect(selectCodeIntelSettingsVisible({ codeIntelSupportState: { state: 'enabled' } })).toBe(true)
    expect(selectCodeIntelSettingsVisible({ codeIntelSupportState: { state: 'unsupported' } })).toBe(false)
  })
})

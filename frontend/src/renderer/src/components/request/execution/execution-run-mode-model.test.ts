import { describe, expect, it } from 'vitest'
import { runModeLabelOf } from './execution-run-mode-model'

describe('runModeLabelOf', () => {
  it.each([
    ['continuous', true, 'continuous'],
    ['continuous', false, 'continuous'],
    ['continuous', undefined, 'continuous'],
    ['step_by_step', true, 'step_by_step'],
    ['step_by_step', false, null],
    ['step_by_step', undefined, null],
    [undefined, true, null],
    [undefined, false, null]
  ] as const)('mode %s with phases %s gives %s', (mode, hasPhases, expected) => {
    expect(runModeLabelOf(mode, hasPhases)).toBe(expected)
  })
})

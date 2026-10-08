import { describe, expect, it } from 'vitest'
import type { RunnableProfile } from '../../../../../shared/code-intel-quality-types'
import { selectDefaultProfile, splitProfileRef } from './quality-profile-selection'

const p = (id: string, ready = true): RunnableProfile => ({
  id,
  title: id,
  kind: 'suite',
  ready,
  heavy: false,
  scopes: [],
  missing: []
})

describe('splitProfileRef', () => {
  it('splits <name>@<scope>/v<version>', () => {
    expect(splitProfileRef('full@repo/v3')).toEqual({ name: 'full', scope: 'repo', version: 3 })
    expect(splitProfileRef('go-test@builtin/v12')).toEqual({
      name: 'go-test',
      scope: 'builtin',
      version: 12
    })
  })

  it('keeps a bare name and survives odd input', () => {
    expect(splitProfileRef('full')).toEqual({ name: 'full', scope: null, version: null })
    expect(splitProfileRef('full@repo')).toEqual({ name: 'full', scope: 'repo', version: null })
    expect(splitProfileRef('')).toEqual({ name: '', scope: null, version: null })
    expect(splitProfileRef('@@//')).toEqual({ name: '@@//', scope: null, version: null })
  })
})

describe('selectDefaultProfile', () => {
  const runnable = [p('lint', false), p('full'), p('quick')]

  it('prefers the user pick, then the gate profile, then the configured one', () => {
    expect(
      selectDefaultProfile({
        uiProfile: 'quick',
        gateProfileRef: 'full@repo/v1',
        configuredName: 'lint',
        runnable
      })
    ).toBe('quick')
    expect(
      selectDefaultProfile({
        uiProfile: null,
        gateProfileRef: 'full@repo/v1',
        configuredName: 'quick',
        runnable
      })
    ).toBe('full')
    expect(
      selectDefaultProfile({
        uiProfile: null,
        gateProfileRef: null,
        configuredName: 'quick',
        runnable
      })
    ).toBe('quick')
  })

  it('drops a pick that is no longer runnable and falls back to the first ready entry', () => {
    expect(
      selectDefaultProfile({
        uiProfile: 'gone',
        gateProfileRef: 'gone@repo/v1',
        configuredName: 'gone',
        runnable
      })
    ).toBe('full')
  })

  it('returns null with nothing runnable and the first entry when none is ready', () => {
    expect(
      selectDefaultProfile({
        uiProfile: null,
        gateProfileRef: null,
        configuredName: null,
        runnable: []
      })
    ).toBeNull()
    expect(
      selectDefaultProfile({
        uiProfile: null,
        gateProfileRef: null,
        configuredName: null,
        runnable: [p('a', false)]
      })
    ).toBe('a')
  })
})

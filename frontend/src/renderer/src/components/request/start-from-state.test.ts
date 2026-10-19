import { describe, expect, it } from 'vitest'
import { baseRefToSend, initialStartFrom, startFromReducer } from './start-from-state'

describe('start-from-state', () => {
  it('starts on the repository default and sends nothing', () => {
    const state = initialStartFrom()
    expect(state).toEqual({ ref: null, hostOffline: false })
    expect(baseRefToSend(state)).toBeUndefined()
  })

  it('select sends the picked ref, useDefault goes back to sending nothing', () => {
    let state = startFromReducer(initialStartFrom(), { type: 'select', ref: ' release/1.2 ' })
    expect(baseRefToSend(state)).toBe('release/1.2')
    state = startFromReducer(state, { type: 'useDefault' })
    expect(baseRefToSend(state)).toBeUndefined()
  })

  it('an empty pick is no choice, never a pinned empty ref', () => {
    const state = startFromReducer(
      startFromReducer(initialStartFrom(), { type: 'select', ref: 'main' }),
      { type: 'select', ref: '   ' }
    )
    expect(state.ref).toBeNull()
  })

  it('a project change forgets the branch and the offline note of the previous project', () => {
    let state = startFromReducer(initialStartFrom(), { type: 'select', ref: 'release/1.2' })
    state = startFromReducer(state, { type: 'hostOffline', offline: true })
    state = startFromReducer(state, { type: 'projectChanged' })
    expect(state).toEqual({ ref: null, hostOffline: false })
  })

  it('hostOffline keeps the picked ref and returns the same state when nothing changes', () => {
    const picked = startFromReducer(initialStartFrom(), { type: 'select', ref: 'dev' })
    const offline = startFromReducer(picked, { type: 'hostOffline', offline: true })
    expect(offline).toEqual({ ref: 'dev', hostOffline: true })
    expect(startFromReducer(offline, { type: 'hostOffline', offline: true })).toBe(offline)
    expect(startFromReducer(offline, { type: 'hostOffline', offline: false }).ref).toBe('dev')
  })
})

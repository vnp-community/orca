import { describe, expect, it } from 'vitest'
import { baseRefDefaultFromWire } from './base-ref-default-wire'

describe('baseRefDefaultFromWire', () => {
  it('reads the gateway field `ref`', () => {
    expect(baseRefDefaultFromWire({ ref: 'origin/main' })).toEqual({
      defaultBaseRef: 'origin/main',
      remoteCount: 0
    })
  })

  it('keeps an answer that already uses the screen field names', () => {
    expect(baseRefDefaultFromWire({ defaultBaseRef: 'main', remoteCount: 2 })).toEqual({
      defaultBaseRef: 'main',
      remoteCount: 2
    })
  })

  it('names nothing when the repo has no default', () => {
    expect(baseRefDefaultFromWire({})).toEqual({ defaultBaseRef: null, remoteCount: 0 })
    expect(baseRefDefaultFromWire({ ref: '' })).toEqual({ defaultBaseRef: null, remoteCount: 0 })
    expect(baseRefDefaultFromWire(undefined)).toEqual({ defaultBaseRef: null, remoteCount: 0 })
  })
})

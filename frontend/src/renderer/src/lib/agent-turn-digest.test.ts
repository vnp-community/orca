import { describe, expect, it } from 'vitest'
import { buildFilesDigest, normalizePrompt, sha256Hex } from './agent-turn-digest'

describe('sha256Hex', () => {
  it('matches known SHA-256 vectors', () => {
    expect(sha256Hex('')).toBe('e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855')
    expect(sha256Hex('abc')).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad')
  })
})

describe('normalizePrompt', () => {
  it('trims, collapses whitespace and applies NFC', () => {
    expect(normalizePrompt('  fix \n  the\tbug ')).toBe('fix the bug')
    expect(normalizePrompt('é')).toBe('é')
  })
})

describe('buildFilesDigest', () => {
  it('is order independent and changes with content', () => {
    expect(buildFilesDigest(['b', 'a'])).toBe(buildFilesDigest(['a', 'b']))
    expect(buildFilesDigest(['a'])).not.toBe(buildFilesDigest(['a', 'b']))
  })
})

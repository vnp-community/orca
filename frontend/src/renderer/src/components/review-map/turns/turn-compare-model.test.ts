import { describe, expect, it } from 'vitest'
import { fileIdentities, fileIdentity, TURN_MARKER_MAX_FILES } from './turn-file-identity'
import { compareTurns, noteProgressHint } from './turn-compare-model'

const entry = { path: 'a.ts', status: 'modified', area: 'unstaged', added: 3, removed: 1 }

describe('fileIdentity', () => {
  it('is stable for equal input and carries the old path only for renames', () => {
    expect(fileIdentity(entry, { headOid: 'h1' })).toEqual(fileIdentity({ ...entry }, { headOid: 'h1' }))
    expect(fileIdentity(entry).o).toBeUndefined()
    expect(fileIdentity({ ...entry, oldPath: 'b.ts' }).o).toBe('b.ts')
  })

  it('changes when path, counts, status or head change', () => {
    const base = fileIdentity(entry, { headOid: 'h1' }).h
    expect(fileIdentity({ ...entry, added: 4 }, { headOid: 'h1' }).h).not.toBe(base)
    expect(fileIdentity({ ...entry, status: 'deleted' }, { headOid: 'h1' }).h).not.toBe(base)
    expect(fileIdentity({ ...entry, path: 'c.ts' }, { headOid: 'h1' }).h).not.toBe(base)
    expect(fileIdentity(entry, { headOid: 'h2' }).h).not.toBe(base)
    expect(fileIdentity(entry, { headOid: 'h1', mergeBase: 'm' }).h).not.toBe(base)
  })

  it('known limit: an edit that keeps the same counts has the same identity', () => {
    expect(fileIdentity(entry).h).toBe(fileIdentity({ ...entry }).h)
  })

  it('length-prefixes parts so boundaries cannot collide', () => {
    expect(fileIdentity({ path: 'ab', oldPath: 'c', status: 's' }).h).not.toBe(
      fileIdentity({ path: 'a', oldPath: 'bc', status: 's' }).h
    )
  })

  it('caps the list and reports truncation', () => {
    const many = Array.from({ length: TURN_MARKER_MAX_FILES + 5 }, (_v, i) => ({ ...entry, path: `f${i}` }))
    const r = fileIdentities(many)
    expect(r.files).toHaveLength(TURN_MARKER_MAX_FILES)
    expect(r.truncated).toBe(true)
    expect(fileIdentities(many.slice(0, 3)).truncated).toBe(false)
  })
})

const marker = (files: [string, string][], symbolKeys?: string[], overlayAvailable = true) => ({
  files: files.map(([p, h]) => ({ p, h })),
  symbolKeys,
  overlayAvailable
})

describe('compareTurns', () => {
  it('labels new, changed, unchanged and reverted files and is always an estimate', () => {
    const r = compareTurns(
      marker([['a', '1'], ['b', '1'], ['c', '1']]),
      marker([['a', '1'], ['b', '2'], ['d', '1']])
    )
    expect(r.estimated).toBe(true)
    expect(r.files).toEqual({
      a: 'unchanged_since',
      b: 'changed_in_turn',
      d: 'new_in_turn',
      c: 'reverted_in_turn'
    })
    expect(r.counts).toEqual({ new_in_turn: 1, changed_in_turn: 1, unchanged_since: 1, reverted_in_turn: 1 })
  })

  it('labels symbols only when both markers have symbol keys', () => {
    const both = compareTurns(marker([], ['s1', 's2']), marker([], ['s2', 's3']))
    expect(both.symbols).toEqual({ s2: 'unchanged_since', s3: 'new_in_turn', s1: 'reverted_in_turn' })
    expect(compareTurns(marker([], ['s1']), marker([])).symbols).toBeNull()
    expect(compareTurns(marker([], ['s1'], false), marker([], ['s1'])).symbols).toBeNull()
  })
})

describe('noteProgressHint', () => {
  it('compares the identity at send time with the current one', () => {
    expect(noteProgressHint({ filePath: 'a', fileIdentityAtSend: 'x' }, { a: 'x' })).toBe('file_unchanged')
    expect(noteProgressHint({ filePath: 'a', fileIdentityAtSend: 'x' }, { a: 'y' })).toBe('file_changed')
  })

  it('is unknown without a stored identity, and a file that left the list counts as changed unless capped', () => {
    expect(noteProgressHint({ filePath: 'a' }, { a: 'x' })).toBe('unknown')
    expect(noteProgressHint({ filePath: 'a', fileIdentityAtSend: 'x' }, {})).toBe('file_changed')
    expect(noteProgressHint({ filePath: 'a', fileIdentityAtSend: 'x' }, {}, true)).toBe('unknown')
  })
})

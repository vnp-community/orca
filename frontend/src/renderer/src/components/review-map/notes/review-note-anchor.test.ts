import { describe, expect, it } from 'vitest'
import type { ReviewNoteAnchor } from '../../../../../shared/code-intel-types'
import {
  anchorLensId,
  buildGraphNoteBody,
  isGraphAnchor,
  parseGraphNoteBody,
  resolveGraphNodeCommentTarget
} from './review-note-anchor'

const symbol: ReviewNoteAnchor = {
  kind: 'graph-node',
  lens: 'impact',
  nodeKey: 'k',
  filePath: 'a/b.ts',
  startLine: 10,
  endLine: 20,
  label: 'doThing'
}

describe('resolveGraphNodeCommentTarget', () => {
  it('symbol: startLine and lineNumber = endLine', () => {
    expect(resolveGraphNodeCommentTarget(symbol)).toEqual({ filePath: 'a/b.ts', startLine: 10, lineNumber: 20 })
  })

  it('symbol without endLine uses startLine', () => {
    expect(resolveGraphNodeCommentTarget({ ...symbol, endLine: undefined })).toEqual({
      filePath: 'a/b.ts',
      startLine: 10,
      lineNumber: 10
    })
  })

  it('ERD table / contract change without lines are file-level notes (0)', () => {
    const erd: ReviewNoteAnchor = { kind: 'graph-node', lens: 'erd', nodeKey: 't', filePath: 'db/0042.sql', label: 'orders' }
    expect(resolveGraphNodeCommentTarget(erd)).toEqual({ filePath: 'db/0042.sql', lineNumber: 0 })
    const contract: ReviewNoteAnchor = { ...erd, lens: 'contract', startLine: 12, endLine: undefined }
    expect(resolveGraphNodeCommentTarget(contract)).toEqual({ filePath: 'db/0042.sql', startLine: 12, lineNumber: 12 })
  })

  it('finding uses its line or 0', () => {
    const f: ReviewNoteAnchor = { kind: 'finding', findingKey: 'f', filePath: 'x.go', startLine: 40, label: 'L' }
    expect(resolveGraphNodeCommentTarget(f)).toEqual({ filePath: 'x.go', startLine: 40, lineNumber: 40 })
    expect(resolveGraphNodeCommentTarget({ ...f, startLine: undefined })).toEqual({ filePath: 'x.go', lineNumber: 0 })
  })

  it('nodes without a file (cluster, service, repo, secret) have no target', () => {
    expect(resolveGraphNodeCommentTarget({ ...symbol, filePath: '' })).toBeNull()
  })

  it('diff-line passes through', () => {
    expect(resolveGraphNodeCommentTarget({ kind: 'diff-line', filePath: 'a', lineNumber: 5 })).toEqual({
      filePath: 'a',
      lineNumber: 5
    })
  })
})

describe('buildGraphNoteBody / parseGraphNoteBody', () => {
  it('builds the prefix with lens and label', () => {
    expect(buildGraphNoteBody(symbol, 'rename this')).toBe('[Review map · impact · doThing] rename this')
  })

  it('masks secrets in the label, flattens separators and caps the length', () => {
    const body = buildGraphNoteBody(
      { ...symbol, label: `postgres://u:hunter2@h/db\n] · ${'x'.repeat(300)}` },
      't'
    )
    expect(body).not.toContain('hunter2')
    const parsed = parseGraphNoteBody(body)!
    expect(parsed.lens).toBe('impact')
    expect(parsed.label.length).toBeLessThanOrEqual(120)
    expect(parsed.text).toBe('t')
  })

  it('leaves diff-line bodies untouched and parse returns null without a prefix', () => {
    expect(buildGraphNoteBody({ kind: 'diff-line', filePath: 'a', lineNumber: 1 }, 'plain')).toBe('plain')
    expect(parseGraphNoteBody('plain')).toBeNull()
  })

  it('accepts a lens id unknown to this client', () => {
    const odd = { ...symbol, lens: 'novel' } as unknown as ReviewNoteAnchor
    expect(buildGraphNoteBody(odd, 'x')).toBe('[Review map · novel · doThing] x')
  })
})

describe('anchor helpers', () => {
  it('classifies anchors', () => {
    expect(isGraphAnchor(symbol)).toBe(true)
    expect(isGraphAnchor({ kind: 'diff-line', filePath: 'a', lineNumber: 1 })).toBe(false)
    expect(isGraphAnchor(undefined)).toBe(false)
    expect(anchorLensId(symbol)).toBe('impact')
    expect(anchorLensId({ kind: 'finding', findingKey: 'f', filePath: 'a', label: 'l' })).toBe('findings')
  })
})

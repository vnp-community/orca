import { describe, expect, it } from 'vitest'
import {
  filesChangedLabel,
  planVersionBadge,
  planVersionNote,
  versionStatsLabel
} from './plan-version-history'

describe('planVersionNote', () => {
  it('cuts what was asked to one line of 80 characters', () => {
    const note = planVersionNote({
      revision: 2,
      revisionFeedback: `line one\nline two ${'x'.repeat(200)}`
    })
    expect([...note]).toHaveLength(80)
    expect(note.endsWith('…')).toBe(true)
    expect(note).not.toContain('\n')
  })

  it('counts characters, not UTF-16 units, so a multi-byte request is not cut in half', () => {
    const note = planVersionNote({ revision: 2, revisionFeedback: '界'.repeat(100) })
    expect([...note]).toHaveLength(80)
    expect(note).not.toContain('�')
  })

  it('names the first version and a later one that carries no request', () => {
    expect(planVersionNote({ revision: 1, revisionFeedback: '' })).toBe('First proposal')
    expect(planVersionNote({ revision: 3, revisionFeedback: '   ' })).toBe('Revised')
  })

  it('keeps a short request whole', () => {
    expect(planVersionNote({ revision: 2, revisionFeedback: 'Add tests' })).toBe('Add tests')
  })
})

describe('planVersionBadge', () => {
  it('is the status of the version', () => {
    expect(planVersionBadge({ status: 'current' })).toBe('current')
    expect(planVersionBadge({ status: 'superseded' })).toBe('superseded')
  })
})

describe('filesChangedLabel', () => {
  it('counts the files a version changed, and says nothing when it kept no file list', () => {
    expect(filesChangedLabel({ filesChanged: 3, fileCount: 5 })).toBe('3 files changed')
    expect(filesChangedLabel({ filesChanged: 0, fileCount: 5 })).toBe('0 files changed')
    expect(filesChangedLabel({ filesChanged: 0, fileCount: 0 })).toBeNull()
  })
})

describe('versionStatsLabel', () => {
  it('reads duration and cost, and leaves out what the run did not report', () => {
    expect(versionStatsLabel({ durationMs: 90_000, costUsd: 1.254 })).toBe('1m 30s, $1.25')
    expect(versionStatsLabel({ durationMs: 12_000 })).toBe('12s')
    expect(versionStatsLabel({ costUsd: 0.5 })).toBe('$0.50')
    expect(versionStatsLabel({ turns: 4 })).toBeNull()
    expect(versionStatsLabel(undefined)).toBeNull()
  })
})

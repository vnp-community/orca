import { describe, expect, it } from 'vitest'
import type { PlanBundleFileEntry } from '../../../../../../shared/request-plan-revision'
import {
  formatBytes,
  isOmittedPlaceholder,
  isSafeDocumentPath,
  orderDocuments
} from './plan-document-model'

const file = (path: string, over: Partial<PlanBundleFileEntry> = {}): PlanBundleFileEntry => ({
  path,
  size: 10,
  omitted: false,
  change: 'unchanged',
  ...over
})

describe('orderDocuments', () => {
  it('orders proposal, design, specs by capability, tasks, then the rest', () => {
    const tabs = orderDocuments([
      file('notes/extra.md'),
      file('tasks.md'),
      file('specs/zeta/spec.md'),
      file('design.md'),
      file('specs/alpha/spec.md'),
      file('proposal.md'),
      file('.openspec.yaml')
    ])
    expect(tabs.map((t) => t.path)).toEqual([
      'proposal.md',
      'design.md',
      'specs/alpha/spec.md',
      'specs/zeta/spec.md',
      'tasks.md',
      '.openspec.yaml',
      'notes/extra.md'
    ])
    expect(tabs.map((t) => t.group)).toEqual([
      'proposal',
      'design',
      'spec',
      'spec',
      'tasks',
      'other',
      'other'
    ])
  })

  it('labels the fixed documents and names a spec by its capability', () => {
    const tabs = orderDocuments([
      file('proposal.md'),
      file('design.md'),
      file('specs/payments/refunds/spec.md'),
      file('tasks.md')
    ])
    expect(tabs.map((t) => t.label)).toEqual(['Proposal', 'Design', 'payments/refunds', 'Tasks'])
  })

  it('works without a design and keeps only what exists', () => {
    const tabs = orderDocuments([file('tasks.md'), file('proposal.md')])
    expect(tabs.map((t) => t.group)).toEqual(['proposal', 'tasks'])
  })

  it('keeps the size, the omitted flag and the change of each file', () => {
    const [tab] = orderDocuments([
      file('design.md', { size: 400_000, omitted: true, change: 'changed' })
    ])
    expect(tab).toMatchObject({ size: 400_000, omitted: true, change: 'changed' })
  })

  it('leaves out paths that could leave the change, and duplicates', () => {
    const tabs = orderDocuments([
      file('../secrets.md'),
      file('/etc/passwd'),
      file('specs/../../x.md'),
      file('C:\\boot.ini'),
      file('a\\b.md'),
      file('specs//spec.md'),
      file('tasks.md'),
      file('tasks.md')
    ])
    expect(tabs.map((t) => t.path)).toEqual(['tasks.md'])
  })

  it('answers an empty list for no files', () => {
    expect(orderDocuments([])).toEqual([])
  })
})

describe('isSafeDocumentPath', () => {
  it.each(['proposal.md', 'specs/a/spec.md', '.openspec.yaml'])('accepts %s', (p) => {
    expect(isSafeDocumentPath(p)).toBe(true)
  })
  it.each(['', '/a', '..', 'a/../b', './a', 'a//b', 'a\\b', 'c:/x', 'a\0b'])('refuses %j', (p) => {
    expect(isSafeDocumentPath(p)).toBe(false)
  })
})

describe('isOmittedPlaceholder', () => {
  it('recognises the stored marker and nothing that merely mentions it', () => {
    expect(isOmittedPlaceholder('<omitted size=9000000 sha256=abc123>')).toBe(true)
    expect(isOmittedPlaceholder('  <omitted size=1>\n')).toBe(true)
    expect(isOmittedPlaceholder('# Title\n<omitted size=1>')).toBe(false)
    expect(isOmittedPlaceholder('')).toBe(false)
  })
})

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [999, '999 B'],
    [1024, '1 KB'],
    [307_200, '300 KB'],
    [1_572_864, '1.5 MB'],
    [-5, '0 B'],
    [Number.NaN, '0 B']
  ])('%s -> %s', (n, text) => {
    expect(formatBytes(n)).toBe(text)
  })
})

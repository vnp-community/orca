import { describe, expect, it } from 'vitest'
import { REQUEST_RPC_METHODS } from './request-rpc-methods'
import { isPlanRevisionsSupported, REQUEST_PLAN_REVISIONS_FEATURE } from './request-plan-revision'
import {
  parsePlanBundleFile,
  parsePlanProposalView,
  parsePlanRevisions
} from './request-plan-revision-parsers'

const FULL_REVISION = {
  revision: 2,
  runId: 'run-2',
  status: 'current',
  engine: 'openspec',
  revisionFeedback: '  add a task for tests  ',
  actorId: 'user-1',
  generatedAt: '2026-10-20T10:00:00Z',
  commit: 'abc123',
  filesChanged: 2,
  fileCount: 5,
  textKept: true,
  stats: { durationMs: '90000', costUsd: 1.25, turns: 12 }
}

describe('parsePlanRevisions', () => {
  it('reads a full page, newest first, with a trimmed request text', () => {
    const page = parsePlanRevisions({
      revisions: [{ ...FULL_REVISION, revision: 1, status: 'superseded' }, FULL_REVISION],
      nextPageToken: 'tok'
    })
    expect(page.nextPageToken).toBe('tok')
    expect(page.revisions.map((r) => r.revision)).toEqual([2, 1])
    expect(page.revisions[0]).toEqual({
      revision: 2,
      runId: 'run-2',
      status: 'current',
      engine: 'openspec',
      revisionFeedback: 'add a task for tests',
      actorId: 'user-1',
      generatedAt: '2026-10-20T10:00:00Z',
      commit: 'abc123',
      filesChanged: 2,
      fileCount: 5,
      textKept: true,
      stats: { durationMs: 90000, costUsd: 1.25, turns: 12 }
    })
  })

  it('fills safe defaults for omitted proto3 zero values', () => {
    const [rev] = parsePlanRevisions({ revisions: [{ revision: 1 }] }).revisions
    expect(rev).toEqual({
      revision: 1,
      runId: '',
      status: 'superseded',
      engine: '',
      revisionFeedback: '',
      actorId: '',
      generatedAt: '',
      commit: '',
      filesChanged: 0,
      fileCount: 0,
      textKept: true
    })
  })

  it('reads a status it does not know as superseded and a pruned text as not kept', () => {
    const [rev] = parsePlanRevisions({
      revisions: [{ revision: 3, status: 'weird', textKept: false }]
    }).revisions
    expect(rev.status).toBe('superseded')
    expect(rev.textKept).toBe(false)
  })

  it('drops broken elements and duplicates and survives wrong types', () => {
    const page = parsePlanRevisions({
      revisions: [
        null,
        'x',
        7,
        [],
        { revision: 0 },
        { revision: 'oops' },
        { revision: 2 },
        { revision: 2 }
      ],
      nextPageToken: 5
    })
    expect(page.revisions).toHaveLength(1)
    expect(page.nextPageToken).toBe('')
  })

  it('drops a cost the run did not report', () => {
    const [rev] = parsePlanRevisions({
      revisions: [{ revision: 1, stats: { hasCost: false, costUsd: 0, durationMs: 5 } }]
    }).revisions
    expect(rev.stats).toEqual({ durationMs: 5 })
    const [bare] = parsePlanRevisions({ revisions: [{ revision: 1, stats: {} }] }).revisions
    expect(bare.stats).toBeUndefined()
  })

  it.each([null, undefined, 'text', 42, [], true, { revisions: null }, { revisions: 'x' }])(
    'never throws and answers an empty page for %j',
    (raw) => {
      expect(parsePlanRevisions(raw)).toEqual({ revisions: [], nextPageToken: '' })
    }
  )
})

describe('parsePlanProposalView', () => {
  it('reads an OpenSpec answer with bundle, checks and knowledge', () => {
    const view = parsePlanProposalView({
      runId: 'r1',
      status: 'succeeded',
      engine: 'openspec',
      proposal: null,
      bundle: {
        changeId: 'c-1',
        branch: 'request/7-proposal',
        commit: 'abc',
        tasksMdDigest: 'dg',
        status: 'ready',
        syncState: 'in_sync',
        files: [
          { path: 'proposal.md', size: '120', omitted: false, sha256: 'aa', change: 'changed' },
          { path: 'design.md', size: 9_000_000, omitted: true, change: 'weird' },
          { path: '', size: 1 },
          'junk'
        ]
      },
      checks: [
        { code: 'STRUCTURE_OK', where: 'change', message: 'ok', severity: 'info' },
        { code: 'SECRETS_REDACTED', message: '2 secrets were masked', severity: 'warning' },
        { severity: 'warning' }
      ],
      knowledge: {
        available: true,
        snapshot: 's1',
        read: ['REQ-1', 3],
        gaps: ['no ADR'],
        conflicts: []
      }
    })
    expect(view.engine).toBe('openspec')
    expect(view.status).toBe('succeeded')
    expect(view.bundle?.files).toEqual([
      { path: 'proposal.md', size: 120, omitted: false, sha256: 'aa', change: 'changed' },
      { path: 'design.md', size: 9_000_000, omitted: true, change: 'unchanged' }
    ])
    expect(view.checks.map((c) => c.code)).toEqual(['STRUCTURE_OK', 'SECRETS_REDACTED'])
    expect(view.knowledge).toEqual({
      available: true,
      snapshot: 's1',
      read: ['REQ-1'],
      gaps: ['no ADR'],
      conflicts: []
    })
  })

  it('reads a native proposal with a phase, a flat task and omitted zero values', () => {
    const view = parsePlanProposalView({
      runId: 'r2',
      status: 'succeeded',
      proposal: {
        title: 'Fix it',
        summary: 'Because.',
        phases: [
          {
            title: 'Phase 1',
            tasks: [
              { title: 'A', estimatedHours: 2.5, labels: ['db'], irreversible: true },
              { title: 'B', dependsOnIndices: [0, -1, 'x', 1.5] }
            ],
            dependsOnPhaseIndices: [0]
          }
        ],
        tasks: [{ title: 'Flat' }]
      }
    })
    expect(view.bundle).toBeNull()
    expect(view.checks).toEqual([])
    expect(view.knowledge).toBeNull()
    expect(view.proposal?.notes).toBe('')
    expect(view.proposal?.phases[0].tasks[0]).toMatchObject({
      title: 'A',
      estimatedHours: 2.5,
      labels: ['db'],
      irreversible: true,
      dependsOnIndices: []
    })
    expect(view.proposal?.phases[0].tasks[1].dependsOnIndices).toEqual([0])
    expect(view.proposal?.phases[0].tasks[1].estimatedHours).toBeUndefined()
    expect(view.proposal?.tasks[0].title).toBe('Flat')
  })

  it.each([null, undefined, 'x', 3, [], true])('never throws and answers empty for %j', (raw) => {
    expect(parsePlanProposalView(raw)).toEqual({
      runId: '',
      status: '',
      engine: '',
      proposal: null,
      bundle: null,
      checks: [],
      knowledge: null,
      errorCode: '',
      errorMessage: ''
    })
  })

  it('reads a failed run and survives wrong types', () => {
    const view = parsePlanProposalView({
      status: 'failed',
      errorCode: 'REQUEST_PLAN_REVISION_NO_CHANGE',
      errorMessage: 5,
      checks: 'x',
      bundle: { files: null }
    })
    expect(view.status).toBe('failed')
    expect(view.errorCode).toBe('REQUEST_PLAN_REVISION_NO_CHANGE')
    expect(view.errorMessage).toBe('')
    expect(view.bundle?.files).toEqual([])
  })
})

describe('parsePlanBundleFile', () => {
  it('reads a file and an omitted placeholder', () => {
    expect(parsePlanBundleFile({ path: 'tasks.md', content: '# x', omitted: false })).toEqual({
      path: 'tasks.md',
      content: '# x',
      omitted: false
    })
    expect(parsePlanBundleFile({ path: 'big.md', omitted: true }).omitted).toBe(true)
  })

  it.each([null, undefined, 'x', 3, []])('answers empty for %j', (raw) => {
    expect(parsePlanBundleFile(raw)).toEqual({ path: '', content: '', omitted: false })
  })
})

describe('Plan revision wire names', () => {
  it('uses the names the gateway registers', () => {
    expect(REQUEST_RPC_METHODS.PLAN_BUNDLE_FILE).toBe('request.planBundleFile')
    expect(REQUEST_RPC_METHODS.PLAN_REVISIONS).toBe('request.planRevisions')
    expect(REQUEST_PLAN_REVISIONS_FEATURE).toBe('request.planRevisions')
  })

  it('tells a gateway with the flag from one without', () => {
    expect(isPlanRevisionsSupported(new Set(['request.baseRef']))).toBe(false)
    expect(isPlanRevisionsSupported(new Set(['request.planRevisions']))).toBe(true)
  })
})

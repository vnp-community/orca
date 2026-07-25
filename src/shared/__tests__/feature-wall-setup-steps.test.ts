// ─── feature-wall-setup-steps.test.ts ────────────────────────────────────────
// Unit tests for Phase 3 feature wall helpers — TASK-041.

import { describe, it, expect } from 'vitest'
import {
  isConnectDevServerComplete,
  isAddDevServerRepoComplete,
  getFirstIncompleteDevServerStepId
} from '../feature-wall-setup-steps'
import type { DevServer } from '../dev-server-types'
import type { Repo } from '../types'

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeServer(status: DevServer['status']): DevServer {
  return {
    id: `ds-${Math.random()}`,
    name: 'Test Server',
    connectionType: 'ssh',
    sshTargetId: 'target-1',
    wsUrl: null,
    workspaceDir: null,
    addedAt: Date.now(),
    status
  }
}

function makeRepo(devServerId: string | null = null): Repo {
  return {
    id: `repo-${Math.random()}`,
    name: 'test-repo',
    path: '/tmp/test-repo',
    addedAt: Date.now(),
    devServerId
  } as unknown as Repo
}

const ALL_DONE: Record<string, boolean> = {
  'connect-dev-server': true,
  'add-dev-server-repo': true,
  'default-agent': true,
  'agent-capabilities': true,
  'task-sources': true,
  'add-two-repos': true,
  'setup-script': true,
  notifications: true,
  'two-worktrees': true,
  browser: true
}

// ── isConnectDevServerComplete ────────────────────────────────────────────────

describe('isConnectDevServerComplete', () => {
  it('no servers → false', () => {
    expect(isConnectDevServerComplete([])).toBe(false)
  })

  it('servers tất cả disconnected → false', () => {
    expect(
      isConnectDevServerComplete([
        makeServer('disconnected'),
        makeServer('error')
      ])
    ).toBe(false)
  })

  it('1 server connected → true', () => {
    expect(
      isConnectDevServerComplete([makeServer('disconnected'), makeServer('connected')])
    ).toBe(true)
  })

  it('server đang connecting vẫn là false', () => {
    expect(isConnectDevServerComplete([makeServer('connecting')])).toBe(false)
  })
})

// ── isAddDevServerRepoComplete ────────────────────────────────────────────────

describe('isAddDevServerRepoComplete', () => {
  it('activeDevServerId null → false', () => {
    expect(isAddDevServerRepoComplete([makeRepo('ds-1')], null)).toBe(false)
  })

  it('không có repo với đúng devServerId → false', () => {
    expect(
      isAddDevServerRepoComplete([makeRepo('ds-other'), makeRepo(null)], 'ds-1')
    ).toBe(false)
  })

  it('có repo với đúng devServerId → true', () => {
    expect(isAddDevServerRepoComplete([makeRepo('ds-1'), makeRepo('ds-other')], 'ds-1')).toBe(true)
  })

  it('không có repos → false', () => {
    expect(isAddDevServerRepoComplete([], 'ds-1')).toBe(false)
  })
})

// ── getFirstIncompleteDevServerStepId ─────────────────────────────────────────

describe('getFirstIncompleteDevServerStepId', () => {
  it('không có server → "connect-dev-server" ưu tiên tuyệt đối', () => {
    expect(
      getFirstIncompleteDevServerStepId({}, [], [], null)
    ).toBe('connect-dev-server')
  })

  it('servers disconnected → "connect-dev-server"', () => {
    expect(
      getFirstIncompleteDevServerStepId(
        { 'connect-dev-server': false },
        [makeServer('disconnected')],
        [],
        null
      )
    ).toBe('connect-dev-server')
  })

  it('có server connected, chưa add repo → "add-dev-server-repo"', () => {
    expect(
      getFirstIncompleteDevServerStepId(
        { 'connect-dev-server': true },
        [makeServer('connected')],
        [],
        'ds-1'
      )
    ).toBe('add-dev-server-repo')
  })

  it('có server + repo, chưa add repos → "add-two-repos"', () => {
    expect(
      getFirstIncompleteDevServerStepId(
        {
          'connect-dev-server': true,
          'add-dev-server-repo': true,
          'default-agent': true,
          'agent-capabilities': true,
          'task-sources': true
        },
        [makeServer('connected')],
        [makeRepo('ds-1')],
        'ds-1'
      )
    ).toBe('add-two-repos')
  })

  it('tất cả done → null', () => {
    expect(
      getFirstIncompleteDevServerStepId(ALL_DONE, [makeServer('connected')], [makeRepo('ds-1')], 'ds-1')
    ).toBeNull()
  })

  it('connect-dev-server trước add-dev-server-repo trong ORDER', () => {
    // Server connected but no repo: expect add-dev-server-repo (not connect-dev-server)
    const result = getFirstIncompleteDevServerStepId(
      { 'connect-dev-server': true },
      [makeServer('connected')],
      [],
      'ds-1'
    )
    expect(result).toBe('add-dev-server-repo')
  })
})

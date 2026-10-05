// @vitest-environment happy-dom
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useJiraProjectPreselect } from './useJiraProjectPreselect'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'

vi.mock('../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))
vi.mock('../store', () => ({ useAppStore: { getState: () => ({ settings: {} }) } }))

const mockRpc = vi.mocked(callRuntimeRpc)
const repos = [
  { id: 'repo-a', projectId: 'pa' },
  { id: 'repo-b', projectId: 'pb' }
]
const item = (jiraSiteId?: string) => ({
  type: 'issue' as const,
  provider: 'jira' as const,
  number: 0,
  title: 'ABC-1 t',
  url: 'https://x.atlassian.net/browse/ABC-1',
  jiraIdentifier: 'ABC-1',
  jiraSiteId
})

describe('useJiraProjectPreselect', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRpc.mockResolvedValue([
      { id: 'pa', jiraProjectKey: 'ABC', jiraSiteId: 's1' },
      { id: 'pb', jiraProjectKey: 'ABC', jiraSiteId: 's2' }
    ])
  })

  it('selects the repo of the project matching key and site', async () => {
    const setRepoId = vi.fn()
    renderHook(() =>
      useJiraProjectPreselect({
        linkedWorkItem: item('s2'),
        skip: false,
        eligibleRepos: repos,
        setRepoId
      })
    )
    await waitFor(() => expect(setRepoId).toHaveBeenCalledWith('repo-b'))
  })

  it('does not guess when the site is unknown and several match', async () => {
    const setRepoId = vi.fn()
    renderHook(() =>
      useJiraProjectPreselect({
        linkedWorkItem: item(),
        skip: false,
        eligibleRepos: repos,
        setRepoId
      })
    )
    await waitFor(() => expect(mockRpc).toHaveBeenCalled())
    await Promise.resolve()
    expect(setRepoId).not.toHaveBeenCalled()
  })

  it('does nothing when skipped', () => {
    renderHook(() =>
      useJiraProjectPreselect({
        linkedWorkItem: item('s1'),
        skip: true,
        eligibleRepos: repos,
        setRepoId: vi.fn()
      })
    )
    expect(mockRpc).not.toHaveBeenCalled()
  })
})

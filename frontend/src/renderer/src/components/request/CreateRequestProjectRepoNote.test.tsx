// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/store'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { CreateRequestProjectRepoNote } from './CreateRequestProjectRepoNote'

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ kind: 'local' })
}))

const repo = (over: Record<string, unknown> = {}): Record<string, unknown> => ({
  id: 'r1',
  url: '/srv/app',
  displayName: 'app',
  devServerId: 'ds-1',
  ...over
})

describe('CreateRequestProjectRepoNote', () => {
  beforeEach(() => {
    useAppStore.setState({ devServers: [{ id: 'ds-1', name: 'Mac Studio' }] } as never)
  })
  afterEach(() => {
    cleanup()
    vi.mocked(callRuntimeRpc).mockReset()
  })

  it('renders nothing and calls nothing without a project', () => {
    const { container } = render(<CreateRequestProjectRepoNote projectId="" />)
    expect(container).toBeEmptyDOMElement()
    expect(callRuntimeRpc).not.toHaveBeenCalled()
  })

  it('names the repo and host the request will run on', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue({ repos: [repo()] })
    render(<CreateRequestProjectRepoNote projectId="p1" />)
    const note = await screen.findByTestId('create-request-repo-note')
    expect(note).toHaveTextContent('repo app (/srv/app) on Mac Studio')
    expect(note).not.toHaveTextContent('first of')
    expect(callRuntimeRpc).toHaveBeenCalledWith(expect.anything(), 'repo.list', { projectId: 'p1' })
  })

  it('says the first repo is used when the project has several', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue({
      repos: [repo(), repo({ id: 'r2', url: '/srv/other', displayName: 'other' })]
    })
    render(<CreateRequestProjectRepoNote projectId="p1" />)
    const note = await screen.findByTestId('create-request-repo-note')
    expect(note).toHaveTextContent('repo app')
    expect(note).toHaveTextContent("first of this project's 2 repos")
  })

  it('warns when the project has no repo', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValue({ repos: [] })
    render(<CreateRequestProjectRepoNote projectId="p1" />)
    expect(await screen.findByTestId('create-request-repo-note')).toHaveTextContent('no repo yet')
  })

  it('shows nothing, not an error, when the repo list cannot be loaded', async () => {
    vi.mocked(callRuntimeRpc).mockRejectedValue(new Error('denied'))
    const { container } = render(<CreateRequestProjectRepoNote projectId="p1" />)
    await waitFor(() => expect(callRuntimeRpc).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('does not show the previous project repo while the next one loads', async () => {
    vi.mocked(callRuntimeRpc).mockResolvedValueOnce({ repos: [repo()] })
    const { rerender } = render(<CreateRequestProjectRepoNote projectId="p1" />)
    await screen.findByTestId('create-request-repo-note')
    vi.mocked(callRuntimeRpc).mockReturnValueOnce(new Promise(() => {}))
    rerender(<CreateRequestProjectRepoNote projectId="p2" />)
    expect(screen.queryByTestId('create-request-repo-note')).toBeNull()
  })
})

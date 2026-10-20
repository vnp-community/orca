// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const getDefault = vi.fn()
const searchRefs = vi.fn()
const listBranches = vi.fn()
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: () =>
    Promise.resolve({
      repos: [{ id: 'repo-1', url: '/work/ledger', displayName: 'ledger', devServerId: '' }]
    }),
  getActiveRuntimeTarget: () => ({ kind: 'local' }),
  subscribeRuntimeStreamChannel: vi.fn()
}))
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: (...args: unknown[]) => getDefault(...args),
  searchRuntimeRepoBaseRefs: (...args: unknown[]) => searchRefs(...args),
  listRuntimeRepoBranches: (...args: unknown[]) => listBranches(...args)
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'
import { StartFromField } from './StartFromField'
import { baseRefToSend, initialStartFrom, startFromReducer } from './start-from-state'

const BRANCHES = [
  { name: 'feature/other', remote: false },
  { name: 'main', remote: false },
  { name: 'release/1.x', remote: false },
  { name: 'origin/hotfix/x', remote: true }
]

function Harness(): React.JSX.Element {
  const [state, dispatch] = useReducer(startFromReducer, undefined, initialStartFrom)
  return (
    <TooltipProvider>
      <StartFromField projectId="p1" state={state} dispatch={dispatch} error={null} />
      <output data-testid="sent">{baseRefToSend(state) ?? 'nothing'}</output>
    </TooltipProvider>
  )
}

const trigger = () => screen.findByRole('button', { name: /^Starting branch:/ })
const sent = () => screen.getByTestId('sent').textContent
const optionNames = () => screen.queryAllByRole('option').map((o) => o.getAttribute('aria-label'))

async function openList(): Promise<void> {
  fireEvent.click(await trigger())
  await screen.findByRole('listbox')
}

beforeEach(() => {
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue([])
  listBranches.mockReset().mockResolvedValue({ status: 'ok', branches: BRANCHES, truncated: false })
  useAppStore.setState({ requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]) })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('the Start from button', () => {
  it('names the exact default branch with a default tag while nothing is chosen', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
    render(<Harness />)
    const button = await screen.findByRole('button', {
      name: 'Starting branch: Default branch (origin/main)'
    })
    // The name is split for middle truncation, so read it as one piece.
    expect(button.querySelector('[data-slot="middle-truncated-ref"]')).toHaveTextContent(
      /^origin\/main$/
    )
    expect(within(button).getByText('default')).toBeInTheDocument()
    expect(button).not.toHaveTextContent('Default branch')
    expect(sent()).toBe('nothing')
  })

  it('says "Default branch" when the default cannot be named', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: null, remoteCount: 0 })
    render(<Harness />)
    const button = await screen.findByRole('button', { name: 'Starting branch: Default branch' })
    expect(button).toHaveTextContent('Default branch')
    expect(within(button).queryByText('default')).toBeNull()
  })

  it('shows the chosen branch instead, names it in the hint and offers the way back to the default', async () => {
    render(<Harness />)
    await openList()
    fireEvent.click(screen.getByRole('option', { name: 'release/1.x' }))
    const button = await screen.findByRole('button', { name: 'Starting branch: release/1.x' })
    expect(within(button).queryByText('default')).toBeNull()
    expect(sent()).toBe('release/1.x')
    expect(
      screen.getByText('Worktrees for this request start from release/1.x of ledger.')
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Use default (main)' }))
    await screen.findByRole('button', { name: 'Starting branch: Default branch (main)' })
    expect(sent()).toBe('nothing')
    expect(screen.queryByRole('button', { name: /^Use default/ })).toBeNull()
  })
})

describe('the branch list in the popover', () => {
  it('opens on the list: default pinned, then local, then remote-only branches', async () => {
    render(<Harness />)
    await openList()
    expect(optionNames()).toEqual([
      'main (default)',
      'feature/other',
      'release/1.x',
      'origin/hotfix/x (remote only)'
    ])
    expect(listBranches).toHaveBeenCalledWith({ activeRuntimeEnvironmentId: null }, 'repo-1')
  })

  it('choosing the default again sends no baseRef', async () => {
    render(<Harness />)
    await openList()
    fireEvent.click(screen.getByRole('option', { name: 'release/1.x' }))
    await screen.findByRole('button', { name: 'Starting branch: release/1.x' })
    fireEvent.click(await trigger())
    await screen.findByRole('listbox')
    fireEvent.click(screen.getByRole('option', { name: 'main (default)' }))
    await screen.findByRole('button', { name: 'Starting branch: Default branch (main)' })
    expect(sent()).toBe('nothing')
  })

  it('choosing a remote-only branch sends its full name', async () => {
    render(<Harness />)
    await openList()
    fireEvent.click(screen.getByRole('option', { name: 'origin/hotfix/x (remote only)' }))
    await screen.findByRole('button', { name: 'Starting branch: origin/hotfix/x' })
    expect(sent()).toBe('origin/hotfix/x')
  })

  it('loads again on every open and keeps "Type a branch name instead"', async () => {
    render(<Harness />)
    await openList()
    expect(screen.getByRole('button', { name: 'Type a branch name instead' })).toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('listbox')).toBeNull())
    await openList()
    expect(listBranches).toHaveBeenCalledTimes(2)
  })

  it('shows the generic default row while the default branch is unknown', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: null, remoteCount: 0 })
    render(<Harness />)
    await openList()
    expect(optionNames()[0]).toBe('Default branch')
  })

  it('falls back to the search picker, quietly, on a gateway without repo.listBranches', async () => {
    listBranches.mockResolvedValue({ status: 'unsupported' })
    searchRefs.mockResolvedValue(['release/1.2'])
    render(<Harness />)
    fireEvent.click(await trigger())
    const search = await screen.findByRole('combobox', { name: 'Search branches' })
    fireEvent.change(search, { target: { value: 'rel' } })
    fireEvent.click(await screen.findByRole('option', { name: 'release/1.2' }))
    await screen.findByRole('button', { name: 'Starting branch: release/1.2' })
    expect(screen.queryByText('Local branches')).toBeNull()
    expect(sent()).toBe('release/1.2')
  })

  it('shows the load error with Retry', async () => {
    listBranches.mockRejectedValueOnce(new Error('git failed'))
    render(<Harness />)
    fireEvent.click(await trigger())
    expect(await screen.findByText('Could not load the branches. Try again.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await screen.findByRole('listbox')
    expect(optionNames()).toContain('release/1.x')
  })
})

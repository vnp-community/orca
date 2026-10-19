// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

type Rpc = (method: string, params: Record<string, unknown>) => unknown
const rpc = vi.fn<Rpc>()
const getDefault = vi.fn()
const searchRefs = vi.fn()
const toasts = vi.hoisted(() => ({
  base: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  success: vi.fn(),
  error: vi.fn()
}))

vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: (_target: unknown, method: string, params: Record<string, unknown>) =>
    Promise.resolve().then(() => rpc(method, params)),
  getActiveRuntimeTarget: () => ({ kind: 'local' }),
  subscribeRuntimeStreamChannel: vi.fn()
}))
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: (...args: unknown[]) => getDefault(...args),
  searchRuntimeRepoBaseRefs: (...args: unknown[]) => searchRefs(...args)
}))
vi.mock('sonner', () => ({
  toast: Object.assign((...args: unknown[]) => toasts.base(...args), toasts)
}))
// Radix Select needs pointer APIs happy-dom lacks; a native <select> keeps the dialog's own logic under test.
vi.mock('@/components/ui/select', () => ({
  Select: ({
    value,
    onValueChange,
    children
  }: {
    value: string
    onValueChange: (v: string) => void
    children: React.ReactNode
  }) => (
    <select value={value} onChange={(e) => onValueChange(e.target.value)}>
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children: React.ReactNode }) => (
    <option value={value}>{children}</option>
  )
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { useAppStore } from '@/store'
import { CreateRequestDialog } from './CreateRequestDialog'
import { REQUEST_BASE_REF_FEATURE } from '../../../../shared/request-base-ref'

const repo = (id: string, name: string) => ({
  id,
  url: `/work/${name}`,
  displayName: name,
  devServerId: ''
})
let repoList: unknown[]
let createAnswer: () => unknown

function baseRpc(method: string, params: Record<string, unknown>): unknown {
  switch (method) {
    case 'project.list':
      return [
        { id: 'p1', name: 'Ledger' },
        { id: 'p2', name: 'Billing' }
      ]
    case 'repo.list':
      return { repos: params.projectId === 'p2' ? [repo('repo-2', 'billing')] : repoList }
    case 'request.create':
      return createAnswer()
    default:
      return {}
  }
}

const created =
  (over: Record<string, unknown> = {}, top: Record<string, unknown> = {}) =>
  () => ({
    request: {
      id: 'r1',
      projectId: 'p1',
      title: 'T',
      type: 'unknown',
      status: 'submitted',
      baseRef: '',
      version: 1,
      ...over
    },
    created: true,
    ...top
  })

function createCall(): Record<string, unknown> {
  const call = rpc.mock.calls.find(([method]) => method === 'request.create')
  if (!call) {
    throw new Error('request.create was not called')
  }
  return call[1]
}

function rpcError(message: string): Error & { code: string } {
  return Object.assign(new Error(message), { code: 'runtime_error' })
}

async function mount(
  initial: Record<string, unknown> = { projectId: 'p1', title: 'Fix rounding' }
) {
  const onOpenChange = vi.fn()
  render(
    <TooltipProvider>
      <CreateRequestDialog open onOpenChange={onOpenChange} initial={initial} />
    </TooltipProvider>
  )
  return { onOpenChange }
}

const openPicker = async () => {
  fireEvent.click(await screen.findByRole('button', { name: /^Starting branch:/ }))
  return screen.findByRole('combobox', { name: 'Search branches' })
}

async function pick(query: string, ref: string): Promise<void> {
  const search = await openPicker()
  fireEvent.change(search, { target: { value: query } })
  fireEvent.click(await screen.findByRole('option', { name: ref }))
}

const submit = async () => {
  fireEvent.click(screen.getByRole('button', { name: 'Create' }))
  await act(async () => {
    await Promise.resolve()
  })
}

beforeEach(() => {
  repoList = [repo('repo-1', 'ledger')]
  createAnswer = created()
  rpc.mockReset().mockImplementation(baseRpc)
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue(['release/1.2', 'release/1.3'])
  Object.values(toasts).forEach((fn) => fn.mockReset())
  useAppStore.setState({
    requestFlowFeatures: new Set([REQUEST_BASE_REF_FEATURE]),
    requestPage: { section: 'requests', requestId: null, backlogView: 'requests', listFilters: {} }
  })
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('Start from field in the create dialog', () => {
  it('shows the repository default and sends no baseRef when nothing was picked', async () => {
    await mount()
    expect(
      await screen.findByRole('button', { name: 'Starting branch: Default branch (origin/main)' })
    ).toBeInTheDocument()
    expect(
      screen.getByText('Worktrees for this request start from this branch of ledger.')
    ).toBeInTheDocument()
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall()).not.toHaveProperty('baseRef')
  })

  it('sends the picked branch, opens the Request and raises no warning when the answer echoes it', async () => {
    createAnswer = created({ baseRef: 'release/1.2' })
    await mount()
    await pick('rel', 'release/1.2')
    expect(
      await screen.findByRole('button', { name: 'Starting branch: release/1.2' })
    ).toBeInTheDocument()
    await submit()
    await waitFor(() => expect(useAppStore.getState().requestPage.requestId).toBe('r1'))
    expect(createCall().baseRef).toBe('release/1.2')
    expect(toasts.warning).not.toHaveBeenCalled()
    expect(toasts.info).not.toHaveBeenCalled()
  })

  it('treats picking the default branch by name as no choice and sends nothing', async () => {
    searchRefs.mockResolvedValue(['origin/main'])
    await mount()
    await pick('ori', 'origin/main')
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall()).not.toHaveProperty('baseRef')
  })

  it('Use default branch drops a picked branch again', async () => {
    await mount()
    await pick('rel', 'release/1.2')
    fireEvent.click(await screen.findByRole('button', { name: 'Use default branch' }))
    expect(
      await screen.findByRole('button', { name: /^Starting branch: Default branch/ })
    ).toBeInTheDocument()
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall()).not.toHaveProperty('baseRef')
  })

  it('a project change forgets the picked branch', async () => {
    await mount()
    await pick('rel', 'release/1.2')
    fireEvent.change(document.querySelectorAll('select')[0], { target: { value: 'p2' } })
    expect(
      await screen.findByText('Worktrees for this request start from this branch of billing.')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /^Starting branch: Default branch/ })
    ).toBeInTheDocument()
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall()).toMatchObject({ projectId: 'p2' })
    expect(createCall()).not.toHaveProperty('baseRef')
  })

  it('hides the field and sends no baseRef when the gateway does not support it', async () => {
    useAppStore.setState({ requestFlowFeatures: new Set() })
    await mount()
    await screen.findByText(/The request runs on repo ledger/)
    expect(screen.queryByTestId('start-from-field')).toBeNull()
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall()).not.toHaveProperty('baseRef')
  })

  it('stays hidden for a project without a repo and while the repos load', async () => {
    repoList = []
    await mount()
    await screen.findByTestId('create-request-repo-note')
    expect(screen.queryByTestId('start-from-field')).toBeNull()
  })

  it('keeps the dialog, the typed text and the choice when the branch is not found, and offers the suggestions', async () => {
    const { onOpenChange } = await mount({
      projectId: 'p1',
      title: 'Fix rounding',
      body: 'details'
    })
    await pick('rel', 'release/1.2')
    rpc.mockImplementation((method, params) => {
      if (method === 'request.create') {
        throw rpcError('REQUEST_BASE_REF_NOT_FOUND: ref not found {"suggestions":["release/1.3"]}')
      }
      return baseRpc(method, params)
    })
    await submit()
    const field = await screen.findByTestId('start-from-field')
    await within(field).findByText(
      'This branch or tag was not found in the repository. Pick another one.'
    )
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByLabelText('Title')).toHaveValue('Fix rounding')
    expect(screen.getByLabelText('Description')).toHaveValue('details')
    expect(screen.getByRole('button', { name: 'Starting branch: release/1.2' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create' })).toBeEnabled()

    rpc.mockImplementation(baseRpc)
    createAnswer = created({ baseRef: 'release/1.3' })
    fireEvent.click(within(field).getByRole('button', { name: 'release/1.3' }))
    await screen.findByRole('button', { name: 'Starting branch: release/1.3' })
    expect(within(field).queryByText(/was not found/)).toBeNull()
    await submit()
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(rpc.mock.calls.findLast(([m]) => m === 'request.create')?.[1]).toMatchObject({
      baseRef: 'release/1.3'
    })
  })

  it('shows an invalid-name refusal under the field and keeps the dialog open', async () => {
    await mount()
    await pick('rel', 'release/1.2')
    rpc.mockImplementation((method, params) => {
      if (method === 'request.create') {
        throw rpcError('REQUEST_BASE_REF_INVALID: contains a forbidden character')
      }
      return baseRpc(method, params)
    })
    await submit()
    const field = await screen.findByTestId('start-from-field')
    await within(field).findByText(
      'This is not a valid branch or tag name. Check it and try again.'
    )
    expect(within(field).getByRole('alert')).toBeInTheDocument()
  })

  it('a refusal that is not about the branch still shows in the dialog error line', async () => {
    rpc.mockImplementation((method, params) => {
      if (method === 'request.create') {
        throw rpcError('REQUEST_PENDING_LIMIT: too many')
      }
      return baseRpc(method, params)
    })
    await mount()
    await submit()
    expect(
      await screen.findByText('Too many requests are waiting for classification. Try again later.')
    ).toBeInTheDocument()
  })
})

describe('manual entry when the host cannot be asked', () => {
  beforeEach(() => {
    getDefault.mockRejectedValue(rpcError('runtime_unavailable'))
  })

  it('explains it, validates the typed name and sends the valid one', async () => {
    createAnswer = created({ baseRef: 'release/9' })
    await mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Starting branch: Default branch' }))
    expect(await screen.findByText(/Cannot reach the host to list branches/)).toBeInTheDocument()
    const input = screen.getByLabelText('Branch or tag name')

    fireEvent.change(input, { target: { value: 'my branch' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByText('A branch name cannot contain spaces.')).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')

    fireEvent.change(input, { target: { value: '' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByText('Enter a branch or tag name.')).toBeInTheDocument()

    fireEvent.change(input, { target: { value: '  release/9 ' } })
    expect(screen.queryByText('Enter a branch or tag name.')).toBeNull()
    fireEvent.keyDown(input, { key: 'Enter' })
    await screen.findByRole('button', { name: 'Starting branch: release/9' })
    await submit()
    await waitFor(() => expect(rpc).toHaveBeenCalledWith('request.create', expect.anything()))
    expect(createCall().baseRef).toBe('release/9')
    expect(toasts.warning).not.toHaveBeenCalled()
  })

  it('Enter in the typed name does not submit the dialog', async () => {
    await mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Starting branch: Default branch' }))
    const input = await screen.findByLabelText('Branch or tag name')
    fireEvent.change(input, { target: { value: 'release/9' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await act(async () => {})
    expect(rpc.mock.calls.some(([m]) => m === 'request.create')).toBe(false)
  })
})

describe('Esc in the picker popover', () => {
  it('first clears the search text, then closes the popover, and never the dialog', async () => {
    const { onOpenChange } = await mount()
    const search = await openPicker()
    fireEvent.change(search, { target: { value: 'rel' } })
    fireEvent.keyDown(search, { key: 'Escape' })
    expect(search).toHaveValue('')
    expect(screen.getByRole('combobox', { name: 'Search branches' })).toBeInTheDocument()
    fireEvent.keyDown(search, { key: 'Escape' })
    await waitFor(() =>
      expect(screen.queryByRole('combobox', { name: 'Search branches' })).toBeNull()
    )
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })
})

describe('what happens after create', () => {
  it("warns that the server didn't apply the branch when the answer does not echo it", async () => {
    createAnswer = created({ baseRef: undefined })
    await mount()
    await pick('rel', 'release/1.2')
    await submit()
    await waitFor(() => expect(toasts.warning).toHaveBeenCalled())
    expect(toasts.warning.mock.calls[0][0]).toBe("This server didn't apply the starting branch.")
    expect(toasts.info).not.toHaveBeenCalled()
  })

  it('tells the user when the host could not confirm the branch, with an Open action', async () => {
    createAnswer = created({ baseRef: 'release/1.2' }, { baseRefUnverified: true })
    await mount()
    await pick('rel', 'release/1.2')
    await submit()
    await waitFor(() => expect(toasts.info).toHaveBeenCalled())
    const [message, options] = toasts.info.mock.calls[0]
    expect(message).toBe(
      "We couldn't reach the host to check this branch. It will be checked when the worktree is created."
    )
    expect(options.action.label).toBe('Open')
    options.action.onClick()
    expect(useAppStore.getState().requestPage.requestId).toBe('r1')
  })

  it('does not compare a branch against a Request the issue already had (created=false)', async () => {
    createAnswer = created({ baseRef: 'something-else' }, { created: false })
    await mount()
    await pick('rel', 'release/1.2')
    await submit()
    await waitFor(() =>
      expect(toasts.base).toHaveBeenCalledWith(
        'This issue already has a request',
        expect.anything()
      )
    )
    expect(toasts.warning).not.toHaveBeenCalled()
  })

  it('sends no check for a default create even if the answer has no baseRef', async () => {
    createAnswer = created({ baseRef: undefined })
    await mount()
    await submit()
    await waitFor(() => expect(useAppStore.getState().requestPage.requestId).toBe('r1'))
    expect(toasts.warning).not.toHaveBeenCalled()
  })
})

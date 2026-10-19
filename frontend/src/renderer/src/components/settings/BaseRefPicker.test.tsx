// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const getDefault = vi.fn()
const searchRefs = vi.fn()
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: (...args: unknown[]) => getDefault(...args),
  searchRuntimeRepoBaseRefs: (...args: unknown[]) => searchRefs(...args)
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (state: { repos: unknown[]; settings: unknown }) => unknown) =>
    selector({ repos: [], settings: null })
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { BaseRefPicker } from './BaseRefPicker'

async function settle(ms = 0): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

async function mount(props: Partial<React.ComponentProps<typeof BaseRefPicker>> = {}) {
  const onSelect = vi.fn()
  render(
    <TooltipProvider>
      <BaseRefPicker repoId="repo-1" onSelect={onSelect} {...props} />
    </TooltipProvider>
  )
  await settle()
  return { onSelect }
}

beforeEach(() => {
  vi.useFakeTimers()
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue(['origin/develop', 'origin/main'])
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

// The behavior Settings and Source Control relied on before the picker was extracted into git-ref-picker/.
describe('BaseRefPicker summary', () => {
  it('follows the primary branch the host reports when nothing is pinned', async () => {
    await mount()
    expect(screen.getByText('origin/main')).toBeInTheDocument()
    expect(screen.getByText('Following primary branch (origin/main)')).toBeInTheDocument()
  })

  it('shows the pinned ref and says so', async () => {
    await mount({ currentBaseRef: 'release/1.4' })
    expect(screen.getByText('release/1.4')).toBeInTheDocument()
    expect(screen.getByText('Pinned for this repo')).toBeInTheDocument()
  })

  it('does not fabricate a default for a repo that has none', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: null, remoteCount: 0 })
    await mount()
    expect(screen.getByText('No default base ref')).toBeInTheDocument()
    expect(screen.getByText('Pick a base branch below')).toBeInTheDocument()
    expect(screen.queryByText(/origin\/main/)).toBeNull()
  })

  it('shows no default and the hint to pick one while the lookup fails', async () => {
    getDefault.mockRejectedValue(new Error('boom'))
    await mount()
    expect(screen.getByText('No default base ref')).toBeInTheDocument()
  })

  it('shows the multiple-remotes tip only for more than one remote', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 3 })
    await mount()
    expect(screen.getByText(/Multiple remotes detected/)).toBeInTheDocument()
  })
})

describe('BaseRefPicker Use Primary', () => {
  it('is absent without a handler', async () => {
    await mount({ currentBaseRef: 'release/1.4' })
    expect(screen.queryByRole('button', { name: 'Use Primary' })).toBeNull()
  })

  it('is disabled until a ref is pinned', async () => {
    await mount({ onUsePrimary: vi.fn() })
    expect(screen.getByRole('button', { name: 'Use Primary' })).toBeDisabled()
  })

  it('calls the handler when a ref is pinned', async () => {
    const onUsePrimary = vi.fn()
    await mount({ currentBaseRef: 'release/1.4', onUsePrimary })
    fireEvent.click(screen.getByRole('button', { name: 'Use Primary' }))
    expect(onUsePrimary).toHaveBeenCalledTimes(1)
  })
})

describe('BaseRefPicker search', () => {
  it('searches after a pause, lists the matches and reports the chosen one', async () => {
    const { onSelect } = await mount()
    fireEvent.change(screen.getByPlaceholderText('Search branches by name...'), {
      target: { value: 'dev' }
    })
    expect(screen.getByText('Searching branches...')).toBeInTheDocument()
    await settle(200)
    expect(searchRefs).toHaveBeenCalledWith(
      { activeRuntimeEnvironmentId: null },
      'repo-1',
      'dev',
      20
    )
    fireEvent.click(screen.getByRole('option', { name: /origin\/develop/ }))
    expect(onSelect).toHaveBeenCalledWith('origin/develop')
    expect(screen.getByPlaceholderText('Search branches by name...')).toHaveValue('')
  })

  it('marks the ref in effect, the pinned one first and the default otherwise', async () => {
    await mount()
    fireEvent.change(screen.getByPlaceholderText('Search branches by name...'), {
      target: { value: 'or' }
    })
    await settle(200)
    expect(screen.getByRole('option', { name: /origin\/main/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('option', { name: /origin\/develop/ })).toHaveAttribute(
      'aria-selected',
      'false'
    )
  })

  it('does not search for fewer than two characters', async () => {
    await mount()
    fireEvent.change(screen.getByPlaceholderText('Search branches by name...'), {
      target: { value: 'o' }
    })
    await settle(500)
    expect(searchRefs).not.toHaveBeenCalled()
  })

  it('says when nothing matches', async () => {
    searchRefs.mockResolvedValue([])
    await mount()
    fireEvent.change(screen.getByPlaceholderText('Search branches by name...'), {
      target: { value: 'zzz' }
    })
    await settle(200)
    expect(screen.getByText('No matching branches found.')).toBeInTheDocument()
  })
})

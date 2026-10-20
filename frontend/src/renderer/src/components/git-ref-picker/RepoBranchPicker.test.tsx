// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const getDefault = vi.fn()
const searchRefs = vi.fn()
const listBranches = vi.fn()
vi.mock('@/runtime/runtime-repo-client', () => ({
  getRuntimeRepoBaseRefDefault: (...args: unknown[]) => getDefault(...args),
  searchRuntimeRepoBaseRefs: (...args: unknown[]) => searchRefs(...args),
  listRuntimeRepoBranches: (...args: unknown[]) => listBranches(...args)
}))
vi.mock('@/store', () => ({
  useAppStore: (selector: (state: { repos: unknown[]; settings: unknown }) => unknown) =>
    selector({ repos: [], settings: null })
}))

import { TooltipProvider } from '@/components/ui/tooltip'
import { RepoBranchPicker, type RepoBranchPickerProps } from './RepoBranchPicker'

const LEDGER = [
  { name: 'develop', remote: false },
  { name: 'feature/other', remote: false },
  { name: 'main', remote: false },
  { name: 'release/1.x', remote: false },
  { name: 'origin/hotfix/x', remote: true }
]
const ok = (branches = LEDGER, truncated = false) => ({ status: 'ok', branches, truncated })

async function settle(ms = 0): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

function tree(props: Partial<RepoBranchPickerProps>) {
  return (
    <TooltipProvider>
      <RepoBranchPicker
        repoId="repo-1"
        defaultName="main"
        onSelect={props.onSelect ?? vi.fn()}
        onUseDefault={props.onUseDefault ?? vi.fn()}
        {...props}
      />
    </TooltipProvider>
  )
}

async function mount(props: Partial<RepoBranchPickerProps> = {}) {
  const onSelect = props.onSelect ?? vi.fn()
  const onUseDefault = props.onUseDefault ?? vi.fn()
  const view = render(tree({ ...props, onSelect, onUseDefault }))
  await settle()
  return { onSelect, onUseDefault, view }
}

const optionNames = () => screen.queryAllByRole('option').map((o) => o.getAttribute('aria-label'))
const type = async (text: string, wait = 0) => {
  fireEvent.change(screen.getByRole('combobox', { name: 'Search branches' }), {
    target: { value: text }
  })
  await settle(wait)
}

beforeEach(() => {
  vi.useFakeTimers()
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue([])
  listBranches.mockReset().mockResolvedValue(ok())
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('RepoBranchPicker list', () => {
  it('lists the pinned default, then local branches, then remote-only ones, under headings', async () => {
    await mount()
    expect(optionNames()).toEqual([
      'main (default)',
      'develop',
      'feature/other',
      'release/1.x',
      'origin/hotfix/x (remote only)'
    ])
    const groups = screen.getAllByRole('group')
    expect(groups.map((g) => g.getAttribute('aria-labelledby'))).toHaveLength(2)
    expect(within(groups[0]).getByText('Local branches')).toBeInTheDocument()
    expect(within(groups[1]).getByText('Remote branches')).toBeInTheDocument()
    expect(within(groups[1]).getByText('remote only')).toBeInTheDocument()
    expect(screen.getByText('default')).toBeInTheDocument()
    expect(listBranches).toHaveBeenCalledTimes(1)
  })

  it('does not ask the host to search while the list is complete', async () => {
    await mount()
    await type('rel', 500)
    expect(searchRefs).not.toHaveBeenCalled()
    expect(optionNames()).toEqual(['release/1.x'])
  })

  it('marks the pinned default as current when nothing is chosen, and the chosen branch otherwise', async () => {
    const { view } = await mount()
    expect(screen.getByRole('option', { name: 'main (default)' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    view.rerender(tree({ value: 'release/1.x' }))
    await settle()
    expect(screen.getByRole('option', { name: 'release/1.x' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('option', { name: 'main (default)' })).toHaveAttribute(
      'aria-selected',
      'false'
    )
  })

  it('picking the default calls onUseDefault; picking another branch calls onSelect', async () => {
    const { onSelect, onUseDefault } = await mount({ value: 'develop' })
    fireEvent.click(screen.getByRole('option', { name: 'main (default)' }))
    expect(onUseDefault).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('option', { name: 'origin/hotfix/x (remote only)' }))
    expect(onSelect).toHaveBeenCalledWith('origin/hotfix/x')
  })

  it('shows a generic default row when the default cannot be named', async () => {
    const { onUseDefault } = await mount({ defaultName: null })
    expect(optionNames()[0]).toBe('Default branch')
    fireEvent.click(screen.getByRole('option', { name: 'Default branch' }))
    expect(onUseDefault).toHaveBeenCalled()
  })

  it('says so when the repo has no branches', async () => {
    listBranches.mockResolvedValue(ok([]))
    await mount()
    expect(screen.getByRole('status')).toHaveTextContent('No branches found')
  })

  it('says nothing matches when the filter finds nothing', async () => {
    await mount()
    await type('zzz')
    expect(screen.getByRole('status')).toHaveTextContent('No matching branches found.')
    expect(optionNames()).toEqual([])
  })
})

describe('RepoBranchPicker truncated list', () => {
  it('also searches the host from two characters and appends the hits the list lacks', async () => {
    listBranches.mockResolvedValue(ok(LEDGER, true))
    searchRefs.mockResolvedValue(['release/1.x', 'release/archive/0.9'])
    await mount()
    await type('r')
    expect(searchRefs).not.toHaveBeenCalled()
    await type('rel', 200)
    expect(searchRefs).toHaveBeenCalledWith(
      { activeRuntimeEnvironmentId: null },
      'repo-1',
      'rel',
      expect.any(Number)
    )
    expect(optionNames()).toEqual(['release/1.x', 'release/archive/0.9'])
    expect(screen.getByText('More matches')).toBeInTheDocument()
  })

  it('offers a retry when the host search fails', async () => {
    listBranches.mockResolvedValue(ok(LEDGER, true))
    searchRefs.mockRejectedValueOnce(new Error('git failed'))
    await mount()
    await type('rel', 200)
    expect(screen.getByRole('status')).toHaveTextContent('Branch search failed. Try again.')
    expect(optionNames()).toEqual(['release/1.x'])
    searchRefs.mockResolvedValue(['release/2.x'])
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await settle(200)
    expect(optionNames()).toEqual(['release/1.x', 'release/2.x'])
  })
})

describe('RepoBranchPicker keyboard', () => {
  it('moves with the arrows, picks with Enter, and wraps around', async () => {
    const { onSelect } = await mount()
    const input = screen.getByRole('combobox', { name: 'Search branches' })
    const active = () =>
      document
        .getElementById(input.getAttribute('aria-activedescendant') ?? '')
        ?.getAttribute('aria-label')
    expect(active()).toBe('main (default)')
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(active()).toBe('develop')
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(active()).toBe('origin/hotfix/x (remote only)')
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('origin/hotfix/x')
  })

  it('Enter on the first row of a filtered list picks the top match', async () => {
    const { onSelect } = await mount()
    await type('feat')
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('feature/other')
  })

  it('Esc clears the search first and only then lets the popover close', async () => {
    await mount()
    await type('rel')
    const input = screen.getByRole('combobox')
    const escape = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
    fireEvent(input, escape)
    expect(escape.defaultPrevented).toBe(true)
    expect(input).toHaveValue('')
    const second = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
    fireEvent(input, second)
    expect(second.defaultPrevented).toBe(false)
  })

  it('puts the cursor in the search box when the list appears', async () => {
    await mount()
    expect(screen.getByRole('combobox')).toHaveFocus()
  })
})

describe('RepoBranchPicker states', () => {
  it('shows only a loading line until the host answers', async () => {
    listBranches.mockReturnValue(new Promise(() => {}))
    await mount()
    expect(screen.getByRole('status')).toHaveTextContent('Loading branches...')
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('shows the error with Retry and still lets the user go back to the default', async () => {
    listBranches.mockRejectedValueOnce(new Error('git failed'))
    const { onUseDefault } = await mount()
    expect(screen.getByRole('status')).toHaveTextContent('Could not load the branches. Try again.')
    expect(optionNames()).toEqual(['main (default)'])
    fireEvent.click(screen.getByRole('option', { name: 'main (default)' }))
    expect(onUseDefault).toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await settle()
    expect(optionNames()).toContain('release/1.x')
    expect(listBranches).toHaveBeenCalledTimes(2)
  })

  it('falls back to the search-only picker, with no error, when the gateway cannot list', async () => {
    listBranches.mockResolvedValue({ status: 'unsupported' })
    searchRefs.mockResolvedValue(['release/1.2'])
    const { onSelect } = await mount()
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.queryByText(/Could not load/)).toBeNull()
    expect(screen.queryByText('Local branches')).toBeNull()
    expect(screen.getByRole('combobox')).toHaveFocus()
    await type('rel', 200)
    fireEvent.click(screen.getByRole('option', { name: 'release/1.2' }))
    expect(onSelect).toHaveBeenCalledWith('release/1.2')
    expect(searchRefs).toHaveBeenCalled()
  })

  it('never shows the previous repo list after the repo changes', async () => {
    const { view } = await mount()
    expect(optionNames()).toContain('release/1.x')
    listBranches.mockReturnValue(new Promise(() => {}))
    view.rerender(tree({ repoId: 'repo-2' }))
    await settle()
    expect(optionNames()).toEqual([])
    expect(screen.getByRole('status')).toHaveTextContent('Loading branches...')
    listBranches.mockResolvedValue(ok([{ name: 'trunk', remote: false }]))
    view.rerender(tree({ repoId: 'repo-3' }))
    await settle()
    expect(optionNames()).toEqual(['main (default)', 'trunk'])
  })
})

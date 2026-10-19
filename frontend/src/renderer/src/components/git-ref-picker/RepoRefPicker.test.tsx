// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react'
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
import { RepoRefPicker, type RepoRefPickerProps } from './RepoRefPicker'
import { splitRefForMiddleTruncation } from './MiddleTruncatedRef'

async function settle(ms = 0): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

async function mount(props: Partial<RepoRefPickerProps> = {}) {
  const onSelect = vi.fn()
  const view = render(
    <TooltipProvider>
      <RepoRefPicker repoId="repo-1" onSelect={onSelect} {...props} />
    </TooltipProvider>
  )
  await settle()
  const input = screen.getByRole('combobox')
  return { onSelect, input, view }
}

async function search(input: HTMLElement, text: string): Promise<void> {
  fireEvent.change(input, { target: { value: text } })
  await settle(200)
}

beforeEach(() => {
  vi.useFakeTimers()
  getDefault.mockReset().mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
  searchRefs.mockReset().mockResolvedValue(['origin/main', 'origin/release/1.4', 'upstream/main'])
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('RepoRefPicker accessibility', () => {
  it('is a labelled combobox that owns a listbox of options once results arrive', async () => {
    const { input } = await mount()
    expect(input).toHaveAccessibleName('Search branches')
    expect(input).toHaveAttribute('aria-autocomplete', 'list')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).toBeNull()

    await search(input, 'ma')
    const listbox = screen.getByRole('listbox', { name: 'Matching branches' })
    expect(input).toHaveAttribute('aria-expanded', 'true')
    expect(input.getAttribute('aria-controls')).toBe(listbox.id)
    expect(within(listbox).getAllByRole('option')).toHaveLength(3)
  })

  it('points aria-activedescendant at the highlighted option, first one to start', async () => {
    const { input } = await mount()
    await search(input, 'ma')
    const options = screen.getAllByRole('option')
    expect(input).toHaveAttribute('aria-activedescendant', options[0].id)

    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(input).toHaveAttribute('aria-activedescendant', options[1].id)
  })

  it('marks the ref in effect with aria-selected and a visible Current label', async () => {
    const { input } = await mount({ value: 'origin/release/1.4' })
    await search(input, 'ma')
    const [first, second] = screen.getAllByRole('option')
    expect(first).toHaveAttribute('aria-selected', 'false')
    expect(second).toHaveAttribute('aria-selected', 'true')
    expect(within(second).getByText('Current')).toBeInTheDocument()
    expect(within(first).queryByText('Current')).toBeNull()
  })

  it('treats the default as the ref in effect when no value is given', async () => {
    const { input } = await mount()
    await search(input, 'ma')
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('announces the number of matches in a polite status region', async () => {
    const { input } = await mount()
    await search(input, 'ma')
    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('aria-live', 'polite')
    expect(status).toHaveTextContent('3 matching branches')
  })
})

describe('RepoRefPicker keyboard', () => {
  it('moves with the arrows, wraps at both ends, and picks with Enter', async () => {
    const { input, onSelect } = await mount()
    await search(input, 'ma')
    const options = screen.getAllByRole('option')
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(input).toHaveAttribute('aria-activedescendant', options[2].id)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(input).toHaveAttribute('aria-activedescendant', options[0].id)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('origin/release/1.4')
  })

  it('cancels the default of the arrows and Enter so the caret and enclosing forms are left alone', async () => {
    const { input } = await mount()
    await search(input, 'ma')
    expect(fireEvent.keyDown(input, { key: 'ArrowDown' })).toBe(false)
    expect(fireEvent.keyDown(input, { key: 'Enter' })).toBe(false)
  })

  it('does not submit the surrounding form with Enter on a query that matches nothing', async () => {
    searchRefs.mockResolvedValue([])
    const { input, onSelect } = await mount()
    await search(input, 'nope')
    expect(fireEvent.keyDown(input, { key: 'Enter' })).toBe(false)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('leaves Enter alone on an empty field', async () => {
    const { input, onSelect } = await mount()
    expect(fireEvent.keyDown(input, { key: 'Enter' })).toBe(true)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('ignores keys while an IME composition is in progress', async () => {
    const { input, onSelect } = await mount()
    await search(input, 'ma')
    fireEvent.keyDown(input, { key: 'Enter', isComposing: true })
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('Escape clears the query and the list first, and does not bubble to a handler around the picker', async () => {
    const outer = vi.fn()
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 1 })
    render(
      <TooltipProvider>
        <div onKeyDown={outer}>
          <RepoRefPicker repoId="repo-1" onSelect={vi.fn()} />
        </div>
      </TooltipProvider>
    )
    await settle()
    const input = screen.getByRole('combobox')
    await search(input, 'ma')
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(input).toHaveValue('')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(outer).not.toHaveBeenCalled()
  })

  it('lets Escape bubble when there is nothing to clear', async () => {
    const outer = vi.fn()
    render(
      <TooltipProvider>
        <div onKeyDown={outer}>
          <RepoRefPicker repoId="repo-1" onSelect={vi.fn()} />
        </div>
      </TooltipProvider>
    )
    await settle()
    fireEvent.keyDown(screen.getByRole('combobox'), { key: 'Escape' })
    expect(outer).toHaveBeenCalledTimes(1)
  })

  it('selecting returns to the resting state', async () => {
    const { input, onSelect } = await mount()
    await search(input, 'ma')
    fireEvent.click(screen.getAllByRole('option')[2])
    expect(onSelect).toHaveBeenCalledWith('upstream/main')
    expect(input).toHaveValue('')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('highlights the row under the pointer without taking focus from the field', async () => {
    const { input } = await mount()
    await search(input, 'ma')
    input.focus()
    const options = screen.getAllByRole('option')
    fireEvent.mouseMove(options[2])
    expect(input).toHaveAttribute('aria-activedescendant', options[2].id)
    expect(fireEvent.mouseDown(options[2])).toBe(false)
  })
})

describe('RepoRefPicker states', () => {
  it('shows searching while the host works and nothing in the list', async () => {
    const { input } = await mount()
    fireEvent.change(input, { target: { value: 'ma' } })
    expect(screen.getByText('Searching branches...')).toBeInTheDocument()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('shows the empty message only after a search that found nothing', async () => {
    searchRefs.mockResolvedValue([])
    const { input } = await mount()
    expect(screen.queryByText('No matching branches found.')).toBeNull()
    await search(input, 'zz')
    expect(screen.getByText('No matching branches found.')).toBeInTheDocument()
  })

  it('shows an error with a Retry that searches again', async () => {
    searchRefs.mockRejectedValueOnce(new Error('git failed'))
    const { input } = await mount()
    await search(input, 'ma')
    expect(screen.getByText('Branch search failed. Try again.')).toBeInTheDocument()

    searchRefs.mockResolvedValueOnce(['origin/main'])
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await settle(200)
    expect(screen.getAllByRole('option')).toHaveLength(1)
    expect(screen.queryByText('Branch search failed. Try again.')).toBeNull()
  })

  it('tells an unreachable host apart from a failed search', async () => {
    searchRefs.mockRejectedValueOnce(Object.assign(new Error('x'), { code: 'runtime_unavailable' }))
    const { input } = await mount()
    await search(input, 'ma')
    expect(
      screen.getByText(
        'Cannot reach the host to search branches. Check the connection and try again.'
      )
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })

  it('shows the multiple-remotes tip only when more than one remote is known', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 2 })
    await mount()
    expect(screen.getByText(/Multiple remotes detected/)).toBeInTheDocument()
    expect(document.querySelectorAll('code')).toHaveLength(2)
  })

  it.each([
    ['one remote', { defaultBaseRef: 'origin/main', remoteCount: 1 }],
    ['an unknown remote count', { defaultBaseRef: null, remoteCount: 0 }]
  ])('hides the multiple-remotes tip for %s', async (_name, answer) => {
    getDefault.mockResolvedValue(answer)
    await mount()
    expect(screen.queryByText(/Multiple remotes detected/)).toBeNull()
  })

  it('hands the header slot what the picker learned, including the tip to place', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: 'origin/main', remoteCount: 2 })
    const renderHeader = vi.fn(({ defaultBaseRef, remotesHint }) => (
      <div data-testid="header">
        {defaultBaseRef}
        {remotesHint}
      </div>
    ))
    await mount({ renderHeader })
    const header = screen.getByTestId('header')
    expect(header).toHaveTextContent('origin/main')
    expect(within(header).getByText(/Multiple remotes detected/)).toBeInTheDocument()
    expect(renderHeader.mock.calls[0][0]).toMatchObject({
      defaultStatus: 'loading',
      remoteCount: 0
    })
  })

  it('uses the given placeholder, else the default one', async () => {
    const first = await mount({ placeholder: 'Find a starting branch' })
    expect(first.input).toHaveAttribute('placeholder', 'Find a starting branch')
    cleanup()
    const second = await mount()
    expect(second.input).toHaveAttribute('placeholder', 'Search branches by name...')
  })

  it('disables the field and ignores clicks and Enter while disabled', async () => {
    const onUseDefault = vi.fn()
    const { input } = await mount({ disabled: true, onUseDefault })
    expect(input).toBeDisabled()
    const option = screen.getByRole('option', { name: /Default branch/ })
    expect(option).toHaveAttribute('aria-disabled', 'true')
    fireEvent.click(option)
    expect(onUseDefault).not.toHaveBeenCalled()
  })
})

describe('RepoRefPicker default option', () => {
  it('is absent unless the caller can fall back to the default', async () => {
    await mount()
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  it('offers the named default at rest and calls onUseDefault', async () => {
    const onUseDefault = vi.fn()
    const { input, onSelect } = await mount({ onUseDefault })
    const option = screen.getByRole('option', { name: /Default branch \(origin\/main\)/ })
    expect(option).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', option.id)
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onUseDefault).toHaveBeenCalledTimes(1)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('does not name a default the host could not resolve', async () => {
    getDefault.mockResolvedValue({ defaultBaseRef: null, remoteCount: 0 })
    await mount({ onUseDefault: vi.fn() })
    expect(screen.getByRole('option', { name: /^Default branch/ })).not.toHaveTextContent('(')
  })

  it('is not the selected row while a branch is pinned', async () => {
    await mount({ onUseDefault: vi.fn(), value: 'release/1.4' })
    expect(screen.getByRole('option', { name: /Default branch/ })).toHaveAttribute(
      'aria-selected',
      'false'
    )
  })

  it('gives way to the results while the user is searching', async () => {
    const { input } = await mount({ onUseDefault: vi.fn() })
    await search(input, 'ma')
    expect(screen.queryByRole('option', { name: /Default branch/ })).toBeNull()
    expect(screen.getAllByRole('option')).toHaveLength(3)
  })
})

describe('RepoRefPicker long refs', () => {
  const LONG =
    'origin/feature/some-extremely-long-branch-name-for-a-very-specific-customer-escalation/ticket-4821'

  it('keeps the whole name in the document in two pieces so CSS can cut the middle', async () => {
    searchRefs.mockResolvedValue([LONG])
    const { input } = await mount()
    await search(input, 'origin')
    const option = screen.getByRole('option')
    expect(option).toHaveTextContent(LONG)
    const label = option.querySelector('[data-slot="middle-truncated-ref"]')
    expect(label?.children).toHaveLength(2)
    expect(label?.children[0]).toHaveClass('truncate')
    expect(label?.children[1]).toHaveClass('shrink-0')
  })

  it('splits a remote ref at its last segment and a flat name at its last characters', () => {
    expect(splitRefForMiddleTruncation('origin/release/1.4')).toEqual({
      head: 'origin/r',
      tail: 'elease/1.4'
    })
    expect(splitRefForMiddleTruncation('a/b/c')).toEqual({ head: '', tail: 'a/b/c' })
    expect(splitRefForMiddleTruncation('feature-with-a-flat-long-name')).toEqual({
      head: 'feature-with-a-flat',
      tail: '-long-name'
    })
    const longSegment = `origin/${'x'.repeat(60)}`
    const { head, tail } = splitRefForMiddleTruncation(longSegment)
    expect(tail).toHaveLength(24)
    expect(head + tail).toBe(longSegment)
  })

  it.each(['main', 'a/b/c', 'origin/feature/very-long', 'x'.repeat(300), 'é/ü/ö'])(
    'never loses a character when splitting %s',
    (ref) => {
      const { head, tail } = splitRefForMiddleTruncation(ref)
      expect(head + tail).toBe(ref)
    }
  )
})

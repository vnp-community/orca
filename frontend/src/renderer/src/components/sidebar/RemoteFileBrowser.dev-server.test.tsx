// @vitest-environment happy-dom
// Browse folder on a dev-server host (SOL-FE-E2E-004 / TASK-FE-E2E-020): bounded listings, one
// call for an absolute path, and errors that stay inline instead of closing the picker.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/store'
import { RemoteFileBrowser } from './RemoteFileBrowser'

type BrowseArgs = { id: string; path: string; limit?: number; query?: string }
type Listing = {
  resolvedPath: string
  entries: { name: string; isDirectory: boolean; isSymlink: boolean }[]
  truncated?: boolean
  totalEntries?: number
}

const dirs = (count: number): Listing['entries'] =>
  Array.from({ length: count }, (_, index) => ({
    name: `dir-${index}`,
    isDirectory: true,
    isSymlink: false
  }))

const browseDir = vi.fn<(args: BrowseArgs) => Promise<Listing>>()
const onSelect = vi.fn()
const onCancel = vi.fn()

async function flush(count = 6): Promise<void> {
  for (let index = 0; index < count; index += 1) {
    await Promise.resolve()
  }
}

async function render(): Promise<{ container: HTMLDivElement; root: Root }> {
  const container = document.createElement('div')
  document.body.appendChild(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(<RemoteFileBrowser devServerId="ds-1" onSelect={onSelect} onCancel={onCancel} />)
    await flush()
  })
  return { container, root }
}

async function typeInto(input: HTMLInputElement, value: string): Promise<void> {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

async function advance(ms: number): Promise<void> {
  await act(async () => {
    vi.advanceTimersByTime(ms)
    await flush()
  })
}

describe('RemoteFileBrowser on a dev server', () => {
  beforeEach(() => {
    ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.useFakeTimers()
    browseDir.mockReset()
    onSelect.mockReset()
    onCancel.mockReset()
    useAppStore.setState({
      devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'connected' }] as never
    })
    Object.defineProperty(window, 'api', {
      configurable: true,
      value: { devServer: { browseDir } }
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.replaceChildren()
  })

  it('asks the host for a capped listing', async () => {
    browseDir.mockResolvedValue({ resolvedPath: '/', entries: dirs(2) })
    const { root } = await render()

    expect(browseDir).toHaveBeenCalledWith({ id: 'ds-1', path: '~', limit: 500 })

    await act(async () => root.unmount())
  })

  it('lists only the typed absolute path, not every ancestor of it', async () => {
    browseDir.mockImplementation(async ({ path }) => ({
      resolvedPath: path === '~' ? '/' : path,
      entries: dirs(2)
    }))
    const { container, root } = await render()
    browseDir.mockClear()

    await typeInto(container.querySelector('input')!, '/private/var/folders/xy/T/repo/')
    await advance(300)

    // Why: listing /private/var/folders/... on the way is what overran the agent's 32 KiB frame.
    expect(browseDir.mock.calls.map(([args]) => args.path)).toEqual([
      '/private/var/folders/xy/T/repo'
    ])

    await act(async () => root.unmount())
  })

  it('falls back to step-by-step resolution to explain a path the host cannot list', async () => {
    browseDir.mockImplementation(async ({ path }) => {
      if (path === '/srv/missing') {
        throw new Error('ENOENT: no such file or directory')
      }
      return {
        resolvedPath: path === '~' ? '/' : path,
        entries:
          path === '~' || path === '/'
            ? [{ name: 'srv', isDirectory: true, isSymlink: false }]
            : dirs(1)
      }
    })
    const { container, root } = await render()
    browseDir.mockClear()

    await typeInto(container.querySelector('input')!, '/srv/missing/')
    await advance(300)

    // One direct attempt, then the walk lists the one directory it has not seen yet ('/' is cached).
    expect(browseDir.mock.calls.map(([args]) => args.path)).toEqual(['/srv/missing', '/srv'])
    expect(container.querySelector('#remote-file-browser-path-error')?.textContent).toContain(
      'missing'
    )

    await act(async () => root.unmount())
  })

  it('shows how many entries are hidden when the host truncated the listing', async () => {
    browseDir.mockResolvedValue({
      resolvedPath: '/tmp',
      entries: dirs(500),
      truncated: true,
      totalEntries: 1000
    })
    const { container, root } = await render()

    expect(
      container.querySelector('[data-testid="remote-file-browser-truncated"]')?.textContent
    ).toBe('Showing 500 of 1000 entries. Type to narrow the list.')

    await act(async () => root.unmount())
  })

  it('trims a listing from a backend that does not cap yet, with the same notice', async () => {
    browseDir.mockResolvedValue({ resolvedPath: '/tmp', entries: dirs(620) })
    const { container, root } = await render()

    expect(
      container.querySelector('[data-testid="remote-file-browser-truncated"]')?.textContent
    ).toBe('Showing 500 of 620 entries. Type to narrow the list.')

    await act(async () => root.unmount())
  })

  it('asks the host to filter when the listing is truncated', async () => {
    browseDir.mockImplementation(async ({ query }) =>
      query
        ? {
            resolvedPath: '/tmp',
            entries: [{ name: 'needle-dir', isDirectory: true, isSymlink: false }],
            truncated: false,
            totalEntries: 1
          }
        : { resolvedPath: '/tmp', entries: dirs(500), truncated: true, totalEntries: 1000 }
    )
    const { container, root } = await render()

    await typeInto(container.querySelector('input')!, 'needle')
    await advance(300)

    expect(browseDir).toHaveBeenCalledWith({
      id: 'ds-1',
      path: '/tmp',
      limit: 500,
      query: 'needle'
    })
    expect(container.textContent).toContain('needle-dir')
    expect(container.querySelector('[data-testid="remote-file-browser-truncated"]')).toBeNull()

    await act(async () => root.unmount())
  })

  it('keeps the picker open with an inline error and a retry when the agent drops', async () => {
    browseDir.mockRejectedValueOnce(
      new Error('INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent')
    )
    browseDir.mockResolvedValue({ resolvedPath: '/', entries: dirs(1) })
    const { container, root } = await render()

    const alert = container.querySelector('[data-testid="remote-file-browser-error"]')
    expect(alert?.textContent).toContain('The host is reconnecting')
    expect(alert?.textContent).toContain('INFRA_AGENT_EXEC_FAILED')
    expect(onCancel).not.toHaveBeenCalled()

    const retry = [...container.querySelectorAll('button')].find((button) =>
      button.textContent?.includes('Try again')
    )
    await act(async () => {
      retry?.click()
      await flush()
    })

    expect(container.querySelector('[data-testid="remote-file-browser-error"]')).toBeNull()
    expect(container.textContent).toContain('dir-0')

    await act(async () => root.unmount())
  })

  it('reloads on its own once the host is connected again', async () => {
    browseDir.mockRejectedValueOnce(new Error('INFRA_DEV_SERVER_NOT_CONNECTED'))
    browseDir.mockResolvedValue({ resolvedPath: '/', entries: dirs(1) })
    useAppStore.setState({
      devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'disconnected' }] as never
    })
    const { container, root } = await render()
    expect(container.querySelector('[data-testid="remote-file-browser-error"]')).not.toBeNull()
    expect(container.querySelector('[data-testid="remote-file-browser-host-offline"]')).toBeNull()

    await act(async () => {
      useAppStore.setState({
        devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'connected' }] as never
      })
      await flush()
    })

    expect(browseDir).toHaveBeenCalledTimes(2)
    expect(container.querySelector('[data-testid="remote-file-browser-error"]')).toBeNull()
    expect(container.textContent).toContain('dir-0')

    await act(async () => root.unmount())
  })

  it('says the host is offline while browsing and never closes the picker', async () => {
    browseDir.mockResolvedValue({ resolvedPath: '/', entries: dirs(1) })
    const { container, root } = await render()

    await act(async () => {
      useAppStore.setState({
        devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'disconnected' }] as never
      })
      await flush()
    })

    expect(
      container.querySelector('[data-testid="remote-file-browser-host-offline"]')?.textContent
    ).toContain('The host is not connected')
    expect(onCancel).not.toHaveBeenCalled()

    await act(async () => root.unmount())
  })
})

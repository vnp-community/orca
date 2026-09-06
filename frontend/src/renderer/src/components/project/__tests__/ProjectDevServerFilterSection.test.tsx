// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { useState } from 'react'
import { ProjectDevServerFilterSection } from '../ProjectDevServerFilterSection'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' })
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

const mockRpc = vi.mocked(callRuntimeRpc)

// Wraps the section so onChange actually round-trips into `selected`, the
// same way ProjectSettings.tsx's real `useState` does.
function Harness(): React.JSX.Element {
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  return <ProjectDevServerFilterSection selectedDevServerIds={selected} onChange={setSelected} />
}

describe('ProjectDevServerFilterSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRpc.mockResolvedValue([
      { id: 'ds-1', name: 'dev-01', status: 'healthy' },
      { id: 'ds-2', name: 'dev-ai', status: 'healthy' }
    ])
  })

  afterEach(cleanup)

  it('lists dev servers via devServer.list, all unchecked by default', async () => {
    render(<Harness />)

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'devServer.list', null)
    })
    expect(screen.getByTestId('dev-server-filter-ds-1')).not.toBeChecked()
    expect(screen.getByTestId('dev-server-filter-ds-2')).not.toBeChecked()
  })

  it('toggles a dev server on and off via onChange', async () => {
    render(<Harness />)
    await waitFor(() => expect(screen.getByTestId('dev-server-filter-ds-1')).toBeInTheDocument())

    fireEvent.click(screen.getByTestId('dev-server-filter-ds-1'))
    await waitFor(() => expect(screen.getByTestId('dev-server-filter-ds-1')).toBeChecked())

    fireEvent.click(screen.getByTestId('dev-server-filter-ds-1'))
    await waitFor(() => expect(screen.getByTestId('dev-server-filter-ds-1')).not.toBeChecked())
  })

  it('does not crash and shows nothing to check when devServer.list resolves a non-array', async () => {
    mockRpc.mockResolvedValue({ notAnArray: true } as never)
    render(<Harness />)

    await waitFor(() => {
      expect(screen.getByText('No dev servers available yet.')).toBeInTheDocument()
    })
  })
})

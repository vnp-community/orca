// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn(),
}))

vi.mock('../../../store', () => ({
  useAppStore: vi.fn().mockImplementation((fn: any) => fn ? fn({ settings: {} }) : undefined),
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local', id: 'local' }),
}))

// Monaco Editor → lightweight stub to avoid loading full monaco in tests
vi.mock('@monaco-editor/react', () => ({
  default: ({ value }: any) => <pre data-testid="monaco-editor">{value}</pre>,
  Editor: ({ value }: any) => <pre data-testid="monaco-editor">{value}</pre>,
}))

import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { FileViewer } from '../FileViewer'

const mockRpc = vi.mocked(callRuntimeRpc)
const mockUseWorkspace = vi.mocked(useWorkspace)
const mockStore = vi.mocked(useAppStore) as any

afterEach(() => { cleanup(); mockRpc.mockReset() })

beforeEach(() => {
  mockStore.getState = vi.fn().mockReturnValue({ settings: {} })
  mockUseWorkspace.mockReturnValue({
    project: { id: 'p1', name: 'test' },
  } as any)
})

describe('FileViewer', () => {
  it('renders file-viewer container', () => {
    mockRpc.mockResolvedValue('content')
    const { container } = render(<FileViewer filePath="src/file.ts" />)
    expect(container.querySelector('[data-testid="file-viewer"]')).toBeTruthy()
  })

  it('fetches file content on mount via workspace.readFile', async () => {
    mockRpc.mockResolvedValue('const x = 1\n')
    render(<FileViewer filePath="src/index.ts" />)
    await waitFor(() => expect(mockRpc).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'local' }),
      'workspace.readFile',
      expect.objectContaining({ projectId: 'p1', path: 'src/index.ts' })
    ), { timeout: 1000 })
    // Monaco stub renders content
    await waitFor(() => screen.getByTestId('monaco-editor'), { timeout: 1000 })
    expect(screen.getByTestId('monaco-editor').textContent).toContain('const x = 1')
  })

  it('shows error message on fetch failure', async () => {
    mockRpc.mockRejectedValue({ message: 'File not found' })
    render(<FileViewer filePath="missing.ts" />)
    await waitFor(() => expect(screen.getByTestId('file-error')).toBeInTheDocument(), { timeout: 1000 })
    expect(screen.getByTestId('file-error').textContent).toContain('File not found')
  })

  it('copy button copies content to clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      writable: true,
      configurable: true,
    })
    mockRpc.mockResolvedValue('file content here')
    render(<FileViewer filePath="test.ts" />)
    await waitFor(() => screen.getByTestId('monaco-editor'), { timeout: 1000 })
    await act(async () => {
      fireEvent.click(screen.getByTestId('copy-btn'))
    })
    expect(writeText).toHaveBeenCalledWith('file content here')
  })
})

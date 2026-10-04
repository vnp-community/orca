// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { TaskPromptEditor } from '../TaskPromptEditor'
import type { OrcaTask } from '../../../../../shared/task-types'

vi.mock('../../../store', () => ({
  useAppStore: Object.assign(vi.fn(), { getState: () => ({ settings: {} }) })
}))
vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: vi.fn().mockReturnValue({
    project: { id: 'proj-1' },
    currentWorktree: { id: 'wt-1', path: '/repo/proj-1', branch: 'main', isMain: true }
  })
}))
vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue('mock-target')
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { toast } from 'sonner'
const mockRpc = vi.mocked(callRuntimeRpc)

// labels/status: the editor derives the spec/approve phase from them.
const task = {
  id: 't1',
  title: 'T',
  promptTemplate: '',
  labels: [],
  status: 'open'
} as unknown as OrcaTask
const taskWithPrompt = { ...task, promptTemplate: 'my text' } as OrcaTask
// happy-dom has no window.confirm to spy on, so install a stub.
function mockConfirm(answer: boolean) {
  const fn = vi.fn().mockReturnValue(answer)
  vi.stubGlobal('confirm', fn)
  window.confirm = fn
  return fn
}
const box = () => screen.getByRole('textbox') as HTMLTextAreaElement
const click = () => fireEvent.click(screen.getByTestId('generate-agent-prompt-btn'))

describe('Generate with AI in TaskPromptEditor', () => {
  beforeEach(() => {
    cleanup()
    vi.clearAllMocks()
    vi.restoreAllMocks()
  })

  it('fills an empty textarea without asking', async () => {
    mockRpc.mockResolvedValue({ prompt: 'generated' })
    const confirm = mockConfirm(false)
    render(<TaskPromptEditor task={task} />)
    click()
    await waitFor(() => expect(box().value).toBe('generated'))
    expect(confirm).not.toHaveBeenCalled()
  })

  it('asks before overwriting and keeps the text when declined', async () => {
    mockRpc.mockResolvedValue({ prompt: 'generated' })
    const confirm = mockConfirm(false)
    render(<TaskPromptEditor task={taskWithPrompt} />)
    click()
    await waitFor(() => expect(confirm).toHaveBeenCalled())
    expect(box().value).toBe('my text')
  })

  it('replaces the text when the user confirms', async () => {
    mockRpc.mockResolvedValue({ prompt: 'generated' })
    mockConfirm(true)
    render(<TaskPromptEditor task={taskWithPrompt} />)
    click()
    await waitFor(() => expect(box().value).toBe('generated'))
  })

  it('shows an error toast and leaves the text alone on failure', async () => {
    mockRpc.mockRejectedValue(new Error('PermissionDenied'))
    render(<TaskPromptEditor task={taskWithPrompt} />)
    click()
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(box().value).toBe('my text')
  })

  it('disables the button while generating', async () => {
    let resolve!: (v: { prompt: string }) => void
    mockRpc.mockReturnValue(new Promise((r) => (resolve = r)))
    render(<TaskPromptEditor task={task} />)
    click()
    await waitFor(() => expect(screen.getByTestId('generate-agent-prompt-btn')).toBeDisabled())
    resolve({ prompt: 'done' })
    await waitFor(() => expect(screen.getByTestId('generate-agent-prompt-btn')).toBeEnabled())
  })
})

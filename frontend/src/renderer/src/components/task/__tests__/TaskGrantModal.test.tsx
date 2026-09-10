// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { TaskGrantModal } from '../TaskGrantModal'
import { useTaskGrants } from '../../../hooks/useTaskGrants'
import type { ReactNode } from 'react'

vi.mock('../../../hooks/useTaskGrants', () => ({
  useTaskGrants: vi.fn()
}))

// Same pattern as StepEditor.test.tsx / ModelSelector.test.tsx — render Select's
// subtree directly instead of driving Radix's real open state.
type TestSelectProps = { value?: string; children?: ReactNode }
type TestSelectItemProps = { value: string; children?: ReactNode }

vi.mock('../../ui/select', () => ({
  Select: (p: TestSelectProps) => (
    <div data-testid="type-select" data-value={p.value}>
      {p.children}
    </div>
  ),
  SelectTrigger: (p: TestSelectProps) => (
    <button data-testid="grant-level-select">{p.children}</button>
  ),
  SelectValue: () => <span />,
  SelectContent: (p: TestSelectProps) => <div>{p.children}</div>,
  SelectItem: (p: TestSelectItemProps) => (
    <div data-testid={`select-item-${p.value}`}>{p.children}</div>
  )
}))

const mockUseTaskGrants = vi.mocked(useTaskGrants)

describe('TaskGrantModal', () => {
  const addGrant = vi.fn()
  const revoke = vi.fn()
  const generateShareLink = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseTaskGrants.mockReturnValue({
      grants: [],
      addGrant,
      revoke,
      generateShareLink,
      isGranting: false
    })
  })

  afterEach(cleanup)

  it('grants empty → shows "No grant list available yet" message', () => {
    render(<TaskGrantModal taskId="t1" />)
    expect(screen.getByTestId('grant-list-empty')).toBeInTheDocument()
  })

  it('empty subjectId → "Grant access" button disabled', () => {
    render(<TaskGrantModal taskId="t1" />)
    expect(screen.getByTestId('grant-submit')).toBeDisabled()
  })

  it('types subjectId, picks a level, clicks "Grant access" → calls addGrant with the form values', () => {
    render(<TaskGrantModal taskId="t1" />)
    fireEvent.change(screen.getByTestId('grant-subject-input'), {
      target: { value: 'user-42' }
    })
    expect(screen.getByTestId('grant-submit')).not.toBeDisabled()
    fireEvent.click(screen.getByTestId('grant-submit'))
    // Default level state is 'user', applyTree defaults false.
    expect(addGrant).toHaveBeenCalledWith('user-42', 'user', false)
  })

  it('renders "Generate share link" button wired to generateShareLink', () => {
    render(<TaskGrantModal taskId="t1" />)
    fireEvent.click(screen.getByTestId('grant-share-link'))
    expect(generateShareLink).toHaveBeenCalled()
  })

  it('renders existing grants with a Revoke button per grant', () => {
    mockUseTaskGrants.mockReturnValue({
      grants: [{ subjectId: 'user-9', level: 'admin' }],
      addGrant,
      revoke,
      generateShareLink,
      isGranting: false
    })
    render(<TaskGrantModal taskId="t1" />)
    expect(screen.getByText('user-9 — admin')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('revoke-user-9'))
    expect(revoke).toHaveBeenCalledWith('user-9')
  })
})

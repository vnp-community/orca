// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { WorkflowLibrary } from '../WorkflowLibrary'
import { useWorkflowLibrary } from '../../../hooks/useWorkflowLibrary'
import type { ReactNode } from 'react'
import type { WorkflowDefinition } from '../../../../../shared/workflow-types'

vi.mock('../../../hooks/useWorkflowLibrary', () => ({
  useWorkflowLibrary: vi.fn()
}))

// Same pattern as ModelSelector.test.tsx / StepEditor.test.tsx — render Tabs' children
// directly so TabsTrigger buttons are queryable without driving Radix's open state.
type TestTabsProps = { children?: ReactNode }
type TestTabsTriggerProps = {
  value: string
  onValueChange?: (v: string) => void
  children?: ReactNode
}

vi.mock('../../ui/tabs', () => ({
  Tabs: (p: TestTabsProps) => <div>{p.children}</div>,
  TabsList: (p: TestTabsProps) => <div>{p.children}</div>,
  TabsTrigger: (p: TestTabsTriggerProps) => (
    <button data-testid={`scope-tab-${p.value}`} onClick={() => p.onValueChange?.(p.value)}>
      {p.children}
    </button>
  )
}))

const mockUseWorkflowLibrary = vi.mocked(useWorkflowLibrary)

describe('WorkflowLibrary', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseWorkflowLibrary.mockReturnValue({
      templates: [],
      loading: false,
      loadError: false,
      reload: vi.fn()
    })
  })

  it('renders 3 scope tabs company/team/personal, defaults to company', () => {
    render(<WorkflowLibrary onUseTemplate={vi.fn()} />)
    expect(screen.getByTestId('scope-tab-company')).toBeInTheDocument()
    expect(screen.getByTestId('scope-tab-team')).toBeInTheDocument()
    expect(screen.getByTestId('scope-tab-personal')).toBeInTheDocument()
    expect(mockUseWorkflowLibrary).toHaveBeenCalledWith('company', '')
  })

  it('empty templates → shows "No templates in this scope yet."', () => {
    render(<WorkflowLibrary onUseTemplate={vi.fn()} />)
    expect(screen.getByTestId('library-empty')).toHaveTextContent('No templates in this scope yet.')
  })

  it('click "Use" on a card → calls onUseTemplate(templateId)', () => {
    mockUseWorkflowLibrary.mockReturnValue({
      templates: [
        { id: 't1', name: 'Deploy', steps: [], scope: 'company' } as unknown as WorkflowDefinition
      ],
      loading: false,
      loadError: false,
      reload: vi.fn()
    })
    const onUseTemplate = vi.fn()
    render(<WorkflowLibrary onUseTemplate={onUseTemplate} />)
    fireEvent.click(screen.getByTestId('template-use'))
    expect(onUseTemplate).toHaveBeenCalledWith('t1')
  })

  it('typing into the search input → useWorkflowLibrary receives the new search value', async () => {
    render(<WorkflowLibrary onUseTemplate={vi.fn()} />)
    fireEvent.change(screen.getByTestId('library-search'), { target: { value: 'deploy' } })
    await waitFor(() => {
      expect(mockUseWorkflowLibrary).toHaveBeenLastCalledWith('company', 'deploy')
    })
  })

  it('loading=true → shows loading state', () => {
    mockUseWorkflowLibrary.mockReturnValue({
      templates: [],
      loading: true,
      loadError: false,
      reload: vi.fn()
    })
    render(<WorkflowLibrary onUseTemplate={vi.fn()} />)
    expect(screen.getByTestId('library-loading')).toBeInTheDocument()
  })

  it('loadError=true → shows error state', () => {
    mockUseWorkflowLibrary.mockReturnValue({
      templates: [],
      loading: false,
      loadError: true,
      reload: vi.fn()
    })
    render(<WorkflowLibrary onUseTemplate={vi.fn()} />)
    expect(screen.getByTestId('library-error')).toBeInTheDocument()
  })
})
